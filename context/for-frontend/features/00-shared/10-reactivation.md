# Reactivation Triggers

Configurable per-workspace rules that decide when a dormant client is
re-engaged. Admins define triggers (price_change, new_feature, anniversary,
manual); a cron evaluates + fires, or admin fires manually.

## Endpoints

### Trigger management (admin)

```
GET    /reactivation/triggers?active=true
POST   /reactivation/triggers
GET    /reactivation/triggers/{id}
DELETE /reactivation/triggers/{id}
```

POST/upsert body:
```json
{
  "code": "price_change",
  "name": "Price change re-engage",
  "description": "Fire when workspace pricing changes",
  "condition": "-",                    // condition DSL (see pkg/conditiondsl)
  "template_code": "REACTIVATE_PRICE", // message_templates.code to send
  "is_active": true
}
```

### Fire reactivation for a client
```
POST /master-data/clients/{id}/reactivate
{"trigger_code": "manual", "note": "Ad-hoc outreach after price update"}
```

**Rate limit:** Max 1 event per client+code per 30 days. The `manual` code
bypasses the rate limit (admin override).

If rate-limited: `409 CONFLICT` — "reactivation for this client+code already
fired within the last 30 days".

### History for a client
```
GET /master-data/clients/{id}/reactivation-history?limit=50
```
Returns `ReactivationEvent[]` — fired_at, trigger_id, outcome (sent/skipped/replied/bounced).

## FE UX

**Admin trigger config:**
- List all triggers with active toggle
- Create form: code (slug), name, description, template picker
- Condition DSL editor (advanced; optional)

**Client detail page:**
- "Reactivate" button (opens trigger picker + note)
- "History" tab shows all past reactivation events for this client

**Dashboard widget:**
- Count of reactivations fired this week / month
- Most effective trigger (reply rate)

## Mutation log integration

Every successful reactivation also writes a `master_data_mutations` row
with `action=reactivate_client`, `source=reactivation`. Shows up in
unified activity feed.
