# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

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

Go payment processing API using **Clean Architecture** with four layers:

1. **Domain** (`internal/domain/`) — Entities, value objects, repository interfaces, domain errors. No external dependencies.
2. **Application** (`internal/application/`) — Use cases that orchestrate domain logic. Each use case is a struct with an `Execute` method. Defines its own ports (`ports.go`) for `TransactionManager` and `OutboxWriter` to avoid importing infrastructure.
3. **Infrastructure** (`internal/infrastructure/`) — Concrete implementations: PostgreSQL repos (`pgx/v5`), Redis client/streams/locks, mock payment providers, observability (zerolog, OpenTelemetry, Prometheus).
4. **Interfaces** (`internal/interfaces/`) — HTTP handlers (`chi` router), middleware (idempotency, metrics), DTOs.

### Two entry points

- `cmd/api/` — HTTP API server. Uses `bootstrap.New()` for shared infrastructure setup, wires dependencies and starts chi router with graceful shutdown.
- `cmd/worker/` — Background worker. Uses `bootstrap.New()` and `errgroup` for concurrent goroutine management. Consumes Redis Streams (payment events), polls outbox table, processes async payments. Uses distributed locks to prevent duplicate processing.

### Bootstrap (`internal/bootstrap/`)

Shared infrastructure initialization: config, logger, tracer, database pool, Redis client, metrics. Used by both `cmd/api` and `cmd/worker` to avoid duplication.

### Reusable packages (`pkg/`)

- `pkg/saga/` — Saga orchestrator: sequential steps with reverse-order compensation on failure. Uses `errors.Join` to collect all compensation errors.
- `pkg/retry/` — Generic retry with exponential backoff using `avast/retry-go`.

## Key Patterns

- **Money as int64 cents** — All monetary values are stored as `int64` (cents) internally. The HTTP API accepts/returns `float64` JSON for backward compatibility; conversion happens at the boundary (handlers/DTOs) using `floatToCents()` and `centsToFloat()`. PostgreSQL `NUMERIC(19,4)` columns are scanned via string intermediary (`internal/infrastructure/postgres/money.go`).
- **Internal transfers** are synchronous — debit/credit within a single DB transaction with deterministic account locking (sorted UUIDs) to prevent deadlocks.
- **External payments** are asynchronous — payment is created as `pending`, written to a transactional **outbox** table (same TX), then published to Redis Streams. Workers process via saga (reserve funds → call provider with circuit breaker).
- **Payment state machine**: `pending → processing → completed/failed`, `failed → processing` (retry), `completed → refunded`. Transitions enforced by `Payment.CanTransitionTo()`.
- **Optimistic locking** on accounts via `version` column.
- **Idempotency** at two levels: HTTP middleware (checks `Idempotency-Key` header against `idempotency_keys` table, with 1MB body size limit) and DB-level unique constraint on `payments.idempotency_key`.
- **Distributed locking** via Redis `SET NX EX` for worker-level deduplication.
- **Dependency inversion** — Repository interfaces defined in domain layer (`internal/domain/*/repository.go`), application-layer ports in `internal/application/payment/ports.go`, infrastructure adapters in `internal/infrastructure/postgres/outbox_adapter.go`.
- **Circuit breakers** — Uses `sony/gobreaker/v2` with generics (`CircuitBreaker[*ProviderResult]`) directly in the provider factory. No wrapper package needed.
- **Typed error detection** — PostgreSQL duplicate key errors detected via `pgconn.PgError` code `23505`, not string matching. Error mapping in handlers uses a registry pattern (`errorMappings` slice with `errors.Is`).
- **Shared account operations** — `internal/application/payment/account_ops.go` provides `debitAccount`/`creditAccount` helpers used across create, process, and refund use cases.
- **Config validation** — `Config.Validate()` checks required fields and valid ranges on startup.

## Testing

- **Test fixtures** — `internal/testutil/fixtures.go` provides `NewTestAccount()`, `NewTestPayment()`, `NewCompletedPayment()`.
- **Mock implementations** — `internal/testutil/mocks.go` provides `MockPaymentRepository`, `MockAccountRepository`, `MockTransactionManager` with optional function overrides.
- **Use case tests** — `internal/application/payment/*_test.go` cover create (idempotency, sync transfer, async, validation, insufficient funds), process (success, provider failure + compensation, retry), and refund (success, invalid state).
- **Handler tests** — `internal/interfaces/http/handlers/*_test.go` cover request parsing, UUID validation, error mapping.
- **Middleware tests** — `internal/interfaces/http/middleware/idempotency_test.go`.

## Database

PostgreSQL with migrations at `internal/infrastructure/postgres/migrations/`. Key tables: `accounts`, `payments`, `payment_events`, `account_transactions`, `outbox`, `idempotency_keys`, `distributed_locks`, `webhooks`, `webhook_deliveries`. Money columns use `NUMERIC(19,4)`.

## Configuration

Viper-based (`internal/config/config.go`). Reads from `config.yaml` (optional) and environment variables with `PAYMENTS_` prefix (e.g., `PAYMENTS_SERVER_PORT`, `PAYMENTS_DATABASE_HOST`). Validated on startup via `Config.Validate()`. Key config sections: `server`, `database`, `redis`, `payment`, `worker`, `observability`.

## Libraries

Always use Context7 MCP when I need library/API documentation, code generation, setup or configuration steps without me having to explicitly ask.
