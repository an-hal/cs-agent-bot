# Plan — feat/06-workflow-engine

> **Branch base**: `master` &nbsp;&nbsp;|&nbsp;&nbsp; **Migration range**: `20260414000500`–`000599` &nbsp;&nbsp;|&nbsp;&nbsp; **Spec dir**: `~/dealls/project-bumi-dashboard/context/for-backend/features/06-workflow-engine/`

## Scope

Visual workflow builder + executable automation rules + pipeline view config (tabs/stats/columns) + cron evaluation engine + audit log. Reuses and extends the existing `TriggerRuleRepo`, `ActionExecutor`, and the strict P0–P5 trigger priority loop in `internal/usecase/cron/runner.go`.

**Read first**: `01-overview.md`, `02-database-schema.md`, `03-golang-models.md`, `04-api-endpoints.md`, `05-cron-engine.md`, `06-progress.md`, `00-shared/01-filter-dsl.md`, `00-shared/05-checker-maker.md`.

> **CRITICAL**: existing repo already has a dynamic rule engine (`USE_DYNAMIC_RULES` flag, `TriggerRuleRepo`, `ActionExecutor`) and the strict P0–P5 trigger priority loop in `internal/usecase/cron/runner.go`. **DO NOT replace it.** Extend by:
> - Adding tables for visual workflow representation (`workflows`, `workflow_nodes`, `workflow_edges`)
> - Adding the `automation_rules` table as a richer superset of the existing `trigger_rules`
> - Adding pipeline view config tables (tabs, stats, columns)
> - Mapping new `automation_rules` rows back to the existing trigger system at runtime
>
> Trigger order P0 → P0.5 → P1 → P2 → P3 → P4 → P5 is **immutable**.

## Migrations

| # | File | Purpose |
|---|---|---|
| 500 | `create_workflows.{up,down}.sql` | Per spec §1. UNIQUE(ws, slug). Indexes per spec. |
| 501 | `create_workflow_nodes.{up,down}.sql` | Per spec §2. UNIQUE(wf, node_id). GIN(data). |
| 502 | `create_workflow_edges.{up,down}.sql` | Per spec §3. UNIQUE(wf, edge_id). |
| 503 | `create_workflow_steps.{up,down}.sql` | Per spec §4. |
| 504 | `create_automation_rules.{up,down}.sql` | Per spec §5. UNIQUE(ws, rule_code). Indexes per spec. **Add a `legacy_trigger_rule_id UUID NULL REFERENCES trigger_rules(id)`** to mirror existing rows. |
| 505 | `create_rule_change_logs.{up,down}.sql` | Per spec §6. INSERT-only; REVOKE UPDATE, DELETE. |
| 506 | `create_pipeline_tabs.{up,down}.sql` | Per spec §7. |
| 507 | `create_pipeline_stats.{up,down}.sql` | Per spec §8. |
| 508 | `create_pipeline_columns.{up,down}.sql` | Per spec §9. |
| 509 | `seed_default_workflows.{up,down}.sql` | Seed 4 default workflows (SDR, BD, AE, CS) per spec §Default Workflows. Use UUIDs from spec table. Insert nodes, edges, steps, tabs, stats, columns. |
| 510 | `mirror_trigger_rules_to_automation_rules.{up,down}.sql` | One-time data backfill: every existing `trigger_rules` row → corresponding `automation_rules` row. Set `legacy_trigger_rule_id`. Forward writes from this branch onward go to BOTH tables until a future cleanup migration drops `trigger_rules`. |

## Entities

