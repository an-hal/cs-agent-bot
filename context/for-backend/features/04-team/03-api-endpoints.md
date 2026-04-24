# API Endpoints — Team Management & RBAC

## Base URL
```
{BACKEND_API_URL}/api/v1
```

All endpoints require `Authorization: Bearer {token}` header.
Workspace-scoped endpoints require `X-Workspace-ID: {uuid}` header.
Team management endpoints require `team.edit` or `team.delete` permission on current workspace.

---

## 1. Team Members

### GET `/team/members`
List semua member yang visible di current workspace.

```
Query params:
  ?offset=0&limit=50
  &status=active                     (optional: active, pending, inactive)
  &role_id=uuid                      (optional: filter by role)
  &search=keyword                    (optional: searches name, email, department)
  &sort_by=name                      (optional: name, email, joined_at, last_active_at)
  &sort_dir=asc                      (optional: asc/desc)

Response 200:
{
  "data": [
    {
      "id": "uuid",
      "name": "Arief Faltah",
      "email": "arief@bumi.id",
      "initials": "AF",
      "avatar_color": "#534AB7",
      "role": {
        "id": "uuid",
        "name": "Super Admin",
        "color": "#EF4444",
        "bg_color": "#FEF2F2"
      },
      "workspaces": [
        { "id": "uuid", "slug": "dealls", "name": "Dealls" },
        { "id": "uuid", "slug": "kantorku", "name": "KantorKu" },
        { "id": "uuid", "slug": "holding", "name": "Sejutacita" }
      ],
      "status": "active",
      "department": "Engineering",
      "joined_at": "2024-01-15T00:00:00Z",
      "last_active_at": "2026-04-05T10:00:00Z"
    }
  ],
  "meta": {
    "offset": 0,
    "limit": 50,
    "total": 27
  },
  "summary": {
    "total": 27,
    "active": 22,
    "pending": 2,
    "inactive": 3
  }
}
```

### GET `/team/members/{id}`
Get single member detail.

```
Response 200:
{
  "data": { ... full member object ... }
}
```

### POST `/team/members/invite`
Undang member baru.

Requires: `team.create` permission.

```json
// Request body:
{
  "name": "Galih Nugroho",
  "email": "galih@kantorku.id",
  "role_id": "uuid-sdr-officer",
  "workspace_ids": ["uuid-kantorku"],
  "department": "SDR"
}

// Response 201:
{
  "data": {
    "id": "uuid",
    "name": "Galih Nugroho",
    "email": "galih@kantorku.id",
    "status": "pending",
    "invite_token": "tok_xxx",
    "invite_expires": "2026-04-12T09:14:00Z"
  },
  "message": "Invitation sent to galih@kantorku.id"
}

// Response 409 (email already in workspace):
{
  "error": "Email galih@kantorku.id is already a member of this workspace"
}
```

Side effect: INSERT INTO team_activity_logs (action='invite_member')

### PUT `/team/members/{id}`
Update member info (name, department, avatar_color).

Requires: `team.edit` permission.

```json
{
  "name": "Galih Nugroho",
  "department": "SDR Lead"
}
```

### PUT `/team/members/{id}/role`
Change member's role.

Requires: `team.edit` permission.
Cannot change Super Admin role unless you are Super Admin.

```json
{
  "role_id": "uuid-ae-officer"
}

// Response 200:
{
  "data": { ... updated member ... },
  "previous_role": "SDR Officer",
  "new_role": "AE Officer"
}
```

Side effect: INSERT INTO team_activity_logs (action='update_role', detail='SDR Officer -> AE Officer')

### PUT `/team/members/{id}/status`
Activate or deactivate member.

Requires: `team.edit` permission.

```json
{
  "status": "inactive",
  "reason": "Mengundurkan diri"
}

// Response 200:
{
  "data": { ... updated member ... },
  "previous_status": "active"
}
```

Side effect: INSERT INTO team_activity_logs (action='deactivate_member')

### PUT `/team/members/{id}/workspaces`
Update workspace assignments for a member.

Requires: `team.edit` permission.

```json
{
  "workspace_ids": ["uuid-dealls", "uuid-kantorku"]
}

// Response 200:
{
  "data": { ... updated member with new workspaces ... }
}
```

### POST `/team/members/{id}/reset-password`
Trigger password reset for member.

Requires: `team.edit` permission.

```
// Response 200:
{
  "message": "Password reset email sent to hendra@dealls.com"
}
```

Side effect: INSERT INTO team_activity_logs (action='reset_password')

### DELETE `/team/members/{id}`
Remove member from workspace entirely.

Requires: `team.delete` permission.
Cannot delete yourself. Cannot delete Super Admin unless you are Super Admin.

