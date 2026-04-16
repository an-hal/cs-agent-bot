# Workflow Engine vs Trigger Rules — Migration Guide

## 1. Data Model Comparison

### Old: `trigger_rules`

Single flat table in `internal/repository/trigger_rule_repo.go`:

```sql
trigger_rules (
  id, rule_id (human key), workspace_id,
  trigger_id, template_id, role, phase, phase_label,
  timing, condition, stop_if, sent_flag, channel,
  status, priority, sort_order,
  created_at, updated_at
)
```

Limitations:
- No audit trail of what changed, when, by whom
- No visual representation of how rules connect
- No configurable pipeline view (tabs, stats, columns are hardcoded)
- Single approval pathway via `approval_requests` but no per-field diffing

### New: `workflows + workflow_nodes + workflow_edges + workflow_steps + automation_rules + rule_change_logs`

Six tables providing structured separation of concerns:

```
workflows            ── pipeline identity (name, slug, stage_filter, status)
workflow_nodes       ── React Flow canvas nodes (position, data JSONB, draggable)
workflow_edges       ── React Flow canvas edges (source, target, style, animated)
workflow_steps       ── step-level configs (timing, condition, template refs)
automation_rules     ── executable rules (trigger_id, condition, sent_flag, channel)
rule_change_logs     ── INSERT-only per-field audit trail (field, old_value, new_value, editor)
```

Plus pipeline config tables for the Pipeline View surface:

```
pipeline_tabs        ── filter DSL per tab
pipeline_stats       ── metric DSL per stat card
pipeline_columns     ── visible columns per workflow
```

Key improvements:
- `rule_change_logs` is INSERT-only at the DB level (`REVOKE UPDATE, DELETE ON rule_change_logs FROM PUBLIC`)
- `workflow_nodes.data` JSONB enables React Flow round-trip without schema migrations
- `automation_rules` cross-references `workflow_nodes` via `triggerId` in the node data
- `pipeline_configs` (tabs/stats/columns) are fully configurable per workflow

---

## 2. Capabilities Comparison

| Capability                          | trigger_rules                          | Workflow Engine                              |
|-------------------------------------|----------------------------------------|----------------------------------------------|
| Rule CRUD                           | Yes                                    | Yes (`/automation-rules`)                    |
| Visual canvas editing               | No                                     | Yes (React Flow via `/workflows/{id}/canvas`)|
| Pipeline step configuration         | No                                     | Yes (`/workflows/{id}/steps`)                |
| Configurable pipeline view tabs     | No (hardcoded)                         | Yes (`/workflows/{id}/tabs`)                 |
| Configurable stat cards             | No (hardcoded)                         | Yes (`/workflows/{id}/stats`)                |
| Configurable table columns          | No (hardcoded)                         | Yes (`/workflows/{id}/columns`)              |
| Per-field change audit              | No                                     | Yes (`rule_change_logs`, INSERT-only)        |
| Checker-maker for status toggle     | Partial (approval_requests only)       | Full (active↔paused gated + approval row)   |
| Stage routing                       | Manual (by role field)                 | Declarative (`stage_filter` array)           |
| Multi-workspace aggregated view     | No                                     | `?holding=true` param (planned)             |
| Redis cache invalidation            | Yes (`/trigger-rules/cache/invalidate`)| Not applicable (DB queries directly)        |

---

## 3. Wiring: Flags and Integration Points

### `USE_DYNAMIC_RULES` (old flag — trigger_rules)

Set in `config/config.go` as `UseDynamicRules bool`.

When true, `internal/usecase/trigger/rule_engine.go` (`RuleEngine`) loads
`trigger_rules` from the DB (cached in Redis) and evaluates them during
`processClient()` before the hardcoded P0–P5 sequence.

Entry point: `internal/delivery/http/dashboard/trigger_rule_handler.go`

### `USE_WORKFLOW_ENGINE` (new flag — workflow engine)

Set in `config/config.go` as `UseWorkflowEngine bool`.

When true, `internal/usecase/cron/workflow_runner.go` (`WorkflowRunner`) is
constructed in `cmd/server/main.go` and attached to the cron runner. It runs
**after** `processClient()` completes — it is additive and never replaces P0–P5.

Entry points:
- `internal/delivery/http/dashboard/workflow_handler.go`
- `internal/delivery/http/dashboard/automation_rule_handler.go`
- `internal/delivery/http/dashboard/pipeline_view_handler.go`

Both flags can be set independently. The system currently supports running
both in parallel (dual-run phase).

---

## 4. Cron Integration

### Legacy: `rule_engine.go`

```
processClient(client)
  → if USE_DYNAMIC_RULES: RuleEngine.Evaluate(client) [evaluates trigger_rules from Redis/DB]
  → P0 blacklist/bot_active/daily-gate checks
  → EvalHealthRisk → EvalCheckIn → EvalNegotiation → EvalInvoice → EvalOverdue → EvalExpansion → EvalCrossSell
```

`RuleEngine.Evaluate` runs inside `processClient` before P0 gates are applied.

### New: `workflow_runner.go`

