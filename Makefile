.PHONY: build test lint clean dev migrate migrate-down migrate-status seed backup restore-backup

# Build variables
BINARY=contentx
BUILD_DIR=./bin
GO=go

# Build the application
build:
	$(GO) build -o $(BUILD_DIR)/$(BINARY) ./cmd/server

# Run all tests
test:
	$(GO) test ./... -v -count=1

# Run tests with coverage
test-cover:
	$(GO) test ./... -coverprofile=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html

# Run linter
lint:
	golangci-lint run ./...

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR) coverage.out coverage.html

# Run in development mode
dev:
	$(GO) run ./cmd/server

# Run database migrations (apply pending, then exit)
migrate:
	$(GO) run ./cmd/server --migrate

# Roll back the last N migrations (default 1)
migrate-down:
	$(GO) run ./cmd/server --migrate-down=1

# Show migration status
migrate-status:
	$(GO) run ./cmd/server --migrate-status

# Seed the database
seed:
	$(GO) run ./cmd/server --seed

# Format code
fmt:
	$(GO) fmt ./...

# Vet code
vet:
	$(GO) vet ./...

# Generate swagger docs
swagger:
	swag init -g cmd/server/main.go --parseDependency --parseInternal -o docs/api

# Build Docker image
docker:
	docker build -t contentx:latest .

# Run with Docker Compose
docker-up:
	docker-compose up -d

# Stop Docker Compose
docker-down:
	docker-compose down

# Create a full backup (database + media) via the admin API.
# Usage: make backup API=http://localhost:8080 TOKEN=xxx
API ?= http://localhost:8080
TOKEN ?= 
backup:
	@curl -sS -X POST "$(API)/api/v1/admin/backup?type=all" \
		-H "Authorization: Bearer $(TOKEN)" | python -m json.tool

# Restore from a backup file via the admin API.
# Usage: make restore-backup API=http://localhost:8080 TOKEN=xxx FILE=db-20260722-150405.sql
restore-backup:
	@curl -sS -X POST "$(API)/api/v1/admin/backup/$(FILE)/restore" \
		-H "Authorization: Bearer $(TOKEN)" | python -m json.tool
