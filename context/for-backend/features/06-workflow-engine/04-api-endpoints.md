# API Endpoints — Workflow Engine

## Base URL
```
{BACKEND_API_URL}/api/v1
```

All endpoints require `Authorization: Bearer {token}` header.
Workspace-scoped endpoints require `X-Workspace-ID: {uuid}` header.

---

## 1. Workflows CRUD

### GET `/workflows`
List all workflows for the current workspace.

```
Query params:
  ?status=active              (optional: active, draft, disabled)

Response 200:
{
  "data": [
    {
      "id": "uuid",
      "name": "SDR Lead Outreach",
      "icon": "emoji-phone",
      "slug": "4fc22c98-1e3b-4901-aa86-9f81b33354d2",
      "description": "Email + WA multi-channel outreach...",
      "status": "active",
      "stage_filter": ["LEAD", "DORMANT"],
      "node_count": 25,
      "edge_count": 23,
      "created_at": "2026-04-01T00:00:00Z",
      "updated_at": "2026-04-09T10:00:00Z",
      "updated_by": "budi@kantorku.id"
    }
  ]
}
```

### GET `/workflows/{id}`
Get single workflow with all nested data (nodes, edges, steps, tabs, stats, columns).

```
Response 200:
{
  "data": {
    "id": "uuid",
    "name": "AE Client Lifecycle",
    "icon": "emoji-trophy",
    "slug": "406e6b25-37f6-4531-aade-aa42df2d52a3",
    "status": "active",
    "stage_filter": ["CLIENT"],
    "nodes": [ ... WorkflowNode[] ... ],
    "edges": [ ... WorkflowEdge[] ... ],
    "steps": [ ... WorkflowStep[] ... ],
    "tabs": [ ... PipelineTab[] ... ],
    "stats": [ ... PipelineStat[] ... ],
    "columns": [ ... PipelineColumn[] ... ]
  }
}

Response 404:
{
  "error": "Workflow not found"
}
```

### GET `/workflows/by-slug/{slug}`
Get workflow by slug (used by frontend route `/pipeline/{slug}`).
Same response as GET `/workflows/{id}`.

### POST `/workflows`
Create a new workflow.

```json
// Request body:
{
  "name": "Custom Pipeline",
  "icon": "emoji-zap",
  "description": "Custom workflow",
  "status": "draft",
  "stage_filter": ["LEAD"]
}

// Response 201:
{
  "data": {
    "id": "uuid",
    "slug": "custom-pipeline",
    ...
  }
}
```

### PUT `/workflows/{id}`
Update workflow metadata (name, icon, description, status, stage_filter).

```json
// Request body (partial):
{
  "name": "Updated Pipeline Name",
  "status": "active"
}

// Response 200:
{
  "data": { ... updated workflow ... }
}
```

### DELETE `/workflows/{id}`
Delete workflow and all nested data (CASCADE).

```
Response 200:
{
  "message": "Deleted",
  "id": "uuid"
}
```

---

## 2. Canvas (Nodes & Edges)

### PUT `/workflows/{id}/canvas`
Save entire canvas (bulk replace all nodes and edges).
Called when user clicks "Simpan" in the Workflow Builder.

```json
// Request body:
{
  "nodes": [
    {
      "node_id": "ae-p01",
      "node_type": "workflow",
      "position_x": 0,
      "position_y": 0,
      "data": {
        "category": "trigger",
        "label": "P0.1 Welcome",
        "icon": "emoji-zap",
        "description": "D+0~5 welcome message",
        "templateId": "TPL-OB-WELCOME",
        "triggerId": "Onboarding_Welcome"
      },
      "draggable": true,
      "selectable": true,
      "connectable": true,
      "z_index": 0
    },
    {
      "node_id": "ae-z-p0",
      "node_type": "zone",
      "position_x": -20,
      "position_y": -30,
      "width": 680,
      "height": 180,
      "data": {
        "label": "P0 - Onboarding",
        "color": "#534AB7",
        "bg": "rgba(83,74,183,0.05)"
      },
      "draggable": true,
      "selectable": false,
      "connectable": false,
      "z_index": -1
    }
  ],
  "edges": [
    {
      "edge_id": "ae-e01",
      "source": "ae-p01",
      "target": "ae-p02",
      "animated": true,
      "style": null,
      "label": null
    },
    {
      "edge_id": "ae-e00",
      "source": "ae-dm",
      "target": "ae-p01",
      "label": "READ Stage='CLIENT'",
      "style": { "stroke": "#2563eb", "strokeWidth": 2, "strokeDasharray": "6,3" }
    }
  ]
}

// Response 200:
{
  "data": {
    "workflow_id": "uuid",
    "node_count": 25,
    "edge_count": 30,
    "saved_at": "2026-04-12T10:00:00Z"
  }
}
```

