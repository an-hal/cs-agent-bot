# Database & Migrations

## Engine

PostgreSQL 13+ accessed via `pgx/v5` with prepared statements. Query construction via `Masterminds/squirrel` (type-safe builder).

## Migration Strategy

- Directory: `migration/` (at repo root)
- Naming: `<timestamp>_<name>.{up,down}.sql` (e.g. `20260420000100_add_workflow_stages.up.sql`)
- Runner: `cmd/migrate/main.go` — CLI supporting `up`, `down`, `create <name>`
- Migration loader implementation: `internal/migration/`
- Total migrations: ~120+ files (earliest `20250330…`, latest `20260420…`)

```bash
make migrate-up
make migrate-down
make migrate-create name=add_something
```

## Core Tables

### Automation / Engagement
- `clients` — Company state + 40+ columns (contract dates, flags, metrics, engagement phase)
- `client_flags` — 34 boolean flags (per-phase progress, reply status)
- `action_log` — **INSERT-only** audit trail (`REVOKE UPDATE, DELETE` enforced at DB level)
- `conversation_states` — per-client conversation tracking
- `escalations` — support tickets (`esc_id`, `company_id`, `status`, Telegram link)
- `cron_log` — scheduled-run history
- `system_config` — key-value store (runtime config)

### Billing
- `invoices` — header (one per billing cycle)
- `invoice_line_items` — line-level detail
- `payment_logs` — every payment event (Paper.id webhook + manual marks)
- `invoice_sequences` — per-workspace invoice number sequences

### Workflow / Rule Engine
- `workflows`, `workflow_nodes`, `workflow_edges`, `workflow_steps` — canvas metadata
- `automation_rules` — rule definitions (condition DSL)
- `trigger_rules` — dynamic trigger configurations (used when `USE_DYNAMIC_RULES=true`)

### Messaging
- `templates`, `message_templates`, `email_templates` — multi-channel template store
- Variable catalog for `[variable_name]` substitution

### Multi-tenancy
- `workspaces`, `workspace_members`, `workspace_users`, `workspace_invitations`

### Generic / Platform
- `collections`, `collection_fields`, `collection_records` — user-defined generic tables (checker-maker approval)
- `custom_fields` — per-entity custom fields
- `activity_log` — unified user-action feed
- `background_jobs` — async export/import tracking (orphaned `processing` rows failed on startup)
- `notifications` — in-app notification records
- `teams`, `team_members` — team roles + assignments

## Required `system_config` Keys

- `PROMO_DEADLINE`
- `PROMO_DISCOUNT_PCT`
- `SURVEY_PLATFORM_URL`
- `CHECKIN_FORM_URL`
- `REFERRAL_BENEFIT`
- `LINK_WA_AE`
- `TELEGRAM_AE_LEAD_ID`

## Payment-Flag Locations (Important)

Payment sequence flags (`pre14_sent`, `pre7_sent`, `pre3_sent`, `post1_sent`, `post4_sent`, `post8_sent`) live on the **`clients`** table — NOT `client_flags`. They reset on each new invoice cycle.

## Cycle-Flag Reset

- `resetCycleFlags()` runs when `Renewed = TRUE`.
- `cs_h7` through `cs_h90` flags are **never** reset (the 90-day cross-sell sequence is one-time).
