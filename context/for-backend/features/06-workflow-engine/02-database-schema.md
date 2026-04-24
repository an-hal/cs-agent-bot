# Database Schema — Workflow Engine

## Tabel Overview

| Table                | Purpose                                              |
|----------------------|------------------------------------------------------|
| `workflows`          | Top-level workflow (pipeline) definition              |
| `workflow_nodes`     | Canvas nodes (React Flow) per workflow                |
| `workflow_edges`     | Canvas edges connecting nodes                        |
| `workflow_steps`     | Pipeline step configs (timing, condition, templates) |
| `automation_rules`   | Executable rules evaluated by cron/events            |
| `rule_change_logs`   | Audit trail for automation rule edits                |
| `pipeline_tabs`      | Tab filter configs per workflow                      |
| `pipeline_stats`     | Stat card configs per workflow                       |
| `pipeline_columns`   | Table column configs per workflow                    |
| `bd_escalations`     | BD escalation queue (ESC-BD-001..009) — see §10      |
| `edge_case_log`      | Audit log for the 32 BD edge cases — see §12         |

> Relasi ke tabel lain: `workspaces(id)`, `master_data(id)`, `action_logs(id)` — lihat `master-data/02-database-schema.md`.

---

## 1. `workflows` — Top-level workflow/pipeline

```sql
CREATE TABLE workflows (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    UUID NOT NULL REFERENCES workspaces(id),

  -- Identity
  name            VARCHAR(255) NOT NULL,      -- e.g. 'SDR Lead Outreach'
  icon            VARCHAR(10),                -- emoji, e.g. emoji phone
  slug            VARCHAR(100),               -- URL-safe identifier (auto-generated from name or manual)
  description     TEXT,

  -- State
  status          VARCHAR(20) NOT NULL DEFAULT 'active',
                  -- Allowed: active, draft, disabled

  -- Stage filter — which master_data records this workflow operates on
  stage_filter    VARCHAR(50)[] NOT NULL DEFAULT '{}',
                  -- e.g. {'LEAD','DORMANT'} for SDR, {'CLIENT'} for AE
                  -- Cron uses this to route records to the correct workflow

  -- Meta
  created_by      VARCHAR(255),
  updated_by      VARCHAR(255),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE(workspace_id, slug)
);

CREATE INDEX idx_wf_workspace ON workflows(workspace_id);
CREATE INDEX idx_wf_status    ON workflows(workspace_id, status);
```

---

## 2. `workflow_nodes` — Canvas nodes (React Flow)

Stores the visual representation of each node on the canvas.

```sql
CREATE TABLE workflow_nodes (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workflow_id     UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,

  -- React Flow identity
  node_id         VARCHAR(100) NOT NULL,       -- e.g. 'ae-p01', 'sdr-wa-h3'
  node_type       VARCHAR(20) NOT NULL,         -- 'workflow' | 'zone'
                  -- 'zone' nodes are visual groupings (non-executable)

  -- Position (React Flow coordinates)
  position_x      FLOAT NOT NULL DEFAULT 0,
  position_y      FLOAT NOT NULL DEFAULT 0,

  -- Dimensions (zone nodes only)
  width           FLOAT,
  height          FLOAT,

  -- Node data (JSONB — all node-specific fields)
  -- For workflow nodes:
  --   category: 'trigger' | 'condition' | 'action' | 'delay'
  --   label: display name
  --   icon: emoji
  --   description: node description
  --   templateId: reference to message_templates
  --   triggerId: reference to automation_rules.trigger_id
  --   timing, condition, stopIf, sentFlag: inline overrides
  -- For zone nodes:
  --   label, color, bg
  data            JSONB NOT NULL DEFAULT '{}',

  -- Display flags
  draggable       BOOLEAN DEFAULT TRUE,
  selectable      BOOLEAN DEFAULT TRUE,
  connectable     BOOLEAN DEFAULT TRUE,
  z_index         INT DEFAULT 0,

  -- Meta
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE(workflow_id, node_id)
);

CREATE INDEX idx_wn_workflow ON workflow_nodes(workflow_id);
CREATE INDEX idx_wn_data_trigger ON workflow_nodes USING GIN(data);
```