**Backend logic:**
1. Begin transaction
2. DELETE all existing nodes WHERE workflow_id = X
3. DELETE all existing edges WHERE workflow_id = X
4. INSERT all new nodes
5. INSERT all new edges
6. UPDATE workflows.updated_at
7. Commit

---

## 2.5 Template Resolution (BD Conditional Branching)

### POST `/workflows/{workflow_id}/resolve-template`

Resolve `template_id` yang harus dipakai untuk sebuah node terhadap state BANTS +
context client target saat ini. Dipanggil oleh **cron engine** tepat sebelum pesan
dikirim, sehingga template selalu merefleksikan kondisi terbaru record (bukan
snapshot saat workflow di-save).

Endpoint ini membungkus logika `resolveTemplate()` yang dispesifikasikan di
`context/for-business/10-BUYING-INTENT-MECHANISM.md` — satu sumber kebenaran
untuk pemilihan template BD D0–D21.

```json
// Request body:
{
  "node_id": "uuid",
  "master_data_id": "uuid",
  "trigger_id": "BD_D7"
}

// Response 200:
{
  "resolved_template_id": "TPL-BD-D7-HIGH-INTENT",
  "variant": "high_intent",
  "reasons": [
    "renewal not imminent (days_to_expiry=152)",
    "buying_intent=high (BANTS=2.4, HOT)",
    "selected: high_intent variant"
  ]
}

// Response 400:
{ "error": "node_id not found" }

// Response 400:
{ "error": "master_data_id not found or not in caller's workspace" }
```

**Field `variant`** — salah satu dari:
`high_intent` · `low_intent` · `renewal_imminent` · `dm_present` · `dealls_user` ·
`enterprise` · `it_attendee` · `vs_free` · `default`.

### Priority Order (evaluated top-down, first match wins)

Urutan evaluasi **harus** persis seperti di bawah. Begitu satu cabang match,
resolver berhenti dan me-return `template_id`-nya.

1. **Renewal imminent** — jika `record.days_to_expiry != NULL AND days_to_expiry <= 90`
   → `TPL-{ROLE}-{day}-RENEWAL-IMMINENT` (role di-derive dari node; untuk BD =
   `TPL-BD-{day}-RENEWAL-IMMINENT`). Renewal mendahului buying_intent karena
   kontrak hampir habis adalah urgency deal terpenting.
2. **Buying intent** —
   - `buying_intent='high'` → `TPL-BD-{day}-HIGH-INTENT`
   - `buying_intent='low'`  → `TPL-BD-{day}-LOW-INTENT`
   - `buying_intent='medium'` atau `NULL` → lanjut ke step 3.
3. **Legacy condition branches** (dicek berurutan, first-true wins):
   - `DM_present_in_call = TRUE` → `TPL-BD-{day}-DM`
   - `Dealls_user = TRUE`        → `TPL-BD-{day}-DEALLS-USER`
   - `Enterprise = TRUE`         → `TPL-BD-{day}-ENTERPRISE`
   - `IT_attendee = TRUE`        → `TPL-BD-{day}-IT-ATTENDEE`
   - `VS_FREE = TRUE`            → `TPL-BD-{day}-VS-FREE`
4. **Default** — `TPL-BD-{day}-DEFAULT`.

`{day}` diambil dari `trigger_id` (mis. `BD_D7` → `D7`, `BD_D21` → `D21`).

### Low-Intent BD Sequence Shortening

Untuk prospect dengan `buying_intent='low'`, cron engine **memendekkan** sequence
BD agar tidak push terlalu keras:

