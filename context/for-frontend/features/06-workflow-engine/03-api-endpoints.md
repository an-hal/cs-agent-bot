# Workflow Engine — API Endpoints

Base prefix: `/api/v1`
Auth: `Authorization: Bearer {jwt}` + `X-Workspace-ID: {uuid}`

All responses use the standard envelope `{ success, message, data, meta? }`.

---

## 1. Workflows

### `GET /workflows`

List all workflows for the active workspace.

**Query params**

| Name     | Type   | Notes                               |
|----------|--------|-------------------------------------|
| `status` | string | `active` \| `draft` \| `disabled`  |

**Response 200**

```json
{
  "success": true,
  "message": "Workflows",
  "data": [
    {
      "id": "a1b2c3d4-...",
      "workspace_id": "ws-uuid",
      "name": "AE Client Lifecycle",
      "icon": "🏆",
      "slug": "406e6b25-37f6-4531-aade-aa42df2d52a3",
      "description": "Onboarding, check-in, renewal negotiation...",
      "status": "active",
      "stage_filter": ["CLIENT"],
      "node_count": 25,
      "edge_count": 23,
      "created_at": "2026-04-01T00:00:00Z",
      "updated_at": "2026-04-09T10:00:00Z",
      "updated_by": "budi@kantorku.id"
    }
  ],
  "meta": { "total": 4 }
}
```

---

### `GET /workflows/{id}`

Get a single workflow with all nested data (nodes, edges, steps, tabs, stats, columns).

**Response 200**

```json
{
  "success": true,
  "message": "Workflow",
  "data": {
    "id": "a1b2c3d4-...",
    "name": "AE Client Lifecycle",
    "status": "active",
    "stage_filter": ["CLIENT"],
    "nodes": [ { "node_id": "ae-p01", "node_type": "workflow", "data": { "category": "action", "label": "P0.1 Welcome" }, "position_x": 0, "position_y": 0, "draggable": true, "selectable": true, "connectable": true, "z_index": 0 } ],
    "edges": [ { "edge_id": "ae-e01", "source": "ae-p01", "target": "ae-p02", "animated": true } ],
    "steps": [ { "step_key": "p0", "label": "P0 Onboarding", "phase": "P0", "timing": "D+0 to D+35" } ],
    "tabs": [ { "tab_key": "semua", "label": "Semua Client", "filter": "all" } ],
    "stats": [ { "stat_key": "total", "label": "Total Client", "metric": "count" } ],
    "columns": [ { "col_key": "Company_Name", "field": "Company_Name", "label": "Company", "width": 220, "visible": true } ]
  }
}
```

**Response 404**

```json
{ "success": false, "error": { "code": "NOT_FOUND", "message": "Workflow not found" } }
```

---

### `GET /workflows/by-slug/{slug}`

Get workflow by slug — used by the frontend route `/pipeline/{slug}`.

Same response shape as `GET /workflows/{id}`.

---

### `POST /workflows`

Create a new workflow.

**Request body**

```json
{
  "name": "Custom Pipeline",
  "icon": "⚡",
  "description": "Custom automation workflow",
  "status": "draft",
  "stage_filter": ["LEAD"]
}
```

**Response 201**

```json
{
  "success": true,
  "message": "Workflow created",
  "data": {
    "id": "new-uuid",
    "slug": "custom-pipeline",
    "status": "draft",
    "stage_filter": ["LEAD"],
    "created_at": "2026-04-14T10:00:00Z"
  }
}
```

---

### `PUT /workflows/{id}`

Update workflow metadata. Send only the fields you want to change.

**Request body (partial)**

```json
{
  "name": "Updated Pipeline Name",
  "status": "active"
}
```

**Response 200**

```json
{ "success": true, "message": "Updated", "data": null }
```

---

### `DELETE /workflows/{id}`

Delete workflow and all cascade data (nodes, edges, steps, tabs, stats, columns).

**Response 200**

```json
{
  "success": true,
  "message": "Deleted",
  "data": { "message": "Deleted", "id": "uuid" }
}
```

---

## 2. Canvas (Nodes & Edges)

### `PUT /workflows/{id}/canvas`