---

## 3. `workflow_edges` — Canvas edges

```sql
CREATE TABLE workflow_edges (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workflow_id     UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,

  -- React Flow identity
  edge_id         VARCHAR(100) NOT NULL,       -- e.g. 'sdr-e01', 'ae-e03'

  -- Connection
  source_node_id  VARCHAR(100) NOT NULL,        -- references workflow_nodes.node_id
  target_node_id  VARCHAR(100) NOT NULL,
  source_handle   VARCHAR(50),                 -- 'bottom', 'top', 'left', 'right' (null = default)
  target_handle   VARCHAR(50),

  -- Display
  label           VARCHAR(255),                -- e.g. "READ Stage='LEAD'", "No reply -> NURTURE"
  animated        BOOLEAN DEFAULT FALSE,

  -- Style (JSONB for flexibility)
  -- { "stroke": "#2563eb", "strokeWidth": 2, "strokeDasharray": "6,3" }
  style           JSONB,

  -- Meta
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE(workflow_id, edge_id)
);

CREATE INDEX idx_we_workflow ON workflow_edges(workflow_id);
CREATE INDEX idx_we_source   ON workflow_edges(workflow_id, source_node_id);
CREATE INDEX idx_we_target   ON workflow_edges(workflow_id, target_node_id);
```

---

## 4. `workflow_steps` — Pipeline step configuration

Pipeline steps map to workflow nodes but carry the **execution configuration** used by
the Step Config page (timing, condition, template assignment).

```sql
CREATE TABLE workflow_steps (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workflow_id       UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,

  -- Identity
  step_key          VARCHAR(50) NOT NULL,       -- e.g. 'p0', 'blast', 'escalation'
  label             VARCHAR(255) NOT NULL,
  phase             VARCHAR(20) NOT NULL,       -- e.g. 'P0', 'SDR', 'BD', 'ESC'
  icon              VARCHAR(10),
  description       TEXT,
  sort_order        INT DEFAULT 0,

  -- Automation config (inline — may override automation_rules)
  timing            TEXT,                       -- e.g. 'D+0 to D+5', 'H-120 to H-115'
  condition         TEXT,                       -- SQL-like condition expression
  stop_if           TEXT,                       -- stop condition expression
  sent_flag         VARCHAR(100),               -- custom_field key to mark as sent
  template_id       VARCHAR(100),               -- template reference (e.g. 'TPL-OB-WELCOME')

  -- Template library references (optional — for Step Config page)
  message_template_id VARCHAR(100),             -- WA/Telegram template from library
  email_template_id   VARCHAR(100),             -- email template from library

  -- Meta
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE(workflow_id, step_key)
);

CREATE INDEX idx_ws_workflow ON workflow_steps(workflow_id);
CREATE INDEX idx_ws_phase    ON workflow_steps(workflow_id, phase);
```

---

## 5. `automation_rules` — Executable rules

Central rule library. Each rule maps a trigger condition to an action.
Rules are workspace-scoped and can be shared across multiple workflow nodes.

