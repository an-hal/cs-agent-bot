# Session Coverage — 2026-04-23

Snapshot of everything shipped in the A→B→C→Postman→Wave 1-5 run against
the FE spec folder `context/for-backend/`.

## What was delivered

### Tables (9 new migrations)
| # | File | Scope |
|---|---|---|
| 1 | `20260423000001_create_user_preferences` | Per-user UI state (theme, sidebar, columns, feed) — namespace JSONB |
| 2 | `20260423000002_create_workspace_integrations` | Per-workspace credentials for HaloAI/Telegram/Paper.id/SMTP; secrets redacted on read |
| 3 | `20260423000003_create_manual_action_queue` | GUARD 20-flow human composition queue |
| 4 | `20260423000010_create_audit_logs_workspace_access` | Cross-workspace access audit for PDP |
| 5 | `20260423000011_create_fireflies_transcripts` | Fireflies webhook store + extraction status |
| 6 | `20260423000012_create_claude_extractions` | Claude BD extraction results + BANTS scoring |
| 7 | `20260423000013_create_reactivation_triggers` (+ `reactivation_events`) | Dormant-client reactivation |
| 8 | `20260423000014_create_coaching_sessions` | BD coaching peer-review |
| 9 | `20260423000015_create_rejection_analysis_log` | Per-reply rejection classification |

All have matching `.down.sql` files. Indexes follow existing conventions.

### Endpoints (27 new paths added on top of existing ~170)
- `GET/PUT/DELETE /preferences[/{namespace}]` — 4 endpoints
- `GET/PUT/DELETE /integrations[/{provider}]` — 4 endpoints
- `POST /approvals/{id}/apply` — central dispatcher (6 request_types)
- `GET/PATCH /manual-actions…` — 4 endpoints
- `GET/POST /audit-logs/workspace-access` — 2 endpoints
- `POST /webhook/fireflies/{workspace_id}`, `GET /fireflies/transcripts[/{id}]` — 3 endpoints
- `GET/POST/DELETE /reactivation/triggers`, `POST /master-data/clients/{id}/reactivate`, `GET /…/reactivation-history` — 6 endpoints
- `GET/POST/PATCH/POST/DELETE /coaching/sessions…` — 6 endpoints
- `GET/POST /rejection-analysis`, `POST /rejection-analysis/analyze`, `GET /rejection-analysis/stats` — 4 endpoints
- `GET /analytics/kpi/bundle?role=&months=` — 1 endpoint

### Usecases (12 new packages)
- `user_preferences`, `workspace_integration`, `approval`, `manual_action`
- `audit_workspace_access`, `fireflies`, `claude_extraction` (+ `FirefliesBridge`), `reactivation`, `coaching`, `rejection_analysis`
- `claude_client` (noop when `ANTHROPIC_API_KEY` empty)
- `fireflies_client`, `smtp_client` (noop when env empty)

Plus two existing packages extended:
- `automation_rule.ApplyToggleStatus` — executes pending `toggle_automation_rule` approvals
- `escalation.SeverityMatrix` — reads `system_config[ESCALATION_SEVERITY_MATRIX]` with 60-second cache, wildcard-capable (`ESC-id:role` | `ESC-id:*` | `*:role`)
- `template.ResolvePriority` — first-match template chooser (renewal → intent → legacy → default)
- `cron.ManualFlowTriggers` + `ChannelDispatcher.manualEnqueue` — trigger_ids in the 20-flow set enqueue to `manual_action_queue` instead of being bot-sent

### Unit tests (11 new files)
Every new usecase has a stub-based test file. Full suite: **28 packages green**, 0 fails.

### Postman
- `docs/postman/cs-agent-bot.postman_collection.json` — 27 folders, 214 requests
- `docs/postman/cs-agent-bot.postman_environment.json` — `https://localhost:8003`

### Config (7 new env vars)
- `ANTHROPIC_API_KEY`, `CLAUDE_MODEL`, `CLAUDE_TIMEOUT_SECONDS`, `CLAUDE_EXTRACT_PROMPT`, `CLAUDE_BANTS_PROMPT`
- `FIREFLIES_API_KEY`, `FIREFLIES_GRAPHQL_URL`
- `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM_ADDR`, `SMTP_USE_TLS`

## What remains (deferred)

### Needs upstream credentials
| Item | Blocker |
|---|---|
| Real Claude API calls | Needs `ANTHROPIC_API_KEY` + Anthropic Go SDK wired into `claude_client` |
| Real Fireflies GraphQL fetch | Needs `FIREFLIES_API_KEY` + GraphQL client impl |
| Real SMTP delivery | SMTP client has TLS/auth impl; just needs host+creds env |
| HaloAI WA send from cron dispatcher | TODO inside `channelDispatcher.Dispatch` (already logs only) |

### Logic still stubbed/incomplete
| Item | Note |
|---|---|
| `POST /master-data/import/preview` | Dedup preview endpoint — existing import flow uses approval; preview-without-commit path not wired |
| Stage-transition approval gate | Existing `master_data.Transition` applies directly; approval-gated variant would need new endpoint + UI |
| `integration_key_change` approval | Direct PUT on `/integrations/{provider}` applies immediately; no approval gate |
| Manual-action cron timing | `manualFlowPriority` + `manualFlowRole` + `buildManualActionInput` done; `DueAt` defaults to now+24h (caller can override) |
| KPI per-role formula variations | `/analytics/kpi/bundle` accepts `role` but echoes it in meta — formulas still identical across roles |

## Verification

```
$ go build ./...       # exit 0
$ go test ./...        # 28 packages ok
```

## File count delta (this session)

| Layer | Added | Modified |
|---|---|---|
| migrations | 18 files (9 up + 9 down) | — |
| entity | 9 | 0 |
| repository | 9 | 0 |
| usecase | 12 packages + tests | 2 (automation_rule, escalation, template, cron) |
| delivery/http | 8 handlers | 3 (deps, route, analytics_handler) |
| cmd/server | — | 1 (main.go) |
| config | — | 1 (config.go) |
| docs | postman collection + env + this file | — |
