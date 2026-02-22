# Go Payments API

A production-ready payment processing API demonstrating distributed systems patterns and resilient architecture.

## Features

### Core Functionality
- **Internal Transfers**: Synchronous account-to-account transfers with ACID guarantees
- **External Payments**: Asynchronous processing with mock payment providers (Stripe, PayPal)
- **Multi-Currency Support**: Handle payments in different currencies
- **Refunds & Cancellations**: Full payment lifecycle management

### Distributed Systems Patterns
- **Transactional Outbox**: Reliable event publishing without distributed transactions
- **Distributed Locking** (Redis): Prevent duplicate processing across instances
- **Circuit Breaker**: Protection against cascading failures from external services
- **Optimistic Locking**: Safe concurrent balance updates
- **Idempotency**: Multi-layer (API + database) for exactly-once semantics
- **Dead Letter Queue**: Failed payment handling with Redis Streams
- **Event Sourcing**: Complete audit trail via payment_events table

### Observability
- **Structured Logging**: JSON logs with correlation IDs
- **Distributed Tracing**: OpenTelemetry with Jaeger
- **Metrics**: Prometheus metrics for business and system monitoring
- **Health Checks**: Kubernetes-ready liveness and readiness probes

## Architecture

```
┌─────────────┐         ┌─────────────┐         ┌─────────────┐
│             │         │             │         │             │
│  Handlers   │────────▶│  Services   │────────▶│  Domain     │
│  (HTTP)     │         │  (Business) │         │  Entities   │
│             │         │             │         │             │
└─────────────┘         └─────────────┘         └─────────────┘
                               │                        │
                               │                        │
                               ▼                        ▼
                        ┌─────────────┐         ┌─────────────┐
                        │             │         │             │
                        │ Redis       │         │ PostgreSQL  │
                        │ Streams     │         │ Repositories│
                        │             │         │             │
                        └─────────────┘         └─────────────┘
                               │
                               │
                               ▼
                        ┌─────────────┐
                        │             │
                        │  Workers    │
                        │  (Async)    │
                        │             │
                        └─────────────┘
```

### Service Layer Pattern

1. **Domain Layer** (`internal/domain/`)
   - Business entities (Payment, Account, Outbox)
   - State machines and business rules
   - Repository interfaces (defined by domain, implemented elsewhere)

2. **Service Layer** (`internal/service/`)
   - AccountService, PaymentService
   - Business workflow orchestration
   - Transaction management

3. **Repository Layer** (`internal/repository/postgres/`)
   - PostgreSQL repository implementations
   - Implements domain repository interfaces

4. **Handler Layer** (`internal/handler/`)
   - HTTP API handlers
   - DTOs and request validation

5. **Infrastructure Layer** (`internal/infrastructure/`)
   - Redis (locks, streams)
   - Observability (logging, tracing, metrics)
   - Configuration management

6. **Providers** (`internal/providers/`)
   - External payment provider integrations
   - Mock providers for testing

## Technology Stack

- **HTTP Framework**: `chi` - lightweight, excellent middleware support
- **Database**: PostgreSQL with `pgx/v5` driver
- **Migration Tool**: `golang-migrate/migrate`
- **Cache/Queue**: Redis with `go-redis/v9` (Streams support)
- **Logging**: `rs/zerolog` - structured, high-performance
- **Observability**: OpenTelemetry (Jaeger traces, Prometheus metrics)
- **Retry**: `avast/retry-go/v4` - exponential backoff
- **Circuit Breaker**: `sony/gobreaker`
- **Config**: `spf13/viper`
- **Validation**: `go-playground/validator/v10`

## Quick Start

### Prerequisites
- Go 1.22+
- Docker & Docker Compose
- Make

### 1. Clone and Setup
```bash
git clone <repo>
cd payments
make deps
```

### 2. Start Infrastructure
```bash
make docker-up
```

This starts:
- PostgreSQL (port 5432)
- Redis (port 6379)
- Jaeger UI (port 16686)
- Prometheus (port 9090)

### 3. Run Migrations
```bash
make migrate-up
```

### 4. Start Services

**Terminal 1 - API Server:**
```bash
make run-api
# API available at http://localhost:8080
```

**Terminal 2 - Background Worker:**
```bash
make run-worker
```

### 5. Test the API

**Create accounts:**
```bash
curl -X POST http://localhost:8080/api/v1/accounts \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user1",
    "initial_balance": 1000.00,
    "currency": "USD"
  }'

curl -X POST http://localhost:8080/api/v1/accounts \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user2",
    "initial_balance": 500.00,
    "currency": "USD"
  }'
```

**Internal transfer (synchronous):**
```bash
curl -X POST http://localhost:8080/api/v1/transfers \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: $(uuidgen)" \
  -d '{
    "source_account_id": "<account1_id>",
    "destination_account_id": "<account2_id>",
    "amount": 100.00,
    "currency": "USD"
  }'
```

**External payment (asynchronous):**
```bash
curl -X POST http://localhost:8080/api/v1/payments \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: $(uuidgen)" \
  -d '{
    "payment_type": "external_payment",
    "source_account_id": "<account1_id>",
    "amount": 50.00,
    "currency": "USD",
    "provider": "stripe"
  }'
```

## API Endpoints

