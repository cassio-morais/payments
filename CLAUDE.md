# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

### Essential Commands
```bash
make build              # Build all binaries (api, worker, migrate) into bin/
make run-api            # Run API server (http://localhost:8080)
make run-worker         # Run background worker
make test               # Run unit tests with race detector and coverage (no Docker required)
make test-integration   # Run integration tests (requires Docker services)
make coverage           # Generate coverage report with summary
make coverage-html      # Generate HTML coverage report (opens coverage.html)
```

### Database & Infrastructure
```bash
make docker-up          # Start PostgreSQL, Redis, Jaeger (localhost:16686), Prometheus (localhost:9090)
make docker-down        # Stop Docker services
make migrate-up         # Run database migrations
make migrate-down       # Rollback database migrations
make migrate-create name=<name>  # Create new migration file
```

### Code Quality
```bash
make lint               # Run golangci-lint
make format             # Run go fmt
make deps               # Download and tidy dependencies
```

### Testing Specific Code
```bash
# Run a single test
go test -v -run TestName ./internal/domain/payment/...

# Run tests for specific package
go test -v ./internal/service/...

# Run with coverage for specific package
go test -v -cover ./internal/domain/payment/...
```

## Architecture

Go payment processing API using **Service Layer Pattern** with simplified layering:

1. **Domain** (`internal/domain/`) — Core entities (account, payment, outbox), value objects, repository interfaces, domain errors. No external dependencies. Domain defines all interfaces that other layers implement.
2. **Service** (`internal/service/`) — Business logic layer with `AccountService` and `PaymentService`. Services orchestrate domain logic, manage transactions, and coordinate between repositories. Replaces the previous use-case pattern for better simplicity.
3. **Repository** (`internal/repository/postgres/`) — Data access implementations using `pgx/v5`. Implements domain repository interfaces.
4. **Controller** (`internal/controller/`) — HTTP controllers using `chi` router, DTOs, request validation.
5. **Middleware** (`internal/middleware/`) — HTTP middleware (idempotency, metrics, CORS).
6. **Infrastructure** (`internal/infrastructure/`) — Cross-cutting concerns: config, observability (zerolog, OpenTelemetry, Prometheus), Redis (streams, locks).
7. **Providers** (`internal/providers/`) — External payment provider integrations (Stripe, PayPal mocks).

### Two entry points

- `cmd/api/` — HTTP API server. Uses `bootstrap.New()` for shared infrastructure setup, wires dependencies and starts chi router with graceful shutdown.
- `cmd/worker/` — Background worker. Uses `bootstrap.New()` and `errgroup` for concurrent goroutine management. Consumes Redis Streams (payment events), polls outbox table, processes async payments. Uses distributed locks to prevent duplicate processing.

### Bootstrap (`internal/bootstrap/`)

Shared infrastructure initialization: config, logger, tracer, database pool, Redis client, metrics. Used by both `cmd/api` and `cmd/worker` to avoid duplication.

## Key Patterns