| Intent       | Sequence touches                                      |
|--------------|-------------------------------------------------------|
| `high` / `medium` / `NULL` | D0 → D2 → D4 → D7 → D10 → D12 → D14 → D21 → NURTURE |
| `low`        | D0 → D2 → D4 → D7 → D10 → NURTURE (skip D12/D14/D21) |

**Implementasi cron:** sebelum firing trigger `BD_D12`, `BD_D14`, atau `BD_D21`,
cron harus re-check `record.buying_intent`. Jika `='low'`, skip trigger tersebut
dan langsung advance record ke tahap NURTURE (`nurture_queued_at = NOW()`).

Ini memastikan low-intent prospect tidak di-spam dengan push closing yang tidak
relevan, dan sebaliknya langsung masuk ke educate/content loop.

### Go Service Signature

```go
type ResolveTemplateInput struct {
    NodeID       uuid.UUID `json:"node_id"`
    MasterDataID uuid.UUID `json:"master_data_id"`
    TriggerID    string    `json:"trigger_id"` // e.g. "BD_D7"
}

type ResolveTemplateResult struct {
    TemplateID string   `json:"resolved_template_id"`
    Variant    string   `json:"variant"`
    Reasons    []string `json:"reasons"`
}

// Resolve applies the priority order + buying_intent + existing condition
// branches to pick the right template_id. Returns a 400-class error if
// node_id / master_data_id is invalid or not in the caller's workspace.
func (s *TemplateResolver) Resolve(
    ctx context.Context,
    wsID uuid.UUID,
    in ResolveTemplateInput,
) (*ResolveTemplateResult, error) {
    node, err := s.nodeRepo.GetByID(ctx, in.NodeID)
    if err != nil {
        return nil, fmt.Errorf("node_id not found: %w", err)
    }
    record, err := s.masterDataRepo.GetByID(ctx, wsID, in.MasterDataID)
    if err != nil {
        return nil, fmt.Errorf("master_data_id not found or not in caller's workspace: %w", err)
    }

    day := extractDay(in.TriggerID) // "BD_D7" -> "D7"
    role := node.Role                // "BD"
    reasons := []string{}

    // 1. Renewal imminent (highest priority)
    if record.DaysToExpiry != nil && *record.DaysToExpiry <= 90 {
        reasons = append(reasons, fmt.Sprintf(
            "renewal imminent (days_to_expiry=%d <= 90)", *record.DaysToExpiry))
        return &ResolveTemplateResult{
            TemplateID: fmt.Sprintf("TPL-%s-%s-RENEWAL-IMMINENT", role, day),
            Variant:    "renewal_imminent",
            Reasons:    append(reasons, "selected: renewal_imminent variant"),
        }, nil
    }
    reasons = append(reasons, fmt.Sprintf(
        "renewal not imminent (days_to_expiry=%s)", daysPretty(record.DaysToExpiry)))

    // 2. Buying intent (BANTS-derived)
    switch record.BuyingIntent {
    case "high":
        reasons = append(reasons, fmt.Sprintf(
            "buying_intent=high (BANTS=%.2f, HOT/WARM)", record.BantsScore))
        return &ResolveTemplateResult{
            TemplateID: fmt.Sprintf("TPL-BD-%s-HIGH-INTENT", day),
            Variant:    "high_intent",
            Reasons:    append(reasons, "selected: high_intent variant"),
        }, nil
    case "low":
        reasons = append(reasons, fmt.Sprintf(
            "buying_intent=low (BANTS=%.2f, COLD)", record.BantsScore))
        return &ResolveTemplateResult{
            TemplateID: fmt.Sprintf("TPL-BD-%s-LOW-INTENT", day),
            Variant:    "low_intent",
            Reasons:    append(reasons, "selected: low_intent variant"),
        }, nil
    }
    reasons = append(reasons, fmt.Sprintf(
        "buying_intent=%q → fall through to legacy branches", record.BuyingIntent))

    // 3. Legacy condition branches (order matters)
    switch {
    case record.DMPresentInCall:
        return resolved(day, "DM", "dm_present", reasons, "DM_present_in_call=TRUE")
    case record.DeallsUser:
        return resolved(day, "DEALLS-USER", "dealls_user", reasons, "Dealls_user=TRUE")
    case record.Enterprise:
        return resolved(day, "ENTERPRISE", "enterprise", reasons, "Enterprise=TRUE")
    case record.ITAttendee:
        return resolved(day, "IT-ATTENDEE", "it_attendee", reasons, "IT_attendee=TRUE")
    case record.VSFree:
        return resolved(day, "VS-FREE", "vs_free", reasons, "VS_FREE=TRUE")
    }

    // 4. Default
    return &ResolveTemplateResult{
        TemplateID: fmt.Sprintf("TPL-BD-%s-DEFAULT", day),
        Variant:    "default",
        Reasons:    append(reasons, "no legacy branch matched", "selected: default variant"),
    }, nil
}
```