Save the entire canvas — replaces all existing nodes and edges atomically
in a single transaction.

**Request body**

```json
{
  "nodes": [
    {
      "node_id": "ae-p01",
      "node_type": "workflow",
      "position_x": 0,
      "position_y": 0,
      "data": {
        "category": "action",
        "label": "P0.1 Welcome",
        "icon": "👋",
        "templateId": "TPL-OB-WELCOME",
        "triggerId": "Onboarding_Welcome",
        "timing": "D+0 to D+5",
        "condition": "days_since_activation BETWEEN 0 AND 5 AND onboarding_sent = FALSE",
        "sentFlag": "onboarding_sent"
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
```

**Response 200**

```json
{
  "success": true,
  "message": "Canvas saved",
  "data": {
    "workflow_id": "uuid",
    "node_count": 25,
    "edge_count": 30,
    "saved_at": "2026-04-12T10:00:00Z"
  }
}
```

---

## 3. Pipeline Steps

### `GET /workflows/{id}/steps`

List all steps ordered by `sort_order`.

**Response 200**

```json
{
  "success": true,
  "message": "Steps",
  "data": [
    {
      "step_key": "p0",
      "label": "P0 Onboarding",
      "phase": "P0",
      "timing": "D+0 to D+35",
      "condition": "days_since_activation BETWEEN 0 AND 35",
      "template_id": "TPL-OB-WELCOME",
      "message_template_id": "TPL-OB-WELCOME",
      "email_template_id": null
    }
  ]
}
```

---

### `PUT /workflows/{id}/steps`

Bulk replace all steps (replaces existing set).

**Request body**

```json
{
  "steps": [
    {
      "step_key": "p0",
      "label": "P0 Onboarding",
      "phase": "P0",
      "icon": "👋",
      "timing": "D+0 to D+35",
      "condition": "days_since_activation BETWEEN 0 AND 35",
      "stop_if": "",
      "sent_flag": "onboarding_sent",
      "template_id": "TPL-OB-WELCOME",
      "message_template_id": "TPL-OB-WELCOME",
      "email_template_id": null
    }
  ]
}
```

**Response 200** — `{ "success": true, "message": "Steps saved", "data": null }`

---

### `GET /workflows/{id}/steps/{stepKey}`

Get a single step by its key.

**Response 200** — `{ "data": WorkflowStep }`

**Response 404** — step not found

---

### `PUT /workflows/{id}/steps/{stepKey}`

Update a single step (partial patch). Used by the Step Config page.

**Request body (partial)**

```json
{
  "timing": "D+0 to D+7",
  "condition": "days_since_activation BETWEEN 0 AND 7 AND onboarding_sent = FALSE",
  "template_id": "TPL-OB-WELCOME",
  "message_template_id": "TPL-OB-WELCOME",
  "email_template_id": "ETPL-KK-AE-001"
}
```

**Response 200** — `{ "success": true, "message": "Step updated", "data": null }`

---

## 4. Pipeline Config (Tabs, Stats, Columns)

### `GET /workflows/{id}/config`

Get tabs + stats + columns for a workflow.

**Response 200**

```json
{
  "success": true,
  "message": "Config",
  "data": {
    "tabs": [
      { "tab_key": "semua", "label": "Semua Client", "icon": "📋", "filter": "all", "sort_order": 0 },
      { "tab_key": "aktif", "label": "Bot Aktif",    "icon": "🟢",  "filter": "bot_active", "sort_order": 1 },
      { "tab_key": "renewal","label": "Renewal",      "icon": "📅", "filter": "expiry:30", "sort_order": 2 },
      { "tab_key": "perhatian","label": "Perhatian",  "icon": "⚠️","filter": "risk", "sort_order": 3 }
    ],
    "stats": [
      { "stat_key": "total",   "label": "Total Client",  "metric": "count",           "color": "text-emerald-400" },
      { "stat_key": "revenue", "label": "Total Revenue",  "metric": "sum:final_price", "color": "text-brand-400"   },
      { "stat_key": "renewal", "label": "Renewal <=30d", "metric": "count:expiry:30", "color": "text-amber-400"   },
      { "stat_key": "risk",    "label": "Perhatian",     "metric": "count:risk",      "color": "text-rose-400"    }
    ],
    "columns": [
      { "col_key": "Company_Name", "field": "Company_Name", "label": "Company", "width": 220, "visible": true },
      { "col_key": "Stage",        "field": "Stage",        "label": "Stage",   "width": 90,  "visible": true },
      { "col_key": "Bot_Active",   "field": "Bot_Active",   "label": "Bot",     "width": 60,  "visible": true }
    ]
  }
}
```