```
// Response 200:
{
  "message": "Member removed",
  "id": "uuid"
}
```

Side effect: INSERT INTO team_activity_logs (action='remove_member')

---

## 2. Roles

### GET `/team/roles`
List semua roles yang tersedia di current workspace.

```
Response 200:
{
  "data": [
    {
      "id": "uuid",
      "name": "Super Admin",
      "description": "Akses penuh ke semua modul dan semua workspace.",
      "color": "#EF4444",
      "bg_color": "#FEF2F2",
      "is_system": true,
      "member_count": 2,
      "workspaces": ["dealls", "kantorku", "holding"]
    },
    {
      "id": "uuid",
      "name": "AE Officer",
      "description": "Akses penuh ke modul Account Executive dan Data Master.",
      "color": "#8B5CF6",
      "bg_color": "#F5F3FF",
      "is_system": false,
      "member_count": 6,
      "workspaces": ["dealls", "kantorku"]
    }
  ]
}
```

### GET `/team/roles/{id}`
Get role detail including full permission matrix.

```
Response 200:
{
  "data": {
    "id": "uuid",
    "name": "AE Officer",
    "description": "...",
    "color": "#8B5CF6",
    "bg_color": "#F5F3FF",
    "is_system": false,
    "member_count": 6,
    "workspaces": ["dealls", "kantorku"],
    "permissions": {
      "dealls": {
        "dashboard":   { "view_list": "true", "view_detail": true, "create": false, "edit": false, "delete": false, "export": false, "import": false },
        "analytics":   { "view_list": "true", "view_detail": true, "create": false, "edit": false, "delete": false, "export": false, "import": false },
        "ae":          { "view_list": "true", "view_detail": true, "create": true,  "edit": true,  "delete": false, "export": true,  "import": false },
        "data_master": { "view_list": "true", "view_detail": true, "create": true,  "edit": true,  "delete": false, "export": true,  "import": false },
        "team":        { "view_list": "false","view_detail": false,"create": false, "edit": false, "delete": false, "export": false, "import": false }
      },
      "kantorku": {
        ...same structure...
      }
    }
  }
}
```

### POST `/team/roles`
Create custom role.

Requires: `team.create` permission.

```json
{
  "name": "BD Officer",
  "description": "Fokus pada Business Development pipeline.",
  "color": "#0EA5E9",
  "bg_color": "#F0F9FF",
  "workspace_ids": ["uuid-dealls", "uuid-kantorku"],
  "permissions": {
    "uuid-dealls": {
      "dashboard":   { "view_list": "true", "view_detail": true },
      "bd":          { "view_list": "true", "view_detail": true, "create": true, "edit": true, "delete": false, "export": true, "import": false },
      "data_master": { "view_list": "true", "view_detail": true, "create": false, "edit": false, "delete": false, "export": false, "import": false }
    }
  }
}

// Response 201
```

Side effect: INSERT INTO team_activity_logs (action='create_role')

### PUT `/team/roles/{id}`
Update role metadata (name, description, color, workspace scope).

Cannot rename system roles.

```json
{
  "name": "BD Officer",
  "description": "Updated description",
  "workspace_ids": ["uuid-dealls", "uuid-kantorku"]
}
```

### PUT `/team/roles/{id}/permissions`
Update permission matrix for a role.

Requires: `team.edit` permission.
Cannot edit Super Admin permissions unless you are Super Admin.

```json
{
  "workspace_id": "uuid-dealls",
  "module_id": "data_master",
  "permissions": {
    "view_list": "true",
    "view_detail": true,
    "create": true,
    "edit": true,
    "delete": false,
    "export": true,
    "import": true
  }
}

// Response 200:
{
  "data": { ... updated role with full permission matrix ... },
  "changed": {
    "module": "data_master",
    "workspace": "dealls",
    "changed_actions": ["import"],
    "detail": "Data Master: Import -> Diizinkan"
  }
}
```

Side effect: INSERT INTO team_activity_logs (action='update_policy', detail='Data Master: Import -> Diizinkan')

### DELETE `/team/roles/{id}`
Delete custom role.

Cannot delete system roles. Cannot delete role that has active members.

```
// Response 200:
{
  "message": "Role deleted",
  "id": "uuid"
}

// Response 409:
{
  "error": "Cannot delete role with 6 active members. Reassign members first."
}
```

---

## 3. Permission Check (middleware utility)

### GET `/team/permissions/check`
Check current user's permissions. Dipakai frontend saat render conditional UI.

```
Query params:
  ?module=data_master
  &action=delete

Response 200:
{
  "allowed": false,
  "role": "AE Officer",
  "module": "data_master",
  "action": "delete"
}
```

