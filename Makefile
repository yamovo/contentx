.PHONY: all build run test clean dev docker-up docker-down frontend

# Variables
BINARY=vortexcms
GO=go
NPM=npm

all: build

# Backend
build:
	CGO_ENABLED=1 $(GO) build -o $(BINARY) cmd/server/main.go

run: build
	./$(BINARY)

test:
	$(GO) test ./... -v -cover

test-short:
	$(GO) test ./... -short

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY)
	rm -rf uploads/ backups/ logs/ *.db

# Frontend
frontend:
	cd web && $(NPM) install && $(NPM) run build

frontend-dev:
	cd web && $(NPM) run dev

# Docker
docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-build:
	docker-compose build

docker-logs:
	docker-compose logs -f app

# Development
dev:
	$(GO) run cmd/server/main.go

# Database
migrate:
	$(GO) run cmd/server/main.go --migrate

seed:
	$(GO) run cmd/server/main.go --seed

# Help
help:
	@echo "VortexCMS Build System"
	@echo "======================"
	@echo "  make build         - Build backend binary"
	@echo "  make run           - Build and run"
	@echo "  make test          - Run all tests"
	@echo "  make frontend      - Build frontend"
	@echo "  make frontend-dev  - Start frontend dev server"
	@echo "  make docker-up     - Start with Docker Compose"
	@echo "  make docker-down   - Stop Docker Compose"
	@echo "  make dev           - Run in development mode"
	@echo "  make clean         - Remove build artifacts"
	@echo "  make help          - Show this help"