---

### `PUT /workflows/{id}/tabs`

Bulk replace tabs.

**Request body**

```json
{
  "tabs": [
    { "tab_key": "semua",    "label": "Semua Client", "icon": "📋", "filter": "all" },
    { "tab_key": "aktif",    "label": "Bot Aktif",    "icon": "🟢",  "filter": "bot_active" },
    { "tab_key": "renewal",  "label": "Renewal",      "icon": "📅", "filter": "expiry:30" },
    { "tab_key": "perhatian","label": "Perhatian",    "icon": "⚠️","filter": "risk" }
  ]
}
```

**Response 200** — `{ "success": true, "message": "Tabs saved" }`

---

### `PUT /workflows/{id}/stats`

Bulk replace stat cards.

**Request body**

```json
{
  "stats": [
    { "stat_key": "total",   "label": "Total Client",  "metric": "count",           "color": "text-emerald-400", "border": "border-emerald-500/20" },
    { "stat_key": "revenue", "label": "Total Revenue",  "metric": "sum:final_price", "color": "text-brand-400",   "border": "border-brand-400/20" },
    { "stat_key": "renewal", "label": "Renewal <=30d", "metric": "count:expiry:30", "color": "text-amber-400",   "border": "border-amber-500/20" },
    { "stat_key": "risk",    "label": "Perhatian",     "metric": "count:risk",      "color": "text-rose-400",    "border": "border-rose-500/20" }
  ]
}
```

**Response 200** — `{ "success": true, "message": "Stats saved" }`

---

### `PUT /workflows/{id}/columns`

Bulk replace column config.

**Request body**

```json
{
  "columns": [
    { "col_key": "Company_Name", "field": "Company_Name", "label": "Company", "width": 220, "visible": true },
    { "col_key": "Stage",        "field": "Stage",        "label": "Stage",   "width": 90,  "visible": true },
    { "col_key": "Bot_Active",   "field": "Bot_Active",   "label": "Bot",     "width": 60,  "visible": true },
    { "col_key": "Risk_Flag",    "field": "Risk_Flag",    "label": "Risk",    "width": 70,  "visible": false }
  ]
}
```

**Response 200** — `{ "success": true, "message": "Columns saved" }`

---

## 5. Pipeline Data

### `GET /workflows/{id}/data`

Get paginated master_data records filtered by the workflow's `stage_filter`,
the selected tab DSL, and an optional search term. Includes computed stats.

**Query params**

| Name       | Type   | Default    | Notes                                      |
|------------|--------|------------|--------------------------------------------|
| `tab`      | string | `semua`    | Tab key from workflow config               |
| `search`   | string | —          | ILIKE on company_name, pic_name, company_id|
| `offset`   | int    | `0`        |                                            |
| `limit`    | int    | `20`       | Max 200                                    |
| `sort_by`  | string | `updated_at`| Column to sort on                         |
| `sort_dir` | string | `desc`     | `asc` or `desc`                            |

**Response 200**

```json
{
  "success": true,
  "message": "Pipeline data",
  "data": {
    "data": [
      {
        "id": "md-uuid",
        "company_id": "KK-001",
        "company_name": "PT Dealls Tech",
        "stage": "CLIENT",
        "bot_active": true,
        "payment_status": "Lunas",
        "days_to_expiry": 15,
        "final_price": 4500000
      }
    ],
    "total": 85,
    "stats": {
      "total":   { "value": "85",       "sub": "total records" },
      "revenue": { "value": "4700000",  "sub": "dari 85 records" },
      "renewal": { "value": "12",       "sub": "dari 85 total" },
      "risk":    { "value": "8",        "sub": "dari 85 total" }
    }
  },
  "meta": { "offset": 0, "limit": 20, "total": 85 }
}
```

