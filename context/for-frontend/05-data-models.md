# Data Models Reference

Common response shapes and entity schemas FE will deserialize. JSON tags are
authoritative; names in parentheses are Go struct field names (for reference).

## Response envelope

Every response:
```json
{
  "status":     "success" | "failed",
  "entity":     "string, optional",
  "state":      "string, optional",
  "message":    "string",
  "data":       object | array | null,
  "meta":       { "total": number, "limit": number, "offset": number },
  "error_code": "string, only on failure",
  "errors":     { "field_name": ["err1", ...] }
}
```

## Pagination

Pass via query: `?limit=50&offset=0`. Response `meta`:
```json
{"total": 342, "limit": 50, "offset": 0}
```
- Max `limit` varies per endpoint (typically 200).
- `limit=0` or omitted → endpoint default (typically 50).

## Time format

All timestamps are RFC3339 UTC: `"2026-04-24T03:14:22Z"`. Send + receive as
ISO strings; FE renders in client timezone.

## UUIDs

All primary IDs are UUIDs rendered as strings. Exception: `invoice_id`,
`company_id`, `template_id` — human-readable business keys (e.g. `INV-2026-001`,
`ACME-CORP-001`, `TPL-OB-WELCOME`).

## Key entities

### Client / MasterData
```json
{
  "id": "uuid",
  "workspace_id": "uuid",
  "company_id": "ACME-001",
  "company_name": "Acme Corp",
  "stage": "lead|prospect|client|dormant",
  "pic_name": "string",
  "pic_nickname": "string",
  "pic_role": "string",
  "pic_wa": "628...",
  "pic_email": "string",
  "owner_name": "string",
  "owner_wa": "string",
  "owner_telegram_id": "string",
  "bot_active": bool,
  "blacklisted": bool,
  "sequence_status": "ACTIVE|LONGTERM|...",
  "snooze_until": "RFC3339 or null",
  "risk_flag": "None|Low|Mid|High",
  "contract_start": "RFC3339",
  "contract_end": "RFC3339",
  "contract_months": 12,
  "days_to_expiry": 45,
  "payment_status": "Pending|Lunas|Terlambat",
  "payment_terms": "Net 30",
  "final_price": 12000000,
  "last_payment_date": "RFC3339",
  "renewed": false,
  "last_interaction_date": "RFC3339",
  "notes": "string",
  "custom_fields": { "industry": "Tech", "hc_size": 250, ... },
  "created_at": "RFC3339",
  "updated_at": "RFC3339"
}
```

### Invoice
```json
{
  "invoice_id": "INV-2026-001",
  "workspace_id": "uuid",
  "company_id": "ACME-001",
  "company_name": "Acme Corp",
  "issue_date": "RFC3339",
  "due_date": "RFC3339",
  "amount": 12000000.00,
  "amount_paid": 0.00,
  "payment_status": "Pending|Lunas|Terlambat",
  "collection_stage": "stage0|stage1|stage2|stage3|stage4",
  "paper_id_url": "https://app.paper.id/...",
  "paper_id_ref": "PAPERID-...",
  "notes": "string",
  "line_items": [
    {"description": "...", "qty": 1, "unit_price": 12000000, "subtotal": 12000000}
  ],
  "created_at": "RFC3339"
}
```

### Workspace
```json
{
  "id": "uuid",
  "slug": "acme-id",
  "name": "Acme Indonesia",
  "description": "string",
  "holding_id": "uuid|null",
  "is_active": true,
  "settings": { /* opaque JSONB — FE owns shape */ },
  "created_at": "RFC3339",
  "updated_at": "RFC3339"
}
```

### UserPreference
```json
{
  "id": "uuid",
  "workspace_id": "uuid",
  "user_email": "string",
  "namespace": "theme|sidebar|columns.clients|...",
  "value": { /* opaque JSONB — FE owns shape */ },
  "created_at": "RFC3339",
  "updated_at": "RFC3339"
}
```

### WorkspaceIntegration
```json
{
  "id": "uuid",
  "workspace_id": "uuid",
  "provider": "haloai|telegram|paper_id|smtp",
  "display_name": "string",
  "config": {
    "api_url": "https://...",
    "wa_api_token": "***REDACTED***"  // redacted on read
  },
  "is_active": true,
  "created_by": "email",
  "updated_by": "email",
  "created_at": "RFC3339",
  "updated_at": "RFC3339"
}
```
**FE rule:** Never send `***REDACTED***` back on PUT. Omit the key to keep
the existing secret, or send the real new value.

