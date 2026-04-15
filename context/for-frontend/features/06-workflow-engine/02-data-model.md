# Workflow Engine — Data Model

All TypeScript types correspond 1:1 to the Go entity structs in
`internal/entity/workflow.go` and `internal/entity/automation_rule.go`.

---

## Workflow

```typescript
type WorkflowStatus = 'active' | 'draft' | 'disabled'

interface Workflow {
  id: string                 // UUID
  workspace_id: string       // UUID
  name: string               // e.g. "AE Client Lifecycle"
  icon: string | null        // emoji string, e.g. "🏆"
  slug: string               // URL-safe, e.g. "ae-client-lifecycle"
  description: string | null
  status: WorkflowStatus
  stage_filter: string[]     // e.g. ["CLIENT"]
  created_by: string | null
  updated_by: string | null
  created_at: string         // ISO 8601
  updated_at: string         // ISO 8601
}

// WorkflowListItem — returned by GET /workflows (list endpoint)
interface WorkflowListItem extends Workflow {
  node_count: number
  edge_count: number
}

// WorkflowFull — returned by GET /workflows/{id} and GET /workflows/by-slug/{slug}
interface WorkflowFull extends Workflow {
  nodes:   WorkflowNode[]
  edges:   WorkflowEdge[]
  steps:   WorkflowStep[]
  tabs:    PipelineTab[]
  stats:   PipelineStat[]
  columns: PipelineColumn[]
}
```

---

## WorkflowNode (React Flow canvas node)

```typescript
type NodeCategory = 'trigger' | 'condition' | 'action' | 'delay'

// NodeData — the JSONB `data` field inside a node
interface NodeData {
  category?:    NodeCategory
  label?:       string
  icon?:        string
  description?: string
  templateId?:  string      // reference to message_templates
  triggerId?:   string      // reference to automation_rules.trigger_id
  timing?:      string
  condition?:   string
  stopIf?:      string
  sentFlag?:    string
  // Zone nodes only:
  color?: string
  bg?:    string
}

interface WorkflowNode {
  id:          string    // internal UUID (row pk)
  workflow_id: string
  node_id:     string    // React Flow node id, e.g. "ae-p01"
  node_type:   'workflow' | 'zone'
  position_x:  number
  position_y:  number
  width?:      number    // zone nodes only
  height?:     number    // zone nodes only
  data:        NodeData  // JSONB
  draggable:   boolean
  selectable:  boolean
  connectable: boolean
  z_index:     number
  created_at:  string
  updated_at:  string
}
```

---

## WorkflowEdge (React Flow canvas edge)

```typescript
interface EdgeStyle {
  stroke?:          string
  strokeWidth?:     number
  strokeDasharray?: string
}

interface WorkflowEdge {
  id:          string    // internal UUID (row pk)
  workflow_id: string
  edge_id:     string    // React Flow edge id, e.g. "ae-e01"
  source:      string    // React Flow source node_id
  target:      string    // React Flow target node_id
  animated:    boolean
  style:       EdgeStyle | null
  label:       string | null
  created_at:  string
  updated_at:  string
}
```

---

## WorkflowStep (pipeline step config)

```typescript
interface WorkflowStep {
  id:                 string
  workflow_id:        string
  step_key:           string    // e.g. "p0", "p1-checkin"
  label:              string    // e.g. "P0 Onboarding"
  phase:              string    // e.g. "P0"
  icon:               string | null
  description:        string | null
  timing:             string | null    // e.g. "D+0 to D+35"
  condition:          string | null
  stop_if:            string | null
  sent_flag:          string | null
  template_id:        string | null    // legacy ref
  message_template_id:string | null
  email_template_id:  string | null
  sort_order:         number
  created_at:         string
  updated_at:         string
}
```

---

## Pipeline Config (Tabs, Stats, Columns)

```typescript
// PipelineTab — configures a clickable filter tab in the pipeline view
interface PipelineTab {
  id:          string
  workflow_id: string
  tab_key:     string    // e.g. "semua", "aktif", "renewal"
  label:       string    // e.g. "Semua Client"
  icon:        string | null
  filter:      string    // Filter DSL string, e.g. "bot_active", "expiry:30"
  sort_order:  number
}

// PipelineStat — configures a stat card in the pipeline view header
interface PipelineStat {
  id:          string
  workflow_id: string
  stat_key:    string    // e.g. "total", "revenue"
  label:       string    // e.g. "Total Client"
  metric:      string    // Metric DSL, e.g. "count", "sum:final_price"
  color:       string | null    // Tailwind class, e.g. "text-emerald-400"
  border:      string | null    // Tailwind class, e.g. "border-emerald-500/20"
  sort_order:  number
}

// PipelineColumn — configures visible columns in the data table
interface PipelineColumn {
  id:          string
  workflow_id: string
  col_key:     string    // e.g. "Company_Name"
  field:       string    // actual DB/JSON field name
  label:       string    // display header
  width:       number    // pixels
  visible:     boolean
  sort_order:  number
}
```

---

## AutomationRule

```typescript
type RuleRole    = 'sdr' | 'bd' | 'ae' | 'cs'
type RuleStatus  = 'active' | 'paused' | 'disabled'
type RuleChannel = 'whatsapp' | 'email' | 'telegram'

interface AutomationRule {
  id:           string
  workspace_id: string
  rule_code:    string         // e.g. "RULE-KK-AE-OB-001"
  trigger_id:   string         // e.g. "Onboarding_Welcome"
  template_id:  string | null
  role:         RuleRole
  phase:        string         // e.g. "P0"
  phase_label:  string | null  // e.g. "Onboarding"
  priority:     number
  timing:       string         // e.g. "D+0 to D+5"
  condition:    string         // SQL-like condition string
  stop_if:      string         // "-" means no stop condition
  sent_flag:    string | null  // DB flag field name
  channel:      RuleChannel
  status:       RuleStatus
  updated_at:   string
  updated_by:   string | null
  created_at:   string
}

// IsExecutable — mirrors entity.AutomationRule.IsExecutable()
// Only active rules with a non-empty trigger_id are evaluated by cron.
function isExecutable(rule: AutomationRule): boolean {
  return rule.status === 'active' && rule.trigger_id !== ''
}
```

---

## RuleChangeLog

```typescript
interface RuleChangeLog {
  id:        string
  rule_id:   string
  field:     string    // e.g. "timing", "status", "condition"
  old_value: string | null
  new_value: string
  edited_by: string
  edited_at: string    // ISO 8601
}

// RuleChangeLogWithCode — returned by GET /automation-rules/change-logs (feed)
interface RuleChangeLogWithCode extends RuleChangeLog {
  rule_code: string
  workspace_id: string
}
```

---

## Pipeline Data Response (GET /workflows/{id}/data)

```typescript
interface StatValue {
  value: string    // e.g. "85" or "4700000"
  sub:   string    // e.g. "total records" or "dari 85 total"
}

interface PipelineDataResponse {
  data:  MasterData[]                   // paginated records
  total: number                         // total matching count
  stats: Record<string, StatValue>      // keyed by stat_key
}
```

---

## CanvasSaveResult (PUT /workflows/{id}/canvas response)

```typescript
interface CanvasSaveResult {
  workflow_id: string
  node_count:  number
  edge_count:  number
  saved_at:    string    // ISO 8601
}
```
