# Architecture

## Clean Architecture Layers

```
entity → repository → usecase → delivery
```

Strict layering: entities never depend on repositories; repositories never depend on usecases; usecases have no HTTP awareness.

## Entry Points (only two)

### Outbound — GCP Cloud Scheduler
```
GET /cron/run  [OIDC JWT]
  → processClient() per active client
    → PostgreSQL (read) → HaloAI Send API → PostgreSQL (write flags + audit)
      → Telegram API (if escalation)
```

### Inbound — HaloAI Webhook
```
POST /webhook/wa  [HaloAI HMAC signature]
  → HTTP 200 immediately (must be ≤5s)
    → goroutine: classify intent → apply rule → update DB
      → HaloAI Send API (reply if needed) → Telegram API (AE notification)
```

## Directory Structure

```
cmd/
  server/main.go              # Wires all deps, starts HTTP server (graceful shutdown)
  migrate/main.go             # Migration CLI: up | down | create <name>

config/
  config.go                   # Typed config loader; fail-fast on missing required keys

internal/
  delivery/http/
    route.go                  # ~200 route registrations (all endpoints)
    deps/                     # DI container (Deps struct) — wired in main.go
    router/                   # Custom net/http router
    middleware/               # OIDC, JWT, HMAC, HaloAI sig, workspace scoping, tracing, logging
    response/                 # Standardized JSON response helpers
    dto/                      # Request/response structs
    dashboard/                # 30+ REST handlers (client CRUD, invoices, reports, activity log, etc.)
    webhook/                  # wa_handler, checkin_handler, handoff_handler, paper_id handler
    cron/                     # cron trigger handler
    auth/                     # login, google oauth, whitelist
    payment/                  # verify_handler
    team/                     # team member CRUD
    health/                   # liveness / readiness

  entity/                     # ~20 domain models: Client, Invoice, ClientFlags, Escalation,
                              # ActionLog, Template, Workflow, AutomationRule, TriggerRule,
                              # Collection, Team, Workspace, WorkspaceMember, etc.

  repository/                 # ~25 repos (interfaces + pgx/squirrel impls + mocks/)

  usecase/
    cron/runner.go            # processClient() — P0→P5 priority loop
    trigger/                  # EvalHealthRisk, EvalCheckIn, EvalNegotiation, EvalInvoice,
                              # EvalOverdue, EvalExpansion, EvalCrossSell
    webhook/                  # ReplyHandler, CheckinFormHandler, HandoffHandler
    classifier/reply.go       # ClassifyReply() — 9 intent categories
    template/resolver.go      # ResolveTemplate() — variable substitution + guard
    haloai/client.go          # SendWA()
    telegram/notify.go        # SendMessage(), escalation formatters
    escalation/handler.go     # TriggerEscalation() — atomic escalation
    workflow/                 # Canvas nodes/edges + rule engine
    automation_rule/          # Dynamic rule evaluation
    invoice/                  # Billing: line items, payment logs, sequences
    analytics/                # Dashboards + forecasting
    reports/                  # Report generation
    master_data/              # Master data CRUD + approval workflow (checker-maker)
    custom_field/             # Custom field definitions + validation
    collection/               # Generic user-defined tables
    pipeline_view/            # Sales pipeline visualization
    team/                     # Team member + role management
    workspace/                # Multi-tenancy
    auth/                     # JWT + whitelist
    notification/             # In-app notifications
    payment/                  # Payment verifier
    dashboard/                # Composite usecase (legacy)

  pkg/
    database/                 # pgx postgres, go-redis clients
    logger/                   # zerolog wrapper
    validator/                # input validation helpers
    pagination/, queryparams/ # HTTP helpers
    apperror/                 # typed error definitions
    ctxutil/                  # context utilities
    conditiondsl/             # condition DSL parser (automation rules)
    filterdsl/                # filter DSL (master-data filtering)
    workday/                  # business-day calculations
    jobstore/                 # local file store for background job exports
    xlsxexport/, xlsximport/  # Excel import/export
    htmlsanitize/             # HTML security

  migration/                  # Migration runner (reads /migration/*.sql)
  service/session/            # Session management
  tracer/                     # OpenTelemetry init

migration/                    # 120+ numbered SQL files (<timestamp>_<name>.{up,down}.sql)

docs/
  features/                   # 10 feature planning docs (02-workspace, 03-master-data, ..., 10-collections)
  API_DOCS.md                 # Endpoint reference
  docs.go, swagger.json       # Auto-generated by swag init
  template-import-data-master.xlsx
```

## Dependency Injection

`main.go` constructs a `Deps` struct (in `internal/delivery/http/deps/`) that wires:
1. Logger + tracer
2. pgx + Redis pools
3. All repositories (pgx + squirrel)
4. All usecases (constructor injection)
5. Route handlers referencing usecases via the Deps struct

`SetupHandler(deps)` registers routes.

## Request-Handling Patterns

- Webhooks return HTTP 200 immediately; actual processing happens in a goroutine.
- 300ms sleep between clients in the cron loop — HaloAI/WA Business API rate-limit compliance.
- All usecase methods accept `context.Context` for cancellation + tracing propagation.
- Background jobs (exports/imports) tracked in `background_jobs` table; orphaned `processing` rows are failed on startup.
