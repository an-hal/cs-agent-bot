# Plan — feat/03-master-data

> **Branch base**: `master` &nbsp;&nbsp;|&nbsp;&nbsp; **Migration range**: `20260414000200`–`000299` &nbsp;&nbsp;|&nbsp;&nbsp; **Spec dir**: `~/dealls/project-bumi-dashboard/context/for-backend/features/03-master-data/`

## Scope

Hybrid-schema CRM client records: fixed core columns + JSONB `custom_fields`, per-workspace `custom_field_definitions`, CRUD, stats, attention tab, bulk import/export, stage transition, flexible query for workflow engine, mutation log.

**Read first**: `01-overview.md`, `02-database-schema.md`, `03-golang-models.md`, `04-api-endpoints.md`, `05-workflow-integration.md`, `06-seed-data.md`, `00-shared/01-filter-dsl.md`, `00-shared/05-checker-maker.md`.

> **Existing repo has `clients` table** (different name from spec's `master_data`). The current bot operates on `clients` for the P0–P5 cron logic. **Do not rename the existing table.** Instead, add a SQL view `master_data` that selects from `clients` (column-aliased) so the spec's API can run unchanged. All NEW columns added by this feature go on `clients`. Document this in the migration files.

## Migrations

| # | File | Purpose |
|---|---|---|
| 200 | `extend_clients_for_master_data.{up,down}.sql` | ALTER `clients` to add: `pic_nickname`, `pic_role`, `owner_telegram_id`, `risk_flag` (default 'None'), `payment_terms`, `final_price BIGINT`, `last_payment_date`, `notes`, `custom_fields JSONB DEFAULT '{}'`. Add indexes per spec. Add `update_updated_at()` function + trigger if not present. |
| 201 | `create_custom_field_definitions.{up,down}.sql` | Per spec § 2 |
| 202 | `create_master_data_view.{up,down}.sql` | `CREATE VIEW master_data AS SELECT id, workspace_id, company_id, company_name, ...alias columns...` from `clients`. Read-only — writes go through usecases. |
| 203 | `create_action_logs.{up,down}.sql` | NEW table per spec § 3. **NOT the same as existing `action_log` (singular)** which is the audit log for AE actions. This `action_logs` (plural) is for workflow node execution traces. Indexes: workspace, master_data, trigger, (workspace,timestamp DESC) |
| 204 | `create_master_data_mutations.{up,down}.sql` | `(id, workspace_id, master_data_id, action, actor_email, changed_fields TEXT[], previous_values JSONB, new_values JSONB, timestamp)`. Indexes: (workspace, timestamp DESC), master_data |
| 205 | `seed_default_custom_fields.{up,down}.sql` | Seed per `06-seed-data.md` (hc_size, nps_score, plan_type, industry, etc. for `dealls` workspace) |

> **Critical**: the existing `clients` table has columns the bot relies on (`pre14_sent`, `post*_sent`, `cs_h7`–`cs_h90`, `bot_active`, `blacklisted`, etc.). Do **not** drop or rename any. Only add. The view is purely additive.

## Entities

`internal/entity/master_data.go`:
```go
type MasterData struct {
    ID                  uuid.UUID
    WorkspaceID         uuid.UUID
    CompanyID           string  // unique per workspace
    CompanyName         string
    Stage               string  // LEAD | PROSPECT | CLIENT | DORMANT
    PICName             string
    PICNickname         string
    PICRole             string
    PICWA               string
    PICEmail            string
    OwnerName           string
    OwnerWA             string
    OwnerTelegramID     string
    BotActive           bool
    Blacklisted         bool
    SequenceStatus      string
    SnoozeUntil         *time.Time
    SnoozeReason        string
    RiskFlag            string  // High | Mid | Low | None
    ContractStart       *time.Time
    ContractEnd         *time.Time
    ContractMonths      int
    DaysToExpiry        int
    PaymentStatus       string  // Paid | Pending | Overdue | Menunggu
    PaymentTerms        string
    FinalPrice          int64
    LastPaymentDate     *time.Time
    Renewed             bool
    LastInteractionDate *time.Time
    Notes               string
    CustomFields        map[string]any
    CreatedAt           time.Time
    UpdatedAt           time.Time
}

type CustomFieldDefinition struct {
    ID              uuid.UUID
    WorkspaceID     uuid.UUID
    FieldKey        string  // snake_case, immutable
    FieldLabel      string
    FieldType       string  // text|number|date|boolean|select|url|email
    IsRequired      bool
    DefaultValue    string
    Placeholder     string
    Description     string
    Options         []string  // for select
    MinValue        *float64
    MaxValue        *float64
    RegexPattern    string
    SortOrder       int
    VisibleInTable  bool
    ColumnWidth     int
}

type MasterDataMutation struct { /* per spec §7 */ }
```

## Repositories

```
internal/repository/
  master_data_repo.go        // List(ws, filter, pag), Get(id), Create, Patch (JSONB merge), Delete, Stats, Attention, Query (flexible conditions), Transition (atomic stage + updates)
  custom_field_def_repo.go   // List, Get, Create (unique field_key), Update (immutable field_key), Delete, Reorder (txn)
  master_data_mutation_repo.go  // Append, List(ws, since, limit)
```

Mocks under `internal/repository/mocks/`.

## Usecases

`internal/usecase/master_data/usecase.go`:
- `List(ctx, ws, filter)` — uses `00-shared/01-filter-dsl.go` `ParseFilter`
- `Get`, `Create` (validates against `custom_field_definitions`: required, type, regex, min/max, select options), `Patch` (JSONB deep-merge, records mutation log with diff), `Delete` (**checker-maker**: creates `approval_request` with `request_type=delete_client_record`, returns `202 Accepted` + approval_id; actual delete on approval)
- `Stats`, `Attention`, `Query` (workflow engine flexible-condition translator: whitelist op set `=, !=, >, >=, <, <=, in, between, contains`; sanitize JSONB key paths)
- `Transition(ctx, id, req)` — txn: snapshot old, update stage + JSONB updates, append `action_log` (workflow trace) + `master_data_mutation` row, return diff

`internal/usecase/master_data/import.go`:
- Background job (uses existing `pkg/jobstore` + `background_jobs` table from feat/09 if present, otherwise stub)
- Parses xlsx via `pkg/xlsximport`
- **checker-maker**: creates approval before execute. Stores file in `EXPORT_STORAGE_PATH/imports/{job_id}.xlsx`. Approval payload: `{file_ref, file_name, mode, row_count, preview}`
- On approval, processes rows, returns counts + errors

`internal/usecase/master_data/export.go`:
- Background job → xlsx via `pkg/xlsxexport`
- Header: core columns + sorted custom fields with `[Custom]` prefix
- Writes to `EXPORT_STORAGE_PATH/exports/{job_id}.xlsx`

`internal/usecase/master_data/template.go`:
- Builds 2-sheet xlsx: Template (header + 1 example), Reference (allowed values from custom defs select fields)

`internal/usecase/custom_field/usecase.go`:
- CRUD + Reorder (single txn updating sort_order)

## HTTP routes

```go
md := api.Group("/master-data")
md.Handle(GET,    "/clients",                     wsRequired(jwtAuth(mdH.List)))
md.Handle(GET,    "/clients/{id}",                wsRequired(jwtAuth(mdH.Get)))
md.Handle(POST,   "/clients",                     wsRequired(jwtAuth(mdH.Create)))
md.Handle(PUT,    "/clients/{id}",                wsRequired(jwtAuth(mdH.Patch)))
md.Handle(DELETE, "/clients/{id}",                wsRequired(jwtAuth(mdH.Delete)))     // creates approval
md.Handle(POST,   "/clients/{id}/transition",     wsRequired(jwtAuth(mdH.Transition)))
md.Handle(POST,   "/query",                       wsRequired(jwtAuth(mdH.Query)))
md.Handle(GET,    "/stats",                       wsRequired(jwtAuth(mdH.Stats)))
md.Handle(GET,    "/attention",                   wsRequired(jwtAuth(mdH.Attention)))
md.Handle(POST,   "/clients/import",              wsRequired(jwtAuth(mdH.Import)))     // creates approval
md.Handle(GET,    "/clients/export",              wsRequired(jwtAuth(mdH.Export)))     // bg job
md.Handle(GET,    "/clients/template",            wsRequired(jwtAuth(mdH.Template)))
md.Handle(GET,    "/mutations",                   wsRequired(jwtAuth(mdH.Mutations)))

md.Handle(GET,    "/field-definitions",           wsRequired(jwtAuth(cfH.List)))
md.Handle(POST,   "/field-definitions",           wsRequired(jwtAuth(cfH.Create)))
md.Handle(PUT,    "/field-definitions/{id}",      wsRequired(jwtAuth(cfH.Update)))
md.Handle(DELETE, "/field-definitions/{id}",      wsRequired(jwtAuth(cfH.Delete)))
md.Handle(PUT,    "/field-definitions/reorder",   wsRequired(jwtAuth(cfH.Reorder)))
```

> **Existing** `dataMaster := api.Group("/data-master")` lives in route.go for legacy dashboard CRUD on `clients`. Keep it. Add a new `/master-data` group alongside. They cohabit.

## Tests

- `usecase/master_data/usecase_test.go`: filter parse cases, JSONB merge depth, transition txn rollback, Query op whitelist (reject SQL injection), holding query expansion
- `usecase/master_data/import_test.go`: row-level error reporting, mode semantics, approval gate
- `usecase/master_data/export_test.go`: header order, custom field columns, holding workspace tags `workspace_name`
- `usecase/custom_field/usecase_test.go`: immutable field_key, unique violation, reorder txn

## Risks / business-rule conflicts with CLAUDE.md

- **Bot writes**: spec says workflow engine writes `Stage`, `Sequence_Status`, `Bot_Active`. CLAUDE.md says bot **never** writes `payment_status`, `renewed`, `rejected`. Translation: the `Patch` and `Transition` usecases must reject any caller-supplied write to those fields **unless caller is a Dashboard JWT user (not the cron/webhook actor)**. Add a `WriteContext` enum: `dashboard_user` (allowed) vs `bot_actor` (denied for those fields).
- **Soft delete vs approval delete**: spec uses checker-maker approval for delete, but "delete" is a hard delete from `clients` (cascades). Existing dashboard `Delete` is a hard delete too. Implement: approval row → on approval, execute hard delete + cascade. Document irreversibility.
- **`master_data` view + writes**: views are read-only; writes go through the usecase to `clients` directly. Make sure all repo write methods target `clients`, not the view.
- **Mutation log vs action_log**: do not conflate. `master_data_mutations` records DASHBOARD edits (who changed what). `action_log` (existing, singular) records BOT/AE actions for audit. `action_logs` (plural, this feature) records workflow node executions. Three distinct tables.

## File checklist

- [ ] migrations 200–205
- [ ] entities (master_data, custom_field_definition, mutation)
- [ ] repos + mocks (3)
- [ ] usecases: master_data (CRUD/stats/query/transition), import (bg+approval), export (bg), template, custom_field, mutation logger
- [ ] handlers: 1 master_data_handler.go (~15 endpoints) + 1 custom_field_handler.go
- [ ] route.go updates
- [ ] deps.go + main.go wiring
- [ ] swag regen
- [ ] `make lint && make unit-test` green
- [ ] commit + push `feat/03-master-data`
