# Audit Workspace Access

Compliance audit trail for cross-workspace access. Every time a user reads
or writes data in a workspace they're not a direct member of (holding/admin
flow), the access is recorded here.

## Endpoints

```
GET  /audit-logs/workspace-access?limit=50&actor=...&kind=read&resource=master_data
POST /audit-logs/workspace-access
```

### Record an access event
```
POST /audit-logs/workspace-access
{
  "access_kind": "read",           // read|write|admin
  "resource": "master_data",       // table or domain touched
  "resource_id": "ACME-001",       // optional specific ID
  "reason": "reviewing from holding view"
}
```

BE auto-captures `actor_email` (from JWT), `workspace_id` (from header),
`ip_address` (from `X-Forwarded-For` or RemoteAddr), `user_agent` (from
header).

### List entries
```
GET /audit-logs/workspace-access?limit=50
```
Response includes pagination `meta`. Filter params:
- `actor` — actor email
- `kind` — `read|write|admin`
- `resource` — table/domain name
- `limit`, `offset`

## FE UX

This is typically an admin-only log view. Surface via:
- Admin dashboard → "Audit" tab
- Compliance export per workspace
- Per-user activity drilldown (filter by `actor`)

Sorting: newest first by `created_at`.
