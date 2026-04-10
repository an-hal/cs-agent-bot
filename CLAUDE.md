# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

---

## What This Service Does

`cs-agent-bot` is a WhatsApp automation bot for Kantorku.id HRIS SaaS. It acts as a virtual Account Executive, managing client lifecycle through automated WhatsApp outreach via HaloAI across six sequential phases (P0–P5). **The bot never marks a client as Paid, Renewed, or Churned — only the AE does that via the Dashboard.**

---

## Development Commands

```bash
# Install tools (swag, mockery, air, golangci-lint)
make install

# Generate Swagger + build + run
make dev

# Hot reload (Air)
make watch

# Build binary only
make build

# Run all tests
make test

# Run unit tests only (internal/usecase)
make unit-test

# Run with coverage report
make test-coverage

# Run single test
go test ./internal/usecase/... -run TestFunctionName -v

# Lint
make lint

# Lint with auto-fix
make lint-fix

# Format
make fmt

# Migrations
make migrate-up
make migrate-down
make migrate-create name=create_something

# Regenerate Swagger docs
make swag

# Docker
make docker-up
make docker-rebuild
```

Required environment: copy `.env.example` to `.env`. The server performs startup validation on missing required env vars (`HALOAI_API_URL`, `WA_API_TOKEN`, `WA_WEBHOOK_SECRET`, `TELEGRAM_BOT_TOKEN`, `TELEGRAM_AE_LEAD_ID`, `HANDOFF_WEBHOOK_SECRET`). `APP_URL` and `SCHEDULER_SA_EMAIL` are required in non-dev environments only.

---

## Architecture

### Entry Points (only two)

**Outbound — GCP Cloud Scheduler (09:00 WIB):**
```
GET /cron/run  [OIDC-authenticated]
  → processClient() per active client
    → PostgreSQL (read state) → HaloAI Send API → PostgreSQL (write flags + audit)
      → Telegram API (if escalation)
```

**Inbound — HaloAI Webhook:**
```
POST /webhook/wa  [HaloAI HMAC signature]
  → HTTP 200 immediately (must be ≤5s)
    → goroutine: classify intent → apply rule → update DB
      → HaloAI Send API (reply if needed) → Telegram API (AE notification)
```

### Package Layout

```
cmd/
  server/main.go        # Entry point: wires all deps, starts HTTP server
  migrate/main.go       # Migration CLI
config/config.go        # Typed config from env; fails fast on missing keys
internal/
  delivery/http/
    route.go            # All route registrations
    deps/               # DI container (Deps struct)
    middleware/         # OIDC (cron), HaloAI sig (webhook), JWT (dashboard), HMAC (handoff)
    dashboard/          # REST handlers for AE dashboard
    webhook/            # wa_handler, checkin_handler, handoff_handler
    cron/               # cron handler
    payment/            # verify_handler
  entity/               # Client, Invoice, ClientFlags, ConversationState, etc.
  repository/           # DB access layer; mocks/ for tests
  usecase/
    cron/runner.go      # processClient() — P0→P5 priority loop
    trigger/            # EvalHealthRisk, EvalCheckIn, EvalNegotiation, EvalInvoice, EvalOverdue, EvalExpansion, EvalCrossSell
    webhook/            # ReplyHandler, CheckinFormHandler, HandoffHandler
    classifier/reply.go # ClassifyReply() — 9 intent categories
    template/resolver.go# ResolveTemplate() — substitution + guard
    haloai/client.go    # SendWA()
    telegram/notify.go  # SendMessage(), escalation formatters
    escalation/handler.go # TriggerEscalation()
    dashboard/          # DashboardUsecase (client CRUD, jobs, exports)
    payment/            # PaymentVerifier
  pkg/
    database/           # pgx/v5 postgres, go-redis/v9
    logger/             # zerolog structured JSON
    jobstore/           # Local file store for background job exports
    xlsximport/         # Excel import helpers
    xlsxexport/         # Excel export helpers
migration/              # Numbered SQL files (up/down)
```

### Trigger Priority Loop — `usecase/cron/runner.go`

```go
// Gate 1: Blacklist — ALWAYS FIRST, before everything else.
if c.Blacklisted { return nil }
// Gate 2: Bot suspended.
if !c.BotActive { return nil }
// Gate 3: Max 1 WA per client per calendar day.
if db.SentTodayAlready(ctx, companyID) { return nil }

// Strict priority — first match fires, returns immediately.
EvalHealthRisk → EvalCheckIn → EvalNegotiation → EvalInvoice → EvalOverdue → EvalExpansion → EvalCrossSell
```

**Never reorder these.** 300ms sleep between clients for WA Business API rate-limit compliance.

### Auth by Route