### GET `/team/permissions/me`
Get full permission matrix for current user in current workspace.

```
Response 200:
{
  "role": {
    "id": "uuid",
    "name": "AE Officer"
  },
  "workspace_id": "uuid-dealls",
  "permissions": {
    "dashboard":   { "view_list": "true", "view_detail": true, "create": false, ... },
    "analytics":   { ... },
    "ae":          { ... },
    "data_master": { ... },
    "team":        { "view_list": "false", ... }
  }
}
```

---

## 4. Team Activity Feed

### GET `/team/activity`
Already defined in activity-log spec. Repeated here for reference.

```
Query params:
  ?limit=50&since=...&action=invite_member

Response: see activity-log/03-api-endpoints.md section 4
```

---

## Design Notes

### Permission Enforcement
Every API endpoint MUST:
1. Extract current user from JWT token
2. Look up team_member record
3. Check role_permissions for (role_id, workspace_id, module_id)
4. Return 403 if action not allowed

```
Middleware pseudocode:
  func RequirePermission(module string, action string) Middleware {
    return func(ctx) {
      member := getMemberFromToken(ctx)
      perm := getPermission(member.role_id, ctx.workspaceId, module)
      if !perm[action] { return 403 }
      next(ctx)
    }
  }
```

### Super Admin Protection
- Only Super Admin can:
  - Change another Super Admin's role
  - Edit Super Admin permission matrix
  - Delete Super Admin members
- System roles (is_system=true) cannot be deleted

### Invitation Flow
1. POST `/team/members/invite` creates member with status='pending' + invite_token
2. Send email with link: `{FRONTEND_URL}/invite?token={invite_token}`
3. Frontend calls `POST /auth/accept-invite` with token + new password
4. Backend sets status='active', user_id linked, invite_token nulled

### Cascade Behavior
- Deleting a role: blocked if any members have that role
- Deleting a workspace: cascade delete member_workspace_assignments and role_permissions
- Deactivating a member: keep record, just set status='inactive' (soft delete)

---

## Checker-Maker Approval Required

The following endpoints require approval before execution.
See `00-shared/05-checker-maker.md` for the full approval system spec.

### POST `/team/members/invite` → Approval Required

Instead of executing directly, create an approval request:

```
POST /approvals
{
  "request_type": "invite_member",
  "payload": {
    "name": "Galih Nugroho",
    "email": "galih@kantorku.id",
    "role_id": "uuid-sdr-officer",
    "role_name": "SDR Officer",
    "workspace_ids": ["uuid-kantorku"],
    "department": "SDR"
  }
}
```

When approved, the system executes the actual invite (creates pending member + sends email).

**Approval payload format:**
| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Name of the person being invited |
| `email` | string | Email to send invitation to |
| `role_id` | UUID | Role to assign |
| `role_name` | string | Role name for reviewer display |
| `workspace_ids` | UUID[] | Workspaces to grant access to |
| `department` | string | Department assignment |

### DELETE `/team/members/{id}` → Approval Required

Instead of executing directly, create an approval request:

```
POST /approvals
{
  "request_type": "remove_member",
  "payload": {
    "member_id": "uuid",
    "member_name": "Galih Nugroho",
    "member_email": "galih@kantorku.id",
    "current_role": "SDR Officer"
  }
}
```

When approved, the system executes the actual member removal.

**Approval payload format:**
| Field | Type | Description |
|-------|------|-------------|
| `member_id` | UUID | The team member to remove |
| `member_name` | string | Display name for the approval reviewer |
| `member_email` | string | Email for reference |
| `current_role` | string | Current role name for context |

### PUT `/team/roles/{id}/permissions` → Approval Required

Instead of executing directly, create an approval request:

```
POST /approvals
{
  "request_type": "change_permission",
  "payload": {
    "role_id": "uuid",
    "role_name": "AE Officer",
    "workspace_id": "uuid-dealls",
    "module_id": "data_master",
    "permissions": {
      "view_list": "true",
      "view_detail": true,
      "create": true,
      "edit": true,
      "delete": false,
      "export": true,
      "import": true
    },
    "changed_actions": ["import"],
    "change_summary": "Data Master: Import -> Diizinkan"
  }
}
```

When approved, the system applies the permission changes.

**Approval payload format:**
| Field | Type | Description |
|-------|------|-------------|
| `role_id` | UUID | Role being modified |
| `role_name` | string | Role name for reviewer display |
| `workspace_id` | UUID | Which workspace's permissions are changing |
| `module_id` | string | Which module's permissions are changing |
| `permissions` | object | Full new permission set to apply |
| `changed_actions` | string[] | List of actions that changed |
| `change_summary` | string | Human-readable summary of what changed |
