# cs-agent-bot

A clean and modular Golang microservice for AI-powered customer service automation using Clean Architecture principles. This service handles conversational AI interactions and message processing for automated customer support.

---

## Prerequisites

- Go 1.23+
- PostgreSQL v13
- Redis v6.2
- OpenAI/Anthropic API credentials (for AI integration)

---

## Getting Started

### 1. Clone and Setup Environment

```bash
# Copy environment file
cp .env.example .env

# Fill in your configuration in .env
```

### 2. Install Dependencies

```bash
go mod download
```

### 3. Run Database Migrations

```bash
# Create the database tables (you'll handle this manually)
# See the suggested schema in the project documentation
```

### 4. Start the Server

```bash
go run ./cmd/server
```

The server will start at `http://localhost:8080`

### 5. Access Swagger Documentation

Open `http://localhost:8080/swagger/index.html` in your browser.

---

## Development with Docker

```bash
# Start all services (app, postgres, redis)
docker-compose up --build

# Stop services
docker-compose down
```

---

## Development with Hot Reload

This project uses [Air](https://github.com/air-verse/air) for live-reloading during development.

### Setup

```bash
# Install Air (included in make install)
make install
```

### Usage

```bash
# Start development server with hot reload
make watch
```

Air will:
- Watch for changes in `cmd/`, `config/`, and `internal/` directories
- Automatically rebuild and restart the server on file changes
- Exclude test files, docs, and migration files from triggering rebuilds

The server runs at `http://localhost:8080` (or your configured `APP_PORT`).

---

## Available Commands

### Application

```bash
# Run the main application
go run ./cmd/server

# Build the application
go build -o bin/cs-agent-bot ./cmd/server
```

### Database Migrations

```bash
# Create a new migration
go run ./cmd/migrate create <migration_name>
# Example: go run ./cmd/migrate create add_conversations_table

# Apply all pending migrations
go run ./cmd/migrate up

# Rollback the last migration
go run ./cmd/migrate down
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests for a specific package
go test ./internal/usecase/...
```

### Code Generation

```bash
# Generate Swagger documentation
swag init -g cmd/server/main.go -o ./docs

# Or use make (if available)
make swag
```

### Linting & Formatting

This project uses [golangci-lint](https://golangci-lint.run/) for comprehensive linting.

```bash
# Run linter (golangci-lint)
make lint

# Run linter with auto-fix
make lint-fix

# Format code only
go fmt ./...

# Run go vet only
go vet ./...
```

---

## Project Structure

```
cs-agent-bot/
├── cmd/
│   ├── server/
│   │   └── main.go             # Application entry point
│   └── migrate/
│       └── main.go             # Database migration CLI
├── config/
│   └── config.go               # Environment configuration loader
├── docs/                       # Swagger documentation (auto-generated)
├── internal/
│   ├── delivery/
│   │   ├── http/
│   │   │   ├── deps/           # Dependency injection container
│   │   │   ├── example/        # Example handlers (reference implementation)
│   │   │   ├── health/         # Health check endpoint
│   │   │   ├── middleware/     # HTTP middlewares (auth, logging)
│   │   │   ├── router/         # Custom HTTP router
│   │   │   └── route.go        # Route definitions
│   │   └── response/           # Standardized API response helpers
│   ├── entity/                 # Domain entities
│   │   ├── conversation.go     # Conversation model
│   │   ├── message.go          # Message model
│   │   ├── agent_event.go      # Event model for webhooks
│   │   └── service.go          # Caller service model
│   ├── migration/              # Migration runner and utilities
│   ├── pkg/
│   │   ├── database/           # Database clients (PostgreSQL, Redis)
│   │   └── logger/             # Logging utilities
│   ├── repository/             # Data access layer
│   │   ├── conversation_repository.go
│   │   ├── message_repository.go
│   │   ├── agent_event_repository.go
│   │   └── service_repository.go
│   ├── service/
│   │   └── session/            # Session management
│   └── usecase/                # Business logic layer
├── migration/                  # SQL migration files
├── scripts/                    # Utility scripts
├── .env.example                # Environment template
├── .env.docker.example         # Docker environment template
├── docker-compose.yml          # Docker Compose configuration
├── Dockerfile                  # Container build configuration
├── go.mod                      # Go module definition
└── README.md                   # This file
```

---

## Architecture

This project follows **Clean Architecture** principles:

```
┌─────────────────────────────────────────────────────────┐
│                      Delivery Layer                      │
│              (HTTP Handlers, Middleware)                 │
├─────────────────────────────────────────────────────────┤
│                      UseCase Layer                       │
│                   (Business Logic)                       │
├─────────────────────────────────────────────────────────┤
│                    Repository Layer                      │
│                    (Data Access)                         │
├─────────────────────────────────────────────────────────┤
│                      Entity Layer                        │
│                   (Domain Models)                        │
└─────────────────────────────────────────────────────────┘
```

### Layer Descriptions

| Layer          | Directory              | Description                                       |
| -------------- | ---------------------- | ------------------------------------------------- |
| **Entity**     | `internal/entity/`     | Core business entities, independent of frameworks |
| **Repository** | `internal/repository/` | Data access interfaces and implementations        |
| **UseCase**    | `internal/usecase/`    | Application business logic                        |
| **Delivery**   | `internal/delivery/`   | HTTP handlers, request/response handling          |

---

## API Endpoints

### Health Check

```
GET /healthz
GET /readiness
```

### Conversations

```
POST   /api/v1/cs-agent-bot/conversations              # Create conversation
GET    /api/v1/cs-agent-bot/conversations/:id          # Get conversation
GET    /api/v1/cs-agent-bot/conversations              # List conversations
PATCH  /api/v1/cs-agent-bot/conversations/:id          # Update conversation
```

### Messages

```
POST   /api/v1/cs-agent-bot/conversations/:id/messages # Send message
GET    /api/v1/cs-agent-bot/conversations/:id/messages # Get message history
```

### Example Resource (Reference Implementation)

```
GET    /api/v1/examples          # List all examples (paginated)
GET    /api/v1/examples/{id}     # Get example by ID
POST   /api/v1/examples          # Create new example
DELETE /api/v1/examples/{id}     # Delete example
```

---

## Database Schema

The service uses the following main tables:

- **services** - Caller services registry with API keys
- **conversations** - User conversation sessions
- **messages** - Messages within conversations
- **agent_events** - Events for webhook delivery to caller services

You'll need to create these tables manually based on your requirements.

---

## Environment Variables

| Variable                   | Description                             | Default           |
| -------------------------- | --------------------------------------- | ----------------- |
| `ENV`                      | Environment (development/production)    | `development`     |
| `APP_PORT`                 | HTTP server port                        | `3000`            |
| `LOG_LEVEL`                | Logging verbosity level                 | `info`            |
| `DB_ENABLED`               | Enable PostgreSQL                       | `false`           |
| `DB_HOST`                  | PostgreSQL host                         | `localhost`       |
| `DB_PORT`                  | PostgreSQL port                         | `5432`            |
| `DB_USER`                  | PostgreSQL user                         | `postgres`        |
| `DB_PASSWORD`              | PostgreSQL password                     | -                 |
| `DB_NAME`                  | PostgreSQL database name                | `cs_agent_bot`    |
| `DB_SSLMODE`               | PostgreSQL SSL mode                     | `disable`         |
| `DB_MAX_OPEN_CONNS`        | Max open connections in pool            | `25`              |
| `DB_MAX_IDLE_CONNS`        | Max idle connections in pool            | `5`               |
| `DB_CONN_MAX_LIFETIME`     | Max connection lifetime                 | `5m`              |
| `DB_CONN_MAX_IDLE_TIME`    | Max connection idle time                | `1m`              |
| `DB_QUERY_TIMEOUT`         | Query execution timeout                 | `30s`             |
| `DB_STATS_LOGGING_ENABLED` | Enable pool stats logging (5m interval) | `false`           |
| `REDIS_ENABLED`            | Enable Redis                            | `false`           |
| `REDIS_HOST`               | Redis host                              | -                 |
| `REDIS_PORT`               | Redis port                              | `6379`            |
| `REDIS_DB`                 | Redis database number                   | `0`               |
| `REDIS_USERNAME`           | Redis username                          | -                 |
| `REDIS_PASSWORD`           | Redis password                          | -                 |
| `DISPATCHER_ENABLED`       | Enable event dispatcher for webhooks    | `true`            |
| `TRACER_SERVICE_NAME`      | Service name for tracing                | `cs-agent-bot`    |

---

## API Response Format

All endpoints return a standardized JSON response:

```json
{
  "status": "success",
  "entity": "conversations",
  "state": "createConversation",
  "message": "Success Create Conversation",
  "data": []
}
```

| Field        | Description                                    |
| ------------ | ---------------------------------------------- |
| `status`     | `success` or `failed`                          |
| `entity`     | Resource name being operated on                |
| `state`      | Operation/action performed                     |
| `message`    | Human-readable result message                  |
| `error_code` | Error code (only on errors, e.g., `NOT_FOUND`) |
| `errors`     | Field-level validation errors (only on 422)    |
| `meta`       | Pagination metadata (only on paginated lists)  |
| `data`       | Response payload (array or object, never null) |
