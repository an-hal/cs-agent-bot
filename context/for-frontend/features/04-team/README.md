# feat/04 — Team (Members, Roles, Permissions)

Per-workspace team management: members, roles, permissions matrix, activity
audit.

## Status

**✅ 95%** — CRUD + permissions + activity logs live. Full permission-cascade
tests still partial.

## Members

```
GET    /team/members
POST   /team/members/invite
GET    /team/members/{id}
PUT    /team/members/{id}
DELETE /team/members/{id}
PUT    /team/members/{id}/role                   # change role
PUT    /team/members/{id}/status                 # active/inactive
PUT    /team/members/{id}/workspaces             # multi-workspace assignment
POST   /team/invitations/{token}/accept          # accept invite
```

Invite body:
```json
{"email": "new@kantorku.id", "role_id": "role-ae"}
```

Change role body:
```json
{"role_id": "role-admin"}
```

## Roles + permissions

```
GET    /team/roles
POST   /team/roles
GET    /team/roles/{id}
PUT    /team/roles/{id}
PUT    /team/roles/{id}/permissions              # replace permission matrix
DELETE /team/roles/{id}
GET    /team/permissions/me                      # effective permissions for caller
```

Create role:
```json
{"name": "Manager", "slug": "manager"}
```

Update permissions:
```json
{
  "permissions": [
    {"resource": "master_data", "actions": ["view_list", "view_detail", "edit"]},
    {"resource": "invoices",    "actions": ["view_list", "view_detail"]},
    ...
  ]
}
```

Action constants (defined in `entity/team.go`):
- `view_list`, `view_detail`, `create`, `edit`, `delete`

### Effective permissions

```
GET /team/permissions/me
→ {"data": {
     "role": "ae",
     "permissions": {
       "master_data": ["view_list", "view_detail", "edit"],
       "invoices": ["view_list", "view_detail"],
       ...
     }
   }}
```

FE uses this to hide/disable UI actions the user can't perform.

## Team activity

Audit stream for team-scoped actions (separate from master_data mutations).

```
GET  /team/activity?limit=50
POST /team/activity    # manual entry (typically used by BE handlers)
```

Entry shape:
```json
{
  "id": "uuid",
  "actor_email": "admin@example.com",
  "action": "invite_member",
  "target_email": "new@example.com",
  "role_id": "uuid",
  "detail": {"source": "bulk_invite", ...},
  "created_at": "RFC3339"
}
```

Common actions: `invite_member`, `change_role`, `remove_member`,
`update_permissions`, `create_role`, `delete_role`, `update_assignments`.

## Multi-workspace assignments

A team member can belong to multiple workspaces with different roles per
workspace.

```
PUT /team/members/{id}/workspaces
{"workspace_ids": ["ws-1", "ws-2"]}
```

## FE UX

**Team page:**
- Member list with role chip + status toggle
- Invite form (email + role dropdown)
- Per-row actions: edit profile, change role, change status, remove, manage workspace assignments

**Roles tab:**
- Grid: resources × actions with checkboxes per role
- Creator of role = auto-admin on that role
- Delete role: blocked if any member currently assigned

**Activity tab:**
- Filter by actor / target / action
- Newest first
- Detail drawer with JSON `detail` payload
