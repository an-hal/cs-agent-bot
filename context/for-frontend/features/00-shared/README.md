# Shared Concerns

Cross-cutting capabilities used across all features. Endpoints live in the
top-level namespace (no `/feature-xxx/` prefix).

## Topics

| Shared concern | Endpoints | Doc |
|---|---|---|
| User preferences | `GET/PUT/DELETE /preferences[/{namespace}]` | [01-user-preferences.md](01-user-preferences.md) |
| Workspace integrations | `GET/PUT/DELETE /integrations[/{provider}]` | [02-workspace-integrations.md](02-workspace-integrations.md) |
| Central approvals | `POST /approvals/{id}/apply` | [03-approvals.md](03-approvals.md) |
| Manual actions (GUARD) | `GET/PATCH /manual-actions[/{id}]/...` | [04-manual-actions.md](04-manual-actions.md) |
| Audit workspace access | `GET/POST /audit-logs/workspace-access` | [05-audit.md](05-audit.md) |
| Fireflies transcripts | `POST /webhook/fireflies/{ws}`, `GET /fireflies/transcripts[/{id}]` | [06-fireflies.md](06-fireflies.md) |
| Claude extraction | Runs async via `FirefliesBridge`; view via `/claude-extractions/{id}` | [07-claude-extraction.md](07-claude-extraction.md) |
| Coaching sessions | `GET/POST/PATCH/DEL /coaching/sessions[/{id}]`, `POST .../submit` | [08-coaching.md](08-coaching.md) |
| Rejection analysis | `GET/POST /rejection-analysis`, `POST .../analyze`, `GET .../stats` | [09-rejection-analysis.md](09-rejection-analysis.md) |
| Reactivation triggers | `GET/POST/DEL /reactivation/triggers`, `POST /master-data/clients/{id}/reactivate` | [10-reactivation.md](10-reactivation.md) |
| PDP compliance | `GET/POST /pdp/erasure-requests/...`, `GET/POST /pdp/retention-policies/...` | [11-pdp.md](11-pdp.md) |
| Sessions (revocation) | `POST /sessions/revoke`, `GET /sessions/revoked` | [12-sessions.md](12-sessions.md) |
| Mock external APIs | `/mock/*` | See [../../04-mock-mode.md](../../04-mock-mode.md) |

## Common patterns

### Workspace scoping
Every endpoint here (except approvals apply) accepts the `X-Workspace-ID`
header and scopes responses to that workspace.

### Central approval dispatch
`POST /approvals/{id}/apply` runs the Apply method for 8 approval types
based on `request_type`. FE typically doesn't need to know which feature
owns the type — the dispatcher routes. See
[03-approvals.md](03-approvals.md).

### Redacted secrets
All `workspace_integrations.config` responses mask any key containing
`token|secret|password|api_key|key` with `"***REDACTED***"`. Don't send
the marker back on PUT — omit the key instead. See
[02-workspace-integrations.md](02-workspace-integrations.md).

### Nil-safe hooks
Several usecases accept optional interfaces (Telegram notifier, activity
logger, master-data writer). When nil (i.e. feature not wired yet), the
operation degrades gracefully — FE sees the core write succeed but
downstream effects (notifications, alerts) don't fire. Check
`notifications` for expected alert; fall back to polling if missing.
