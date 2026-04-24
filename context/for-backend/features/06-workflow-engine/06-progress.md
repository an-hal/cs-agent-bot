# Workflow Engine — Implementation Progress

## 2026-04-23 — BD engine + cron + edge cases sync

### FE (shipped)
- **Daily Task Validation Dashboard** — Calendar + Kanban views for task review/approval
- **BD Coaching Tracker** — surfaces BD performance metrics + coaching action history
- **BD Escalations matrix** — visual matrix of active escalations by severity × division
- **BD Setup Status checklist** — onboarding completeness gate per BD
- Playbook pages: GUARD Flows, Edge Cases, Objection Handling, Coaching Rubric, Decision Log
- Reference pages: BD Functions reference, BD Schema viewer
- Dashboards: Handoff SLA dashboard
- Widgets: AntiStall, PromoCountdown, IntakeRate, WorkingDay calendar, BDCalendarCapacity

### Backend spec (documented, implementation pending)
- 4 new cron jobs added to `05-cron-engine.md`:
  - **BD blast** — D0/D3/D7/D14/D21 outbound cadence
  - **AE escalation SLA** — fire when handoff SLA breaches
  - **SDR nurture recycling** — rotate dormant leads back into nurture
  - **Intake rate limiting** — cap new lead assignment per BD per working day
- New tables in `02-database-schema.md`: `bd_escalations`, `branching_state` JSONB column on master_data, `edge_case_log`
- 9 core Go function signatures added to `03-golang-models.md`: `processProspect`, `evaluateBlast`, `routeToCS`, `evaluateBranching`, `applyEdgeCase`, `computeIntakeQuota`, `escalateBD`, `recycleNurture`, `evaluateHandoffSLA`
- 32 BD edge cases catalog added to `04-api-endpoints.md` (or new section) — each with trigger condition + handler ref
- New endpoints: `POST /workflows/escalations`, `PUT /workflows/escalations/{id}/resolve`, `GET /workflows/branching/{client_id}`, `POST /workflows/cs-routing`

### Open dependencies (backend)
- Implement all 4 crons (BD blast / AE SLA / SDR recycling / intake limiting)
- Implement branching engine — reads `branching_state` JSONB, evaluates next-step DSL, writes back
- Implement all 9 core functions in Go
- Implement edge case handlers — 32 cases, each maps trigger → action
- Wire `bd_escalations` table to Telegram alert path (shared with 05-messaging D7/D10)

### Cross-refs
- FE: `app/dashboard/[workspace]/workflow/daily-tasks/page.tsx`, `app/dashboard/[workspace]/workflow/bd-*/page.tsx`, `components/widgets/*`
- Edge cases catalog drives `edge_case_log` writes — every fired case logged
- Handoff SLA also surfaces in 09-analytics-reports (Health Score input)
- Backend gap doc: `claude/for-backend/06-workflow-engine/gap-bd-engine.md`, `gap-edge-cases-catalog.md`, `gap-cron-jobs.md`

---

> **Overall: 32% complete** (22/69 items done or partial)
> - Frontend: 75% done (18 done + 4 partial)
> - Backend (Go): 0% done (38 items not started)
> - Optional improvements: 0% done (9 items)

---

## DONE — Frontend ✅ (18 items)

| # | Item | File | Notes |
|---|------|------|-------|
| 1 | WorkflowItem data model | `contexts/WorkflowContext.tsx` | Full type: key, name, icon, steps[], tabs[], stats[], columns[] |
| 2 | PipelineStep/Tab/Stat/Column types | `contexts/WorkflowContext.tsx` | All match spec schema: filter DSL, metric DSL, column config |
| 3 | 4 default workflows (seed data) | `contexts/WorkflowContext.tsx` | SDR Lead Outreach, BD Deal Closing, AE Client Lifecycle, CS Customer Support |
| 4 | Per-pipeline column presets | `contexts/WorkflowContext.tsx` | SDR_COLUMNS, BD_COLUMNS, AE_COLUMNS with correct field mappings |
| 5 | Pipeline steps per workflow | `contexts/WorkflowContext.tsx` | AE: P0-P6 with timing/condition/templateId. SDR/BD/CS: steps defined |
| 6 | Pipeline tabs per workflow | `contexts/WorkflowContext.tsx` | Tab filter DSL: 'all', 'bot_active', 'risk', 'stage:X', 'expiry:30', etc. |
| 7 | Pipeline stats per workflow | `contexts/WorkflowContext.tsx` | Metric DSL: 'count', 'count:bot_active', 'sum:Final_Price', 'avg:Days_to_Expiry' |
| 8 | localStorage persistence | `contexts/WorkflowContext.tsx` | Save/load from localStorage with migration for old format |
| 9 | Workflow list page | `app/dashboard/[workspace]/workflow/page.tsx` | Table of all workflows with node/edge counts, status, updated_at |
| 10 | Workflow Builder (React Flow) | `components/features/WorkflowBuilder.tsx` | Visual canvas with drag, zoom, connect. Supports 'workflow' + 'zone' node types |
| 11 | Workflow Builder Wrapper | `components/features/WorkflowBuilderWrapper.tsx` | Dynamic import wrapper for SSR safety |
| 12 | SDR pipeline nodes + edges | `app/dashboard/[workspace]/workflow/page.tsx` | ~25 nodes: P1-P4, email outreach, WA blast, nurture, seasonal, escalation |
| 13 | BD pipeline nodes + edges | Same file | ~13 nodes: blast D0-D21, first payment, nurture, escalation |
| 14 | AE pipeline nodes + edges | Same file | ~25 nodes: P0-P6, onboarding through overdue, referral |
| 15 | CS pipeline nodes + edges | Same file | ~8 nodes: ticket intake, resolution, SLA, escalation |
| 16 | Pipeline view page | `app/dashboard/[workspace]/pipeline/[slug]/page.tsx` | Tabs, stat cards, filterable data table, stage routing per slug |
| 17 | Filter engine (pipeline-utils) | `lib/pipeline-utils.ts` | `applyFilter()` implements full filter DSL matching spec exactly |
| 18 | Metric computation engine | `lib/pipeline-utils.ts` | `computeMetric()` implements count, count:X, sum:X, avg:X DSL |