### Notes

- Field-level data model (`buying_intent`, `bants_score`, `days_to_expiry`,
  `DM_present_in_call`, `Dealls_user`, `Enterprise`, `IT_attendee`, `VS_FREE`,
  dsb.) dispesifikasikan di `09-data-models.md` (master_data schema + BD
  custom fields).
- Sumber logika bisnis lengkap (BANTS → buying_intent mapping, BD override
  mechanism, semua variant template WA) ada di
  `context/for-business/10-BUYING-INTENT-MECHANISM.md`. Endpoint ini HANYA
  implementasi teknis — business rules harus sinkron dengan dokumen tersebut.
- `reasons[]` wajib diisi untuk auditability. Cron engine menulis `reasons` ke
  `cron_job_logs.resolution_trace` sehingga BD team bisa trace "kenapa template
  X yang kepilih" via dashboard.
- Endpoint ini **read-only** — tidak ada side effect di DB. Aman dipanggil
  berkali-kali (idempotent).

---

## 3. Pipeline Steps

### GET `/workflows/{id}/steps`
List all steps for a workflow (ordered by sort_order).

```
Response 200:
{
  "data": [
    {
      "key": "p0",
      "label": "P0 Onboarding",
      "phase": "P0",
      "icon": "emoji-wave",
      "description": "Welcome + check-in + usage check",
      "timing": "D+0 to D+35",
      "condition": "days_since_activation BETWEEN 0 AND 35",
      "templateId": "TPL-OB-WELCOME",
      "messageTemplateId": null,
      "emailTemplateId": null
    }
  ]
}
```

### PUT `/workflows/{id}/steps`
Bulk save all steps for a workflow (replace all).

```json
// Request body:
{
  "steps": [
    {
      "key": "p0",
      "label": "P0 Onboarding",
      "phase": "P0",
      "icon": "emoji-wave",
      "description": "Welcome + check-in + usage check (D+0 to D+35)",
      "timing": "D+0 to D+35",
      "condition": "days_since_activation BETWEEN 0 AND 35",
      "stopIf": "",
      "sentFlag": "",
      "templateId": "TPL-OB-WELCOME",
      "messageTemplateId": null,
      "emailTemplateId": null
    }
  ]
}
```

### PUT `/workflows/{id}/steps/{stepKey}`
Update a single step (used by Step Config page).

```json
// Request body (partial):
{
  "timing": "D+0 to D+7",
  "condition": "days_since_activation BETWEEN 0 AND 7 AND onboarding_sent = FALSE",
  "templateId": "TPL-OB-WELCOME",
  "messageTemplateId": "TPL-OB-WELCOME",
  "emailTemplateId": "ETPL-KK-AE-001"
}

// Response 200:
{
  "data": { ... updated step ... }
}
```

---

## 4. Pipeline Config (Tabs, Stats, Columns)

### GET `/workflows/{id}/config`
Get all pipeline config (tabs + stats + columns) for a workflow.

```
Response 200:
{
  "tabs": [ ... PipelineTab[] ... ],
  "stats": [ ... PipelineStat[] ... ],
  "columns": [ ... PipelineColumn[] ... ]
}
```

### PUT `/workflows/{id}/tabs`
Bulk save tabs for a workflow.

```json
{
  "tabs": [
    { "key": "semua", "label": "Semua Client", "icon": "emoji-clipboard", "filter": "all" },
    { "key": "aktif", "label": "Bot Aktif", "icon": "emoji-green", "filter": "bot_active" },
    { "key": "renewal", "label": "Renewal", "icon": "emoji-calendar", "filter": "expiry:30" },
    { "key": "perhatian", "label": "Perhatian", "icon": "emoji-warning", "filter": "risk" }
  ]
}
```

