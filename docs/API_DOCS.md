# CS Agent Bot - API Documentation

> Base URL: `http://localhost:8081`
> Route Prefix: `/api` (configurable via `APP_ROUTE_PREFIX`)

## Authentication

### JWT Auth (Dashboard endpoints)

```
Authorization: Bearer <JWT_TOKEN>
```

### Workspace ID (Most dashboard endpoints)

```
X-Workspace-ID: <WORKSPACE_UUID>
```

### HMAC Auth (Webhook endpoints)

```
X-Handoff-Secret: <HMAC_SHA256_SIGNATURE>
```

---

## Response Format

### Success (single resource)

```json
{
  "requestId": "uuid",
  "traceId": "hex",
  "status": "success",
  "message": "Description",
  "data": { ... }
}
```

### Success (paginated list)

```json
{
  "requestId": "uuid",
  "traceId": "hex",
  "status": "success",
  "message": "Description",
  "meta": {
    "offset": 0,
    "limit": 10,
    "total": 100
  },
  "data": [ ... ]
}
```

### Error

```json
{
  "requestId": "uuid",
  "traceId": "hex",
  "status": "error",
  "message": "Error description",
  "errorCode": "NOT_FOUND"
}
```

---

## Health Check

### GET /readiness

```bash
curl http://localhost:8081/readiness
```

---

## Workspaces

### GET /api/workspaces

List all workspaces for the authenticated user.

```bash
curl 'http://localhost:8081/api/workspaces' \
  -H 'Authorization: Bearer <JWT_TOKEN>'
```

**Response:**

```json
{
  "status": "success",
  "data": [
    {
      "id": "6c710989-7024-4bb9-9c86-6e60f1315cf3",
      "slug": "my-workspace",
      "name": "My Workspace",
      "logo": "",
      "color": "#FF5733",
      "plan": "premium",
      "is_holding": false,
      "member_ids": ["user-id-1", "user-id-2"],
      "created_at": "2026-01-01T00:00:00Z"
    }
  ]
}
```

---

## Clients

### GET /api/data-master/clients

List clients with filters and pagination.

```bash
curl 'http://localhost:8081/api/data-master/clients?offset=0&limit=10' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3'
```

**Query Parameters:**

| Param | Type | Description |
|-------|------|-------------|
| `search` | string | Search company_id, company_name |
| `segment` | string | Filter by segment |
| `payment_status` | string | Filter by payment status |
| `sequence_cs` | string | Filter by CS sequence |
| `plan_type` | string | Filter by plan type |
| `bot_active` | bool | Filter by bot active status |
| `offset` | int | Pagination offset (default: 0) |
| `limit` | int | Page size (default: 10, max: 100) |

**With filters:**

```bash
curl 'http://localhost:8081/api/data-master/clients?segment=Enterprise&payment_status=paid&offset=0&limit=20' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3'
```

---

### GET /api/data-master/clients/{company_id}

Get a single client by company ID.

```bash
curl 'http://localhost:8081/api/data-master/clients/COMP-001' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3'
```

**Response:**

```json
{
  "status": "success",
  "data": {
    "company_id": "COMP-001",
    "company_name": "PT Example Corp",
    "pic_name": "John Doe",
    "pic_wa": "6281234567890",
    "pic_email": "john@example.com",
    "pic_role": "HR Manager",
    "owner_name": "Jane Smith",
    "owner_wa": "6281234567891",
    "owner_telegram_id": "123456789",
    "segment": "Enterprise",
    "plan_type": "Premium",
    "contract_months": 12,
    "contract_start": "2026-01-01T00:00:00Z",
    "contract_end": "2026-12-31T00:00:00Z",
    "activation_date": "2026-01-15T00:00:00Z",
    "payment_status": "paid",
    "nps_score": 8,
    "usage_score": 75,
    "bot_active": true,
    "blacklisted": false,
    "renewed": false,
    "rejected": false,
    "checkin_replied": true,
    "response_status": "active"
  }
}
```

