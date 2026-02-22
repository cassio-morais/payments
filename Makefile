.PHONY: help build test docker-up docker-down migrate-up migrate-down run-api run-worker clean

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build all binaries
	@echo "Building binaries..."
	@go build -o bin/api ./cmd/api
	@go build -o bin/worker ./cmd/worker
	@go build -o bin/migrate ./cmd/migrate
	@echo "Build complete!"

test: ## Run tests
	@echo "Running tests..."
	@go test -v -race -cover ./...

test-integration: ## Run integration tests
	@echo "Running integration tests..."
	@go test -v -race -tags=integration ./tests/integration/...

docker-up: ## Start all Docker services
	@echo "Starting Docker services..."
	@docker-compose -f deployments/docker/docker-compose.yml up -d
	@echo "Services started!"

docker-down: ## Stop all Docker services
	@echo "Stopping Docker services..."
	@docker-compose -f deployments/docker/docker-compose.yml down
	@echo "Services stopped!"

docker-build: ## Build Docker images
	@echo "Building Docker images..."
	@docker-compose -f deployments/docker/docker-compose.yml build
	@echo "Images built!"

migrate-up: ## Run database migrations up
	@echo "Running migrations..."
	@migrate -path internal/infrastructure/postgres/migrations -database "postgresql://payments:payments@localhost:5432/payments?sslmode=disable" up
	@echo "Migrations complete!"

migrate-down: ## Run database migrations down
	@echo "Rolling back migrations..."
	@migrate -path internal/infrastructure/postgres/migrations -database "postgresql://payments:payments@localhost:5432/payments?sslmode=disable" down
	@echo "Rollback complete!"

migrate-create: ## Create a new migration (usage: make migrate-create name=migration_name)
	@migrate create -ext sql -dir internal/infrastructure/postgres/migrations -seq $(name)

run-api: ## Run API server locally
	@echo "Starting API server..."
	@go run ./cmd/api

run-worker: ## Run worker locally
	@echo "Starting worker..."
	@go run ./cmd/worker

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf bin/
	@go clean
	@echo "Clean complete!"

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies ready!"

lint: ## Run linter
	@echo "Running linter..."
	@golangci-lint run ./...

format: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Format complete!"