### PUT `/workflows/{id}/stats`
Bulk save stat cards for a workflow.

```json
{
  "stats": [
    { "key": "total", "label": "Total Client", "metric": "count", "color": "text-emerald-400", "border": "border-emerald-500/20" },
    { "key": "revenue", "label": "Total Revenue", "metric": "sum:Final_Price", "color": "text-brand-400", "border": "border-brand-400/20" },
    { "key": "renewal", "label": "Renewal <=30d", "metric": "count:expiry:30", "color": "text-amber-400", "border": "border-amber-500/20" },
    { "key": "risk", "label": "Perhatian", "metric": "count:risk", "color": "text-rose-400", "border": "border-rose-500/20" }
  ]
}
```

### PUT `/workflows/{id}/columns`
Bulk save column config for a workflow.

```json
{
  "columns": [
    { "key": "Company_Name", "field": "Company_Name", "label": "Company", "width": 220, "visible": true },
    { "key": "Stage", "field": "Stage", "label": "Stage", "width": 90, "visible": true },
    { "key": "Bot_Active", "field": "Bot_Active", "label": "Bot", "width": 60, "visible": true },
    { "key": "Risk_Flag", "field": "Risk_Flag", "label": "Risk", "width": 70, "visible": false }
  ]
}
```

---

## 5. Pipeline Data (records filtered by workflow)

### GET `/workflows/{id}/data`
Get master_data records filtered by this workflow's stage_filter, with tab filter and stat computation.

```
Query params:
  ?tab=semua                   (tab_key — applies the tab's filter DSL)
  &search=keyword              (searches company_name, pic_name, company_id)
  &offset=0&limit=20
  &sort_by=updated_at
  &sort_dir=desc

Response 200:
{
  "data": [ ... ClientRecord[] ... ],
  "meta": {
    "offset": 0,
    "limit": 20,
    "total": 85
  },
  "stats": {
    "total": { "value": "85", "sub": "total records" },
    "revenue": { "value": "Rp 4.7M", "sub": "dari 85 records" },
    "renewal": { "value": "12", "sub": "dari 85 total" },
    "risk": { "value": "8", "sub": "dari 85 total" }
  }
}
```

**Backend logic:**
1. Get workflow → get stage_filter
2. Query master_data WHERE workspace_id = X AND stage IN (stage_filter)
3. If tab param provided → get tab's filter → apply ParseFilter()
4. Apply search, sort, pagination
5. Compute all stat metrics via ComputeMetric()
6. Return data + stats

---

## 6. Automation Rules CRUD

### GET `/automation-rules`
List all rules for the current workspace.

```
Query params:
  ?role=ae                     (optional: sdr, bd, ae, cs)
  &status=active               (optional: active, paused, disabled)
  &phase=P4                    (optional)
  &search=keyword              (searches trigger_id, template_id, condition)

Response 200:
{
  "data": [
    {
      "id": "uuid",
      "rule_code": "RULE-KK-AE-OB-001",
      "trigger_id": "Onboarding_Welcome",
      "template_id": "TPL-OB-WELCOME",
      "role": "ae",
      "phase": "P0",
      "phase_label": "Onboarding",
      "timing": "D+0 to D+5",
      "condition": "days_since_activation BETWEEN 0 AND 5 AND onboarding_sent = FALSE",
      "stop_if": "-",
      "sent_flag": "onboarding_sent",
      "channel": "whatsapp",
      "status": "active",
      "updated_at": "2026-04-03T14:22:00Z",
      "updated_by": "arief.faltah@dealls.com"
    }
  ]
}
```

### GET `/automation-rules/{id}`
Get single rule with change logs.

```
Response 200:
{
  "data": { ... AutomationRule ... },
  "change_logs": [
    {
      "field": "timing",
      "old_value": "H-90",
      "new_value": "H-90 to H-85",
      "edited_by": "reza.mahendra@dealls.com",
      "edited_at": "2026-04-05T14:20:00Z"
    }
  ]
}
```

### POST `/automation-rules`
Create a new rule.