```sql
CREATE TABLE automation_rules (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    UUID NOT NULL REFERENCES workspaces(id),

  -- Identity
  rule_code       VARCHAR(100) NOT NULL,        -- human-readable e.g. 'RULE-KK-AE-OB-001'
  trigger_id      VARCHAR(100) NOT NULL,        -- e.g. 'Onboarding_Welcome', 'BD_D0'
  template_id     VARCHAR(100),                 -- message template ref

  -- Classification
  role            VARCHAR(10) NOT NULL,          -- 'sdr' | 'bd' | 'ae' | 'cs'
  phase           VARCHAR(20) NOT NULL,          -- 'P0', 'P1', ..., 'P6', 'SDR', 'BD', 'BD-FP', 'BD-NUR', 'ESC', 'CS'
  phase_label     VARCHAR(100),                 -- display label e.g. 'Onboarding', 'Renewal Negotiation'
  priority        VARCHAR(20),                  -- same as phase (used for ordering in UI)

  -- Execution config
  timing          TEXT NOT NULL,                -- e.g. 'D+0 to D+5', 'H-90 to H-85', 'Immediate'
  condition       TEXT NOT NULL,                -- SQL-like condition string
  stop_if         TEXT DEFAULT '-',             -- stop condition
  sent_flag       VARCHAR(200),                 -- flag(s) to set after execution
  channel         VARCHAR(20) DEFAULT 'whatsapp',
                  -- Allowed: whatsapp, email, telegram

  -- State
  status          VARCHAR(20) NOT NULL DEFAULT 'active',
                  -- Allowed: active, paused, disabled

  -- Meta
  updated_at      TIMESTAMPTZ,
  updated_by      VARCHAR(255),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE(workspace_id, rule_code)
);

CREATE INDEX idx_ar_workspace   ON automation_rules(workspace_id);
CREATE INDEX idx_ar_status      ON automation_rules(workspace_id, status);
CREATE INDEX idx_ar_role        ON automation_rules(workspace_id, role);
CREATE INDEX idx_ar_trigger     ON automation_rules(trigger_id);
CREATE INDEX idx_ar_phase       ON automation_rules(workspace_id, phase);
```

---

## 6. `rule_change_logs` — Audit trail for rule edits

```sql
CREATE TABLE rule_change_logs (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  rule_id         UUID NOT NULL REFERENCES automation_rules(id) ON DELETE CASCADE,
  workspace_id    UUID NOT NULL REFERENCES workspaces(id),

  field           VARCHAR(50) NOT NULL,         -- e.g. 'timing', 'condition', 'status', 'created'
  old_value       TEXT,
  new_value       TEXT,

  edited_by       VARCHAR(255) NOT NULL,
  edited_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_rcl_rule       ON rule_change_logs(rule_id);
CREATE INDEX idx_rcl_workspace  ON rule_change_logs(workspace_id);
CREATE INDEX idx_rcl_edited_at  ON rule_change_logs(workspace_id, edited_at DESC);
```

---

## 7. `pipeline_tabs` — Tab filter configs per workflow

```sql
CREATE TABLE pipeline_tabs (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workflow_id     UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,

  tab_key         VARCHAR(50) NOT NULL,         -- e.g. 'semua', 'contacted', 'qualified'
  label           VARCHAR(100) NOT NULL,
  icon            VARCHAR(10),                  -- emoji
  filter          VARCHAR(100) DEFAULT 'all',
                  -- Filter DSL:
                  --   'all' — no filter
                  --   'bot_active' — Bot_Active = TRUE
                  --   'risk' — Risk_Flag IN (High, Mid) OR !Bot_Active OR Payment = Terlambat
                  --   'stage:LEAD' — Stage = LEAD
                  --   'stage:DORMANT' — Stage = DORMANT
                  --   'value_tier:High,Mid' — Value_Tier IN (High, Mid)
                  --   'sequence:active' — sequence_status = ACTIVE
                  --   'payment:Menunggu' — Payment_Status = Menunggu
                  --   'expiry:30' — Days_to_Expiry <= 30
  sort_order      INT DEFAULT 0,

  UNIQUE(workflow_id, tab_key)
);

CREATE INDEX idx_pt_workflow ON pipeline_tabs(workflow_id);
```

---

## 8. `pipeline_stats` — Stat card configs per workflow

