# cs-agent-bot — Frontend Integration Guide

Documentation for FE developers integrating against cs-agent-bot BE.
Snapshot 2026-04-24. BE spec coverage: **±97%**.

## Read this first

| File | When |
|---|---|
| [00-GETTING-STARTED.md](00-GETTING-STARTED.md) | Initial setup — run BE locally, auth, headers, Postman |
| [01-auth-and-errors.md](01-auth-and-errors.md) | JWT + workspace header scheme, standardized error codes |
| [02-endpoint-catalog.md](02-endpoint-catalog.md) | Complete ±245 endpoint list by category |
| [03-integration-state.md](03-integration-state.md) | External integrations (Claude, Fireflies, HaloAI, SMTP, Paper.id, Telegram) — what's real vs mock |
| [04-mock-mode.md](04-mock-mode.md) | FE QA via mock endpoints + outbox inspection |
| [05-data-models.md](05-data-models.md) | Key response shapes (envelope, pagination, common entities) |
| [06-feature-status.md](06-feature-status.md) | Per-feature implementation status |
| [07-known-gaps.md](07-known-gaps.md) | FE-spec vs BE alignment — gaps closed in Wave C1-C3 |

## Feature documentation

Per-feature detailed specs live in `features/`. Each feature has at minimum:
- `01-overview.md` — what the feature does, FE-facing
- `02-data-model.md` — entity shapes + JSON tags
- `03-api-endpoints.md` — method + path + payload + response
- `04-progress.md` — implementation status

### Features
- [features/00-shared/](features/00-shared/) — cross-cutting (user_preferences, integrations, approvals, manual actions, PDP, sessions, audit, coaching, rejection analysis, reactivation, mock APIs)
- [features/01-auth/](features/01-auth/) — login, whitelist, session revocation
- [features/02-workspace/](features/02-workspace/) — workspaces + members + theme + holding
- [features/03-master-data/](features/03-master-data/) — clients, import+preview, reactivation, handoff
- [features/04-team/](features/04-team/) — members, roles, permissions, activity
- [features/05-messaging/](features/05-messaging/) — WhatsApp + email templates (existing)
- [features/06-workflow-engine/](features/06-workflow-engine/) — workflows + automation rules + manual actions (existing)
- [features/07-invoices/](features/07-invoices/) — invoices + PDF + Paper.id + aging cron
- [features/08-activity-log/](features/08-activity-log/) — action log, mutations, team activity, unified feed
- [features/09-analytics-reports/](features/09-analytics-reports/) — KPI, reports, per-role bundle, cache
- [features/10-collections/](features/10-collections/) — user-defined tables

## Quick summary

- **Base URL:** `https://localhost:8003` (dev); production URL from ops
- **Route prefix:** `{APP_ROUTE_PREFIX}`, typically `/v1/cs-agent-bot`
- **Auth:** Bearer JWT for dashboard; HMAC for webhooks; OIDC for cron
- **Workspace scope:** `X-Workspace-ID` header required on most endpoints
- **Response envelope:** `{status, entity, state, message, data, meta, error_code, errors}`
- **Pagination:** `?limit=50&offset=0` → response includes `meta: {total, limit, offset}`
- **Mock mode:** Auto-active in dev (`MOCK_EXTERNAL_APIS=true` default)

## Postman

Import `../cs-agent-bot/docs/postman/cs-agent-bot.postman_collection.json`
(228 requests, 28 folders). Environment at the same folder. Variables
pre-filled for `localhost:8003` — override for staging/prod.