### ApprovalRequest
```json
{
  "id": "uuid",
  "workspace_id": "uuid",
  "request_type": "create_invoice|mark_invoice_paid|collection_schema_change|delete_client_record|toggle_automation_rule|bulk_import_master_data|stage_transition|integration_key_change",
  "description": "string",
  "payload": { /* request-type-specific */ },
  "status": "pending|approved|rejected|expired",
  "maker_email": "string",
  "maker_at": "RFC3339",
  "checker_email": "string",
  "checker_at": "RFC3339",
  "rejection_reason": "string",
  "expires_at": "RFC3339",
  "applied_at": "RFC3339",
  "created_at": "RFC3339",
  "updated_at": "RFC3339"
}
```

### ManualAction
```json
{
  "id": "uuid",
  "workspace_id": "uuid",
  "master_data_id": "uuid",
  "trigger_id": "AE_P4_REN90_OPENER",
  "flow_category": "renewal_opener",
  "role": "sdr|bd|ae|admin",
  "assigned_to_user": "email",
  "suggested_draft": "string — pre-rendered template",
  "context_summary": { "nps_score": 9, "days_since_activation": 14, ... },
  "status": "pending|in_progress|sent|skipped|expired",
  "priority": "P0|P1|P2",
  "due_at": "RFC3339",
  "sent_at": "RFC3339|null",
  "sent_channel": "wa|email|call|meeting",
  "actual_message": "string",
  "skipped_reason": "string",
  "created_at": "RFC3339",
  "updated_at": "RFC3339"
}
```

### ActivityFeedEntry (unified `/activity-log/feed`)
```json
{
  "source": "action_log|mutation|activity_log",
  "id": "uuid",
  "timestamp": "RFC3339",
  "actor_email": "string|optional",
  "action": "string",
  "resource": "string|optional",
  "resource_id": "string|optional",
  "company_id": "ACME-001",
  "company_name": "Acme Corp",
  "summary": "string|optional",
  "mutation_source": "dashboard|bot|import|api|reactivation|handoff"  // only when source=mutation
}
```

### FirefliesTranscript
```json
{
  "id": "uuid",
  "workspace_id": "uuid",
  "fireflies_id": "ff-external-id",
  "meeting_title": "Discovery — Acme",
  "meeting_date": "RFC3339",
  "duration_seconds": 1800,
  "host_email": "bd@kantorku.id",
  "participants": ["bd@kantorku.id", "cfo@acme.co.id"],
  "transcript_text": "long string",
  "extraction_status": "pending|running|succeeded|failed",
  "extraction_error": "string",
  "extracted_at": "RFC3339|null",
  "master_data_id": "uuid|null",
  "created_at": "RFC3339",
  "updated_at": "RFC3339"
}
```

### ClaudeExtraction
```json
{
  "id": "uuid",
  "workspace_id": "uuid",
  "source_type": "fireflies|manual_note|email",
  "source_id": "string",
  "master_data_id": "uuid|null",
  "extracted_fields": {
    "company_size_estimate": "200-500 HC",
    "primary_pain_point": "Payroll accuracy",
    "decision_maker_role": "CFO",
    "competitor_mentioned": "talenta",
    "next_step": "Schedule product demo",
    "meeting_sentiment_note": "Leaning positive"
  },
  "extraction_prompt": "bd_extract_v1",
  "extraction_model": "mock-claude-sonnet-4-6",
  "bants_budget": 4,
  "bants_authority": 5,
  "bants_need": 4,
  "bants_timing": 4,
  "bants_sentiment": 4,
  "bants_score": 82.0,
  "bants_classification": "hot|warm|cold",
  "buying_intent": "high|medium|low",
  "coaching_notes": "string",
  "status": "pending|running|succeeded|failed|superseded",
  "prompt_tokens": 1234,
  "completion_tokens": 234,
  "latency_ms": 400,
  "created_at": "RFC3339",
  "updated_at": "RFC3339"
}
```

### Notification
```json
{
  "id": "uuid",
  "workspace_id": "uuid",
  "recipient_email": "string",
  "type": "info|warning|escalation|...",
  "icon": "string",
  "message": "string",
  "href": "/app/clients/...",
  "source_feature": "master_data|invoice|...",
  "source_id": "string",
  "read": false,
  "read_at": "RFC3339|null",
  "telegram_sent": false,
  "email_sent": false,
  "created_at": "RFC3339"
}
```

### PDP entities