---

## 6. Automation Rules

### `GET /automation-rules`

List automation rules for the active workspace.

**Query params**

| Name     | Type   | Notes                                          |
|----------|--------|------------------------------------------------|
| `role`   | string | `sdr` \| `bd` \| `ae` \| `cs`                 |
| `status` | string | `active` \| `paused` \| `disabled`             |
| `phase`  | string | e.g. `P0`, `P1`                               |
| `search` | string | ILIKE on rule_code, trigger_id, condition      |

**Response 200**

```json
{
  "success": true,
  "message": "Automation rules",
  "data": [
    {
      "id": "rule-uuid",
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
      "updated_by": "arief@dealls.com"
    }
  ],
  "meta": { "total": 42 }
}
```

---

### `GET /automation-rules/{id}`

Get a single rule with its change log history.

**Response 200**

```json
{
  "success": true,
  "message": "Automation rule",
  "data": {
    "data": { "...AutomationRule..." },
    "change_logs": [
      {
        "field": "timing",
        "old_value": "H-90",
        "new_value": "H-90 to H-85",
        "edited_by": "reza@dealls.com",
        "edited_at": "2026-04-05T14:20:00Z"
      }
    ]
  }
}
```

---

### `POST /automation-rules`

Create a new rule.

**Request body**

```json
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
```

**Response 201** — `{ "data": AutomationRule }`

**Response 400** — validation error (missing required fields)

**Response 409** — `{ "error": { "message": "Rule code already exists" } }` (duplicate rule_code)

---

### `PUT /automation-rules/{id}`

Update rule fields. Send only the fields to change.

**Field changes (timing, condition, template, etc.) — applied directly:**

```json
{
  "timing": "D+0 to D+7",
  "condition": "days_since_activation BETWEEN 0 AND 7"
}
```

**Response 200**

```json
{
  "success": true,
  "message": "Updated",
  "data": {
    "data": { "...updated AutomationRule..." },
    "changes": [
      { "field": "timing", "old_value": "D+0 to D+5", "new_value": "D+0 to D+7" }
    ]
  }
}
```

**Status toggle (active ↔ paused) — requires approval:**

```json
{ "status": "paused" }
```

**Response 400** (approval gate)

```json
{
  "success": false,
  "error": {
    "code": "VALIDATION",
    "message": "status change requires approval — approval request created"
  }
}
```

An `ApprovalRequest` row is created in the database. The checker must
approve via the approvals endpoint before the status change takes effect.

---

### `DELETE /automation-rules/{id}`

Delete rule and all its change logs.

**Response 200**

```json
{ "success": true, "message": "Deleted", "data": { "message": "Deleted", "id": "uuid" } }
```

---

### `GET /automation-rules/change-logs`

Get the workspace-level change log feed for all rules.

**Query params**

| Name    | Type   | Default | Notes                  |
|---------|--------|---------|------------------------|
| `limit` | int    | `50`    | Max entries to return  |

**Response 200**

```json
{
  "success": true,
  "message": "Change logs",
  "data": [
    {
      "id": "log-uuid",
      "rule_id": "rule-uuid",
      "rule_code": "RULE-KK-AE-REN-001",
      "workspace_id": "ws-uuid",
      "field": "timing",
      "old_value": "H-90",
      "new_value": "H-90 to H-85",
      "edited_by": "reza@dealls.com",
      "edited_at": "2026-04-05T14:20:00Z"
    }
  ],
  "meta": { "total": 12 }
}
```

---

## Error Reference

| HTTP | Code            | Meaning                                  |
|------|-----------------|------------------------------------------|
| 400  | `VALIDATION`    | Missing required field or format error   |
| 400  | `VALIDATION`    | Status toggle requires approval          |
| 404  | `NOT_FOUND`     | Workflow or rule not found               |
| 409  | `CONFLICT`      | Duplicate rule_code in workspace         |
| 500  | `INTERNAL`      | Unexpected server error                  |