```json
// Request body:
{
  "rule_code": "RULE-KK-AE-CUSTOM-001",
  "trigger_id": "Custom_Trigger",
  "template_id": "TPL-CUSTOM-001",
  "role": "ae",
  "phase": "P2",
  "phase_label": "Custom Phase",
  "timing": "D+90 to D+95",
  "condition": "custom_field_x = TRUE AND Bot_Active = TRUE",
  "stop_if": "-",
  "sent_flag": "custom_sent",
  "channel": "whatsapp",
  "status": "active"
}

// Response 201:
{
  "data": { "id": "uuid", ... }
}

// Response 409 (duplicate rule_code):
{
  "error": "Rule code RULE-KK-AE-CUSTOM-001 already exists"
}
```

### PUT `/automation-rules/{id}`
Update rule fields. Automatically logs changes to rule_change_logs.

```json
// Request body (partial — only changed fields):
{
  "timing": "D+0 to D+7",
  "condition": "days_since_activation BETWEEN 0 AND 7",
  "status": "paused"
}

// Response 200:
{
  "data": { ... updated rule ... },
  "changes": [
    { "field": "timing", "old_value": "D+0 to D+5", "new_value": "D+0 to D+7" },
    { "field": "status", "old_value": "active", "new_value": "paused" }
  ]
}
```

### DELETE `/automation-rules/{id}`
Delete rule and its change logs.

```
Response 200:
{
  "message": "Deleted",
  "id": "uuid"
}
```

### GET `/automation-rules/change-logs`
Get change log feed for all rules in workspace.

```
Query params:
  ?limit=50
  &since=2026-04-01T00:00:00Z

Response 200:
{
  "data": [
    {
      "id": "uuid",
      "rule_id": "uuid",
      "rule_code": "RULE-KK-AE-REN-001",
      "field": "timing",
      "old_value": "H-90",
      "new_value": "H-90 to H-85",
      "edited_by": "reza.mahendra@dealls.com",
      "edited_at": "2026-04-05T14:20:00Z"
    }
  ]
}
```

---

## 7. Node Spec Lookup

### GET `/workflows/node-specs`
Return all pre-populated node specs from Excel data (used by config panel to display spec data).

```
Query params:
  ?role=ae                     (optional: sdr, bd, ae, cs — filter by pipeline role)

Response 200:
{
  "data": {
    "ae": [
      {
        "triggerId": "Onboarding_Welcome",
        "phase": "P0.1",
        "action": "Send welcome message + wiki / tutorial link",
        "timing": "D+0 to D+5 (working day)",
        "condition": "days_since_activation BETWEEN 0 AND 5...",
        "templateId": "TPL-OB-WELCOME",
        "waMessage": "Halo Tim *[Company_Name]* ...",
        "variables": "[Company_Name], [link_wiki]",
        "stopIf": "-",
        "sentFlag": "onboarding_sent",
        "notes": "One-time send. D+0 after go-live.",
        "dataRead": "Stage, Company_Name, Bot_Active, onboarding_sent",
        "dataWrite": "onboarding_sent = TRUE"
      }
    ],
    "sdr": [ ... ],
    "bd": [ ... ],
    "cs": [ ... ]
  }
}
```

### GET `/workflows/node-specs/find`
Find a specific node spec by trigger_id or template_id.

```
Query params:
  ?trigger_id=Onboarding_Welcome
  OR
  ?template_id=TPL-OB-WELCOME

Response 200:
{
  "data": { ... NodeConfig ... }
}

Response 404:
{
  "error": "Node spec not found"
}
```

---

## 8. Holding View (aggregated workflows)

### GET `/workflows?holding=true`
When workspace is holding (is_holding=true), return workflows from all member workspaces.

```
Backend logic:
  1. Check workspace.is_holding = true
  2. Get member_ids
  3. Query workflows WHERE workspace_id IN (member_ids)
  4. Add workspace_name to each workflow

Response: same as GET /workflows but with workflows from all members.
```

### GET `/automation-rules?holding=true`
Same pattern — aggregated rules from all member workspaces.
Each rule includes `workspace` field ('dealls' or 'kantorku') for display.

---

## Checker-Maker Approval Required

The following endpoints require approval before execution.
See `00-shared/05-checker-maker.md` for the full approval system spec.

