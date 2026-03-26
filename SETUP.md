# Customer Service Agent Bot - Setup Guide

## Overview

Customer Service Agent Bot is an AI-powered customer service automation microservice built with Clean Architecture principles in Go.

---

## Prerequisites

- Go 1.23+
- PostgreSQL v13
- Redis v6.2 (for session storage)

---

## Getting Started

### 1. Environment Setup

```bash
# Copy environment file
cp .env.example .env

# Edit .env with your configuration
```

### 2. Install Dependencies

```bash
go mod download
```

### 3. Run Database Migrations

```bash
go run ./cmd/migrate up
```

### 4. Start the Server

```bash
# Development mode
go run ./cmd/server

# Or with hot reload
make watch
```

The server will start at `http://localhost:8080`

---

## Development with Docker

```bash
# Start all services (app, postgres, redis)
docker-compose up --build

# Stop services
docker-compose down
```

---

## Available Commands

| Command | Description |
|---------|-------------|
| `make dev` | Generate docs, build, and run |
| `make watch` | Run with hot reload (Air) |
| `make build` | Build binary |
| `make test` | Run all tests |
| `make migrate-up` | Run database migrations |
| `make migrate-down` | Rollback last migration |
| `make swag` | Generate Swagger docs |

---

## API Documentation

Once running, access Swagger documentation at:
```
http://localhost:8080/swagger/index.html
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

---

## Project Structure

```
cs-agent-bot/
├── cmd/
│   ├── server/main.go           # Application entry point
│   └── migrate/main.go           # Database migration CLI
├── config/config.go              # Environment configuration
├── internal/
│   ├── delivery/
│   │   ├── http/
│   │   │   ├── deps/             # Dependency injection
│   │   │   ├── example/          # Reference implementation
│   │   │   ├── health/           # Health check endpoints
│   │   │   ├── middleware/       # HTTP middlewares
│   │   │   └── router/           # Custom HTTP router
│   │   └── response/            # API response helpers
│   ├── entity/                  # Domain entities
│   ├── pkg/                     # Shared utilities
│   │   ├── database/           # Database clients
│   │   ├── logger/             # Logging
│   │   └── validator/          # Input validation
│   └── tracer/                 # OpenTelemetry tracing
├── migration/                   # SQL migration files
├── .env.example                # Environment template
├── docker-compose.yml          # Docker setup
├── Dockerfile                  # Production build
├── go.mod                      # Go module definition
└── README.md                   # Project documentation
```

---

## Configuration

Key environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `ENV` | Environment (development/production) | `development` |
| `APP_PORT` | HTTP server port | `3000` |
| `LOG_LEVEL` | Logging verbosity | `info` |
| `DB_NAME` | PostgreSQL database name | `cs_agent_bot` |
| `TRACER_SERVICE_NAME` | Service name for tracing | `cs-agent-bot` |
| `APP_ROUTE_PREFIX` | API route prefix | `/v1/cs-agent-bot` |

---

## API Response Format

All endpoints return a standardized JSON response:

```json
{
  "status": "success",
  "entity": "examples",
  "state": "getAll",
  "message": "Success Get All Examples",
  "data": []
}
```
