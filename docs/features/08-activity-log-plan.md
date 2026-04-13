# Plan — feat/08-activity-log

> **Branch base**: `master` &nbsp;&nbsp;|&nbsp;&nbsp; **Migration range**: `20260414000700`–`000799` &nbsp;&nbsp;|&nbsp;&nbsp; **Spec dir**: `~/dealls/project-bumi-dashboard/context/for-backend/features/08-activity-log/`

## Scope

Unified audit trail across three log types: bot action logs (workflow execution traces), data mutation logs (Master Data edits), team activity logs (RBAC events). Plus escalation system. Polling endpoints, summary stats, holding view.

**Read first**: `01-overview.md`, `02-database-schema.md`, `03-api-endpoints.md`, `04-escalation.md`, `05-progress.md`.

> **Existing repo has `action_log` (singular)** as the AE/bot audit log — INSERT-only with DB-level `REVOKE UPDATE, DELETE`. **All audit tables this feature creates must follow the same INSERT-only pattern.** CLAUDE.md rule 10 is sacred. Existing dashboard `activity-logs` route + `ResourceType` filter live in `internal/delivery/http/dashboard/activity_handler.go` — REUSE and EXTEND, don't replace.

## Migrations

| # | File | Purpose |
|---|---|---|
| 700 | `extend_action_log_for_actor.{up,down}.sql` | ALTER existing `action_log` (or `action_logs` from feat/03) to add: `actor_type VARCHAR(10) DEFAULT 'bot'`, `actor_name`, `actor_email`, `reply_text`. Add indexes per spec. **Do not break INSERT-only constraint.** |
| 701 | `create_data_mutation_logs.{up,down}.sql` | Per spec §2. **Append-only**: `REVOKE UPDATE, DELETE`. Indexes per spec. |
| 702 | `create_team_activity_logs.{up,down}.sql` | Per spec §3. **Append-only**: `REVOKE UPDATE, DELETE`. Indexes per spec. |
| 703 | `create_unified_activity_view.{up,down}.sql` | `CREATE OR REPLACE VIEW unified_activity_logs AS ...` per spec §4. Read-only. |
| 704 | `extend_escalations_table.{up,down}.sql` | If existing `escalations` table from current bot has different schema, ALTER to add: `severity`, `assigned_to`, `notified_via`, `notified_at`. Add `master_data_id` if missing. Indexes per spec. **Preserve existing dedup logic** — do not allow UPDATE to `status='OPEN'` rows. |

> **Key**: this feature ADDS audit columns to existing tables and CREATES new audit tables. It must NEVER drop or relax existing INSERT-only constraints. Test the constraint after migration: `INSERT` works, `UPDATE` fails, `DELETE` fails.

## Entities

`internal/entity/activity_log.go`:
```go
type DataMutationLog struct {
    ID             uuid.UUID
    WorkspaceID    uuid.UUID
    Action         string  // add_client | edit_client | delete_client | import_bulk | export_bulk
    ActorEmail     string
    ActorName      string
    MasterDataID   *uuid.UUID
    CompanyID      string  // denormalized
    CompanyName    string  // denormalized
    ChangedFields  []string
    PreviousValues map[string]any
    NewValues      map[string]any
    Count          *int    // for bulk
    Note           string
    Timestamp      time.Time
}

type TeamActivityLog struct {
    ID          uuid.UUID
    WorkspaceID uuid.UUID
    Action      string  // invite_member | update_role | update_policy | activate_member | ...
    ActorEmail  string
    ActorName   string
    TargetName  string
    TargetEmail string
    TargetID    *uuid.UUID
    Detail      string
    Timestamp   time.Time
}

type Escalation struct {
    ID             uuid.UUID
    WorkspaceID    uuid.UUID
    MasterDataID   uuid.UUID
    TriggerID      string  // ESC-BD-SCORE4, ESC-AE-OVERDUE15, ...
    Severity       string  // critical | high | medium
    Reason         string
    AssignedTo     *uuid.UUID
    Status         string  // OPEN | ACKNOWLEDGED | RESOLVED | DISMISSED
    ResolvedAt     *time.Time
    ResolvedBy     *uuid.UUID
    ResolutionNote string
    NotifiedVia    string  // telegram | email
    NotifiedAt     *time.Time
    CreatedAt      time.Time
}

type UnifiedLogEntry struct {
    ID          uuid.UUID
    WorkspaceID uuid.UUID
    Category    string  // bot | data | team
    ActorType   string  // bot | human
    Actor       string
    Action      string
    Target      string
    Detail      string
    Status      string
    Timestamp   time.Time
}
```

## Repositories

