# Central Approval Dispatcher

Single endpoint that applies any of the 8 approval types. FE doesn't need to
know which feature owns each type — dispatcher routes internally.

## Endpoint

```
POST /approvals/{id}/apply
```

Headers: `Authorization: Bearer {jwt}` + `X-Workspace-ID`.

## Approval types (all handled by one endpoint)

| request_type | Feature | Created by | Side-effect on apply |
|---|---|---|---|
| `create_invoice` | Invoices | `POST /invoices` | Inserts invoice + line items + initial payment_log |
| `mark_invoice_paid` | Invoices | `POST /invoices/{id}/mark-paid` | Flips `payment_status=Lunas` + payment_log |
| `collection_schema_change` | Collections | Schema-change requests | Applies create/delete collection or add/delete field |
| `delete_client_record` | Master Data | `DELETE /master-data/clients/{id}` | Hard deletes master_data row |
| `toggle_automation_rule` | Automation Rules | `PUT /automation-rules/{id}` with status change | Flips `status=active/paused` |
| `bulk_import_master_data` | Master Data | `POST /master-data/clients/import` | Not routed via central dispatcher (needs row payload) — use `/master-data/clients/import/commit/{id}` instead |
| `stage_transition` | Master Data | New: `POST /master-data/clients/{id}/transition-request` | Runs Transition with to_stage from payload |
| `integration_key_change` | Workspace Integrations | New endpoint TBD | Upserts integration config from approval payload |

## Rules on apply

1. Approval must be `status=pending` (not approved/rejected/expired).
2. `maker_email` (creator) must be DIFFERENT from checker email (the JWT user
   calling apply). BE rejects self-approval with `400 BAD_REQUEST`.
3. `bulk_import_master_data` cannot be applied via this dispatcher because
   the bulk row payload is too large for JSONB. Use
   `POST /data-master/import/commit/{approval_id}` with the rows re-uploaded.

## Workflow

```
1. User A creates a request
    POST /invoices/{id}/mark-paid   → 202 with approval_id
2. User B (checker) lists pending approvals:
    GET /approvals?status=pending&workspace_id=...  (approvals list isn't exposed yet — see FE gap)
3. User B applies:
    POST /approvals/{id}/apply
    → 200 OK with updated ApprovalRequest (status=approved, checker_email, checker_at populated)
```

## Response

```json
{
  "status": "success",
  "data": {
    "id": "uuid",
    "workspace_id": "uuid",
    "request_type": "mark_invoice_paid",
    "status": "approved",
    "maker_email": "ae@example.com",
    "maker_at": "2026-04-24T...",
    "checker_email": "admin@example.com",
    "checker_at": "2026-04-24T...",
    "applied_at": "2026-04-24T...",
    "payload": { /* request-type-specific */ }
  }
}
```

## Error handling

| HTTP | When |
|---|---|
| 400 | Approval not pending / self-approval / malformed payload / missing client_id etc. |
| 404 | `approval_request {id}` not found for this workspace |
| 500 | Downstream feature Apply failed (invoice DB error, etc.) — retry safe |

## List approvals (FE gap — not currently exposed as dedicated endpoint)

The list view is typically built by filtering per-feature endpoints that
create approvals. A unified `GET /approvals?status=pending` is on the roadmap
if FE needs it — ping the BE team.

Short-term workarounds:
- **Invoice approvals**: use `GET /invoices?payment_status=Pending&has_pending_approval=true` (filter server-side when FE requests).
- **Collection schema**: fetched per collection.
- **Integration rotation**: visible in audit log + integration `updated_by`.

## Payload shapes (common request_types)

### mark_invoice_paid
```json
{
  "invoice_id": "INV-2026-001",
  "marked_by": "ae@example.com",
  "note": "confirmed transfer Mandiri"
}
```

### delete_client_record
```json
{"client_id": "uuid", "reason": "client requested deletion"}
```

### toggle_automation_rule
```json
{
  "rule_id": "uuid",
  "rule_code": "AE_P4_REN90",
  "from_status": "active",
  "target_status": "paused"
}
```

### stage_transition
```json
{
  "client_id": "uuid",
  "from_stage": "prospect",
  "to_stage": "client",
  "reason": "first payment confirmed",
  "updates": {},
  "custom_updates": {"paid_amount": 12000000}
}
```

### integration_key_change
```json
{
  "provider": "haloai",
  "display_name": "HaloAI Prod (rotated)",
  "config": {
    "api_url": "https://halo.example",
    "wa_api_token": "NEW_TOKEN"
  },
  "is_active": true
}
```
