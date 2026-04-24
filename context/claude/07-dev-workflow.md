# Development Workflow

## Commands (Makefile)

```bash
# Tooling install (swag, mockery, air, golangci-lint)
make install

# Generate Swagger + build + run
make dev

# Hot reload via Air
make watch

# Build binary only
make build

# Tests
make test                 # All tests
make unit-test            # internal/usecase only
make test-coverage        # HTML coverage report
go test ./internal/usecase/... -run TestFunctionName -v

# Lint / format
make lint
make lint-fix
make fmt

# Migrations
make migrate-up
make migrate-down
make migrate-create name=create_something

# Swagger regeneration
make swag

# Docker
make docker-up
make docker-rebuild
```

## Environment Setup

Copy `.env.example` → `.env`. Startup validation **fails fast** on missing required keys.

### Required Everywhere
- `HALOAI_API_URL`
- `WA_API_TOKEN`
- `WA_WEBHOOK_SECRET`
- `TELEGRAM_BOT_TOKEN`
- `TELEGRAM_AE_LEAD_ID`
- `HANDOFF_WEBHOOK_SECRET`
- Database: `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`
- Redis: `REDIS_HOST`, `REDIS_PORT`

### Required in Non-Dev Only
- `APP_URL` — public URL (used as OIDC audience)
- `SCHEDULER_SA_EMAIL` — GCP Cloud Scheduler service account email

### Feature Flags / Optional
- `USE_WORKFLOW_ENGINE=true` — enable workflow engine
- `USE_DYNAMIC_RULES=true` — use `TriggerRuleRepo` instead of hardcoded triggers
- `EXPORT_STORAGE_PATH` — background-job export file store
- `CRON_ENABLED=true`, `CRON_HOUR=8` — scheduled run timing
- `PROMO_DEADLINE`, `SURVEY_PLATFORM_URL`, `CHECKIN_FORM_URL` — business config
- `JWT_VALIDATE_URL` — external JWT validator endpoint

## Testing Approach

- **Total test files:** ~46 across the codebase.
- **Unit tests:** `internal/usecase/*` — business logic with mocks.
- **Handler tests:** `internal/delivery/http/dashboard/*_test.go`.
- **Scenario tests:** full client journey simulations (`workflow_scenarios_test.go`, `filterdsl_scenario_test.go`).
- **Package-level tests:** `conditiondsl_test.go`, `filterdsl_test.go`, `workday_test.go`.
- **Mocks:** `internal/repository/mocks/` (generated via `mockery`).
- **Assertion library:** `testify/assert`.

## Docker

- `Dockerfile` — multi-stage production build (golang:1.25-alpine → alpine:3.19), `-ldflags="-s -w"`.
- `Dockerfile.dev` — dev container with hot-reload tooling.
- `docker-compose.yml` — local dev stack (app + dependencies).

## Observability

- **Logging:** zerolog structured JSON → stdout.
- **Tracing:** OpenTelemetry with GCP exporter (Zipkin fallback). Init in `internal/tracer/`.
- **Liveness:** `/tmp/app_server_live` file created on startup (for liveness probes).
- **Readiness:** `GET /readiness` endpoint.
