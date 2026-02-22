# Database Schema & Flow

Complete guide to the payment system's 9 tables and how they work together.

## Core Tables

### 1. `accounts` — User accounts with balances

```sql
id, user_id, balance, currency, version, status, created_at, updated_at
```

- Each user has one account per currency (unique constraint: `user_id + currency`)
- **`balance`**: Current account balance in cents (NUMERIC, no floats)
- **`version`**: Optimistic locking to prevent concurrent update conflicts
- **`status`**: Account state (active, frozen, etc.)
- All balance updates increment `version` to detect concurrent modifications

### 2. `payments` — Payment requests (high-level intent)

```sql
id, idempotency_key, payment_type, source_account_id, destination_account_id,
amount, currency, status, provider, provider_transaction_id, retry_count,
max_retries, last_error, saga_id, saga_step, metadata, created_at, updated_at, completed_at
```

- **`payment_type`**: `internal_transfer` (Alice → Bob) or `external_payment` (Alice → Stripe/PayPal)
- **`status` state machine**:
  - `pending` → `processing` → `completed` (success path)
  - `pending` → `processing` → `failed` (failure path)
  - `completed` → `refunded` (refund path)
  - `failed` → `processing` (retry path)
- **`saga_id` + `saga_step`**: Multi-step compensation if something fails (reserve funds → call provider → confirm)
- **`retry_count`**: Number of times this payment has been retried (max 3)
- **`idempotency_key`**: Unique constraint ensures same key returns same response (prevents duplicate payments)
- **`provider_transaction_id`**: External provider's ID (for Stripe/PayPal lookups)
- **`metadata`**: Additional data (reason for payment, user reference, etc.)

### 3. `account_transactions` — Double-entry bookkeeping ledger

```sql
id, account_id, payment_id, transaction_type (debit/credit), amount,
balance_after, description, created_at
```

- **One payment creates 2+ ledger entries** (e.g., Alice `-$100`, Bob `+$100`)
- Immutable audit trail — entries are never updated, only inserted
- **`balance_after`**: Balance of this account *after* this transaction (for reconciliation)
- **`transaction_type`**: `debit` (money out) or `credit` (money in)
- Links to `payment_id` so you can trace "which payment caused this balance change?"
- Example:
  ```
  Alice: [-$100, balance_after: $900]
  Bob:   [+$100, balance_after: $1100]
  ```

### 4. `payment_events` — Event sourcing (payment lifecycle audit trail)

```sql
id, payment_id, event_type, event_data (JSON), created_at
```

- Records every state change of a payment (different from accounting)
- Event types: `PaymentCreated`, `PaymentProcessing`, `PaymentCompleted`, `PaymentFailed`, `PaymentRefunded`
- **`event_data`**: JSON payload with event-specific details (amounts, provider errors, etc.)
- Can replay all events to reconstruct full payment history
- Key difference from `account_transactions`: this tracks payment lifecycle, not accounting ledger

---

## Supporting Tables (Reliability & Delivery)

### 5. `outbox` — Transactional Outbox Pattern

```sql
id, aggregate_type, aggregate_id, event_type, payload (JSON),
status (pending/published/failed), retry_count, max_retries,
created_at, published_at
```

- **Purpose**: Guarantee payment events are published to Redis Streams even if external systems fail
- **How it works**:
  1. Payment created in DB + outbox row written in **same transaction** → atomicity guarantee
  2. Background worker polls outbox for `status = pending` entries
  3. Publishes event to Redis Streams
  4. Marks row as `status = published` (or `failed` if max retries exceeded)
  5. If Redis is down, worker retries with exponential backoff (up to `max_retries`)
- Guarantees **at-least-once delivery** of payment events
- Workers/subscribers can filter by `event_type` in Redis Streams

### 6. `idempotency_keys` — HTTP Request Deduplication

```sql
key (PRIMARY KEY), response_body (TEXT), response_status (INT),
created_at, expires_at
```

- **Purpose**: Prevent duplicate payments if client retries the same request
- **Flow**:
  1. Client sends `Idempotency-Key` header (e.g., UUID) with payment request
  2. HTTP middleware checks this table for the key
  3. If key exists and not expired → return cached `response_body` (same payment ID)
  4. If key not found → process payment, then insert row
  5. Row expires after TTL (default 24 hours) to clean up
- DB constraint: `payments.idempotency_key` is UNIQUE (prevents race condition between checks)
- Example:
  ```
  POST /payments HTTP/1.1
  Idempotency-Key: a1b2c3d4-e5f6-7890-abcd-ef1234567890
  {"from": alice, "to": bob, "amount": 100}

  // First request → process, insert idempotency_keys row, return payment_id=xyz
  // Retry with same header → cached response, return payment_id=xyz (same)
  ```

### 7. `distributed_locks` — Worker Process Deduplication