---

### POST /api/data-master/clients

Create a new client.

```bash
curl -X POST 'http://localhost:8081/api/data-master/clients' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3' \
  -H 'Content-Type: application/json' \
  -d '{
    "company_id": "COMP-002",
    "company_name": "PT New Client",
    "pic_name": "Ahmad",
    "pic_wa": "6281234567890",
    "pic_email": "ahmad@newclient.com",
    "pic_role": "HR Manager",
    "owner_name": "Budi",
    "owner_wa": "6281234567891",
    "owner_telegram_id": "987654321",
    "segment": "SMB",
    "plan_type": "Basic",
    "hc_size": "50-100"
  }'
```

**Required fields:** `company_id`, `company_name`

---

### PUT /api/data-master/clients/{company_id}

Partially update a client. Send only fields you want to change.

```bash
curl -X PUT 'http://localhost:8081/api/data-master/clients/COMP-001' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3' \
  -H 'Content-Type: application/json' \
  -d '{
    "segment": "Enterprise",
    "payment_status": "paid",
    "bot_active": true
  }'
```

---

### DELETE /api/data-master/clients/{company_id}

Soft-delete a client (sets blacklisted=true).

```bash
curl -X DELETE 'http://localhost:8081/api/data-master/clients/COMP-001' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3'
```

---

## Client Import / Export

### POST /api/data-master/clients/import

Upload an XLSX file to import clients.

```bash
curl -X POST 'http://localhost:8081/api/data-master/clients/import' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3' \
  -F 'file=@/path/to/clients.xlsx'
```

**With update_existing flag:**

```bash
curl -X POST 'http://localhost:8081/api/data-master/clients/import?update_existing=true' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3' \
  -F 'file=@/path/to/clients.xlsx'
```

**Response (202 Accepted):**

```json
{
  "status": "success",
  "message": "Import started",
  "data": {
    "job_id": "abc-123",
    "workspace_id": "6c710989-7024-4bb9-9c86-6e60f1315cf3",
    "job_type": "import",
    "status": "pending",
    "entity_type": "client",
    "filename": "clients.xlsx",
    "total_rows": 0,
    "processed": 0,
    "success": 0,
    "failed": 0,
    "skipped": 0,
    "created_by": "user@example.com",
    "created_at": "2026-04-09T10:00:00Z",
    "updated_at": "2026-04-09T10:00:00Z"
  }
}
```

**Max file size:** 5 MB

---

### POST /api/data-master/clients/export

Start a background export of clients to XLSX.

```bash
curl -X POST 'http://localhost:8081/api/data-master/clients/export' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3'
```

**With filters:**

```bash
curl -X POST 'http://localhost:8081/api/data-master/clients/export?segment=Enterprise&payment_status=paid' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3'
```

**Query Parameters:**

| Param | Type | Description |
|-------|------|-------------|
| `search` | string | Search filter |
| `segment` | string | Filter by segment |
| `payment_status` | string | Filter by payment status |
| `sequence_cs` | string | Filter by CS sequence |
| `plan_type` | string | Filter by plan type |

**Response (202 Accepted):** Same as import job response.

---

## Background Jobs

### GET /api/jobs

List background jobs for the workspace.

```bash
curl 'http://localhost:8081/api/jobs?offset=0&limit=10' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3'
```

**Query Parameters:**

| Param | Type | Description |
|-------|------|-------------|
| `job_type` | string | Filter: `import` or `export` |
| `entity_type` | string | Filter: `client` |
| `offset` | int | Pagination offset |
| `limit` | int | Page size |

---

### GET /api/jobs/{job_id}

Get the status of a specific background job.

```bash
curl 'http://localhost:8081/api/jobs/abc-123' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3'
```

**Response:**

