# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Recent Improvements

### Test Coverage (28.1%)
- **Overall**: 28.1% coverage (up from 19.5%)
- **14 test files, 180+ tests** covering domain, service, and provider layers
- **Coverage by layer**:
  - Domain: 94-100% (payment state machine, account operations)
  - Service: 77.3% (business logic orchestration)
  - Providers: 82.1% (Stripe/PayPal mock integrations)
  - Money conversion: 100% with critical bug fix
- **No Docker required** for unit tests - only integration tests need Docker

### Code Quality
- **90% comment reduction** - removed verbose comments, self-documenting code
- **SOLID principles enforced** - dependency inversion, repository pattern
- **Clean code practices** - simplified logic, removed duplication

### Critical Bug Fix: Money Conversion
- **Issue**: Negative amounts < 100 cents converted incorrectly
- **Example**: -99 cents → "0.99" (was showing positive)
- **Fixed**: -99 cents → "-0.99" (correct negative display)
- **Test coverage**: 100% for money conversion edge cases
- **Location**: `internal/infrastructure/postgres/money.go`

## Build & Development Commands

```bash
make build              # Build all binaries (api, worker, migrate) into bin/
make run-api            # Run API server (port 8080)
make run-worker         # Run background worker
make test               # Run unit tests with race detector and coverage
make test-integration   # Run integration tests (requires Docker services)
make lint               # Run golangci-lint
make format             # Run go fmt
make deps               # Download and tidy dependencies
make docker-up          # Start PostgreSQL, Redis, Jaeger, Prometheus
make docker-down        # Stop Docker services
make migrate-up         # Run database migrations
make migrate-down       # Rollback database migrations
make migrate-create name=<name>  # Create new migration
```

Run a single test: `go test -v -run TestName ./internal/domain/payment/...`

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

**Current Coverage**: 28.1% overall - Domain: 94-100%, Service: 77.3%, Providers: 82.1%

- **Run tests**: `make test` (unit, no Docker) | `make test-integration` (requires Docker)
- **Test fixtures** — `internal/testutil/fixtures.go` provides `NewTestAccount()`, `NewTestPayment()`, `NewCompletedPayment()`.
- **Mock implementations** — `internal/testutil/mocks.go` provides `MockPaymentRepository`, `MockAccountRepository`, `MockOutboxRepository`, `MockTransactionManager` with optional function overrides.
- **Controller tests** — `internal/controller/*_test.go` cover request parsing, service integration, UUID validation, error mapping. Tests use service layer with mocked dependencies.
- **Middleware tests** — `internal/middleware/idempotency_test.go`.
- **14 test files, 180+ tests** - No Docker required for unit tests

## Database

PostgreSQL with migrations at `internal/infrastructure/postgres/migrations/`. Key tables: `accounts`, `payments`, `payment_events`, `account_transactions`, `outbox`, `idempotency_keys`, `distributed_locks`, `webhooks`, `webhook_deliveries`. Money columns use `NUMERIC(19,4)`.

## Configuration

Viper-based (`internal/config/config.go`). Reads from `config.yaml` (optional) and environment variables with `PAYMENTS_` prefix (e.g., `PAYMENTS_SERVER_PORT`, `PAYMENTS_DATABASE_HOST`). Validated on startup via `Config.Validate()`. Key config sections: `server`, `database`, `redis`, `payment`, `worker`, `observability`.

## Libraries

Always use Context7 MCP when I need library/API documentation, code generation, setup or configuration steps without me having to explicitly ask.