`internal/entity/workflow.go`:
```go
type Workflow struct {
    ID          uuid.UUID
    WorkspaceID uuid.UUID
    Name        string
    Icon        string
    Slug        string
    Description string
    Status      string  // active|draft|disabled
    StageFilter []string
    CreatedBy   string
    UpdatedBy   string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type WorkflowNode struct {
    ID         uuid.UUID
    WorkflowID uuid.UUID
    NodeID     string  // e.g. ae-p01
    NodeType   string  // workflow | zone
    PositionX  float64
    PositionY  float64
    Width      *float64
    Height     *float64
    Data       map[string]any  // JSONB
    Draggable  bool
    Selectable bool
    Connectable bool
    ZIndex     int
}

type WorkflowEdge struct {
    ID            uuid.UUID
    WorkflowID    uuid.UUID
    EdgeID        string
    SourceNodeID  string
    TargetNodeID  string
    SourceHandle  string
    TargetHandle  string
    Label         string
    Animated      bool
    Style         map[string]any
}

type WorkflowStep struct { /* per spec §4 */ }

type AutomationRule struct {
    ID                  uuid.UUID
    WorkspaceID         uuid.UUID
    RuleCode            string  // unique per ws
    TriggerID           string
    TemplateID          string
    Role                string  // sdr|bd|ae|cs
    Phase               string
    PhaseLabel          string
    Priority            string
    Timing              string
    Condition           string
    StopIf              string
    SentFlag            string
    Channel             string
    Status              string  // active|paused|disabled
    LegacyTriggerRuleID *uuid.UUID
    UpdatedBy           string
    UpdatedAt           time.Time
    CreatedAt           time.Time
}

type RuleChangeLog struct { /* per spec §6 */ }
type PipelineTab struct { /* per spec §7 */ }
type PipelineStat struct { /* per spec §8 */ }
type PipelineColumn struct { /* per spec §9 */ }
```

## Repositories

```
internal/repository/
  workflow_repo.go               // CRUD + ListByWorkspace
  workflow_node_repo.go          // CRUD scoped to workflow, BulkUpsert (canvas save)
  workflow_edge_repo.go          // BulkUpsert
  workflow_step_repo.go          // CRUD + Reorder
  automation_rule_repo.go        // CRUD with diff edit log; ListByPhase, ListActive
  rule_change_log_repo.go        // Append-only
  pipeline_tab_repo.go, pipeline_stat_repo.go, pipeline_column_repo.go
```

## Usecases

`internal/usecase/workflow/usecase.go`:
- `List`, `Get`, `Create`, `Update`, `Delete` (cascade nodes/edges/steps)
- `SaveCanvas(ctx, wfID, nodes, edges)` — single txn: delete-then-bulk-insert nodes & edges. Emits a notification "Canvas saved" via feat/02 notification usecase.
- `GetForCron(ctx, wsID, stage)` — returns active workflows whose `stage_filter` contains `stage`. Used by cron runner to route records.

`internal/usecase/automation_rule/usecase.go`:
- CRUD with diff-based `rule_change_logs`. `Update` inside txn writes log entries for each changed field.
- **checker-maker**: `Pause/Resume/Disable` → creates `toggle_automation_rule` approval before applying.
- `WriteThrough(ctx, rule)` — also writes to `trigger_rules` (mirror) for legacy compatibility until cleanup migration.
- `Evaluate(ctx, rule, masterData)` — given a rule and a master_data record, parse `condition` against the record fields and return a bool. Reuse existing `condition_parser` if present.

`internal/usecase/cron/workflow_runner.go`:
- **Augments** `internal/usecase/cron/runner.go`. Does NOT replace `processClient()`.
- New flow when `USE_WORKFLOW_ENGINE=true` (separate flag from `USE_DYNAMIC_RULES`):
  1. After existing P0–P5 priority loop completes for a record, check for any **additional** workflow nodes the priority loop didn't cover (custom workspace-defined automations).
  2. For each, evaluate condition, execute action (template render + send via feat/05), write `sent_flag` + `action_logs` row.
- Add a guard: workflow_runner is **gated** behind the priority loop. Existing P0–P5 logic runs first; workflow_runner is purely additive. **Never lets workflow nodes mutate `payment_status`, `renewed`, `rejected`** (CLAUDE.md rule 1+2).

`internal/usecase/pipeline_view/usecase.go`:
- `GetPipelineView(ctx, wfID)` — returns `{tabs, stats, columns, records}`. Records query uses `00-shared/01-filter-dsl.go ParseFilter` per active tab.
- `GetStat(ctx, wfID, statKey)` — uses `ComputeMetric` from filter DSL.

## HTTP routes