### POST `/master-data/clients/{id}/transition` (manual stage change) → Approval Required

When a user manually triggers a stage transition (not via cron/automation), create an approval request:

```
POST /approvals
{
  "request_type": "stage_transition",
  "payload": {
    "client_id": "uuid",
    "company_name": "PT Dealls Tech",
    "company_id": "DE-001",
    "current_stage": "LEAD",
    "new_stage": "PROSPECT",
    "updates": {
      "sequence_status": "ACTIVE",
      "bot_active": true
    },
    "custom_field_updates": {
      "qualified_at": "2026-04-12T10:00:00Z",
      "qualified_by": "SDR-Budi"
    },
    "trigger_id": "SDR_QUALIFY_HANDOFF",
    "reason": "HC >= 50, role match, interest signal"
  }
}
```

When approved, the system executes the actual stage transition via `POST /master-data/clients/{id}/transition`.

**Approval payload format:**
| Field | Type | Description |
|-------|------|-------------|
| `client_id` | UUID | The record being transitioned |
| `company_name` | string | Display name for the approval reviewer |
| `company_id` | string | Company ID for reference |
| `current_stage` | string | Current stage before transition |
| `new_stage` | string | Target stage |
| `updates` | object | Core field updates to apply |
| `custom_field_updates` | object | Custom field updates to apply |
| `trigger_id` | string | Trigger that initiated the transition |
| `reason` | string | Justification for the transition |

> **Note:** Automated transitions triggered by the cron engine do NOT require approval — only manual user-initiated transitions go through checker-maker.

### PUT `/automation-rules/{id}` (status change active↔paused) → Approval Required

When updating a rule's status between `active` and `paused`, create an approval request:

```
POST /approvals
{
  "request_type": "toggle_automation",
  "payload": {
    "rule_id": "uuid",
    "rule_code": "RULE-KK-AE-OB-001",
    "trigger_id": "Onboarding_Welcome",
    "current_status": "active",
    "new_status": "paused",
    "affected_clients_count": 12
  }
}
```

When approved, the system applies the status change.

**Approval payload format:**
| Field | Type | Description |
|-------|------|-------------|
| `rule_id` | UUID | The automation rule being toggled |
| `rule_code` | string | Rule code for reviewer display |
| `trigger_id` | string | Trigger ID for context |
| `current_status` | string | Current status (`active` or `paused`) |
| `new_status` | string | Target status |
| `affected_clients_count` | number | Approximate number of clients affected by this rule |

> **Note:** Only status changes between `active` and `paused` require approval. Other field edits (timing, condition, template) do NOT require approval.

---

## 9. BD Escalations (#26)

Backed by `bd_escalations` table (see `02-database-schema.md` §10). Read access is open
to the prospect's owner; resolve action is gated to `Lead`-only per division.

### GET `/escalations`

```
Query params:
  ?status=open|acked|resolved   (default: open)
  &priority=P0|P1|P2
  &esc_id=ESC-BD-001
  &assigned_to_email=foo@x.id
  &offset=0&limit=20

Response 200:
{
  "data": [
    {
      "id": "uuid",
      "esc_id": "ESC-BD-001",
      "prospect_id": "uuid",
      "company_name": "PT Example",
      "priority": "P0",
      "trigger_reason": "DM_present=FALSE @ D7 + buying_intent=high",
      "sla_seconds": 3600,
      "assigned_to_email": "bd.budi@dealls.com",
      "ack_at": null,
      "fallback_at": null,
      "resolved_at": null,
      "created_at": "2026-04-22T08:30:00Z",
      "is_overdue": true
    }
  ],
  "meta": { "offset": 0, "limit": 20, "total": 5 }
}
```

`is_overdue = true` iff `ack_at IS NULL AND created_at + sla_seconds < NOW()`.

### PUT `/escalations/{id}/resolve`

**Auth gate:** caller's role must be `Lead` (BD Lead, AE Lead, or SDR Lead — must
match the division of the escalation's owner). Non-Lead users receive 403.

```json
// Request:
{
  "status": "resolved",            // 'acked' | 'resolved'
  "resolution_note": "Called BD owner — prospect rescheduled to Friday"
}

// Response 200:
{ "data": { "id": "uuid", "status": "resolved",
            "resolved_at": "2026-04-22T10:00:00Z",
            "resolved_by": "ae.lead@dealls.com" } }

// Response 403:
{ "error": "only Lead role can resolve escalations" }
```