```sql
CREATE TABLE pipeline_stats (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workflow_id     UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,

  stat_key        VARCHAR(50) NOT NULL,         -- e.g. 'total', 'revenue', 'risk'
  label           VARCHAR(100) NOT NULL,        -- display label e.g. 'Total Client'
  metric          VARCHAR(100) NOT NULL,
                  -- Metric DSL:
                  --   'count' — total records
                  --   'count:bot_active' — count where Bot_Active = TRUE
                  --   'count:risk' — count risk conditions
                  --   'count:stage:LEAD' — count by stage
                  --   'count:value_tier:High,Mid' — count by value tier
                  --   'count:payment:Menunggu' — count by payment status
                  --   'count:expiry:30' — count contracts expiring in 30d
                  --   'sum:Final_Price' — sum of Final_Price
                  --   'avg:Days_to_Expiry' — average Days_to_Expiry
  color           VARCHAR(50),                  -- tailwind text class e.g. 'text-brand-400'
  border          VARCHAR(50),                  -- tailwind border class e.g. 'border-brand-400/20'
  sort_order      INT DEFAULT 0,

  UNIQUE(workflow_id, stat_key)
);

CREATE INDEX idx_ps_workflow ON pipeline_stats(workflow_id);
```

---

## 9. `pipeline_columns` — Table column configs per workflow

```sql
CREATE TABLE pipeline_columns (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workflow_id     UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,

  column_key      VARCHAR(50) NOT NULL,         -- e.g. 'Company_Name', 'Stage', 'Bot_Active'
  field           VARCHAR(100) NOT NULL,         -- master_data field name or custom_fields key
  label           VARCHAR(100) NOT NULL,         -- display header
  width           INT DEFAULT 120,              -- column width in pixels
  visible         BOOLEAN DEFAULT TRUE,
  sort_order      INT DEFAULT 0,

  UNIQUE(workflow_id, column_key)
);

CREATE INDEX idx_pc_workflow ON pipeline_columns(workflow_id);
```

---

## Entity Relationships

```
workspaces ─┬─► workflows ─┬─► workflow_nodes
             │               ├─► workflow_edges
             │               ├─► workflow_steps
             │               ├─► pipeline_tabs
             │               ├─► pipeline_stats
             │               └─► pipeline_columns
             │
             ├─► automation_rules ──► rule_change_logs
             │
             ├─► master_data ──► action_logs
             │     └─► bd_escalations  (#26)
             │     └─► edge_case_log   (#36)
             │
             └─► custom_field_definitions
```

---

## 10. `bd_escalations` — BD Escalation Matrix (#26)

BD-side escalation queue (separate from AE escalations in `08-activity-log/04-escalation.md`).
Tracks the 9 BD escalation IDs (`ESC-BD-001..009`) with priority + ack/resolve audit.

```sql
CREATE TABLE bd_escalations (
  id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id       UUID NOT NULL REFERENCES workspaces(id),
  esc_id             VARCHAR(16) NOT NULL,         -- 'ESC-BD-001' .. 'ESC-BD-009'
  prospect_id        UUID NOT NULL REFERENCES master_data(id) ON DELETE CASCADE,

  priority           VARCHAR(4) NOT NULL,          -- 'P0' | 'P1' | 'P2'
  trigger_reason     TEXT NOT NULL,                -- e.g. "DM_present=FALSE @ D7 + buying_intent=high"
  sla_seconds        INT NOT NULL,                 -- 3600 (P0) | 86400 (P1/P2)

  assigned_to_email  VARCHAR(255) NOT NULL,        -- BD owner or AE Lead
  ack_at             TIMESTAMPTZ,                  -- when assignee acknowledged
  fallback_at        TIMESTAMPTZ,                  -- when re-routed to AE Lead (NULL if not)
  resolved_at        TIMESTAMPTZ,
  resolved_by        VARCHAR(255),
  resolution_note    TEXT,

  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bdesc_workspace ON bd_escalations(workspace_id);
CREATE INDEX idx_bdesc_open      ON bd_escalations(workspace_id, prospect_id) WHERE resolved_at IS NULL;
CREATE INDEX idx_bdesc_sla_scan  ON bd_escalations(created_at, ack_at) WHERE ack_at IS NULL AND resolved_at IS NULL;

-- Constraint: at most ONE active escalation per (prospect_id, esc_id)
CREATE UNIQUE INDEX idx_bdesc_unique_active
  ON bd_escalations(prospect_id, esc_id)
  WHERE resolved_at IS NULL;
```

