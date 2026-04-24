# feat/08 — Activity Log

Three audit streams + a unified feed view.

## Status

**✅ 100%** — action log, master_data_mutations (100% write-coverage with
source tag), activity_log, team_activity_logs, unified `/activity-log/feed`
endpoint, escalation severity matrix from system_config.

## Streams

| Stream | Scope | Writer | FE endpoint |
|---|---|---|---|
| `action_log` | Bot actions (WA sends, reply classifications, escalations) | Cron + webhook handlers | `GET /activity-logs/recent` |
| `master_data_mutations` | Every write to master_data (create/edit/delete/transition/reactivate/handoff) | All master_data usecases | `GET /master-data/mutations` |
| `activity_log` | User-facing feature actions | Dashboard handlers | `GET /activity-logs` |
| `team_activity_logs` | Team-scoped (invites, role changes, permissions) | Team usecase | `GET /team/activity` |

## Unified feed

```
GET /activity-log/feed?limit=50
```

Returns newest-first UNION across `action_log + master_data_mutations +
activity_log` with per-source limit merge. Shape:

```json
{
  "data": [
    {
      "source": "mutation",
      "id": "uuid",
      "timestamp": "2026-04-24T...",
      "actor_email": "ae@example.com",
      "action": "edit_client",
      "resource": "master_data",
      "resource_id": "uuid",
      "company_id": "ACME-001",
      "company_name": "Acme Corp",
      "summary": "Updated contract_end",
      "mutation_source": "dashboard"
    },
    {
      "source": "action_log",
      "id": "uuid",
      "timestamp": "2026-04-24T...",
      "company_id": "BETA-001",
      "action": "wa_sent",
      "summary": "TPL-REN-90"
    },
    {
      "source": "activity_log",
      "id": "uuid",
      "timestamp": "2026-04-24T...",
      "actor_email": "admin@example.com",
      "action": "export_clients",
      "resource": "master_data"
    }
  ]
}
```

## Non-unified endpoints

### Action log (bot-initiated — dashboard sidebar)

Separate endpoints for the dashboard's bot-activity sidebar. Distinct from
`/activity-logs` which serves user-facing feature actions.

```
GET /action-log/recent?limit=50                      # last N bot actions
GET /action-log/today?limit=200                      # today's bot actions
GET /action-log/summary?hours=24                     # aggregated counts
GET /activity-log/today                              # alias to /action-log/today
```

Summary response:
```json
{
  "data": {
    "total": 128,
    "messages_sent": 92,
    "replies_received": 34,
    "escalations_fired": 5,
    "ae_notifications": 18,
    "reply_rate_pct": 36.96
  }
}
```

### Activity log (user-facing)
```
GET /activity-logs                                   # recent workspace activity
GET /activity-logs/recent                            # last 24h
GET /activity-logs/stats                             # aggregated counts
GET /activity-logs/companies/{company_id}/summary    # per-client summary
POST /activity-logs                                  # append (internal)
```

### Master data mutations
```
GET /master-data/mutations?since=2026-04-20&limit=100
```

### Team activity
```
GET /team/activity?limit=50
POST /team/activity
```

### Audit workspace access
See [../00-shared/05-audit.md](../00-shared/05-audit.md).

## Mutation source tagging

Every `master_data_mutations` row has a `source` field:

| Source | Meaning |
|---|---|
| `dashboard` | User edit from dashboard (most common) |
| `bot` | Automated bot write (payment flags, engagement state) |
| `import` | Bulk import |
| `api` | Direct API caller (e.g. handoff webhook) |
| `reactivation` | Reactivation trigger fired |
| `handoff` | BD→AE handoff auto-copy |

FE can filter/group the feed by source.

## Escalations

```
GET  /data-master/escalations
GET  /data-master/escalations/{id}
PUT  /data-master/escalations/{id}/resolve   {"note": "..."}
GET  /data-master/clients/{company_id}/escalations
```

Escalations table holds support-ticket-style entries. Severity is looked up
from `system_config[ESCALATION_SEVERITY_MATRIX]` — per-role × per-esc tier:

```json
{
  "default": "P2",
  "rules": {
    "ESC-001:ae": "P1",
    "ESC-003:ae": "P0",
    "ESC-005:*":  "P0"
  }
}
```

## FE UX

**Unified feed** (dashboard home widget):
- Chronological list, source pill color-coded
- Click-through to the source item (client, invoice, role, etc.)
- Filter by actor / action / source

**Per-client timeline** (client detail page):
- Only mutations + action_log for that company_id
- Group by day

**Team activity** (admin page):
- Same as above but filtered to `team_activity_logs`
- Show actor + target member

**Escalation inbox** (AE/admin):
- Open escalations with severity sorted P0 → P2
- "Resolve" button with note field