- **Money as int64 cents** — All monetary values are stored as `int64` (cents) internally. The HTTP API accepts/returns `float64` JSON for backward compatibility; conversion happens at the boundary (handlers/DTOs) using `floatToCents()` and `centsToFloat()`. PostgreSQL `NUMERIC(19,4)` columns are scanned via string intermediary (`internal/infrastructure/postgres/money.go`). **Critical bug fix**: Negative amounts < 100 cents now convert correctly (e.g., -99 cents → "-0.99", not "0.99").
- **Internal transfers** are synchronous — debit/credit within a single DB transaction with deterministic account locking (sorted UUIDs) to prevent deadlocks.
- **External payments** are asynchronous — payment is created as `pending`, written to a transactional **outbox** table (same TX), then published to Redis Streams. Workers process using straightforward flow: reserve funds → call provider (with circuit breaker) → mark completed, or compensate on failure.
- **Payment state machine**: `pending → processing → completed/failed`, `failed → processing` (retry), `completed → refunded`. Transitions enforced by `Payment.CanTransitionTo()`.
- **Optimistic locking** on accounts via `version` column.
- **Idempotency** at two levels: HTTP middleware (checks `Idempotency-Key` header against `idempotency_keys` table, with 1MB body size limit) and DB-level unique constraint on `payments.idempotency_key`.
- **Distributed locking** via Redis `SET NX EX` for worker-level deduplication.
- **Dependency inversion** — Repository interfaces defined in domain layer (`internal/domain/*/repository.go`), concrete implementations in `internal/repository/postgres/`. Services depend on domain interfaces, not concrete types. TransactionManager interface defined in `internal/service/transaction.go`.
- **Circuit breakers** — Uses `sony/gobreaker/v2` with generics (`CircuitBreaker[*ProviderResult]`) directly in the provider factory. No wrapper package needed.
- **Typed error detection** — PostgreSQL duplicate key errors detected via `pgconn.PgError` code `23505`, not string matching. Error mapping in handlers uses a registry pattern (`errorMappings` slice with `errors.Is`).
- **Shared account operations** — `PaymentService` provides internal `debitAccount`/`creditAccount` helpers used across payment processing methods.
- **Config validation** — `Config.Validate()` checks required fields and valid ranges on startup.

## Testing

**Current Coverage**: 28.1% overall (14 test files, 180+ tests)
- Domain: 94-100% (payment state machine, account operations)
- Service: 77.3% (business logic orchestration)
- Providers: 82.1% (Stripe/PayPal mock integrations)
- Money conversion: 100% with critical bug fix for negative amounts < 100 cents

### Test Infrastructure
- **Fixtures** (`internal/testutil/fixtures.go`) — `NewTestAccount()`, `NewTestPayment()`, `NewCompletedPayment()`
- **Mocks** (`internal/testutil/mocks.go`) — `MockPaymentRepository`, `MockAccountRepository`, `MockOutboxRepository`, `MockTransactionManager` with optional `*Func` overrides for custom behavior
- **No Docker for unit tests** — Only integration tests require Docker services

### Test Organization
- **Domain tests** (`internal/domain/*/...`) — Entity behavior, state machines, business rules
- **Service tests** (`internal/service/*_test.go`) — Business logic with mocked repositories
- **Controller tests** (`internal/controller/*_test.go`) — HTTP request/response, validation, error mapping
- **Middleware tests** (`internal/middleware/*_test.go`) — Idempotency, metrics, CORS

### Writing Tests
- Use test fixtures from `testutil` for consistent test data
- Mock repositories should return proper domain errors (not `nil, nil`) to avoid nil pointer dereferences
- Service tests should use `MockTransactionManager` to verify transactional behavior
- Controller tests use `httptest.NewRecorder` with chi router

## Database

PostgreSQL with migrations at `internal/infrastructure/postgres/migrations/`. Key tables: `accounts`, `payments`, `payment_events`, `account_transactions`, `outbox`, `idempotency_keys`, `distributed_locks`, `webhooks`, `webhook_deliveries`. Money columns use `NUMERIC(19,4)`.

## Configuration

Viper-based (`internal/infrastructure/config/config.go`). Reads from `config.yaml` (optional) and environment variables with `PAYMENTS_` prefix. All config validated on startup via `Config.Validate()`.