```sql
lock_key (PRIMARY KEY), owner (VARCHAR), acquired_at, expires_at
```

- **Purpose**: Prevent two workers from processing the same payment simultaneously
- **Flow**:
  1. Worker tries to acquire lock via Redis `SET NX EX` (non-blocking)
  2. If Redis fails, fallback to this table `INSERT OR IGNORE`
  3. Lock expires automatically (TTL) to handle crashed workers
  4. Other workers skip locked payments or acquire lock after expiry
- Lock keys: `lock:payment:{payment_id}`, `lock:outbox`, `lock:webhook_delivery:{id}`
- Example: Only one worker processes `payment_123` at a time, even with 10 workers running

---

## Webhook Tables

### 8. `webhooks` — Webhook Subscriptions

```sql
id, url, events (VARCHAR[] ARRAY), secret, status (active/inactive), created_at
```

- Registered endpoints that external systems subscribe to
- **`events`**: Array of event types this webhook cares about
  - Example: `['payment.completed', 'payment.failed']`
- **`secret`**: Used to sign webhook payloads with HMAC-SHA256 (verification)
- **`status`**: Can disable webhooks without deleting them
- Example:
  ```sql
  INSERT INTO webhooks (url, events, secret) VALUES (
    'https://example.com/payment-webhook',
    ARRAY['payment.completed', 'payment.refunded'],
    'secret123'
  )
  ```

### 9. `webhook_deliveries` — Webhook Delivery Log & Retry Queue

```sql
id, webhook_id, payment_id, event_type, payload (JSON),
status (pending/delivered/failed), retry_count, max_retries,
response_status, response_body, created_at, delivered_at
```

- **Purpose**: Ensure webhooks are delivered at least once, with automatic retries
- **Flow**:
  1. Payment completes → find all webhooks interested in this event
  2. Create `webhook_deliveries` row for each webhook
  3. Background worker sends HTTPS POST to webhook URL with signature:
     ```
     POST https://example.com/payment-webhook HTTP/1.1
     X-Webhook-Signature: sha256=hmac_hash

     {
       "event": "payment.completed",
       "payment_id": "xyz",
       ...
     }
     ```
  4. If 2xx response → mark as `delivered`
  5. If non-2xx or network error → increment `retry_count`, retry up to `max_retries`
  6. Store response for debugging (why did delivery fail?)
- Guarantees **at-least-once delivery** with timeout handling

---

## Complete Payment Flow Example

### Scenario: Alice sends $100 to Bob (internal transfer)

```
1. HTTP POST /payments
   {
     "from": alice_account_id,
     "to": bob_account_id,
     "amount": 100.00,
     "idempotency_key": "xyz-123"
   }

   ↓

2. Middleware checks idempotency_keys table
   → Key "xyz-123" not found → proceed
   → (If found and not expired, return cached response instead)

   ↓

3. BEGIN DATABASE TRANSACTION

   a) Create payment row:
      payments {
        id: pay_123,
        source_account_id: alice,
        destination_account_id: bob,
        amount: 10000 (cents),
        status: pending,
        idempotency_key: xyz-123
      }

   b) Execute double-entry bookkeeping:

      account_transactions {
        id: tx_alice,
        account_id: alice,
        payment_id: pay_123,
        transaction_type: debit,
        amount: 10000,
        balance_after: 90000
      }

      account_transactions {
        id: tx_bob,
        account_id: bob,
        payment_id: pay_123,
        transaction_type: credit,
        amount: 10000,
        balance_after: 110000
      }

   c) Update accounts balances:
      accounts { id: alice, balance: 90000, version: version+1 }
      accounts { id: bob, balance: 110000, version: version+1 }

   d) Write to outbox (for reliable event delivery):
      outbox {
        id: outbox_1,
        aggregate_type: payment,
        aggregate_id: pay_123,
        event_type: PaymentCompleted,
        payload: { payment_id: pay_123, amount: 100.00, ... },
        status: pending
      }

   e) Create audit event:
      payment_events {
        id: evt_1,
        payment_id: pay_123,
        event_type: PaymentCreated,
        event_data: { ... }
      }

   f) Update payment status:
      payments { id: pay_123, status: completed }

   g) Cache idempotent response:
      idempotency_keys {
        key: xyz-123,
        response_body: JSON of payment,
        response_status: 200,
        expires_at: now() + 24 hours
      }

   COMMIT TRANSACTION ✓
   (All or nothing — if any step fails, entire TX rolls back)

   ↓

4. HTTP Response: 200 OK
   {
     "id": "pay_123",
     "amount": 100.00,
     "status": "completed",
     ...
   }

   ↓

5. Background Worker (polls every 5 seconds)

   a) Poll outbox table for pending entries:
      SELECT * FROM outbox WHERE status = 'pending' LIMIT 100
      → Finds outbox_1

   b) Publish to Redis Streams:
      XADD payments:events * type=PaymentCompleted payload={...}

   c) Mark as published:
      UPDATE outbox SET status = 'published', published_at = NOW()
      WHERE id = outbox_1

   ↓

6. Webhook Delivery Worker

   a) Payment event triggers webhook search:
      SELECT * FROM webhooks WHERE 'payment.completed' = ANY(events)

   b) Create delivery rows:
      webhook_deliveries {
        webhook_id: webhook_456,
        payment_id: pay_123,
        event_type: payment.completed,
        payload: { payment_id: pay_123, ... },
        status: pending
      }

   c) Send HTTPS POST:
      POST https://customer.com/webhook HTTP/1.1
      X-Webhook-Signature: sha256=hmac(payload, secret)

      { "event": "payment.completed", "payment_id": "pay_123", ... }

   d) Response: 200 OK
      UPDATE webhook_deliveries SET status = 'delivered', delivered_at = NOW()

   (If fails, retry up to 3 times with exponential backoff)

   ↓

7. Final State:

   accounts:
   - alice: balance=90000, version=n+1 ✓
   - bob: balance=110000, version=n+1 ✓

   payments:
   - pay_123: status=completed ✓

   account_transactions:
   - tx_alice (debit 10000): balance_after=90000 ✓
   - tx_bob (credit 10000): balance_after=110000 ✓

   payment_events:
   - evt_1: PaymentCreated ✓

   outbox:
   - outbox_1: status=published ✓

   webhook_deliveries:
   - wh_delivery_1: status=delivered ✓

   idempotency_keys:
   - xyz-123: cached response ✓
```