```json
{
  "status": "success",
  "data": {
    "job_id": "abc-123",
    "workspace_id": "6c710989-7024-4bb9-9c86-6e60f1315cf3",
    "job_type": "export",
    "status": "done",
    "entity_type": "client",
    "filename": "export_clients_20260409_100000.xlsx",
    "total_rows": 150,
    "processed": 150,
    "success": 150,
    "failed": 0,
    "skipped": 0,
    "created_by": "user@example.com",
    "created_at": "2026-04-09T10:00:00Z",
    "updated_at": "2026-04-09T10:00:30Z"
  }
}
```

---

### GET /api/jobs/{job_id}/download

Download the XLSX file from a completed export job.

```bash
curl 'http://localhost:8081/api/jobs/abc-123/download' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3' \
  -o export.xlsx
```

**Response:** Binary XLSX file  
**Content-Type:** `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`

---

## Activity Logs

### GET /api/activity-logs

List activity logs with optional filters.

```bash
curl 'http://localhost:8081/api/activity-logs?offset=0&limit=10' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3'
```

**Query Parameters:**

| Param | Type | Description |
|-------|------|-------------|
| `category` | string | Filter: `bot`, `data`, `team` |
| `since` | string | ISO 8601 timestamp, entries after this time |
| `offset` | int | Pagination offset |
| `limit` | int | Page size |

**With filters:**

```bash
curl 'http://localhost:8081/api/activity-logs?category=bot&since=2026-04-01T00:00:00Z&offset=0&limit=20' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3'
```

**Response:**

```json
{
  "status": "success",
  "meta": { "offset": 0, "limit": 10, "total": 50 },
  "data": [
    {
      "id": 1,
      "workspace_id": "6c710989-7024-4bb9-9c86-6e60f1315cf3",
      "category": "bot",
      "actor_type": "system",
      "actor": "cron",
      "action": "send_checkin",
      "target": "COMP-001",
      "detail": "Sent check-in message",
      "ref_id": "",
      "status": "success",
      "occurred_at": "2026-04-09T08:00:00Z"
    }
  ]
}
```

---

### POST /api/activity-logs

Record a new activity log.

```bash
curl -X POST 'http://localhost:8081/api/activity-logs' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3' \
  -H 'Content-Type: application/json' \
  -d '{
    "category": "data",
    "action": "update_client",
    "target": "COMP-001",
    "detail": "Updated payment status to paid",
    "ref_id": "COMP-001",
    "status": "success"
  }'
```

**Required fields:** `category` (must be `data` or `team`), `action`

---

## Invoices

### GET /api/data-master/invoices

List invoices with filters.

```bash
curl 'http://localhost:8081/api/data-master/invoices?offset=0&limit=10' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3'
```

**Query Parameters:**

| Param | Type | Description |
|-------|------|-------------|
| `company_id` | string | Filter by company |
| `status` | string | Filter by payment status |
| `search` | string | Search invoice_id, company_id, notes |
| `collection_stage` | string | Filter by collection stage |
| `offset` | int | Pagination offset |
| `limit` | int | Page size |

**With filters:**

```bash
curl 'http://localhost:8081/api/data-master/invoices?company_id=COMP-001&status=unpaid' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3'
```

**Response:**

```json
{
  "status": "success",
  "meta": { "offset": 0, "limit": 10, "total": 5 },
  "data": [
    {
      "invoice_id": "INV-2026-001",
      "company_id": "COMP-001",
      "issue_date": "2026-03-01T00:00:00Z",
      "due_date": "2026-03-31T00:00:00Z",
      "amount": 15000000,
      "payment_status": "unpaid",
      "paid_at": null,
      "amount_paid": 0,
      "reminder_count": 2,
      "collection_stage": "reminder",
      "created_at": "2026-03-01T00:00:00Z",
      "notes": "",
      "link_invoice": "https://invoice.example.com/INV-2026-001",
      "last_reminder_date": "2026-03-20T00:00:00Z",
      "workspace_id": "6c710989-7024-4bb9-9c86-6e60f1315cf3"
    }
  ]
}
```

---

### GET /api/data-master/invoices/{invoice_id}