```
processClient(client) [unchanged — P0–P5 always runs]
  → after processClient returns:
  → if USE_WORKFLOW_ENGINE: WorkflowRunner.RunForRecord(client)
    → GetActiveForStage(client.Stage)
    → GetActiveByRole(role derived from workflow slug)
    → for each active rule: evaluate (condition eval deferred to cron-engine milestone)
    → 300ms delay between rules (WA rate limit compliance)
```

Guarantees enforced in `workflow_runner.go`:
- Returns immediately if `useWorkflow = false`
- Never writes `payment_status`, `renewed`, or `rejected`
- Respects context cancellation
- All four tests in `workflow_runner_test.go` assert these invariants

---

## 5. Staged Removal Plan for `trigger_rules`

### Phase 1 — Dual-Run (current state)

Both `USE_DYNAMIC_RULES` and `USE_WORKFLOW_ENGINE` can run simultaneously.
`automation_rules` are seeded via migration `000608` (backfill from `trigger_rules`).

Validation: compare `action_log` entries to confirm parity of triggered
messages between the two systems.

### Phase 2 — Read-Shadow

- All new rule writes go to `automation_rules` only.
- `trigger_rules` writes are mirrored: any PUT/POST to `/data-master/trigger-rules`
  also writes an equivalent row to `automation_rules`.
- Cron reads from `automation_rules`; Redis cache for `trigger_rules` is kept
  warm for fallback but not the primary source.

### Phase 3 — Deprecate trigger_rule_handler Endpoints

Return `410 Gone` from:
- `GET /data-master/trigger-rules`
- `GET /data-master/trigger-rules/{rule_id}`
- `POST /data-master/trigger-rules`
- `PUT /data-master/trigger-rules/{rule_id}`
- `DELETE /data-master/trigger-rules/{rule_id}`
- `POST /data-master/trigger-rules/cache/invalidate`

The frontend must migrate all reads/writes to `/automation-rules` before this
phase begins.

### Phase 4 — Drop USE_DYNAMIC_RULES

Remove from `config/config.go`:
```go
// REMOVE:
UseDynamicRules bool
```

Delete:
- `internal/usecase/trigger/rule_engine.go`
- Redis cache keys `trigger_rules:*`
- All references to `RuleEngine` in `cmd/server/main.go` and `deps.go`
- `dataMaster.Handle(... "/trigger-rules" ...)` routes in `route.go`
- `internal/delivery/http/dashboard/trigger_rule_handler.go`

### Phase 5 — Final Migration: Drop trigger_rules Table

Create migration `000611_drop_trigger_rules.up.sql`:

```sql
-- Remove the backfill FK (added in 000608)
ALTER TABLE automation_rules DROP COLUMN IF EXISTS legacy_trigger_rule_id;

-- Drop the old table
DROP TABLE IF EXISTS trigger_rules;
```

Down migration restores the table structure from `000500` (the original
trigger_rules migration).

### Phase 6 — Rollback Plan per Phase

| Phase | Rollback                                                                        |
|-------|---------------------------------------------------------------------------------|
| 1     | Set `USE_WORKFLOW_ENGINE=false` — no code changes needed                        |
| 2     | Set `USE_WORKFLOW_ENGINE=false`; mirror writes are already in both tables        |
| 3     | Restore 200 responses in `trigger_rule_handler.go` from git history             |
| 4     | Cherry-pick the `rule_engine.go` deletion commit and revert it; restore config  |
| 5     | Run `000611_drop_trigger_rules.down.sql`; restore `legacy_trigger_rule_id` FK   |

For phases 4 and 5, a full rollback window of 30 days of `action_log` audit
data should be preserved to verify correct rule evaluation before proceeding.

---

## 6. Key File Reference

| Concern                   | Old                                                      | New                                                                 |
|---------------------------|----------------------------------------------------------|---------------------------------------------------------------------|
| Repository                | `internal/repository/trigger_rule_repo.go`              | `internal/repository/workflow_repo.go`, `automation_rule_repo.go`  |
| Usecase                   | `internal/usecase/trigger/rule_engine.go`               | `internal/usecase/workflow/usecase.go`, `automation_rule/usecase.go`|
| Cron integration          | Called inside `processClient()` via `RuleEngine`         | `internal/usecase/cron/workflow_runner.go` (additive, post-P0–P5)  |
| HTTP handlers             | `internal/delivery/http/dashboard/trigger_rule_handler.go`| `workflow_handler.go`, `automation_rule_handler.go`, `pipeline_view_handler.go`|
| Config flag               | `USE_DYNAMIC_RULES` → `UseDynamicRules bool`             | `USE_WORKFLOW_ENGINE` → `UseWorkflowEngine bool`                    |
| DB migrations             | `000500_create_trigger_rules.up.sql`                     | `000600`–`000608` (workflows, nodes, edges, steps, rules, logs, config, seed, backfill)|
| Audit trail               | None                                                     | `rule_change_logs` (INSERT-only, REVOKE at DB level)               |
| Pipeline view config      | Hardcoded in frontend                                    | `pipeline_tabs`, `pipeline_stats`, `pipeline_columns` per workflow  |
| Filter/metric DSL         | N/A                                                      | `internal/pkg/filterdsl` (13 filter prefixes, 4 metric types, fully tested)|