## PARTIAL ⚠️ (4 items)

| # | Item | What's Done | What's Missing |
|---|------|-------------|----------------|
| 19 | Workflow CRUD | Add/remove/update via context (client-side) | No API calls — all localStorage. No slug generation, no status management |
| 20 | Canvas save | Nodes + edges defined in hardcoded arrays per workflow | No PUT `/workflows/{id}/canvas` call — changes are in-memory only |
| 21 | Tab/stat/column editing | Pipeline page has edit modals for tabs, stats, columns | Edits are client-side only (updateWorkflow in context), no API persistence |
| 22 | Node data with templateId/triggerId | All nodes reference templateId and triggerId in data field | References are hardcoded strings, not validated against actual templates/rules |

## NOT DONE — Backend (Go) Required 🔴 (38 items)

### Critical (blocks real data)

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 23 | `workflows` table | 02-database-schema | id, workspace_id, name, icon, slug, status, stage_filter[], created/updated |
| 24 | `workflow_nodes` table | 02-database-schema | node_id, node_type, position, dimensions, data JSONB, display flags |
| 25 | `workflow_edges` table | 02-database-schema | edge_id, source/target node_id, handles, label, style JSONB |
| 26 | `workflow_steps` table | 02-database-schema | step_key, label, phase, timing, condition, template refs |
| 27 | `pipeline_tabs` table | 02-database-schema | tab_key, label, icon, filter DSL string |
| 28 | `pipeline_stats` table | 02-database-schema | stat_key, label, metric DSL string, color/border |
| 29 | `pipeline_columns` table | 02-database-schema | column_key, field, label, width, visible |
| 30 | GET `/workflows` | 04-api-endpoints | List workflows for workspace with optional status filter |
| 31 | GET `/workflows/{id}` | 04-api-endpoints | Get full workflow with nodes, edges, steps, tabs, stats, columns |
| 32 | GET `/workflows/by-slug/{slug}` | 04-api-endpoints | Get workflow by slug (used by frontend route) |

### High Priority

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 33 | POST `/workflows` | 04-api-endpoints | Create workflow with auto-slug generation |
| 34 | PUT `/workflows/{id}` | 04-api-endpoints | Update workflow metadata (name, icon, status, stage_filter) |
| 35 | DELETE `/workflows/{id}` | 04-api-endpoints | Delete with CASCADE on all nested entities |
| 36 | PUT `/workflows/{id}/canvas` | 04-api-endpoints | Bulk replace nodes + edges in a transaction |
| 37 | GET `/workflows/{id}/steps` | 04-api-endpoints | List pipeline steps |
| 38 | PUT `/workflows/{id}/steps` | 04-api-endpoints | Bulk save pipeline steps |
| 39 | PUT `/workflows/{id}/steps/{stepKey}` | 04-api-endpoints | Update single step (Step Config page) |
| 40 | GET `/workflows/{id}/config` | 04-api-endpoints | Get pipeline config (tabs + stats + columns) |
| 41 | PUT `/workflows/{id}/tabs` | 04-api-endpoints | Bulk save tabs |
| 42 | PUT `/workflows/{id}/stats` | 04-api-endpoints | Bulk save stat cards |
| 43 | PUT `/workflows/{id}/columns` | 04-api-endpoints | Bulk save column config |

### Medium Priority — Automation Rules

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 44 | `automation_rules` table | 02-database-schema | rule_code, trigger_id, template_id, role, phase, timing, condition, status |
| 45 | `rule_change_logs` table | 02-database-schema | Audit trail for rule edits |
| 46 | GET `/automation-rules` | 04-api-endpoints | List rules with filters (role, status, phase, search) |
| 47 | GET `/automation-rules/{id}` | 04-api-endpoints | Get rule with change logs |
| 48 | POST `/automation-rules` | 04-api-endpoints | Create rule |
| 49 | PUT `/automation-rules/{id}` | 04-api-endpoints | Update rule + auto-log changes |
| 50 | DELETE `/automation-rules/{id}` | 04-api-endpoints | Delete rule and change logs |
| 51 | GET `/automation-rules/change-logs` | 04-api-endpoints | Change log feed for all rules |