Get a single invoice.

```bash
curl 'http://localhost:8081/api/data-master/invoices/INV-2026-001' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3'
```

---

### PUT /api/data-master/invoices/{invoice_id}

Partially update an invoice.

```bash
curl -X PUT 'http://localhost:8081/api/data-master/invoices/INV-2026-001' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3' \
  -H 'Content-Type: application/json' \
  -d '{
    "payment_status": "paid",
    "notes": "Payment received via bank transfer"
  }'
```

---

## Escalations

### GET /api/data-master/escalations

List escalations with filters.

```bash
curl 'http://localhost:8081/api/data-master/escalations?offset=0&limit=10' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3'
```

**Query Parameters:**

| Param | Type | Description |
|-------|------|-------------|
| `company_id` | string | Filter by company |
| `status` | string | Filter: `Open` or `Resolved` |
| `priority` | string | Filter by priority |
| `search` | string | Search company_id, reason, esc_id, notes |
| `offset` | int | Pagination offset |
| `limit` | int | Page size |

**With filters:**

```bash
curl 'http://localhost:8081/api/data-master/escalations?status=Open&priority=P1%20Critical' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3'
```

**Response:**

```json
{
  "status": "success",
  "meta": { "offset": 0, "limit": 10, "total": 3 },
  "data": [
    {
      "escalation_id": "esc-uuid-001",
      "company_id": "COMP-001",
      "esc_id": "ESC-001",
      "status": "Open",
      "created_at": "2026-04-09T08:00:00Z",
      "resolved_at": null,
      "priority": "P1 Critical",
      "reason": "Payment overdue 30+ days",
      "notified_party": "CS Lead",
      "telegram_message_sent": "true",
      "resolved_by": "",
      "notes": "",
      "workspace_id": "6c710989-7024-4bb9-9c86-6e60f1315cf3"
    }
  ]
}
```

---

## Message Templates

### GET /api/data-master/message-templates

List message templates.

```bash
curl 'http://localhost:8081/api/data-master/message-templates?offset=0&limit=10' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3'
```

**Query Parameters:**

| Param | Type | Description |
|-------|------|-------------|
| `category` | string | Filter by template category |
| `language` | string | Filter: `id` or `en` |
| `search` | string | Search template_name, wa_content |
| `active` | bool | Filter by active status |
| `offset` | int | Pagination offset |
| `limit` | int | Page size |

**With filters:**

```bash
curl 'http://localhost:8081/api/data-master/message-templates?category=checkin&language=id&active=true' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3'
```

**Response:**

```json
{
  "status": "success",
  "meta": { "offset": 0, "limit": 10, "total": 5 },
  "data": [
    {
      "template_id": "tpl-uuid-001",
      "template_name": "checkin_greeting",
      "wa_content": "Halo {company_name}, kami dari tim CS ingin melakukan check-in...",
      "template_category": "checkin",
      "language": "id",
      "channel": "wa",
      "email_subject": null,
      "email_body_html": null,
      "email_body_text": null,
      "active": true,
      "created_at": "2026-01-01T00:00:00Z",
      "updated_at": "2026-04-01T00:00:00Z"
    }
  ]
}
```

---

### GET /api/data-master/message-templates/{template_id}

Get a single template.

```bash
curl 'http://localhost:8081/api/data-master/message-templates/tpl-uuid-001' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3'
```

---

### PUT /api/data-master/message-templates/{template_id}

Partially update a template.

```bash
curl -X PUT 'http://localhost:8081/api/data-master/message-templates/tpl-uuid-001' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'X-Workspace-ID: 6c710989-7024-4bb9-9c86-6e60f1315cf3' \
  -H 'Content-Type: application/json' \
  -d '{
    "wa_content": "Halo {company_name}, ini pesan baru dari CS...",
    "active": true
  }'
```

---

## Trigger Rules

### GET /api/data-master/trigger-rules

List trigger rules. **No workspace required.**

