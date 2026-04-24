# Endpoint Catalog

Generated 2026-04-24 from `route.go`. Base path: `{APP_ROUTE_PREFIX}` (typically
`/v1/cs-agent-bot`). All dashboard endpoints require JWT + `X-Workspace-ID`
header unless marked otherwise.

## Auth

| Method | Path | Auth |
|---|---|---|
| POST | `/auth/login` | Public |
| POST | `/auth/google` | Public |
| POST | `/auth/logout` | Public |
| GET | `/whitelist/check` | Public |
| GET/POST | `/whitelist[/{id}]` | JWT |

## Webhooks & Cron (non-JWT)

| Method | Path | Auth |
|---|---|---|
| GET | `/cron/run` | OIDC |
| POST | `/handoff/new-client` | HMAC |
| POST | `/payment/verify` | HMAC |
| POST | `/webhook/wa` | HaloAI sig |
| POST | `/webhook/checkin-form` | Public |
| POST | `/webhook/paperid/{workspace_id}` | HMAC |
| POST | `/webhook/fireflies/{workspace_id}` | HMAC |
| GET | `/cron/invoices/overdue` | OIDC |
| GET | `/cron/invoices/escalate` | OIDC |
| GET | `/cron/analytics/rebuild-snapshots` | OIDC |
| GET | `/cron/pdp/retention` | JWT |
| GET | `/cron/sessions/cleanup` | OIDC |

## Workspaces

| Method | Path |
|---|---|
| GET/POST | `/workspaces` |
| GET/PUT/DELETE | `/workspaces/{id}` |
| POST | `/workspaces/{id}/switch` |
| GET | `/workspaces/{id}/members` |
| POST | `/workspaces/{id}/members/invite` |
| PUT/DELETE | `/workspaces/{id}/members/{member_id}` |
| POST | `/workspaces/invitations/{token}/accept` |
| GET/PUT | `/workspace/theme` |
| GET | `/workspace/holding/expand` |

## Notifications

| Method | Path |
|---|---|
| GET/POST | `/notifications` |
| GET | `/notifications/count` |
| PUT | `/notifications/{id}/read` |
| PUT | `/notifications/read-all` |

## User Preferences

| Method | Path |
|---|---|
| GET | `/preferences` |
| GET/PUT/DELETE | `/preferences/{namespace}` |

## Workspace Integrations

| Method | Path |
|---|---|
| GET | `/integrations` |
| GET/PUT/DELETE | `/integrations/{provider}` |

## Approvals (central dispatcher)

| Method | Path | Supports request_type |
|---|---|---|
| POST | `/approvals/{id}/apply` | create_invoice, mark_invoice_paid, collection_schema_change, delete_client_record, toggle_automation_rule, stage_transition, integration_key_change (bulk_import uses own endpoint) |

## Manual Actions (GUARD)

| Method | Path |
|---|---|
| GET | `/manual-actions` |
| GET | `/manual-actions/{id}` |
| PATCH | `/manual-actions/{id}/mark-sent` |
| PATCH | `/manual-actions/{id}/skip` |

## Activity Logs

| Method | Path |
|---|---|
| GET/POST | `/activity-logs` |
| GET | `/activity-logs/recent` |
| GET | `/activity-logs/stats` |
| GET | `/activity-logs/companies/{company_id}/summary` |
| GET | `/activity-log/feed` |
| GET/POST | `/audit-logs/workspace-access` |

## Background Jobs

| Method | Path |
|---|---|
| GET | `/jobs` |
| GET | `/jobs/{job_id}` |
| GET | `/jobs/{job_id}/download` |

## Data Master (legacy)

| Method | Path |
|---|---|
| GET/POST | `/data-master/clients` |
| GET/PUT/DELETE | `/data-master/clients/{company_id}` |
| POST | `/data-master/clients/import` |
| POST | `/data-master/clients/export` |
| GET | `/data-master/clients/{company_id}/escalations` |
| GET | `/data-master/invoices[/{invoice_id}]` |
| PUT | `/data-master/invoices/{invoice_id}` |
| GET/PUT | `/data-master/escalations[/{id}]` |
| GET | `/data-master/message-templates[/{template_id}]` |
| PUT | `/data-master/message-templates/{template_id}` |
| CRUD | `/data-master/trigger-rules` |
| POST | `/data-master/trigger-rules/cache/invalidate` |
| GET | `/data-master/template-variables` |
| GET/PUT | `/data-master/system-config[/{key}]` |

## Master Data (modern)

| Method | Path |
|---|---|
| GET/POST | `/master-data/clients` |
| GET | `/master-data/clients/export` |
| GET | `/master-data/clients/template` |
| POST | `/master-data/clients/import` |
| POST | `/master-data/clients/import/preview` |
| GET/PUT/DELETE | `/master-data/clients/{id}` |
| POST | `/master-data/clients/{id}/transition` |
| POST | `/master-data/clients/{id}/reactivate` |
| GET | `/master-data/clients/{id}/reactivation-history` |
| POST | `/master-data/query` |
| GET | `/master-data/stats` |
| GET | `/master-data/attention` |
| GET | `/master-data/mutations` |
| CRUD | `/master-data/field-definitions` |

## Invoices

