# Feature Implementation Plans (02–09)

These plans are designed for **handoff to fresh Claude Code sessions** so each feature can be implemented in parallel. They are intentionally terse — each plan tells the executing session **what to build, in what order, with what conventions** — but the session itself must **read the full spec** in `~/dealls/project-bumi-dashboard/context/for-backend/features/NN-*/`.

## How to use

For each feature, in a **fresh session** at `/home/anhalim/dealls/cs-agent-bot`:

```
1. Create branch:    git checkout -b feat/NN-<slug> master
2. Read this plan:   docs/features/NN-<slug>-plan.md
3. Read full spec:   ~/dealls/project-bumi-dashboard/context/for-backend/features/NN-<slug>/*.md
4. Read shared:      ~/dealls/project-bumi-dashboard/context/for-backend/features/00-shared/*.md
5. Read CLAUDE.md    (repo root) — business rules override spec on conflict
6. Implement using the file checklist in the plan
7. make lint && make unit-test && make swag
8. git commit (no AI attribution)  &&  git push -u origin feat/NN-<slug>
```

## Migration number ranges (assigned)

| Feature | Range | Slug |
|---|---|---|
| 02 | `20260414000100`–`000199` | `feat/02-workspace` |
| 03 | `20260414000200`–`000299` | `feat/03-master-data` |
| 04 | `20260414000300`–`000399` | `feat/04-team` |
| 05 | `20260414000400`–`000499` | `feat/05-messaging` |
| 06 | `20260414000500`–`000599` | `feat/06-workflow-engine` |
| 07 | `20260414000600`–`000699` | `feat/07-invoices` |
| 08 | `20260414000700`–`000799` | `feat/08-activity-log` |
| 09 | `20260414000800`–`000899` | `feat/09-analytics-reports` |

## Cross-feature dependencies

```
01-auth ──┐
          ├──> 02-workspace ──┐
          │                   ├──> 03-master-data ──┐
          │                   │                     ├──> 05-messaging
          │                   │                     ├──> 06-workflow-engine
          │                   │                     ├──> 07-invoices ──> 08-activity-log
          │                   │                     └──> 09-analytics-reports
          └──> 04-team ───────┘
```

Branches base off `master` and stub cross-branch deps so `go build ./...` passes. Final integration happens via PR merges in dependency order.

## Repo conventions (read once)

- **Language**: Go 1.23+
- **DB**: PostgreSQL via `pgx/v5`; migrations in `migration/` (numbered `up.sql` + `down.sql`)
- **HTTP**: custom `internal/delivery/http/router` + middleware chain; handlers return `error` (mapped via `apperror`)
- **Auth**: JWT via `internal/delivery/http/middleware.JWTAuthMiddleware`, workspace required via `WorkspaceIDMiddleware`
- **Logging**: `zerolog` JSON
- **Tracing**: `tracer.Tracer` interface (OpenTelemetry); every handler/usecase opens a span
- **Errors**: `internal/pkg/apperror` — `BadRequest`, `ValidationError`, `Unauthorized`, `Forbidden`, `Conflict`, `NotFound`, `TooManyRequests`, `InternalError`
- **Response**: `internal/delivery/response.StandardSuccess(w, r, status, message, data)`
- **Layering**: `entity → repository → usecase → delivery/http`. Mocks live in `internal/repository/mocks/`
- **DI**: assemble in `internal/delivery/http/deps/deps.go`, wire in `cmd/server/main.go`, route in `internal/delivery/http/route.go`
- **Workspace multi-tenancy**: every dashboard endpoint requires `workspace_id` header; repository methods take `workspaceID uuid.UUID` as first arg after `ctx`
- **Append-only audit**: `action_log` table is INSERT-only (REVOKE UPDATE/DELETE at DB level)
- **Tests**: standard `go test`, table-driven, mocks over real DB for usecases
- **Background jobs**: long-running ops use `background_jobs` table + `pkg/jobstore` for file outputs
- **Sanitization**: HTML email bodies use `github.com/microcosm-cc/bluemonday` (allowlist in `00-shared/03-html-sanitization.md`)

## CLAUDE.md hard rules (NEVER violate)

1. Bot **never** writes `payment_status`, `renewed`, or `rejected` — AE only via Dashboard
2. `blacklisted` checked before `bot_active`, always first
3. P1 halts on reply; P2 and P3 **never** halt on reply
4. Webhook returns HTTP 200 first (≤5s), then processes async
5. Trigger priority `P0 → P0.5 → P1 → P2 → P3 → P4 → P5` is **strict** — never reorder
6. Template send guard: any unresolved `[variable]` → abort send
7. `quotation_link` must be non-null before REN30
8. `PROMO_DEADLINE` checked before REN45 and CS_H60
9. Escalation dedup: only one Open row per `(esc_id, company_id)`
10. `cs_h7`–`cs_h90` flags are **never** reset on renewal cycle reset