### Medium Priority — Pipeline Data

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 52 | GET `/workflows/{id}/data` | 04-api-endpoints | Master data filtered by stage_filter + tab filter + search + pagination + stats |
| 53 | Filter DSL parser (Go) | 02-database-schema | ParseFilter() converting DSL strings to SQL WHERE clauses |
| 54 | Metric DSL computation (Go) | 02-database-schema | ComputeMetric() for count, count:X, sum:X, avg:X server-side |

### Medium Priority — Cron Engine

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 55 | POST `/cron/evaluate` | 05-cron-engine | Main cron endpoint: evaluate all records against workflow nodes |
| 56 | processClient flow | 05-cron-engine | Gate checks, stage routing, node evaluation, action execution |
| 57 | Working day + holiday check | 03-golang-models | IsWorkingDay(), WorkingDaysSince() helpers |
| 58 | Stage transition logic | 05-cron-engine | LEAD->PROSPECT, PROSPECT->CLIENT, dormant recycling |
| 59 | Action execution + logging | 05-cron-engine | Execute matched actions, write sent_flags, log to action_logs |

### Low Priority

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 60 | GET `/workflows/node-specs` | 04-api-endpoints | Return pre-populated node specs from Excel data |
| 61 | GET `/workflows/node-specs/find` | 04-api-endpoints | Find node spec by trigger_id or template_id |
| 62 | Holding view for workflows | 04-api-endpoints | Aggregated workflows from all member workspaces |
| 63 | Holding view for rules | 04-api-endpoints | Aggregated rules from all member workspaces |
| 64 | Seed data: 4 default workflows | 01-overview | SDR, BD, AE, CS workflows with nodes, edges, steps, tabs, stats, columns |

### Checker-Maker Approval

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 74 | Checker-maker for POST `/master-data/clients/{id}/transition` | 04-api-endpoints | Require approval (type: `stage_transition`) for manual stage changes |
| 75 | Checker-maker for PUT `/automation-rules/{id}` (status toggle) | 04-api-endpoints | Require approval (type: `toggle_automation`) when changing rule status active↔paused |

## NOT DONE — Optional Frontend Improvements 🟡 (9 items)

| # | Item | Priority | Description |
|---|------|----------|-------------|
| 65 | Step Config page | High | `/pipeline/{slug}/{step}` — per-step timing/condition/template editing (page not yet created) |
| 66 | Automation Rules page | High | `/automation` — list/edit/pause/enable rules (page not yet created) |
| 67 | Connect workflow list to GET API | High | Replace localStorage with fetch from `/workflows` |
| 68 | Connect canvas save to PUT API | High | Call PUT `/workflows/{id}/canvas` on save button click |
| 69 | Connect pipeline data to GET API | High | Replace mock data with fetch from `/workflows/{id}/data` |
| 70 | Connect tab/stat/column config to PUT API | Medium | Persist config edits via API instead of localStorage |
| 71 | Node spec panel in builder | Medium | Show spec data from `/workflows/node-specs/find` when selecting a node |
| 72 | Rule change log viewer | Low | Display rule edit history from `/automation-rules/change-logs` |
| 73 | Workflow status toggle (active/draft/disabled) | Low | UI to change workflow status via PUT `/workflows/{id}` |

---

## Recommended Implementation Order (Backend)

```
Week 1: #23 workflows + #24 workflow_nodes + #25 workflow_edges
         #30 GET /workflows + #31 GET /workflows/{id} + #32 GET by slug
Week 2: #33 POST + #34 PUT + #35 DELETE workflows
         #36 PUT /canvas (bulk save nodes + edges)
Week 3: #26 workflow_steps + #27 pipeline_tabs + #28 pipeline_stats + #29 pipeline_columns
         #37-43 steps + config CRUD endpoints
Week 4: #44-45 automation_rules + rule_change_logs tables
         #46-51 rules CRUD endpoints
Week 5: #52 GET /workflows/{id}/data + #53 filter DSL parser + #54 metric DSL
Week 6: #55-59 cron engine (evaluate, processClient, stage transitions)
Later:  #57 working day helpers + #60-64 node specs, holding view, seed data
```

## Dependency Chain

```
workflows ──→ workflow_nodes ──→ PUT /canvas
  │           workflow_edges ──┘
  │
  ├──→ workflow_steps ──→ GET/PUT /steps
  ├──→ pipeline_tabs  ──→ PUT /tabs ──→ filter DSL parser
  ├──→ pipeline_stats ──→ PUT /stats ──→ metric DSL computation
  └──→ pipeline_columns ──→ PUT /columns
                                │
automation_rules ──→ rule_change_logs
  │                       │
  └──→ CRUD endpoints     └──→ change log feed
                                │
master_data + workflows ──→ GET /workflows/{id}/data
                                │
                                └──→ cron engine (evaluate + processClient)
                                        │
                                        └──→ action_logs + sent_flag writes
```