### Health & Metrics
- `GET /health` - Basic health check
- `GET /health/live` - Liveness probe
- `GET /health/ready` - Readiness probe
- `GET /metrics` - Prometheus metrics

### Accounts
- `POST /api/v1/accounts` - Create account
- `GET /api/v1/accounts/:id` - Get account details
- `GET /api/v1/accounts/:id/balance` - Get balance
- `GET /api/v1/accounts/:id/transactions` - Transaction history

### Payments
- `POST /api/v1/payments` - Create payment (202 Accepted)
- `GET /api/v1/payments/:id` - Get payment status
- `GET /api/v1/payments` - List payments
- `POST /api/v1/payments/:id/refund` - Refund payment
- `POST /api/v1/payments/:id/cancel` - Cancel payment

### Transfers
- `POST /api/v1/transfers` - Internal transfer (201 Created)

## Configuration

Configuration via environment variables with `PAYMENTS_` prefix:

```bash
# Server
PAYMENTS_SERVER_PORT=8080
PAYMENTS_SERVER_READ_TIMEOUT=15s
PAYMENTS_SERVER_WRITE_TIMEOUT=15s

# Database
PAYMENTS_DATABASE_HOST=localhost
PAYMENTS_DATABASE_PORT=5432
PAYMENTS_DATABASE_USER=payments
PAYMENTS_DATABASE_PASSWORD=payments
PAYMENTS_DATABASE_DATABASE=payments

# Redis
PAYMENTS_REDIS_HOST=localhost
PAYMENTS_REDIS_PORT=6379

# Payment processing
PAYMENTS_PAYMENT_MAX_RETRIES=3
PAYMENTS_PAYMENT_RETRY_DELAY=1s
PAYMENTS_PAYMENT_LOCK_TTL=30s

# Observability
PAYMENTS_OBSERVABILITY_LOG_LEVEL=info
PAYMENTS_OBSERVABILITY_JAEGER_ENDPOINT=http://localhost:14268/api/traces
```

## Observability

### Logs
Structured JSON logs with correlation IDs:
```json
{
  "level": "info",
  "payment_id": "123e4567-e89b-12d3-a456-426614174000",
  "trace_id": "abc123",
  "message": "payment processed successfully"
}
```

### Traces
View distributed traces in Jaeger UI: http://localhost:16686

### Metrics
Query raw metrics in Prometheus: http://localhost:9090

### Dashboards (Grafana)
Grafana provides visualization of metrics with pre-built dashboards.

**Access:** http://localhost:3000  (login: `admin` / `admin`)

The **Payments Overview** dashboard shows currently collected metrics:
- **HTTP Traffic** — request rate, error rate %, P95 latency
- **Payment Errors** — errors by type and error_type
- **Worker** — Redis Streams processing rate

Jaeger is also wired as a Grafana datasource — click through to view distributed traces at http://localhost:16686.

## Testing

```bash
# Unit tests
make test

# Integration tests (requires Docker)
make test-integration

# All tests
make test-all
```

## Development

### Project Structure
```
payments/
├── cmd/                    # Entry points
│   ├── api/                # HTTP API server
│   ├── worker/             # Background worker
│   └── migrate/            # Migration tool
├── internal/               # Private application code
│   ├── domain/             # Business logic
│   ├── application/        # Use cases
│   ├── infrastructure/     # External implementations
│   └── interfaces/         # HTTP handlers, workers
├── pkg/                    # Reusable packages
├── deployments/docker/     # Docker configurations
└── tests/                  # Tests
```

### Make Commands
```bash
make help              # Show all commands
make build             # Build binaries
make test              # Run tests
make docker-up         # Start Docker services
make docker-down       # Stop Docker services
make migrate-up        # Run migrations
make run-api           # Start API server
make run-worker        # Start worker
make clean             # Clean build artifacts
```

## Resilience Features

### Retry with Exponential Backoff
- Start: 1s, doubles each attempt (1s, 2s, 4s, 8s)
- Max delay: 30s
- Max attempts: 5

### Idempotency (Two Levels)
- **API Level**: Header-based with 24h cache
- **Database Level**: Unique constraint on idempotency_key

### Distributed Locking
- Redis SET NX EX with Lua scripts
- TTL: 30 seconds with extension
- Safe release (owner check)

### Circuit Breaker
- Threshold: 10 requests, 60% failure rate
- States: Closed → Open (30s) → Half-Open → Closed
- Per-provider isolation

### Graceful Shutdown
- 30-second timeout for HTTP drain
- Workers complete current messages
- Proper resource cleanup

## Production Considerations

### Scaling
- **Horizontal**: Run multiple API and worker instances
- **Database**: Connection pooling (25 max connections)
- **Redis**: Single instance for locks, consider Redis Cluster for high availability

### Monitoring
- Set up alerts on circuit breaker opens
- Monitor payment processing duration
- Track DLQ size
- Alert on high retry rates

### Security
- Use environment variables for secrets
- Enable TLS for production
- Implement rate limiting per account
- Validate all inputs

### Performance
- Database indexes on frequently queried fields
- Redis pipeline for batch operations
- Consider read replicas for reporting queries
- Cache frequently accessed data

## License

MIT

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## Support

For issues and questions, please open a GitHub issue.
