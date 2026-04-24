# =============================================================================
# Payment Aggregator — Makefile
# =============================================================================

BINARY_NAME=payment-aggregator
MAIN_PATH=./cmd/api/main.go
MIGRATION_PATH=./migrations
DB_URL?=$(shell grep DATABASE_URL .env | cut -d '=' -f2-)

.PHONY: all build run dev clean test lint swagger migrate-up migrate-down \
        migrate-create docker-up docker-down docker-logs help

# Default target
all: help

# =============================================================================
# Development
# =============================================================================

## run: Run the application directly
run:
	go run $(MAIN_PATH)

## dev: Run with hot reload (requires: go install github.com/air-verse/air@latest)
dev:
	air

## build: Build binary to ./bin/
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p bin
	CGO_ENABLED=0 go build -ldflags="-w -s" -o bin/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Build complete: ./bin/$(BINARY_NAME)"

## clean: Remove build artifacts
clean:
	@rm -rf bin/
	@echo "Cleaned build artifacts"

# =============================================================================
# Testing
# =============================================================================

## test: Run all tests
test:
	go test -v -race -coverprofile=coverage.out ./...

## test-unit: Run unit tests only
test-unit:
	go test -v -race -run "Unit" ./internal/usecase/...

## test-integration: Run integration tests only
test-integration:
	go test -v -race -run "Integration" ./...

## coverage: Show test coverage report in browser
coverage: test
	go tool cover -html=coverage.out

# =============================================================================
# Code Quality
# =============================================================================

## lint: Run golangci-lint (requires: brew install golangci-lint)
lint:
	golangci-lint run ./...

## fmt: Format all Go files
fmt:
	gofmt -s -w .
	goimports -w .

## vet: Run go vet
vet:
	go vet ./...

## tidy: Tidy go modules
tidy:
	go mod tidy

# =============================================================================
# Database Migrations
# =============================================================================

## migrate-up: Run all pending migrations
migrate-up:
	@echo "Running migrations..."
	migrate -path $(MIGRATION_PATH) -database "$(DB_URL)" up
	@echo "Migrations complete"

## migrate-down: Rollback last migration
migrate-down:
	migrate -path $(MIGRATION_PATH) -database "$(DB_URL)" down 1

## migrate-down-all: Rollback ALL migrations (dangerous!)
migrate-down-all:
	@echo "WARNING: Rolling back ALL migrations!"
	migrate -path $(MIGRATION_PATH) -database "$(DB_URL)" down -all

## migrate-status: Show migration status
migrate-status:
	migrate -path $(MIGRATION_PATH) -database "$(DB_URL)" version

## migrate-create: Create new migration file (usage: make migrate-create name=add_users_table)
migrate-create:
	@if [ -z "$(name)" ]; then echo "Usage: make migrate-create name=migration_name"; exit 1; fi
	migrate create -ext sql -dir $(MIGRATION_PATH) -seq $(name)
	@echo "Created migration: $(name)"

# =============================================================================
# Swagger Documentation
# =============================================================================

## swagger: Generate swagger docs
swagger:
	@echo "Generating swagger docs..."
	swag init -g $(MAIN_PATH) -o ./docs
	@echo "Swagger docs generated at ./docs"

# =============================================================================
# Docker
# =============================================================================

## docker-up: Start all services with docker-compose
docker-up:
	docker compose up -d
	@echo "Services started. API: http://localhost:8080"

## docker-down: Stop all services
docker-down:
	docker compose down

## docker-down-v: Stop all services and remove volumes
docker-down-v:
	docker compose down -v

## docker-logs: Follow logs from all services
docker-logs:
	docker compose logs -f

## docker-logs-api: Follow logs from API service only
docker-logs-api:
	docker compose logs -f api

## docker-build: Rebuild docker images
docker-build:
	docker compose build --no-cache

# =============================================================================
# Setup
# =============================================================================

## setup: Install all required tools
setup:
	@echo "Installing development tools..."
	go install github.com/air-verse/air@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@echo "All tools installed"

## env: Copy .env.example to .env (only if .env doesn't exist)
env:
	@if [ ! -f .env ]; then cp .env.example .env; echo "Created .env from .env.example"; \
	else echo ".env already exists, skipping"; fi

# =============================================================================
# Help
# =============================================================================

## help: Show this help message
help:
	@echo "Payment Aggregator — Available Commands:"
	@echo ""
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'