BD escalation ID catalog:

| ESC ID         | Trigger                                              | Priority | SLA    |
|----------------|------------------------------------------------------|----------|--------|
| ESC-BD-001     | DM absent @ D7 + buying_intent=high                  | P0       | 1h     |
| ESC-BD-002     | Budget rejected mid-cycle (D10–D14)                  | P0       | 1h     |
| ESC-BD-003     | Enterprise prospect silent 7d                        | P1       | 24h    |
| ESC-BD-004     | Competitor contract end < 30d, no commitment         | P1       | 24h    |
| ESC-BD-005     | First payment overdue D+15                           | P0       | 1h     |
| ESC-BD-006     | BD review unconfirmed @ D14                          | P2       | 24h    |
| ESC-BD-007     | Implementation timeline blocker raised               | P1       | 24h    |
| ESC-BD-008     | Feature gap blocker mid-cycle                        | P1       | 24h    |
| ESC-BD-009     | Deal complexity high + no DM access                  | P0       | 1h     |

---

## 11. `prospects.branching_state` — BD Branching State (#29)

Add column on `master_data` (or `prospects` view) to record per-D-day branch
decision so subsequent fires can read prior state and audit can trace template
selection over time.

```sql
ALTER TABLE master_data
  ADD COLUMN IF NOT EXISTS branching_state JSONB NOT NULL DEFAULT '{}'::jsonb;

-- Example value:
-- {
--   "D2": "TPL-BD-D2-DM-PRESENT-ENTERPRISE",
--   "D7": "TPL-BD-D7-HIGH-INTENT",
--   "D10": "TPL-BD-D10-RENEWAL-IMMINENT"
-- }

CREATE INDEX idx_md_branching_gin ON master_data USING GIN (branching_state);
```

The 14 BD branching variables (read by `resolveTemplate()`):

| #  | Variable                              | Type     | Source field                                   |
|----|---------------------------------------|----------|------------------------------------------------|
| 1  | `DM_present_in_call`                  | bool     | `custom_fields->>'dm_present_in_call'`         |
| 2  | `current_system_contract_end`         | date     | `custom_fields->>'competitor_contract_end'`    |
| 3  | `next_step_agreed`                    | bool     | `custom_fields->>'next_step_agreed'`           |
| 4  | `budget_allocated`                    | bool     | `custom_fields->>'budget_allocated'`           |
| 5  | `uses_dealls`                         | bool     | `custom_fields->>'uses_dealls'`                |
| 6  | `prospect_size_tier`                  | enum     | `custom_fields->>'size_tier'` — `SMB` \| `MID` \| `ENTERPRISE` |
| 7  | `prospect_role_type`                  | enum     | `custom_fields->>'role_type'` — `HR` \| `IT` \| `OWNER` \| `FINANCE` |
| 8  | `switching_urgency`                   | enum     | `custom_fields->>'switching_urgency'` — `HIGH` \| `MED` \| `LOW` |
| 9  | `buying_intent`                       | enum     | `custom_fields->>'buying_intent'` — `high` \| `medium` \| `low` |
| 10 | `bd_review_confirmed`                 | bool     | `custom_fields->>'bd_review_confirmed'`        |
| 11 | `expansion_plan`                      | bool     | `custom_fields->>'expansion_plan'`             |
| 12 | `feature_gap_is_blocker`              | bool     | `custom_fields->>'feature_gap_blocker'`        |
| 13 | `implementation_timeline_urgency`     | enum     | `custom_fields->>'impl_timeline'` — `IMMEDIATE` \| `Q1` \| `Q2+` |
| 14 | `deal_complexity`                     | enum     | `custom_fields->>'deal_complexity'` — `LOW` \| `MED` \| `HIGH` |

---

## 12. `edge_case_log` — BD Edge Case Tracker (#36)

