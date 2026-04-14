# Workflow Engine — Overview

The Workflow Engine (feat/06) provides three integrated surfaces for the
Kantorku.id HRIS Dashboard:

1. **Workflow Builder** — visual canvas (React Flow) to construct and edit
   automation pipelines
2. **Pipeline View** — per-workflow table of master_data records with
   configurable tabs, stat cards, and columns
3. **Automation Rules** — a library of executable rules with change-log
   audit trail and checker-maker approval for status toggles

All endpoints live under `/api/v1/workflows` and `/api/v1/automation-rules`
in `internal/delivery/http/route.go` and require:

- `Authorization: Bearer {jwt}`
- `X-Workspace-ID: {uuid}`

The feature is gated by `USE_WORKFLOW_ENGINE=true` in the server environment.
When disabled, the cron engine runs the legacy P0–P5 trigger sequence only.
When enabled, the workflow runner runs **additively after** P0–P5 — it never
replaces the existing logic.

---

## Data Domains

```
workflows             ── top-level pipeline definition (name, icon, slug, status, stage_filter)
workflow_nodes        ── React Flow canvas nodes (position, data JSONB)
workflow_edges        ── React Flow canvas edges (source, target, style)
workflow_steps        ── pipeline step configs (timing, condition, template refs)
pipeline_tabs         ── tab filter configs per workflow
pipeline_stats        ── stat card configs per workflow
pipeline_columns      ── visible column configs per workflow
automation_rules      ── executable rules evaluated by cron/events
rule_change_logs      ── INSERT-only audit trail for rule field changes
```

---

## Four Default Workflows (Pre-seeded)

Every non-holding workspace ships with four workflows:

| UUID slug                              | Name                 | Stage Filter        |
|----------------------------------------|----------------------|---------------------|
| `4fc22c98-1e3b-4901-aa86-9f81b33354d2`| SDR Lead Outreach    | LEAD, DORMANT       |
| `0c85261e-277c-4143-93b3-bb6714eaff08`| BD Deal Closing      | PROSPECT            |
| `406e6b25-37f6-4531-aade-aa42df2d52a3`| AE Client Lifecycle  | CLIENT              |
| `01400f6a-cdc9-43a0-8409-b96e316bec91`| CS Customer Support  | CLIENT              |

The frontend can navigate to a workflow by slug via
`GET /api/v1/workflows/by-slug/{slug}`.

---

## Stage Routing

A record's `stage` field determines which workflow processes it during
the cron run:

| Stage      | Pipeline             |
|------------|----------------------|
| `LEAD`     | SDR Lead Outreach    |
| `DORMANT`  | SDR Lead Outreach    |
| `PROSPECT` | BD Deal Closing      |
| `CLIENT`   | AE Client Lifecycle  |
| `CLIENT`   | CS Customer Support  |

---

## Node Types

Canvas nodes have a `type` and a `category` (inside the `data` JSONB):

| type       | category    | Description                      |
|------------|-------------|----------------------------------|
| `workflow` | `trigger`   | Entry point / event              |
| `workflow` | `action`    | Send message / update data       |
| `workflow` | `condition` | Branch / decision                |
| `workflow` | `delay`     | Wait / timing control            |
| `zone`     | —           | Visual grouping only (non-exec)  |

---

## Automation Rule Status and Checker-Maker

Automation rule status can be `active`, `paused`, or `disabled`.

- Changing status between `active` ↔ `paused` **requires approval** via the
  checker-maker system. The API returns a 400 with message
  `"status change requires approval — approval request created"` and creates
  an `ApprovalRequest` row.
- All other field changes (timing, condition, template, etc.) are applied
  directly and appended to `rule_change_logs`.

---

## Filter DSL (Tabs and Stats)

Tab filters and stat card metrics use a short DSL string stored in the
database. The backend parses these at query time — the frontend only needs
to store and display them.

**Filter prefixes:**
- `all` — no additional filter
- `bot_active` — bot_active = TRUE
- `risk` — risk_flag IN ('High','Mid') OR bot_active = FALSE OR payment_status IN ('Overdue','Terlambat')
- `stage:{v1},{v2}` — stage IN (...)
- `value_tier:{v1},{v2}` — custom_fields value tier IN (...)
- `payment:{value}` — payment_status = value
- `expiry:{days}` — days_to_expiry <= days
- `sequence:{value}` — sequence_status = value

**Metric DSL:**
- `count` — COUNT(*) in workspace
- `count:{filter}` — COUNT(*) with filter applied
- `sum:{field}` — SUM of a numeric field (e.g. `sum:final_price`)
- `avg:{field}` — AVG of a numeric field

---

## Response Envelope

All endpoints use the standard envelope:

```json
{
  "success": true,
  "message": "...",
  "data": { ... },
  "meta": { ... }
}
```

Error responses:

```json
{
  "success": false,
  "error": { "code": "NOT_FOUND", "message": "Workflow not found" }
}
```