**PDPErasureRequest:**
```json
{
  "id": "uuid",
  "workspace_id": "uuid",
  "subject_email": "string",
  "subject_kind": "contact|employee|lead|user",
  "requester": "string",
  "reason": "string",
  "scope": ["master_data", "action_log", "fireflies_transcripts", ...],
  "status": "pending|approved|executed|rejected|expired",
  "rejection_reason": "string",
  "reviewed_by": "email",
  "reviewed_at": "RFC3339|null",
  "executed_at": "RFC3339|null",
  "execution_summary": {
    "scope": [...],
    "subject": "...",
    "scrubbed": {"master_data_mutations": 14, "fireflies_transcripts": 3},
    "skipped_tables": []
  },
  "expires_at": "RFC3339",
  "created_at": "RFC3339",
  "updated_at": "RFC3339"
}
```

**PDPRetentionPolicy:**
```json
{
  "id": "uuid",
  "workspace_id": "uuid",
  "data_class": "action_log|master_data_mutations|fireflies_transcripts|...",
  "retention_days": 365,
  "action": "delete|anonymize|archive",
  "is_active": true,
  "last_run_at": "RFC3339|null",
  "last_run_rows": 0,
  "created_by": "email",
  "created_at": "RFC3339",
  "updated_at": "RFC3339"
}
```

### CollectionField
```json
{
  "id": "uuid",
  "collection_id": "uuid",
  "key": "price",
  "label": "Price",
  "type": "text|textarea|number|boolean|date|datetime|enum|multi_enum|url|email|link_client|file",
  "required": true,
  "options": {
    "choices": ["A", "B", "C"],      // for enum / multi_enum
    "min": 0, "max": 100,            // for number
    "maxLength": 255                 // for text / textarea / file
  },
  "default_value": null,
  "order": 1,
  "created_at": "RFC3339",
  "updated_at": "RFC3339"
}
```

### Analytics KPI Bundle response
```json
{
  "role": "ae|bd|sdr|admin",
  "kpi": { /* full KPIData shape from /analytics/kpi */ },
  "role_kpi": {
    "role": "ae",
    "metrics": { "active_clients": 123, "renewal_rate": 0.85, ... }
  },
  "distributions": { /* ... */ },
  "engagement": { /* ... */ },
  "revenue_trend": { /* ... */ },
  "forecast_accuracy": 0.92
}
```

Per-role metric keys:
- **SDR**: `leads_this_month`, `qualified_rate`, `avg_response_time_hours`, `active_prospects`
- **BD**: `prospects_in_pipeline`, `win_rate`, `avg_deal_cycle_days`, `closed_won_this_month`
- **AE**: `active_clients`, `renewal_rate`, `churn_rate`, `mrr`, `expansion_rate`, `overdue_invoices_count`
- **admin** (or unknown): full `kpi` payload returned under `role_kpi.metrics`

## JSONB custom fields

`master_data.custom_fields`, `collection_records.data`, `workspace.settings`,
`workspace_themes.theme`, `user_preferences.value` are all free-form JSONB.
FE owns the shape. BE enforces constraints only for `collection_records.data`
(against its collection's `CollectionField[]` schema).

## Enums

| Field | Values |
|---|---|
| `payment_status` | `Pending`, `Lunas`, `Terlambat`, `Ditangguhkan` |
| `collection_stage` | `stage0`, `stage1`, `stage2`, `stage3`, `stage4` |
| `stage` (master_data) | `lead`, `prospect`, `client`, `dormant` |
| `risk_flag` | `None`, `Low`, `Mid`, `High` |
| `sequence_status` | `ACTIVE`, `LONGTERM`, `PAUSED`, `COMPLETED` |
| `request_type` (approval) | see ApprovalRequest above |
| `source` (mutation) | `dashboard`, `bot`, `import`, `api`, `reactivation`, `handoff` |
| `access_kind` (audit) | `read`, `write`, `admin` |
| `manual_action.status` | `pending`, `in_progress`, `sent`, `skipped`, `expired` |
| `manual_action.priority` | `P0`, `P1`, `P2` |
| `manual_action.sent_channel` | `wa`, `email`, `call`, `meeting` |
| `rejection_category` | `price`, `authority`, `timing`, `feature`, `tone`, `other` |
| `rejection_severity` | `low`, `mid`, `high` |
| `pdp_erasure.status` | `pending`, `approved`, `executed`, `rejected`, `expired` |
| `pdp_retention.action` | `delete`, `anonymize`, `archive` |
| `coaching.status` | `draft`, `submitted`, `reviewed` |
| `coaching.session_type` | `peer_review`, `self_review`, `manager_review` |
| `claude.bants_classification` | `hot`, `warm`, `cold` |
| `claude.buying_intent` | `high`, `medium`, `low` |