---

## Key Guarantees These Tables Provide

| Guarantee | Mechanism | Tables Involved |
|-----------|-----------|-----------------|
| **No duplicate payments** | Idempotency key deduplication + DB UNIQUE constraint | `idempotency_keys`, `payments` |
| **Accounting accuracy** | Double-entry bookkeeping (every debit matched with credit) | `account_transactions`, `accounts` |
| **Audit trail** | Event sourcing captures all state changes | `payment_events` |
| **Reliable event publishing** | Transactional outbox (write event + outbox in same TX) | `outbox`, `payments` |
| **Webhook reliability** | Retry loop with exponential backoff | `webhook_deliveries`, `webhooks` |
| **No race conditions** | Distributed locks + optimistic locking on accounts | `distributed_locks`, `accounts.version` |
| **Worker failover** | Locks auto-expire, other workers pick up | `distributed_locks` |

---

## Transaction Isolation & Data Consistency

### ACID Properties Enforced

- **Atomicity**: Single payment → single DB transaction (all tables updated together)
- **Consistency**: Optimistic locking on accounts prevents concurrent modification conflicts
- **Isolation**: PostgreSQL default `READ COMMITTED` isolation level
- **Durability**: Outbox table ensures events survive worker crashes

### Example Race Condition Prevention

```
Scenario: Two workers both try to process payment_123

Worker 1:                          Worker 2:
SELECT account WHERE id=alice      SELECT account WHERE id=alice
(version=5, balance=1000)          (version=5, balance=1000)
↓                                  ↓
Process payment (-100)             Process payment (-100)
↓                                  ↓
UPDATE accounts SET balance=900    UPDATE accounts SET balance=900
WHERE id=alice AND version=5       WHERE id=alice AND version=5
✓ Success (1 row updated)          ✗ Fail (0 rows updated, version no longer 5)

Result: Payment only processed once, second worker fails and retries
```

### Idempotency At Two Levels

1. **HTTP level**: `Idempotency-Key` header (prevents duplicate API requests)
2. **DB level**: `payments.idempotency_key` UNIQUE constraint (prevents race condition)

---

## Monitoring & Observability

Key metrics to track:

- **`outbox` pending count**: High pending count = events not being published (worker down?)
- **`webhook_deliveries` failed count**: High failure rate = webhook endpoints unreachable
- **`payments` by status**: Count of pending/processing/completed/failed
- **`account_transactions` per second**: Payment throughput
- **Lock contention**: `distributed_locks` row age (expired locks not cleaned up?)
- **Idempotency key cache hit rate**: How many requests are duplicates?

---

## Schema Design Decisions

- **Money as `NUMERIC(19,4)`**: Precise decimal (not float) for financial amounts
- **Optimistic locking on accounts**: Prevents expensive row locks, allows high concurrency
- **Separate `account_transactions` table**: Enables double-entry bookkeeping, audit trail, reconciliation
- **Separate `payment_events` table**: Event sourcing for replay-ability independent of accounting
- **Outbox pattern**: Decouples DB writes from external event publishing (fault-tolerant)
- **Webhook delivery table**: Explicit retry queue (vs. trying to retry in-memory and losing on crash)
