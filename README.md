# Go Payments API

A production-ready payment processing API demonstrating distributed systems patterns and resilient architecture.

## Features

- **Internal Transfers**: Synchronous account-to-account transfers with ACID guarantees
- **External Payments**: Asynchronous processing with mock providers (Stripe, PayPal)
- **Multi-Currency Support**: Handle payments in different currencies
- **Refunds & Cancellations**: Full payment lifecycle management
- **Distributed Systems Patterns**: Transactional outbox, distributed locking (Redis), circuit breaker, optimistic locking, multi-layer idempotency, dead letter queue, event sourcing
- **Observability**: Structured logging with correlation IDs, OpenTelemetry/Jaeger tracing, Prometheus metrics, health checks

## Architecture

```
┌─────────────┐         ┌─────────────┐         ┌─────────────┐
│             │         │             │         │             │
│ Controllers │────────▶│  Services   │────────▶│  Domain     │
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

4. **Controller Layer** (`internal/controller/`)
   - HTTP API controllers
   - DTOs and request validation

5. **Infrastructure Layer** (`internal/infrastructure/`)
   - Redis (locks, streams)
   - Observability (logging, tracing, metrics)
   - Configuration management

6. **Providers** (`internal/providers/`)
   - External payment provider integrations
   - Mock providers for testing

## Technology Stack

- **HTTP**: `chi` router | **Database**: PostgreSQL with `pgx/v5` | **Migrations**: `golang-migrate/migrate`
- **Cache/Queue**: Redis with `go-redis/v9` (Streams) | **Logging**: `rs/zerolog`
- **Observability**: OpenTelemetry (Jaeger, Prometheus) | **Retry**: `avast/retry-go/v4`
- **Circuit Breaker**: `sony/gobreaker` | **Config**: `spf13/viper` | **Validation**: `go-playground/validator/v10`

## Database Schema

9 tables implementing double-entry bookkeeping, transactional outbox, and event sourcing:

- **Core**: `accounts` (balances with optimistic locking), `payments` (payment intent and state machine), `account_transactions` (double-entry ledger), `payment_events` (event sourcing audit trail)
- **Reliability**: `outbox` (transactional outbox pattern), `idempotency_keys` (request deduplication), `distributed_locks` (worker coordination)
- **Webhooks**: `webhooks` (subscriptions), `webhook_deliveries` (delivery log with retries)

See [DATABASE.md](/Users/cassiomorais/source/payments/DATABASE.md) for complete schema details and payment flow diagrams.

## Quick Start

**Prerequisites**: Go 1.22+, Docker & Docker Compose, Make

```bash
# 1. Setup
git clone <repo> && cd payments && make deps

# 2. Start infrastructure (PostgreSQL, Redis, Jaeger, Prometheus)
make docker-up

# 3. Run migrations (creates 9 tables)
make migrate-up

# 4. Start API server (terminal 1)
make run-api  # http://localhost:8080

# 5. Start background worker (terminal 2)
make run-worker

# 6. Test the API
# Create two accounts
curl -X POST http://localhost:8080/api/v1/accounts \
  -H "Content-Type: application/json" \
  -d '{"user_id":"alice","initial_balance":1000.00,"currency":"USD"}'

curl -X POST http://localhost:8080/api/v1/accounts \
  -H "Content-Type: application/json" \
  -d '{"user_id":"bob","initial_balance":500.00,"currency":"USD"}'

# Internal transfer (synchronous, returns 201 Created)
curl -X POST http://localhost:8080/api/v1/transfers \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: $(uuidgen)" \
  -d '{"source_account_id":"<alice_id>","destination_account_id":"<bob_id>","amount":100.00,"currency":"USD"}'

# External payment (asynchronous, returns 202 Accepted)
curl -X POST http://localhost:8080/api/v1/payments \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: $(uuidgen)" \
  -d '{"payment_type":"external_payment","source_account_id":"<alice_id>","amount":50.00,"currency":"USD","provider":"stripe"}'
```

**Key Commands**:
- `make test` - Run unit tests (no Docker required)
- `make test-integration` - Run integration tests (requires Docker)
- `make help` - Show all available commands

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

Environment variables with `PAYMENTS_` prefix (or `config.yaml`):

```bash
# Server: PAYMENTS_SERVER_PORT=8080
# Database: PAYMENTS_DATABASE_HOST=localhost, PAYMENTS_DATABASE_PORT=5432
# Redis: PAYMENTS_REDIS_HOST=localhost, PAYMENTS_REDIS_PORT=6379
# Payment: PAYMENTS_PAYMENT_MAX_RETRIES=3, PAYMENTS_PAYMENT_LOCK_TTL=30s
# Observability: PAYMENTS_OBSERVABILITY_LOG_LEVEL=info
```

## Observability

- **Logs**: Structured JSON with correlation IDs (`rs/zerolog`)
- **Traces**: Jaeger UI at http://localhost:16686
- **Metrics**: Prometheus at http://localhost:9090
- **Dashboards**: Grafana at http://localhost:3000 (`admin`/`admin`) - HTTP traffic, error rates, P95 latency, worker processing

## Testing

```bash
make test              # Unit tests (no Docker required) - 28.1% coverage
make test-integration  # Integration tests (requires Docker)
make test-all          # All tests
```

**Test Coverage**: 28.1% overall (14 files, 180+ tests)
- Domain: 94-100% | Service: 77.3% | Providers: 82.1%
- Money conversion: 100% with critical bug fix for negative amounts < 100 cents

## Development

**Project Structure**: Service layer pattern with domain-driven design
```
cmd/               # Entry points (api, worker, migrate)
internal/domain/   # Business entities & repository interfaces
internal/service/  # Business logic (AccountService, PaymentService)
internal/repository/postgres/  # Data access implementations
internal/controller/  # HTTP handlers & DTOs
internal/infrastructure/  # Config, observability, Redis
internal/providers/  # External payment integrations
```

**Make Commands**: `make help` for all commands | Key: `build`, `test`, `docker-up`, `migrate-up`, `run-api`, `run-worker`

## Resilience Features

- **Retry**: Exponential backoff (1s → 2s → 4s → 8s, max 30s, 5 attempts)
- **Idempotency**: API level (header-based, 24h cache) + DB level (unique constraint)
- **Distributed Locking**: Redis SET NX EX, 30s TTL with extension, safe release
- **Circuit Breaker**: 10 req threshold, 60% failure rate, per-provider isolation
- **Graceful Shutdown**: 30s HTTP drain, workers complete current messages

## Production Considerations

**Scaling**: Horizontal (multiple API/worker instances) | DB connection pooling (25 max) | Redis Cluster for HA

**Monitoring**: Alert on circuit breaker opens, payment duration, DLQ size, retry rates

**Security**: Environment secrets, TLS in production, rate limiting, input validation

**Performance**: DB indexes on queried fields, Redis pipelines, read replicas for reporting

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