```
internal/repository/
  data_mutation_log_repo.go    // Append (INSERT-only), List(ws, filter, pag), CountByDay
  team_activity_log_repo.go    // Append, List(ws, filter, pag)
  unified_log_repo.go          // List(ws, filter) querying the view; supports holding
  escalation_repo.go           // Create (with dedup: skip if Open exists), Get, ListOpen, Resolve, ListByCompany
```

## Usecases

`internal/usecase/activity_log/usecase.go`:
- `RecordDataMutation(ctx, req)` — called from feat/03 master_data usecase on every mutating call
- `RecordTeamActivity(ctx, req)` — called from feat/04 team usecase
- `RecordBotAction(ctx, req)` — wraps existing bot action_log writes (extend `actor_type='bot'` automatically)
- `ListUnified(ctx, ws, filter, cursor)` — paginates the view; for holding workspace, expands `member_ids`
- `Stats(ctx, ws)` — 7 stat cards per spec
- `Recent(ctx, ws, since)` — for the polling endpoint
- `CompanySummary(ctx, ws, companyID)` — total_sent, reply_rate, last_sent_at, last_trigger_id, current_phase

`internal/usecase/escalation/handler.go` (extend existing):
- `Create(ctx, req)` — per CLAUDE.md rule 9: dedup on `(esc_id, company_id)` Open rows. Existing handler already does this; preserve.
- `Resolve(ctx, id, note, callerID)` — sets status=RESOLVED. **Triggers re-activation** of `bot_active=TRUE` on the related client (existing behavior).
- `List(ctx, ws, filter)`, `Get(ctx, id)`, `ListByCompany(ctx, companyID)`
- `Notify(ctx, escalation)` — sends Telegram via existing `usecase/telegram`. Tracks `notified_at`. 30-minute fallback to AE Lead (existing logic).

## HTTP routes

```go
al := api.Group("/activity-logs")
al.Handle(GET,    "",                     wsRequired(jwtAuth(activityH.List)))         // existing — extend filters
al.Handle(POST,   "",                     wsRequired(jwtAuth(activityH.Record)))       // existing
al.Handle(GET,    "/recent",              wsRequired(jwtAuth(activityH.Recent)))       // polling
al.Handle(GET,    "/stats",               wsRequired(jwtAuth(activityH.Stats)))
al.Handle(GET,    "/companies/{company_id}/summary", wsRequired(jwtAuth(activityH.CompanySummary)))

esc := api.Group("/escalations")
esc.Handle(GET,    "",                    wsRequired(jwtAuth(escH.List)))              // existing — extend
esc.Handle(GET,    "/{id}",               wsRequired(jwtAuth(escH.Get)))
esc.Handle(PUT,    "/{id}/resolve",       wsRequired(jwtAuth(escH.Resolve)))
api.Handle(GET,    "/clients/{company_id}/escalations", wsRequired(jwtAuth(escH.ListByCompany)))
```

## Tests

- `usecase/activity_log/usecase_test.go` — record dispatch, holding view expansion, stats correctness, polling cursor semantics
- `usecase/escalation/handler_test.go` — dedup (no second Open row for same pair), Resolve triggers bot re-activation, Telegram notify mocked
- Integration test: INSERT into `data_mutation_logs` succeeds; UPDATE/DELETE fail with permission error (proves INSERT-only constraint)
- Cross-feature: feat/03 `Patch` and feat/04 `InviteMember` must call `RecordDataMutation` / `RecordTeamActivity` — verify with stub recorder

## Risks / business-rule conflicts with CLAUDE.md

- **CRITICAL — INSERT-only constraint**: rule 10. All audit tables must `REVOKE UPDATE, DELETE FROM <role>`. Test this in CI with a `_constraint_test.go` that attempts UPDATE/DELETE and expects an error.
- **Escalation dedup**: rule 9. Existing handler preserves this. Resolve test must prove it.
- **`payment_status` writes**: rules 1, 2, 3. The activity log records what happened but never causes those writes itself.
- **Holding view expansion**: when the active workspace is holding, the unified view must `WHERE workspace_id IN (member_ids)`. Add a workspace badge in response per spec.

## File checklist

- [ ] migrations 700–704 (each with constraint tests)
- [ ] entities (data_mutation_log, team_activity_log, escalation, unified_log_entry)
- [ ] repos + mocks (4)
- [ ] usecases: activity_log, escalation (extend)
- [ ] handlers: activity_handler (extend), escalation_handler (extend)
- [ ] route.go updates
- [ ] swag regen
- [ ] `make lint && make unit-test` green; explicit constraint test
- [ ] commit + push `feat/08-activity-log`