```bash
curl 'http://localhost:8081/api/data-master/trigger-rules?offset=0&limit=10' \
  -H 'Authorization: Bearer <JWT_TOKEN>'
```

**Query Parameters:**

| Param | Type | Description |
|-------|------|-------------|
| `rule_group` | string | Filter: `HEALTH`, `CHECKIN`, `NEGOTIATION`, `INVOICE`, `OVERDUE`, `EXPANSION`, `CROSS_SELL` |
| `action_type` | string | Filter: `send_wa`, `send_email`, `escalate`, `alert_telegram`, `create_invoice`, `skip_and_set_flag` |
| `active` | bool | Filter by active status |
| `search` | string | Search rule_id, group, description |
| `offset` | int | Pagination offset |
| `limit` | int | Page size |

**With filters:**

```bash
curl 'http://localhost:8081/api/data-master/trigger-rules?rule_group=INVOICE&active=true' \
  -H 'Authorization: Bearer <JWT_TOKEN>'
```

**Response:**

```json
{
  "status": "success",
  "meta": { "offset": 0, "limit": 10, "total": 15 },
  "data": [
    {
      "rule_id": "INVOICE_OVERDUE_7D",
      "rule_group": "INVOICE",
      "priority": 1,
      "sub_priority": 1,
      "condition": { "field": "days_overdue", "op": "gte", "value": 7 },
      "action_type": "send_wa",
      "template_id": "tpl-invoice-reminder",
      "flag_key": "invoice_reminder_7d_sent",
      "escalation_id": null,
      "esc_priority": null,
      "esc_reason": null,
      "extra_flags": null,
      "stop_on_fire": false,
      "active": true,
      "description": "Send WA reminder when invoice is 7+ days overdue",
      "workspace_id": null,
      "created_at": "2026-04-09T00:00:00Z",
      "updated_at": "2026-04-09T00:00:00Z"
    }
  ]
}
```

---

### GET /api/data-master/trigger-rules/{rule_id}

Get a single trigger rule.

```bash
curl 'http://localhost:8081/api/data-master/trigger-rules/INVOICE_OVERDUE_7D' \
  -H 'Authorization: Bearer <JWT_TOKEN>'
```

---

### POST /api/data-master/trigger-rules

Create a new trigger rule.

```bash
curl -X POST 'http://localhost:8081/api/data-master/trigger-rules' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'Content-Type: application/json' \
  -d '{
    "rule_id": "CUSTOM_RULE_001",
    "rule_group": "INVOICE",
    "priority": 2,
    "sub_priority": 1,
    "condition": {
      "op": "and",
      "conditions": [
        { "field": "days_overdue", "op": "gte", "value": 14 },
        { "field": "payment_status", "op": "eq", "value": "unpaid" }
      ]
    },
    "action_type": "send_wa",
    "template_id": "tpl-invoice-reminder-14d",
    "flag_key": "invoice_reminder_14d_sent",
    "stop_on_fire": false,
    "active": true,
    "description": "Send WA reminder when invoice is 14+ days overdue and unpaid"
  }'
```

**Required fields:** `rule_id`, `rule_group`, `flag_key`, `action_type`

---

### PUT /api/data-master/trigger-rules/{rule_id}

Partially update a trigger rule.

```bash
curl -X PUT 'http://localhost:8081/api/data-master/trigger-rules/CUSTOM_RULE_001' \
  -H 'Authorization: Bearer <JWT_TOKEN>' \
  -H 'Content-Type: application/json' \
  -d '{
    "active": false,
    "description": "Disabled - no longer needed"
  }'
```

---

### DELETE /api/data-master/trigger-rules/{rule_id}

Soft-delete a trigger rule.

```bash
curl -X DELETE 'http://localhost:8081/api/data-master/trigger-rules/CUSTOM_RULE_001' \
  -H 'Authorization: Bearer <JWT_TOKEN>'
```

---

### POST /api/data-master/trigger-rules/cache/invalidate

Invalidate the trigger rule cache.

