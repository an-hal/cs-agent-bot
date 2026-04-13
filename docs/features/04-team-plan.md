# Plan — feat/04-team

> **Branch base**: `master` &nbsp;&nbsp;|&nbsp;&nbsp; **Migration range**: `20260414000300`–`000399` &nbsp;&nbsp;|&nbsp;&nbsp; **Spec dir**: `~/dealls/project-bumi-dashboard/context/for-backend/features/04-team/`

## Scope

Team management + granular RBAC: roles, permission matrix (per role × per workspace × per module × per action), team members with multi-workspace assignment, invitation flow, activity log integration, permission-check middleware.

**Read first**: `01-overview.md`, `02-database-schema.md`, `03-api-endpoints.md`, `00-shared/05-checker-maker.md`.

## Migrations

| # | File | Purpose |
|---|---|---|
| 300 | `create_roles.{up,down}.sql` | Per spec §1. UNIQUE(name). `is_system` flag. Trigger for updated_at. |
| 301 | `create_role_permissions.{up,down}.sql` | Per spec §2. `view_list VARCHAR(5)` (`'false'|'true'|'all'`). UNIQUE(role_id,ws_id,module_id). Indexes per spec. |
| 302 | `create_team_members.{up,down}.sql` | Per spec §3. `email UNIQUE`, `status CHECK IN ('active','pending','inactive')`, invite_token/expires. |
| 303 | `create_member_workspace_assignments.{up,down}.sql` | Per spec §4. UNIQUE(member_id, workspace_id). |
| 304 | `create_role_workspace_scope.{up,down}.sql` | Per spec §5. UNIQUE(role_id, workspace_id). |
| 305 | `seed_system_roles.{up,down}.sql` | Seed `Super Admin`, `Admin`, `Manager`, `AE Officer`, `SDR Officer`, `BD Officer`, `CS Officer`, `Finance`, `Viewer`. All `is_system=true`. Pre-populate permission matrix per default-role rules in `01-overview.md` § Default Roles. |

> **Note**: skip FK to `users(id)` — there's no users table on master. `team_members.user_id` is nullable; the authoritative identity is `email`.

## Entities

`internal/entity/team.go`:
```go
type Role struct {
    ID          uuid.UUID
    Name        string
    Description string
    Color       string
    BgColor     string
    IsSystem    bool
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type RolePermission struct {
    RoleID      uuid.UUID
    WorkspaceID uuid.UUID
    ModuleID    string  // dashboard|analytics|reports|ae|sdr|bd|cs|data_master|team
    ViewList    string  // "false" | "true" | "all"
    ViewDetail  bool
    CanCreate   bool
    CanEdit     bool
    CanDelete   bool
    CanExport   bool
    CanImport   bool
}

type TeamMember struct {
    ID            uuid.UUID
    UserID        *uuid.UUID  // nullable
    Name          string
    Email         string  // unique
    Initials      string
    RoleID        uuid.UUID
    Status        string  // active|pending|inactive
    Department    string
    AvatarColor   string
    InviteToken   string
    InviteExpires *time.Time
    InvitedBy     *uuid.UUID
    JoinedAt      *time.Time
    LastActiveAt  *time.Time
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

type MemberWorkspaceAssignment struct {
    ID          uuid.UUID
    MemberID    uuid.UUID
    WorkspaceID uuid.UUID
    AssignedAt  time.Time
    AssignedBy  *uuid.UUID
}
```

## Repositories

```
internal/repository/
  role_repo.go                // List, Get, Create, Update, Delete (block if is_system OR has members), CountMembersByRole
  role_permission_repo.go     // Upsert(role,ws,module,perms), ListByRole(role), GetPermission(role,ws,module)
  team_member_repo.go         // List(ws, filter, pag), Get, GetByEmail, Create (pending), Update, UpdateRole, UpdateStatus, AssignWorkspaces, Delete, AcceptInvite(token)
  member_workspace_assignment_repo.go  // Set (txn: delete-then-insert per member)
```

Mocks in `internal/repository/mocks/`.

## Usecases

`internal/usecase/team/usecase.go`:
- `ListMembers(ctx, wsID, filter)` — joins with role + assignments; summary (active/pending/inactive counts)
- `InviteMember(ctx, callerRole, req)` — **checker-maker**: creates approval `invite_member`. On approval: create pending member, generate invite token (URL-safe 32 bytes), send email, log activity
- `UpdateMember`, `UpdateMemberRole` (Super Admin guard), `UpdateMemberStatus`, `AssignMemberWorkspaces`, `ResetPassword` (triggers email)
- `DeleteMember` — **checker-maker**: creates approval `remove_member`. On approval: hard-delete `member_workspace_assignments` rows + soft-delete member (`status=inactive`). Forbid self-delete, forbid Super Admin delete by non-Super-Admin
- `AcceptInvitation(token, password)` — bridges to `ms-auth-proxy` to create backing auth identity, sets `status=active`, nulls token
- `ListRoles`, `GetRole(id)` (returns full permission matrix grouped by workspace slug), `CreateRole`, `UpdateRole` (block rename if is_system), `DeleteRole` (block if members or is_system)
- `UpdateRolePermissions(ctx, callerRole, roleID, wsID, moduleID, perms)` — **checker-maker** (`change_permission`). Super Admin guard on editing Super Admin role permissions.
- `CheckPermission(ctx, memberID, wsID, moduleID, action)` — returns `(allowed bool, scope string)` where scope is `"ws"` for `view_list='true'` or `"all"` for `view_list='all'`
- `GetMyPermissions(ctx, memberID, wsID)` — full matrix for current WS

