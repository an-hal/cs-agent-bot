# Domain Features

Beyond the core WhatsApp automation loop, the service hosts a full dashboard platform. Feature planning docs live under `docs/features/` (numbered `02-` through `10-`).

## Workspaces & Multi-tenancy (`usecase/workspace/`)
- All dashboard endpoints scoped by `workspace_id` header (enforced by `WorkspaceIDMiddleware`).
- Repositories automatically filter by workspace.
- Invitations flow: invite → email → accept → member.
- Plan doc: `docs/features/02-workspace-plan.md`.

## Master Data (`usecase/master_data/`)
- Client CRUD with custom-field support.
- Checker-maker approval workflow for sensitive changes.
- Excel import (`pkg/xlsximport`) + template at `docs/template-import-data-master.xlsx`.
- Filter DSL (`pkg/filterdsl`) for complex queries.
- Plan doc: `docs/features/03-master-data-plan.md`.

## Teams (`usecase/team/`)
- Roles: Admin, Manager, Agent.
- Workspace-scoped member assignments.
- Plan doc: `docs/features/04-team-plan.md`.

## Messaging / Templates (`usecase/template/`)
- Multi-channel: WhatsApp, Email, Telegram.
- Variable catalog with `[variable_name]` substitution.
- Send-guard: any un-substituted `[...]` aborts the send (Rule 12).
- Template preview + audit.
- Plan doc: `docs/features/05-messaging-plan.md`.

## Workflow Engine (`usecase/workflow/`, `usecase/automation_rule/`)
- Canvas-based UI: nodes, edges, stages.
- Condition DSL (`pkg/conditiondsl`) for rule expressions.
- Dynamic rule engine: when `USE_DYNAMIC_RULES=true`, the cron runner uses `TriggerRuleRepo` + `ActionExecutor` instead of (or alongside) the hardcoded trigger sequence.
- Rules cached in Redis — invalidate via `POST /data-master/trigger-rules/cache/invalidate`.
- Plan docs: `docs/features/06-workflow-engine-plan.md`, `06-workflow-engine-vs-trigger-rules.md`.

## Invoices (`usecase/invoice/`)
- Full billing: header + line items + payment logs + per-workspace sequences.
- Invoice issued at H-30 (contract end − 30 days).
- `due_date = contract_end`.
- Paper.id webhook integration for automatic payment status.
- AE marks paid via dashboard (`POST /invoices/{id}/mark-paid`).
- Payment flags on `clients` table, reset on new cycle.
- Plan doc: `docs/features/07-invoices-plan.md`.

## Activity Log (`usecase/...` — unified audit)
- `activity_log` table: user actions + resource changes.
- Distinct from `action_log` (which is bot-automation INSERT-only).
- Aggregate stats endpoints.
- Plan doc: `docs/features/08-activity-log-plan.md`.

## Analytics & Reports (`usecase/analytics/`, `usecase/reports/`)
- Revenue forecasting, cohort analysis, trend reports.
- Dashboard endpoints under `/analytics/*`.
- Plan doc: `docs/features/09-analytics-reports-plan.md`.

## Collections (`usecase/collection/`)
- User-defined generic tables (schema + records).
- Fields with custom types + validation.
- Checker-maker approval for schema changes.
- Plan doc: `docs/features/10-collections-plan.md`.

## Pipeline View (`usecase/pipeline_view/`)
- Sales-pipeline visualization (Kanban-style stages).

## Notifications (`usecase/notification/`)
- In-app notification records (dashboard alerts).

## Background Jobs (`pkg/jobstore`)
- Long-running exports/imports persisted in `background_jobs` table.
- Files written to `EXPORT_STORAGE_PATH`.
- Clients download via `GET /jobs/{job_id}/download`.
- Orphaned `processing` rows failed on startup.