| Route | Auth |
|---|---|
| `GET /cron/run` | OIDC JWT (GCP Cloud Scheduler SA) |
| `POST /webhook/wa` | HaloAI HMAC signature |
| `POST /handoff/new-client` | HMAC |
| `POST /payment/verify` | HMAC |
| Dashboard (`/data-master/*`, `/jobs`, `/activity-logs`) | JWT (via `JWT_VALIDATE_URL`) + `workspace_id` header |

### Background Jobs

Export/import operations run as background jobs stored in the `background_jobs` table. Results are written to local disk (`EXPORT_STORAGE_PATH`). Clients download via `GET /jobs/{job_id}/download`. Orphaned `processing` jobs are failed on startup.

### Dynamic Rule Engine

When `USE_DYNAMIC_RULES=true`, the cron runner uses `TriggerRuleRepo` + `ActionExecutor` instead of (or alongside) the hardcoded trigger sequence. Rules are cached in Redis; invalidate via `POST /data-master/trigger-rules/cache/invalidate`.

---

## Critical Business Rules

| # | Rule |
|---|---|
| 1 | `blacklisted` checked before `bot_active`, always first. Never reverse this order. |
| 2 | P1 halts on reply. P2 and P3 **never** halt on reply — payment collection continues regardless. |
| 3 | Bot never writes `payment_status`, `renewed`, or `rejected`. AE only via Dashboard. |
| 4 | Invoice issued at H-30. `due_date = contract_end`. Payment flags (`pre14_sent`, `post*_sent`) are on `clients`, not `client_flags`. Reset on new invoice cycle. |
| 5 | Check-in branch: `contract_months >= 9` → Branch A, `< 9` → Branch B. |
| 6 | `CheckinReplied=TRUE` skips REN60 and REN45 entirely; client goes directly to REN30 at H-30. |
| 7 | `resetCycleFlags()` on `Renewed=TRUE`. `cs_h7`–`cs_h90` flags are **never** reset (90-day sequence is one-time). |
| 8 | `quotation_link` must be non-null before REN30. If null: defer + alert AE. |
| 9 | Check `PROMO_DEADLINE` before REN45 and CS_H60. If expired: skip + alert AE Lead. |
| 10 | `action_log` is INSERT only. `REVOKE UPDATE, DELETE` is granted at DB level. |
| 11 | Return HTTP 200 to HaloAI webhook **before** any processing. All logic runs in a goroutine. |
| 12 | Template send guard: any remaining `[variable]` in the resolved message → abort send. |

---

## Trigger Priority Reference

| Priority | Phase | Halts on Reply? |
|---|---|---|
| P0 | Health & Risk | No |
| P0.5 | Check-in | Yes (`CheckinReplied=TRUE`) |
| P1 | Renewal Negotiation | Yes (any meaningful reply) |
| P2 | Invoice + Payment | **Never** |
| P3 | Overdue Recovery | **Never** |
| P4 | NPS + Referral | Yes (`NPSReplied=TRUE`) |
| P5 | Cross-sell ATS | Yes (rejected/interested) |

---

## Reply Intent Classification — `usecase/classifier/reply.go`

Nine categories, first match wins: `angry`, `paid_claim`, `nps`, `cs_interested`, `wants_human`, `reject`, `delay`, `positive`, `ooo`.

Voice notes, images, and videos are always `wants_human`.

When `SequenceCS == "ACTIVE"` or `"LONGTERM"`, replies are routed to `handleCSReply()` before general classification.

---

## Escalation — `usecase/escalation/handler.go`

Every escalation atomically: sets `BotActive=FALSE`, appends to `action_log`, sends Telegram, sets `escalations.status = Open`. **Deduplication:** if an Open row for the same `esc_id + company_id` exists, only send a Telegram reminder (no new row). Re-activation only when AE sets `status = Resolved` in Dashboard. 30-minute fallback re-sends to `TELEGRAM_AE_LEAD_ID`.

| ESC ID | Trigger |
|---|---|
| ESC-001 | Overdue >= D+15 |
| ESC-002 | Objection/complaint reply |
| ESC-003 | NPSScore <= 5 |
| ESC-004 | REN0 sent, no reply (Mid/High) |
| ESC-005 | High-value churn (ACV > threshold) |
| ESC-006 | Angry reply |

---

## Database Tables

Eight tables in deployment order: `clients`, `invoices`, `client_flags`, `conversation_states`, `escalations`, `action_log`, `cron_log`, `system_config`. Plus: `workspaces`, `workspace_users`, `templates`, `background_jobs`, `trigger_rules` (added later migrations).

`system_config` keys required at runtime: `PROMO_DEADLINE`, `PROMO_DISCOUNT_PCT`, `SURVEY_PLATFORM_URL`, `CHECKIN_FORM_URL`, `REFERRAL_BENEFIT`, `LINK_WA_AE`, `TELEGRAM_AE_LEAD_ID`.

---

## Workspace Multi-tenancy

All dashboard endpoints require a `workspace_id` header (enforced by `WorkspaceIDMiddleware`). Repository calls are scoped by workspace. The cron runner fetches active clients per workspace.