### Key Configuration Sections
- **Server**: `PAYMENTS_SERVER_PORT=8080`, `PAYMENTS_SERVER_READ_TIMEOUT`, `PAYMENTS_SERVER_WRITE_TIMEOUT`
- **Database**: `PAYMENTS_DATABASE_HOST=localhost`, `PAYMENTS_DATABASE_PORT=5432`, `PAYMENTS_DATABASE_USER`, `PAYMENTS_DATABASE_PASSWORD`, `PAYMENTS_DATABASE_NAME`
- **Redis**: `PAYMENTS_REDIS_HOST=localhost`, `PAYMENTS_REDIS_PORT=6379`, `PAYMENTS_REDIS_PASSWORD`
- **Payment**: `PAYMENTS_PAYMENT_MAX_RETRIES=3`, `PAYMENTS_PAYMENT_LOCK_TTL=30s`
- **Worker**: `PAYMENTS_WORKER_CONCURRENCY`, `PAYMENTS_WORKER_POLL_INTERVAL`
- **Observability**: `PAYMENTS_OBSERVABILITY_LOG_LEVEL=info`, `PAYMENTS_OBSERVABILITY_ENABLE_TRACING`, `PAYMENTS_OBSERVABILITY_ENABLE_METRICS`

### Observability Stack
When `make docker-up` is run, the following services are available:
- **Jaeger UI**: http://localhost:16686 (distributed tracing)
- **Prometheus**: http://localhost:9090 (metrics)
- **Grafana**: http://localhost:3000 (dashboards, credentials: `admin`/`admin`)

### API Health Endpoints
- `GET /health` — Basic health check
- `GET /health/live` — Liveness probe (K8s ready)
- `GET /health/ready` — Readiness probe (checks DB/Redis connectivity)
- `GET /metrics` — Prometheus metrics endpoint

## Security Considerations

A comprehensive security audit is documented in `SECURITY.md`. Key security patterns already implemented:

### Implemented Security Patterns
- **Idempotency Keys** — Prevents duplicate payment processing (HTTP header + DB constraint)
- **Optimistic Locking** — Account `version` column prevents race conditions
- **Deterministic Lock Ordering** — Sorted UUIDs prevent deadlocks in internal transfers
- **Parameterized Queries** — All SQL uses pgx placeholders (no SQL injection)
- **Payment State Machine** — Enforces valid state transitions
- **Transaction Isolation** — Proper use of database transactions
- **Circuit Breaker Pattern** — Protects against cascading failures with external providers
- **Distributed Locks** — Redis-based locks prevent duplicate worker processing
- **Money as int64 Cents** — Avoids float precision issues (conversion at HTTP boundary only)

### Critical Security Gaps (see SECURITY.md for full details)
- No authentication/authorization middleware (all endpoints publicly accessible)
- No resource-level ownership checks (IDOR vulnerabilities)
- Missing input validation for integer overflow in money conversion
- No TLS/HTTPS enforcement
- Missing rate limiting
- Hardcoded default passwords in config
- Float64 precision loss in HTTP money handling

When implementing security improvements, refer to `SECURITY.md` for detailed remediation guidance.

## Development Guidelines

### Money Handling
- **CRITICAL**: All monetary values are `int64` cents internally
- PostgreSQL stores as `NUMERIC(19,4)`, scanned via string intermediary (`internal/infrastructure/postgres/money.go`)
- HTTP API uses `float64` JSON for backward compatibility — conversion at boundary only
- **Known bug fix**: Negative amounts < 100 cents now convert correctly (e.g., -99 cents → "-0.99", not "0.99")
- When adding new money fields, always use `int64` for cents, convert only at API boundary

### Error Handling
- Use typed errors from `internal/domain/*/errors.go`
- PostgreSQL errors: Check `pgconn.PgError` code `23505` for duplicates (not string matching)
- Service layer: Return domain errors, let controller map to HTTP status
- Controller error mapping: Use `errorMappings` registry pattern with `errors.Is`

### Concurrency Patterns
- Internal transfers: Synchronous with deterministic locking (sorted account UUIDs)
- External payments: Asynchronous via outbox pattern + Redis Streams
- Distributed locks: Redis `SET NX EX` with 30s TTL
- Use `golang.org/x/sync/errgroup` for managing concurrent goroutines (see `cmd/worker`)

## Libraries & Documentation

Always use Context7 MCP when needing library/API documentation, code generation, or configuration steps for the tech stack (chi, pgx, go-redis, viper, zerolog, OpenTelemetry, prometheus, gobreaker, retry-go).