```go
wf := api.Group("/workflows")
wf.Handle(GET,    "",                              wsRequired(jwtAuth(wfH.List)))
wf.Handle(GET,    "/{id}",                         wsRequired(jwtAuth(wfH.Get)))
wf.Handle(POST,   "",                              wsRequired(jwtAuth(wfH.Create)))
wf.Handle(PUT,    "/{id}",                         wsRequired(jwtAuth(wfH.Update)))
wf.Handle(DELETE, "/{id}",                         wsRequired(jwtAuth(wfH.Delete)))
wf.Handle(GET,    "/{id}/canvas",                  wsRequired(jwtAuth(wfH.GetCanvas)))
wf.Handle(PUT,    "/{id}/canvas",                  wsRequired(jwtAuth(wfH.SaveCanvas)))
wf.Handle(GET,    "/{id}/steps",                   wsRequired(jwtAuth(stepH.List)))
wf.Handle(PUT,    "/{id}/steps/{step_key}",        wsRequired(jwtAuth(stepH.Update)))
wf.Handle(GET,    "/{id}/pipeline",                wsRequired(jwtAuth(pipelineH.GetView)))
wf.Handle(GET,    "/{id}/tabs",                    wsRequired(jwtAuth(tabH.List)))
wf.Handle(PUT,    "/{id}/tabs",                    wsRequired(jwtAuth(tabH.BulkUpdate)))
wf.Handle(GET,    "/{id}/stats",                   wsRequired(jwtAuth(statH.List)))
wf.Handle(PUT,    "/{id}/stats",                   wsRequired(jwtAuth(statH.BulkUpdate)))
wf.Handle(GET,    "/{id}/columns",                 wsRequired(jwtAuth(colH.List)))
wf.Handle(PUT,    "/{id}/columns",                 wsRequired(jwtAuth(colH.BulkUpdate)))

ar := api.Group("/automation-rules")
ar.Handle(GET,    "",                              wsRequired(jwtAuth(ruleH.List)))
ar.Handle(GET,    "/{id}",                         wsRequired(jwtAuth(ruleH.Get)))
ar.Handle(POST,   "",                              wsRequired(jwtAuth(ruleH.Create)))
ar.Handle(PUT,    "/{id}",                         wsRequired(jwtAuth(ruleH.Update)))
ar.Handle(DELETE, "/{id}",                         wsRequired(jwtAuth(ruleH.Delete)))
ar.Handle(POST,   "/{id}/toggle",                  wsRequired(jwtAuth(ruleH.Toggle)))   // checker-maker
ar.Handle(GET,    "/{id}/history",                 wsRequired(jwtAuth(ruleH.History)))
```

## Tests

- `usecase/workflow/usecase_test.go` — SaveCanvas txn rollback on partial failure, Delete cascades
- `usecase/automation_rule/usecase_test.go` — diff log entries on update, mirror to trigger_rules, approval-gated toggle
- `usecase/cron/workflow_runner_test.go` — runs AFTER priority loop; rejects mutations to payment_status/renewed/rejected; honors stage filter
- `usecase/pipeline_view/usecase_test.go` — filter DSL parsing per tab, metric computation correctness, holding workspace expansion

## Risks / business-rule conflicts with CLAUDE.md

- **Trigger priority is sacred**. Workflow nodes from this engine run AFTER the existing P0–P5 loop, never instead. Add an integration test that proves this.
- **Bot writes blacklist**: workflow_runner must call a shared validator before any DB write: `if writeContext.IsBot && fieldIn(payment_status, renewed, rejected) → reject`.
- **Webhook 200-first**: the cron runner already returns 200; this feature only runs in cron, not in webhook handlers. No conflict.
- **`trigger_rules` cohabitation**: keep both tables until a follow-up cleanup migration. All writes go to both via `WriteThrough`.
- **`USE_DYNAMIC_RULES` vs `USE_WORKFLOW_ENGINE`**: two separate flags. Existing flag controls trigger_rules engine; new flag controls workflow_runner. They can coexist.

## File checklist

- [ ] migrations 500–510
- [ ] entities (9 types)
- [ ] repos + mocks (9)
- [ ] usecases: workflow, automation_rule, pipeline_view, cron/workflow_runner
- [ ] handlers: workflow, step, tab, stat, column, automation_rule, pipeline_view (~7 files)
- [ ] config: `UseWorkflowEngine bool` env flag
- [ ] route.go + deps.go + main.go wiring
- [ ] swag regen
- [ ] `make lint && make unit-test` green; explicit test for "P0 always fires before workflow_runner"
- [ ] commit + push `feat/06-workflow-engine`