### POST `/escalations` (internal)

Created automatically by `triggerEscalation()` (see `03-golang-models.md`).
Not exposed to UI for direct creation.

---

## 10. Prospect Branching State (#29)

### GET `/prospects/{id}/branching-state`

Read-only inspector — exposes which BD template variants will fire (or have
fired) per D-day so BD/AE Lead can audit before the next blast.

```
Response 200:
{
  "prospect_id": "uuid",
  "company_name": "PT Example",
  "intake_at": "2026-04-15T09:00:00Z",
  "current_d_day": "D7",
  "history": {
    "D0":  { "template_id": "TPL-BD-D0-DEFAULT", "fired_at": "2026-04-15T09:05:00Z" },
    "D2":  { "template_id": "TPL-BD-D2-DM-PRESENT-ENTERPRISE", "fired_at": "2026-04-17T09:10:00Z" },
    "D4":  { "template_id": "TPL-BD-D4-HIGH-INTENT", "fired_at": "2026-04-19T09:00:00Z" }
  },
  "next": {
    "d_day": "D7",
    "predicted_template_id": "TPL-BD-D7-RENEWAL-IMMINENT",
    "variant": "renewal_imminent",
    "reasons": [
      "renewal imminent (days_to_expiry=85 <= 90)",
      "selected: renewal_imminent variant"
    ]
  },
  "branching_variables": {
    "DM_present_in_call": true,
    "buying_intent": "high",
    "prospect_size_tier": "ENTERPRISE",
    "uses_dealls": false,
    ... 10 more ...
  }
}
```

`history` mirrors `master_data.branching_state` JSONB column. `next.predicted_*`
calls `resolveTemplate()` with current state — useful for "what will the bot send
tomorrow?" audits. **Read-only — no DB writes.**

---

## 11. CS Upsell Template Routing (#59)

Product-type-aware variant resolver for CS cross-sell/upsell templates. Used by
the AE pipeline's CS hooks (`CS_Awareness`, `CS_SocialProof`, `CS_Pricing_Promo`,
`CS_Upsell_Renewal`) and by ad-hoc CS team broadcasts.

### GET `/templates/cs-upsell`

```
Query params:
  ?product_type=ATS|HRIS|PAYROLL|TIMESHEET   (current product the client uses)
  &nps_score=9                               (latest NPS, integer 0-10)
  &target_product=ATS|HRIS|PAYROLL|TIMESHEET (optional — defaults to "next-best" rule)
  &phase=P1|P2|P3|P4                         (AE phase context)

Response 200:
{
  "template_id": "TPL-CS-UPSELL-HRIS-TO-ATS-NPS9-P3",
  "variant": "hris_to_ats_promoter_p3",
  "reasons": [
    "current_product=HRIS, target=ATS (next-best rule)",
    "nps_score=9 → promoter tone",
    "phase=P3 (promo selling)",
    "selected: hris_to_ats_promoter_p3 variant"
  ],
  "fallback_used": false
}

Response 404:
{ "error": "no template variant matches product_type=X target=Y nps=Z phase=P" }
```

**Variant key convention:** `{current}_to_{target}_{nps_bucket}_{phase}` where
`nps_bucket = promoter (>=9) | passive (7-8) | detractor (<=6)`.

**Routing precedence (first-match wins):**

| # | Rule                                              | Notes                                |
|---|---------------------------------------------------|--------------------------------------|
| 1 | Exact match `(current, target, nps_bucket, phase)`| Most specific                        |
| 2 | Drop `phase` → `(current, target, nps_bucket)`    |                                      |
| 3 | Drop `nps_bucket` → `(current, target)`           |                                      |
| 4 | Generic `cs_upsell_default` for `(current, target)`| Final fallback (sets `fallback_used=true`) |

> **HOLD rule:** when `nps_score < 8` AND `phase IN ('P2','P3')`, the resolver
> returns 204 No Content instead of a template — caller must skip the send. This
> matches the "HOLD if NPS < 8" gate in `05-cron-engine.md`'s AE phase routing.