```bash
curl -X POST 'http://localhost:8081/api/data-master/trigger-rules/cache/invalidate' \
  -H 'Authorization: Bearer <JWT_TOKEN>'
```

---

## Template Variables

### GET /api/data-master/template-variables

List available template variables for message templates.

```bash
curl 'http://localhost:8081/api/data-master/template-variables' \
  -H 'Authorization: Bearer <JWT_TOKEN>'
```

**With channel filter:**

```bash
curl 'http://localhost:8081/api/data-master/template-variables?channel=wa' \
  -H 'Authorization: Bearer <JWT_TOKEN>'
```

---

## Webhooks (Internal / External)

### POST /api/handoff/new-client

Onboard a new client from Business Development. **HMAC authenticated.**

```bash
curl -X POST 'http://localhost:8081/api/handoff/new-client' \
  -H 'X-Handoff-Secret: <HMAC_SHA256_SIGNATURE>' \
  -H 'Content-Type: application/json' \
  -d '{
    "company_id": "COMP-003",
    "company_name": "PT New Corp",
    "pic_name": "Siti",
    "pic_wa": "6281234567890",
    "owner_name": "Andi",
    "owner_wa": "6281234567891",
    "owner_telegram_id": "111222333",
    "segment": "Enterprise",
    "contract_months": 12,
    "contract_start": "2026-04-01",
    "contract_end": "2027-03-31",
    "activation_date": "2026-04-15"
  }'
```

**Required fields:** `company_id`, `company_name`, `pic_name`, `pic_wa`, `owner_name`, `segment`, `contract_months`, `contract_start`, `contract_end`, `owner_telegram_id`

---

### POST /api/payment/verify

Verify a client's payment. **HMAC authenticated.**

```bash
curl -X POST 'http://localhost:8081/api/payment/verify' \
  -H 'X-Verify-Secret: <HMAC_SHA256_SIGNATURE>' \
  -H 'Content-Type: application/json' \
  -d '{
    "company_id": "COMP-001",
    "verified_by": "finance@example.com",
    "invoice_id": "INV-2026-001",
    "notes": "Bank transfer confirmed"
  }'
```

**Required fields:** `company_id`, `verified_by`

---

### POST /api/webhook/wa

Receive incoming WhatsApp message events from HaloAI. **Signature authenticated.**

```bash
curl -X POST 'http://localhost:8081/api/webhook/wa' \
  -H 'X-Signature: <HALOAI_HMAC_SIGNATURE>' \
  -H 'Content-Type: application/json' \
  -d '{
    "event": "new_message",
    "customer": {
      "name": "John",
      "phone": "6281234567890"
    },
    "room_id": "room-123",
    "message": {
      "from_me": false,
      "body": "Hello, I need help",
      "has_media": false,
      "timestamp": 1712649600,
      "ack": 0
    },
    "trigger": {
      "type": "incoming",
      "message": "Hello, I need help",
      "messageId": "msg-abc",
      "timestamp": 1712649600
    },
    "request_id": "req-uuid-123"
  }'
```

---

### POST /api/webhook/checkin-form

Process check-in form submission. **No authentication required.**

```bash
curl -X POST 'http://localhost:8081/api/webhook/checkin-form' \
  -H 'Content-Type: application/json' \
  -d '{
    "company_id": "COMP-001"
  }'
```

**Required fields:** `company_id`

---

## Cron

### GET /api/cron/run

Trigger the cron job manually. **OIDC authenticated (Cloud Scheduler).**

```bash
curl 'http://localhost:8081/api/cron/run' \
  -H 'Authorization: Bearer <OIDC_TOKEN>'
```

**Response (202 Accepted):**

```json
{
  "status": "success",
  "message": "Cron run accepted",
  "data": [
    {
      "job_id": "cron-job-001",
      "workspace_id": "6c710989-...",
      "job_type": "cron",
      "status": "pending",
      "entity_type": "cron_run"
    }
  ]
}
```
