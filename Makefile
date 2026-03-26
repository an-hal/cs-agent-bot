APP_NAME=cs-agent-bot
BUILD_DIR=bin
CMD_SERVER=./cmd/server
CMD_MIGRATE=./cmd/migrate
SWAG=swag

.PHONY: all swag build run dev clean install test migrate-up migrate-down migrate-create watch lint-fix

all: dev

# ==================== Setup ====================

# Install development dependencies
install:
	@echo "Installing dependency packages..."
	go mod download
	go install github.com/swaggo/swag/cmd/swag@latest
	go install github.com/vektra/mockery/v2@latest
	go install github.com/air-verse/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# ==================== Development ====================

# Dev: Generate Swagger + Build + Run
dev:
	@$(MAKE) swag
	@$(MAKE) build
	@$(MAKE) run

# Run without building (hot reload friendly)
run-dev:
	@echo "Running app in development mode..."
	go run $(CMD_SERVER)

# Hot reload with Air
watch:
	@echo "Starting hot reload with Air..."
	@air

# ==================== Build ====================

# Build binary
build:
	@echo "🔨 Building app binary..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME) $(CMD_SERVER)
	go build -o $(BUILD_DIR)/migrate $(CMD_MIGRATE)

# Run binary
run:
	@echo "🚀 Running app..."
	./$(BUILD_DIR)/$(APP_NAME)

# Clean build
clean:
	@echo "🧹 Cleaning build directory..."
	rm -rf $(BUILD_DIR)

# ==================== Documentation ====================

# Generate Swagger docs
swag:
	@echo "📚 Generating Swagger docs..."
	$(SWAG) init -g $(CMD_SERVER)/main.go -o ./docs

# ==================== Testing ====================

# Run all tests
test:
	@echo "🧪 Running all tests..."
	go test ./... -v

# Run unit tests only
unit-test:
	@echo "🧲 Running unit tests..."
	go test ./internal/usecase -v

# Run tests with coverage
test-coverage:
	@echo "📊 Running tests with coverage..."
	go test ./... -cover -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "📄 Coverage report: coverage.html"

# ==================== Database Migrations ====================

# Run all pending migrations
migrate-up:
	@echo "⬆️  Running migrations..."
	go run $(CMD_MIGRATE) up

# Rollback the last migration
migrate-down:
	@echo "⬇️  Rolling back last migration..."
	go run $(CMD_MIGRATE) down

# Create a new migration (usage: make migrate-create name=create_users_table)
migrate-create:
	@if [ -z "$(name)" ]; then \
		echo "❌ Error: Please provide a migration name"; \
		echo "Usage: make migrate-create name=create_users_table"; \
		exit 1; \
	fi
	@echo "📝 Creating migration: $(name)"
	go run $(CMD_MIGRATE) create $(name)

# ==================== Docker ====================

# Docker Build and Run
docker-up:
	@echo "🐳 Starting Docker containers..."
	docker-compose up -d

# Docker stop
docker-down:
	@echo "🐳 Stopping Docker containers..."
	docker-compose down

# Docker Rebuild and Run
docker-rebuild:
	@echo "🐳 Rebuilding Docker containers..."
	docker-compose down --remove-orphans --volumes
	docker-compose build --no-cache
	docker-compose up -d

# View Docker logs
docker-logs:
	docker-compose logs -f

# ==================== Code Quality ====================

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

# Run golangci-lint
lint:
	@echo "Running golangci-lint..."
	@golangci-lint run ./...

# Run golangci-lint with auto-fix
lint-fix:
	@echo "Running golangci-lint with auto-fix..."
	@golangci-lint run --fix ./...

# Tidy go modules
tidy:
	@echo "Tidying go modules..."
	go mod tidy

# ==================== Help ====================

help:
	@echo "Available commands:"
	@echo ""
	@echo "  Setup:"
	@echo "    make install          - Install development dependencies"
	@echo ""
	@echo "  Development:"
	@echo "    make dev              - Generate docs, build, and run"
	@echo "    make watch            - Run with hot reload (Air)"
	@echo "    make run-dev          - Run without building (go run)"
	@echo ""
	@echo "  Build:"
	@echo "    make build            - Build application binary"
	@echo "    make run              - Run built binary"
	@echo "    make clean            - Remove build artifacts"
	@echo ""
	@echo "  Documentation:"
	@echo "    make swag             - Generate Swagger documentation"
	@echo ""
	@echo "  Testing:"
	@echo "    make test             - Run all tests"
	@echo "    make unit-test        - Run unit tests only"
	@echo "    make test-coverage    - Run tests with coverage report"
	@echo ""
	@echo "  Migrations:"
	@echo "    make migrate-up       - Run pending migrations"
	@echo "    make migrate-down     - Rollback last migration"
	@echo "    make migrate-create name=xxx - Create new migration"
	@echo ""
	@echo "  Docker:"
	@echo "    make docker-up        - Start Docker containers"
	@echo "    make docker-down      - Stop Docker containers"
	@echo "    make docker-rebuild   - Rebuild and start containers"
	@echo "    make docker-logs      - View container logs"
	@echo ""
	@echo "  Code Quality:"
	@echo "    make lint             - Run golangci-lint"
	@echo "    make lint-fix         - Run golangci-lint with auto-fix"
	@echo "    make fmt              - Format code"
	@echo "    make vet              - Run go vet"
	@echo "    make tidy             - Tidy go modules"