`internal/usecase/team/activity.go`:
- Appends to `team_activity_logs` (table shared with feat/08). Define the append-only interface here; rely on feat/08 or stub locally.

## Permission middleware

`internal/delivery/http/middleware/rbac.go`:
```go
func RequirePermission(module, action string, uc team.Usecase) Middleware {
    return func(next Handler) Handler {
        return func(w http.ResponseWriter, r *http.Request) error {
            user, _ := GetJWTUser(r.Context())
            ws, _ := ctxutil.GetWorkspaceID(r.Context())
            member, err := uc.GetMemberByEmail(r.Context(), user.Email)
            if err != nil { return apperror.Forbidden("no team_member record") }
            allowed, scope, err := uc.CheckPermission(r.Context(), member.ID, ws, module, action)
            if !allowed { return apperror.Forbidden("insufficient_permission") }
            ctx := context.WithValue(r.Context(), ctxkey.PermissionScope, scope)
            return next(w, r.WithContext(ctx))
        }
    }
}
```

Other features (02–09) should start using `RequirePermission("data_master", "edit")` etc. once this ships. Until then, fall back to `jwtAuth` only.

## HTTP routes

```go
team := api.Group("/team")
team.Handle(GET,    "/members",                   wsRequired(jwtAuth(teamH.ListMembers)))
team.Handle(GET,    "/members/{id}",              wsRequired(jwtAuth(teamH.GetMember)))
team.Handle(POST,   "/members/invite",            wsRequired(jwtAuth(teamH.Invite)))           // creates approval
team.Handle(PUT,    "/members/{id}",              wsRequired(jwtAuth(teamH.UpdateMember)))
team.Handle(PUT,    "/members/{id}/role",         wsRequired(jwtAuth(teamH.UpdateRole)))
team.Handle(PUT,    "/members/{id}/status",       wsRequired(jwtAuth(teamH.UpdateStatus)))
team.Handle(PUT,    "/members/{id}/workspaces",   wsRequired(jwtAuth(teamH.UpdateMemberWorkspaces)))
team.Handle(POST,   "/members/{id}/reset-password", wsRequired(jwtAuth(teamH.ResetPassword)))
team.Handle(DELETE, "/members/{id}",              wsRequired(jwtAuth(teamH.DeleteMember)))     // creates approval

team.Handle(GET,    "/roles",                     wsRequired(jwtAuth(teamH.ListRoles)))
team.Handle(GET,    "/roles/{id}",                wsRequired(jwtAuth(teamH.GetRole)))
team.Handle(POST,   "/roles",                     wsRequired(jwtAuth(teamH.CreateRole)))
team.Handle(PUT,    "/roles/{id}",                wsRequired(jwtAuth(teamH.UpdateRole)))
team.Handle(PUT,    "/roles/{id}/permissions",    wsRequired(jwtAuth(teamH.UpdateRolePermissions))) // creates approval
team.Handle(DELETE, "/roles/{id}",                wsRequired(jwtAuth(teamH.DeleteRole)))

team.Handle(GET,    "/permissions/check",         wsRequired(jwtAuth(teamH.CheckPermission)))
team.Handle(GET,    "/permissions/me",            wsRequired(jwtAuth(teamH.MyPermissions)))

// Invite accept is public — no JWT
api.Handle(POST,    "/team/invitations/{token}/accept", teamH.AcceptInvitation)
```

## Tests

- `usecase/team/usecase_test.go`: permission resolution (all/true/false scopes), Super Admin guard, self-delete block, invite expiry, role delete block when members attached
- `usecase/team/rbac_middleware_test.go`: denies without member record, injects scope into context
- Seed verification test: `TestDefaultSystemRoles_HaveCorrectMatrix` asserts every seeded role × workspace × module combo matches the spec table

## Risks / business-rule conflicts with CLAUDE.md

- **Existing auth uses `whitelist` table** (from feat/01-auth). `team_members.email` should intersect — any new member invite must also append to `whitelist` (or the whitelist check should consult `team_members` too). Pick one:
  - **Option A (preferred)**: make `team_members` the canonical dashboard user list; deprecate `whitelist` in the next release. Team usecase writes to both on invite.
  - Option B: keep separate. Document that whitelist is gate, team_members is RBAC metadata.
- **System roles cannot reference workspaces that don't exist yet**. Seed migration should only create `role_permissions` rows for workspaces present at migration time (query `workspaces` in the seed SQL). New workspaces get default permissions added via feat/02 workspace-create usecase hook.

## File checklist

- [ ] migrations 300–305
- [ ] entities (role, role_permission, team_member, assignment)
- [ ] repos + mocks (4)
- [ ] usecases: team (members, roles, permissions, invitations), rbac_middleware
- [ ] handlers: team_handler.go (~18 endpoints)
- [ ] middleware: rbac.go + tests
- [ ] route.go + deps.go + main.go wiring
- [ ] swag regen
- [ ] `make lint && make unit-test` green
- [ ] commit + push `feat/04-team`
