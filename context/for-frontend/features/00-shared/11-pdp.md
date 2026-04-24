# PDP Compliance

Two capabilities: **erasure requests** (right-to-be-forgotten / SAR) and
**retention policies** (auto-purge per data class). Both admin-scoped.

## Erasure requests

### Create
```
POST /pdp/erasure-requests
{
  "subject_email": "user@example.com",
  "subject_kind": "contact",          // contact|employee|lead|user
  "reason": "Client requested data deletion under PDP Pasal 13",
  "scope": [
    "master_data",
    "action_log",
    "master_data_mutations",
    "fireflies_transcripts"
  ]
}
```
If `scope` omitted, default is all 4 above.

### List
```
GET /pdp/erasure-requests?status=pending&limit=50&subject_email=user@example.com
```

### Get one
```
GET /pdp/erasure-requests/{id}
```

### Approve (different user required)
```
POST /pdp/erasure-requests/{id}/approve
```
- Cannot self-approve (requester ≠ reviewer)
- Moves status `pending → approved`

### Reject
```
POST /pdp/erasure-requests/{id}/reject
{"reason": "Insufficient documentation"}
```

### Execute (after approval)
```
POST /pdp/erasure-requests/{id}/execute
```
- Requires `status=approved`
- Runs real SQL DELETE across the scope tables where `actor_email` /
  `recipient_email` / `host_email` / `assigned_to_user` matches
- Returns with `status=executed`, `execution_summary` populated

Execution summary:
```json
{
  "scope": ["fireflies_transcripts", "master_data_mutations"],
  "subject": "user@example.com",
  "scrubbed": {
    "fireflies_transcripts": 3,
    "master_data_mutations": 14
  },
  "skipped_tables": []
}
```

### Lifecycle
```
pending → approved → executed
       ↓
     rejected
       ↓
     expired (30d default)
```

## Retention policies

Per-data-class auto-purge. Runs via `/cron/pdp/retention`.

### Upsert policy
```
POST /pdp/retention-policies
{
  "data_class": "action_log",
  "retention_days": 730,
  "action": "delete",                 // delete|anonymize|archive
  "is_active": true
}
```

### Supported data classes (whitelist)
- `action_log`
- `master_data_mutations`
- `fireflies_transcripts` (supports anonymize)
- `claude_extractions`
- `notifications`
- `manual_action_queue`
- `rejection_analysis_log`
- `audit_logs_workspace_access`
- `reactivation_events`

### List policies
```
GET /pdp/retention-policies?active=true
```

### Delete policy
```
DELETE /pdp/retention-policies/{id}
```

### Trigger retention run (admin or cron)
```
GET /cron/pdp/retention
```
Returns summary:
```json
{
  "workspace_id": "...",
  "total_rows": 342,
  "details": {
    "action_log": 128,
    "master_data_mutations": 204,
    "fireflies_transcripts": 10
  }
}
```

## FE UX

**Compliance admin page:**
- Erasure requests inbox with filter chips (pending/approved/executed/rejected)
- Create new erasure form
- Retention policies table (one row per data_class) with toggle + edit

**Approval flow:**
- Requester submits → pending
- Reviewer (different user) opens request → "Approve" or "Reject" buttons
- After approve → "Execute" button appears (destructive, confirm dialog)

**Execution summary display:**
- Show `scrubbed` count per table as a friendly list
- Warn about `skipped_tables` (data class not mapped for scrubbing)

## Rate-limit + safety

- Erasure requests auto-expire after 30 days (configurable per row)
- Retention run is idempotent (safe to re-trigger)
- Policies with `retention_days ≤ 0` mean "keep forever" (no-op)