Audit table for the 32 BD edge cases (4 CRITICAL + 10 HIGH + 7 MEDIUM + 4 LOW + 7 FP-SPECIFIC).
One row per occurrence; used by analytics and ops for pattern detection.

```sql
CREATE TABLE edge_case_log (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    UUID NOT NULL REFERENCES workspaces(id),
  master_data_id  UUID NOT NULL REFERENCES master_data(id) ON DELETE CASCADE,

  case_code       VARCHAR(32) NOT NULL,            -- e.g. 'EC-BD-CRIT-01', 'EC-FP-04'
  severity        VARCHAR(16) NOT NULL,            -- 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' | 'FP'
  trigger_context JSONB NOT NULL DEFAULT '{}',     -- snapshot of relevant fields at fire time

  outcome         VARCHAR(32),                     -- 'resolved_auto' | 'resolved_manual' | 'escalated' | 'unresolved'
  outcome_note    TEXT,
  resolved_at     TIMESTAMPTZ,
  resolved_by     VARCHAR(255),

  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_eclog_workspace ON edge_case_log(workspace_id);
CREATE INDEX idx_eclog_case      ON edge_case_log(workspace_id, case_code);
CREATE INDEX idx_eclog_master    ON edge_case_log(master_data_id);
CREATE INDEX idx_eclog_unresolved ON edge_case_log(workspace_id, severity)
  WHERE outcome IS NULL OR outcome = 'unresolved';
```

> Full list of 32 edge cases lives in `07-manual-flows.md` § "Edge Cases & High-Risk Scenarios".

## Filter DSL (Backend Parser)

The `filter` and `metric` fields use a simple DSL that the backend must parse
into SQL WHERE clauses. Implementation:

```go
// ParseFilter converts filter DSL string → SQL WHERE clause
// Used by: pipeline_tabs, pipeline_stats
func ParseFilter(filter string, args *[]interface{}) string {
    switch {
    case filter == "" || filter == "all":
        return "TRUE"
    case filter == "bot_active":
        return "bot_active = TRUE"
    case filter == "risk":
        return "(risk_flag IN ('High','Mid') OR bot_active = FALSE OR payment_status = 'Terlambat')"
    case strings.HasPrefix(filter, "stage:"):
        stages := strings.Split(filter[6:], ",")
        // build IN clause
    case strings.HasPrefix(filter, "value_tier:"):
        tiers := strings.Split(filter[11:], ",")
        // query custom_fields->>'value_tier' IN (...)
    case strings.HasPrefix(filter, "sequence:"):
        return fmt.Sprintf("sequence_status = '%s'", filter[9:])
    case strings.HasPrefix(filter, "payment:"):
        return fmt.Sprintf("payment_status = '%s'", filter[8:])
    case strings.HasPrefix(filter, "expiry:"):
        days, _ := strconv.Atoi(filter[7:])
        return fmt.Sprintf("days_to_expiry >= 0 AND days_to_expiry <= %d", days)
    }
    return "TRUE"
}
```

## Metric DSL (Backend Computation)

```go
// ComputeMetric calculates a stat value from records
func ComputeMetric(records []DataMaster, metric string) (value string, subtitle string) {
    switch {
    case metric == "count":
        return strconv.Itoa(len(records)), "total records"
    case strings.HasPrefix(metric, "count:"):
        filter := metric[6:]
        filtered := ApplyFilter(records, filter)
        return strconv.Itoa(len(filtered)), fmt.Sprintf("dari %d total", len(records))
    case strings.HasPrefix(metric, "sum:"):
        field := metric[4:]
        sum := sumField(records, field)
        return formatCurrency(sum), fmt.Sprintf("dari %d records", len(records))
    case strings.HasPrefix(metric, "avg:"):
        field := metric[4:]
        avg := avgField(records, field)
        return strconv.Itoa(avg), fmt.Sprintf("rata-rata dari %d data", countNonZero(records, field))
    }
    return "-", ""
}
```
