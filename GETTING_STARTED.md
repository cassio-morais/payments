# Getting Started: Understanding & Running the Payments API

## Phase 1: Run Unit Tests (no Docker needed)

```bash
cd ~/source/payments

# Run all 23 unit tests with verbose output
go test ./... -v -count=1
```

This runs tests for:
- `internal/domain/payment/` — 18 tests: payment state machine (pending→processing→completed/failed), validation, retry logic
- `internal/domain/account/` — account debit/credit, insufficient funds, version increments
- `pkg/saga/` — saga compensation (rollback on failure)

## Phase 2: Start Infrastructure

```bash
# Start Colima (Docker runtime for macOS)
colima start

# Start Postgres, Redis, Jaeger, Prometheus
docker-compose -f deployments/docker/docker-compose.yml up -d

# Wait and verify all healthy
sleep 15
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
```

You should see 4 containers: `payments-postgres` (healthy), `payments-redis` (healthy), `payments-jaeger`, `payments-prometheus` (healthy).

## Phase 3: Run Database Migrations

```bash
go run ./cmd/migrate/ \
  -direction up \
  -db "postgresql://payments:payments@localhost:5432/payments?sslmode=disable"
```

This creates 9 tables: `accounts`, `payments`, `payment_events`, `account_transactions`, `idempotency_keys`, `distributed_locks`, `outbox`, `webhooks`, `webhook_deliveries`.

To inspect the schema:
```bash
docker exec payments-postgres psql -U payments -d payments -c "\dt"
```

## Phase 4: Start the API Server

```bash
go run ./cmd/api/
```

You'll see logs confirming PostgreSQL + Redis connections. Leave this running in a terminal.

## Phase 5: Exercise the API (new terminal)

### 5a. Health checks

```bash
curl -s http://localhost:8080/health/live | python3 -m json.tool
curl -s http://localhost:8080/health/ready | python3 -m json.tool
```

### 5b. Create two accounts

```bash
# Alice with $1000
curl -s -X POST http://localhost:8080/api/v1/accounts \
  -H "Content-Type: application/json" \
  -d '{"user_id":"alice","currency":"USD","initial_balance":1000}' | python3 -m json.tool

# Bob with $500
curl -s -X POST http://localhost:8080/api/v1/accounts \
  -H "Content-Type: application/json" \
  -d '{"user_id":"bob","currency":"USD","initial_balance":500}' | python3 -m json.tool
```

Save the returned `id` values as `ALICE_ID` and `BOB_ID`.

### 5c. Internal transfer (synchronous)

```bash
curl -s -X POST http://localhost:8080/api/v1/transfers \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: my-transfer-001" \
  -d "{
    \"source_account_id\": \"$ALICE_ID\",
    \"destination_account_id\": \"$BOB_ID\",
    \"amount\": 250,
    \"currency\": \"USD\"
  }" | python3 -m json.tool
```

Expected: `status: "completed"` (201 Created). This executes atomically in a single DB transaction.

### 5d. Verify balances changed

```bash
curl -s "http://localhost:8080/api/v1/accounts/$ALICE_ID/balance" | python3 -m json.tool
# Expected: 750

curl -s "http://localhost:8080/api/v1/accounts/$BOB_ID/balance" | python3 -m json.tool
# Expected: 750
```

### 5e. Test idempotency (send same request again)

```bash
curl -s -X POST http://localhost:8080/api/v1/transfers \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: my-transfer-001" \
  -d "{
    \"source_account_id\": \"$ALICE_ID\",
    \"destination_account_id\": \"$BOB_ID\",
    \"amount\": 250,
    \"currency\": \"USD\"
  }" | python3 -m json.tool
```

Expected: returns the **same payment** (same `id`), balance unchanged at 750. No double charge.

### 5f. Test insufficient funds

```bash
curl -s -X POST http://localhost:8080/api/v1/transfers \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: big-transfer" \
  -d "{
    \"source_account_id\": \"$ALICE_ID\",
    \"destination_account_id\": \"$BOB_ID\",
    \"amount\": 999999,
    \"currency\": \"USD\"
  }" | python3 -m json.tool
```

Expected: `code: "insufficient_funds"` (422 Unprocessable Entity).

### 5g. External payment (asynchronous)

```bash
curl -s -X POST http://localhost:8080/api/v1/payments \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: ext-pay-001" \
  -d "{
    \"payment_type\": \"external_payment\",
    \"source_account_id\": \"$ALICE_ID\",
    \"amount\": 50,
    \"currency\": \"USD\",
    \"provider\": \"stripe\"
  }" | python3 -m json.tool
```

Expected: `status: "pending"` (202 Accepted). This writes to the outbox for async processing.

### 5h. Check transaction history

```bash
curl -s "http://localhost:8080/api/v1/accounts/$ALICE_ID/transactions" | python3 -m json.tool
```

### 5i. List all payments

```bash
curl -s "http://localhost:8080/api/v1/payments?limit=10" | python3 -m json.tool
```

## Phase 6: Explore Observability

**Prometheus metrics:**
```bash
curl -s http://localhost:8080/metrics | grep payments_
```

**Prometheus UI:** Open http://localhost:9090 — query `payments_http_request_duration_seconds_count`

**Jaeger tracing UI:** Open http://localhost:16686 — select service "payments-api" to see request traces

**Grafana dashboards:** Open http://localhost:3000 (admin/admin) →
Dashboards → Payments Overview — see HTTP request rates, error rates,
and worker message processing.

## Phase 7: Start the Worker (optional, new terminal)

```bash
go run ./cmd/worker/
```

This polls the outbox table and processes pending external payments via Redis Streams. The `stripe` mock provider will process the pending payment from step 5g.

## Phase 8: Inspect the Database Directly

```bash
docker exec -it payments-postgres psql -U payments -d payments
```

```sql
-- See accounts
SELECT id, user_id, balance, currency, version FROM accounts;

-- See payments with status
SELECT id, payment_type, status, amount, currency FROM payments;

-- See audit trail
SELECT event_type, event_data FROM payment_events;

-- See transaction ledger
SELECT account_id, transaction_type, amount, balance_after, description FROM account_transactions;

-- See outbox
SELECT aggregate_type, event_type, status FROM outbox;

\q
```

## Phase 9: Cleanup

```bash
# Stop API/worker (Ctrl+C in their terminals)

# Stop and remove containers + data
docker-compose -f deployments/docker/docker-compose.yml down -v
```