| Method | Path |
|---|---|
| GET/POST | `/invoices` |
| GET | `/invoices/stats` |
| GET | `/invoices/by-stage` |
| GET/PUT/DELETE | `/invoices/{invoice_id}` |
| POST | `/invoices/{invoice_id}/mark-paid` |
| POST | `/invoices/{invoice_id}/send-reminder` |
| GET | `/invoices/{invoice_id}/payment-logs` |
| GET | `/invoices/{invoice_id}/pdf` |

## Team

| Method | Path |
|---|---|
| GET | `/team/members` |
| POST | `/team/members/invite` |
| GET/PUT/DELETE | `/team/members/{id}` |
| PUT | `/team/members/{id}/role` |
| PUT | `/team/members/{id}/status` |
| PUT | `/team/members/{id}/workspaces` |
| POST | `/team/invitations/{token}/accept` |
| GET/POST | `/team/roles` |
| GET/PUT/DELETE | `/team/roles/{id}` |
| PUT | `/team/roles/{id}/permissions` |
| GET | `/team/permissions/me` |
| GET/POST | `/team/activity` |

## Messaging Templates

| Method | Path |
|---|---|
| CRUD | `/templates/messages` |
| CRUD | `/templates/emails` |
| POST | `/templates/preview` |
| GET | `/templates/edit-logs[/{template_id}]` |
| GET | `/templates/variables` |

## Workflows

| Method | Path |
|---|---|
| GET/POST | `/workflows` |
| GET | `/workflows/by-slug/{slug}` |
| GET/PUT/DELETE | `/workflows/{id}` |
| PUT | `/workflows/{id}/canvas` |
| GET/PUT | `/workflows/{id}/steps` |
| GET/PUT | `/workflows/{id}/steps/{stepKey}` |
| GET | `/workflows/{id}/config` |
| PUT | `/workflows/{id}/tabs` |
| PUT | `/workflows/{id}/stats` |
| PUT | `/workflows/{id}/columns` |
| GET | `/workflows/{id}/data` |

## Automation Rules

| Method | Path |
|---|---|
| GET/POST | `/automation-rules` |
| GET | `/automation-rules/change-logs` |
| GET/PUT/DELETE | `/automation-rules/{id}` |

## Analytics

| Method | Path |
|---|---|
| GET | `/dashboard/stats` |
| GET | `/analytics/kpi` |
| GET | `/analytics/kpi/bundle?role=&months=` |
| GET | `/analytics/distributions` |
| GET | `/analytics/engagement` |
| GET | `/analytics/revenue-trend` |
| GET | `/analytics/forecast-accuracy` |
| GET/PUT | `/revenue-targets` |

## Reports

| Method | Path |
|---|---|
| GET | `/reports/executive-summary` |
| GET | `/reports/revenue-contracts` |
| GET | `/reports/client-health` |
| GET | `/reports/engagement-retention` |
| GET | `/reports/workspace-comparison` |
| POST | `/reports/export` |

## Collections

| Method | Path |
|---|---|
| GET/POST | `/collections` |
| GET/PATCH/DELETE | `/collections/{id}` |
| POST | `/collections/{id}/fields` |
| PATCH/DELETE | `/collections/{id}/fields/{field_id}` |
| POST | `/collections/approvals/{approval_id}/approve` |
| GET | `/collections/{id}/records` |
| GET | `/collections/{id}/records/distinct` |
| POST | `/collections/{id}/records` |
| PATCH/DELETE | `/collections/{id}/records/{record_id}` |
| POST | `/collections/{id}/records/bulk` |

## Fireflies

| Method | Path |
|---|---|
| GET | `/fireflies/transcripts` |
| GET | `/fireflies/transcripts/{id}` |

## Reactivation

| Method | Path |
|---|---|
| GET/POST | `/reactivation/triggers` |
| GET/DELETE | `/reactivation/triggers/{id}` |

## Coaching

| Method | Path |
|---|---|
| GET/POST | `/coaching/sessions` |
| GET/PATCH/DELETE | `/coaching/sessions/{id}` |
| POST | `/coaching/sessions/{id}/submit` |

## Rejection Analysis

| Method | Path |
|---|---|
| GET/POST | `/rejection-analysis` |
| POST | `/rejection-analysis/analyze` |
| GET | `/rejection-analysis/stats` |

## PDP Compliance

| Method | Path |
|---|---|
| GET/POST | `/pdp/erasure-requests` |
| GET | `/pdp/erasure-requests/{id}` |
| POST | `/pdp/erasure-requests/{id}/approve` |
| POST | `/pdp/erasure-requests/{id}/reject` |
| POST | `/pdp/erasure-requests/{id}/execute` |
| GET/POST | `/pdp/retention-policies` |
| DELETE | `/pdp/retention-policies/{id}` |

## Sessions

| Method | Path |
|---|---|
| POST | `/sessions/revoke` |
| GET | `/sessions/revoked?user_email=` |

## Mock (FE QA)

| Method | Path |
|---|---|
| GET | `/mock/outbox?provider=&limit=` |
| GET | `/mock/outbox/{id}` |
| DELETE | `/mock/outbox?provider=` |
| POST | `/mock/claude/extract` |
| POST | `/mock/fireflies/fetch` |
| POST | `/mock/haloai/send` |
| POST | `/mock/smtp/send` |

## Health

| Method | Path | Auth |
|---|---|---|
| GET | `/readiness` | Public |

---

**Total: ±245 routes** (up from ~170 at start of 2026-04-23). Complete
Postman v2.1 collection at `docs/postman/cs-agent-bot.postman_collection.json`.
