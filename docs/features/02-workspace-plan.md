# Plan — feat/02-workspace

> **Branch base**: `master` &nbsp;&nbsp;|&nbsp;&nbsp; **Migration range**: `20260414000100`–`000199` &nbsp;&nbsp;|&nbsp;&nbsp; **Spec dir**: `~/dealls/project-bumi-dashboard/context/for-backend/features/02-workspace/`

## Scope

Multi-workspace CRM core: workspace CRUD, members, settings, holding view, theme, global search, notifications. All other features depend on this — workspace_id is the tenant key.

**Read first** (in this order):
1. `01-overview.md` — multi-workspace + holding architecture
2. `02-database-schema.md` — 4 tables: `workspaces`, `workspace_members`, `workspace_settings`, `workspace_invitations`
3. `03-api-endpoints.md` — full endpoint contracts
4. `04-global-search.md` — `pg_trgm` + parallel queries
5. `05-theme-system.md` — 9 presets + palette gen
6. `07-notifications.md` — cross-cutting notification hub
7. `00-shared/02-user-preferences.md`, `04-integrations.md`

**Note**: existing repo has a partial `workspaces` table (created by an earlier feature) and dashboard `WorkspaceHandler.List` stub. This plan **extends** them, not duplicates. Inspect `internal/repository/workspace_repo.go` and `internal/usecase/dashboard/workspace_*.go` first.

## Migrations

| # | File | Purpose |
|---|---|---|
| 100 | `create_workspaces_extended.{up,down}.sql` | Add `slug`, `logo`, `color`, `plan`, `is_holding`, `member_ids UUID[]`, `settings JSONB`, `is_active`. Indexes: slug, holding, active, GIN(settings). Trigger `update_updated_at()` |
| 101 | `create_workspace_members.{up,down}.sql` | `(workspace_id, user_id, role, permissions JSONB, is_active, invited_at, joined_at, invited_by)` UNIQUE(ws,user). Indexes: ws, user, (user,active), (ws,role) |
| 102 | `create_workspace_settings.{up,down}.sql` | `(workspace_id, setting_key, setting_value)` UNIQUE(ws,key). Indexes: ws, (ws,key) |
| 103 | `create_workspace_invitations.{up,down}.sql` | `(workspace_id, email, role, invite_token UNIQUE, status, invited_by, accepted_at, expires_at)`. Indexes: token, ws, email, (status,expires_at) |
| 104 | `create_user_preferences.{up,down}.sql` | `(user_email, workspace_id, theme_id, dark_mode, preferences JSONB)` UNIQUE(user_email,ws). Index on (user_email,ws). See `00-shared/02`. |
| 105 | `create_workspace_integrations.{up,down}.sql` | Per-WS API creds for HaloAI, Telegram, Paper.id, email-from. **Encrypt at rest** (use Go-side AES-GCM with `INTEGRATIONS_KEY` env). See `00-shared/04` |
| 106 | `create_notifications.{up,down}.sql` | `(workspace_id, recipient_id, type, icon, message, href, source_feature, source_id, read, read_at, telegram_sent, email_sent)`. Indexes: (ws,recipient,read,created DESC), partial unread, created DESC |
| 107 | `enable_pg_trgm.{up,down}.sql` | `CREATE EXTENSION IF NOT EXISTS pg_trgm;` plus GIN trigram indexes on `master_data.company_name`, `invoices.invoice_id` (use `IF NOT EXISTS` — these tables may not exist on this branch yet) |
| 108 | `seed_default_workspaces.{up,down}.sql` | Insert `dealls`, `kantorku`, `holding` per spec § Seed Data |

> **Note**: `workspace_settings` is intentionally separate from `workspaces.settings` JSONB — JSONB for dynamic merge, table for individually-queryable rows. Use JSONB for the main settings flow; the table is for future per-key lookups.

## Entities

`internal/entity/workspace.go` (extend existing if present):
```go
type Workspace struct {
    ID        uuid.UUID
    Slug      string
    Name      string
    Logo      string
    Color     string
    Plan      string  // Basic | Pro | Enterprise | Holding
    IsHolding bool
    MemberIDs []uuid.UUID  // nil unless holding
    Settings  map[string]any
    IsActive  bool
    CreatedAt time.Time
    UpdatedAt time.Time
}

type WorkspaceMember struct {
    ID          uuid.UUID
    WorkspaceID uuid.UUID
    UserID      uuid.UUID  // FK to users — may be NULL until users table exists
    UserEmail   string     // denormalized for lookups when users table absent
    Role        string     // owner | admin | member | viewer
    Permissions map[string]any
    IsActive    bool
    InvitedAt   *time.Time
    JoinedAt    *time.Time
    InvitedBy   *uuid.UUID
}

type WorkspaceInvitation struct {
    ID          uuid.UUID
    WorkspaceID uuid.UUID
    Email       string
    Role        string
    InviteToken string
    Status      string  // pending | accepted | expired | revoked
    InvitedBy   uuid.UUID
    AcceptedAt  *time.Time
    ExpiresAt   time.Time
    CreatedAt   time.Time
}
```

`internal/entity/user_preferences.go`, `integrations.go`, `notification.go` per shared specs.

## Repositories

```
internal/repository/
  workspace_repo.go        // List(userID), Get(id), Create, Update, SoftDelete, ListMembers, IsHolding, ResolveMemberIDs
  workspace_member_repo.go // Add, UpdateRole, Remove, ListByWorkspace, GetUserRole
  workspace_invitation_repo.go // Create (random URL-safe token), GetByToken, Accept, Expire (cron), List
  user_preferences_repo.go // Get(email,ws), Upsert(email,ws, partial map)
  workspace_integration_repo.go // Get(wsID), Update(wsID, struct) — encrypt sensitive fields
  notification_repo.go     // Create, ListByRecipient(wsID,userID, filter, cursor), Count, MarkRead, MarkAllRead
  search_repo.go           // SearchClients, SearchInvoices, SearchTemplates (parallel-callable)
```

Mocks under `internal/repository/mocks/`.

## Usecases

`internal/usecase/workspace/usecase.go`:
- `List(ctx, userID)` — for current user, splits regular vs holding, expands `member_ids`
- `Get(ctx, wsID, callerID)` — verifies membership, attaches `members` + `stats`
- `Create(ctx, callerID, req)` — creates ws + adds caller as owner. Slug uniqueness check (return `Conflict`). Returns spec response.
- `Update(ctx, wsID, callerRole, partial)` — owner/admin only. **Slug immutable**. JSONB settings deep-merge.
- `SoftDelete(ctx, wsID, callerRole)` — owner only. Cascades `is_active=false` to members. Master data preserved.
- `Switch(ctx, callerID, wsID)` — audit-only; appends to `action_log` with `resource_type=workspace`, `action=switch`. Returns workspace + role.

`internal/usecase/workspace_member/usecase.go`:
- `List`, `Invite` (creates invitation row, sends email if SMTP configured, returns token), `UpdateRole`, `Remove` (forbid removing self if owner), `AcceptInvitation(token)`.

`internal/usecase/preferences/usecase.go`:
- `Get(email, ws)`, `Update(email, ws, partial)` — JSONB merge, not replace.

`internal/usecase/integrations/usecase.go`:
- `Get(wsID)` — decrypts, returns `has_*` booleans (never raw secrets)
- `Update(wsID, partial)` — encrypts, sets `*_active` flags. **API key changes trigger checker-maker approval** per `00-shared/05` § "change_integration"
- `Test(wsID, provider)` — calls each provider's lightweight ping (HaloAI: GET /me, Telegram: getMe, Paper.id: GET /v1/businesses, SMTP: connect-only)

`internal/usecase/notification/usecase.go`:
- `Create(ctx, req)` — INSERT, dispatch to channels (in_app always; telegram via `usecase/telegram`; email via SMTP). Idempotent on `(ws, source_feature, source_id, type)` for a 5-minute window
- `List(ctx, ws, recipient, filters, cursor)`, `Count(ctx, ws, recipient)`, `MarkRead(id)`, `MarkAllRead(ws, recipient)`

`internal/usecase/search/usecase.go`:
- `Search(ctx, ws, q, types, perTypeLimit)` — `errgroup.WithContext` parallel queries. Use `pg_trgm` `%` operator for fuzzy. Holding view expands to `member_ids`.

## HTTP routes

```go
// internal/delivery/http/route.go (add to api group)
api.Handle(GET,    "/workspaces",                              jwtAuth(workspaceH.List))         // existing — extend
api.Handle(GET,    "/workspaces/{id}",                         jwtAuth(workspaceH.Get))
api.Handle(POST,   "/workspaces",                              jwtAuth(workspaceH.Create))
api.Handle(PUT,    "/workspaces/{id}",                         jwtAuth(workspaceH.Update))
api.Handle(DELETE, "/workspaces/{id}",                         jwtAuth(workspaceH.SoftDelete))
api.Handle(POST,   "/workspaces/{id}/switch",                  jwtAuth(workspaceH.Switch))

api.Handle(GET,    "/workspaces/{id}/members",                 jwtAuth(memberH.List))
api.Handle(POST,   "/workspaces/{id}/members/invite",          jwtAuth(memberH.Invite))
api.Handle(PUT,    "/workspaces/{id}/members/{member_id}",     jwtAuth(memberH.UpdateRole))
api.Handle(DELETE, "/workspaces/{id}/members/{member_id}",     jwtAuth(memberH.Remove))
api.Handle(POST,   "/workspaces/invitations/{token}/accept",   jwtAuth(memberH.AcceptInvitation))

api.Handle(GET,    "/workspaces/{id}/settings",                jwtAuth(workspaceH.GetSettings))
api.Handle(PUT,    "/workspaces/{id}/settings",                jwtAuth(workspaceH.UpdateSettings))
api.Handle(GET,    "/workspaces/{id}/theme",                   jwtAuth(workspaceH.GetTheme))
api.Handle(PUT,    "/workspaces/{id}/theme",                   jwtAuth(workspaceH.UpdateTheme))

api.Handle(GET,    "/preferences",                             wsRequired(jwtAuth(prefsH.Get)))
api.Handle(PUT,    "/preferences",                             wsRequired(jwtAuth(prefsH.Update)))
api.Handle(PUT,    "/preferences/theme",                       wsRequired(jwtAuth(prefsH.UpdateTheme)))

api.Handle(GET,    "/integrations",                            wsRequired(jwtAuth(integH.Get)))
api.Handle(PUT,    "/integrations",                            wsRequired(jwtAuth(integH.Update)))
api.Handle(POST,   "/integrations/test/{provider}",            wsRequired(jwtAuth(integH.Test)))

api.Handle(GET,    "/notifications",                           wsRequired(jwtAuth(notifH.List)))
api.Handle(GET,    "/notifications/count",                     wsRequired(jwtAuth(notifH.Count)))
api.Handle(PUT,    "/notifications/{id}/read",                 wsRequired(jwtAuth(notifH.MarkRead)))
api.Handle(PUT,    "/notifications/read-all",                  wsRequired(jwtAuth(notifH.MarkAllRead)))

api.Handle(GET,    "/search",                                  wsRequired(jwtAuth(searchH.Search)))
```

## DI wiring

`internal/delivery/http/deps/deps.go`:
- Add fields: `WorkspaceRepo`, `WorkspaceMemberRepo`, `WorkspaceInvitationRepo`, `UserPreferencesRepo`, `IntegrationsRepo`, `NotificationRepo`, `SearchRepo`
- Add usecases: `WorkspaceUC`, `WorkspaceMemberUC`, `PreferencesUC`, `IntegrationsUC`, `NotificationUC`, `SearchUC`
- New env: `INTEGRATIONS_KEY` (32 bytes hex) — add to `config/config.go` + `validateRequired` (production only)
- New env: `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_USE_TLS` — required if email_active anywhere

## Tests (target ≥80% coverage on usecase/)

- `usecase/workspace/usecase_test.go` — table-driven: List filtering, holding expansion, Create slug-conflict, Update merge semantics, SoftDelete cascade
- `usecase/workspace_member/usecase_test.go` — Invite duplicate, role-removal-of-self, AcceptInvitation expiry/revoked
- `usecase/notification/usecase_test.go` — idempotent dedup window, channel dispatch, MarkAllRead scoping
- `usecase/search/usecase_test.go` — holding query path, type filtering, parallel goroutine cancellation
- `usecase/integrations/usecase_test.go` — encryption round-trip, has_* never exposes raw, approval triggered on key change
- Repo tests: keep light, mock pgx Pool

## Risks / business-rule conflicts with CLAUDE.md

- **`workspace_id` header naming**: spec uses `X-Workspace-ID`; existing `WorkspaceIDMiddleware` reads `workspace_id` (lowercase). Reuse the existing middleware as-is — frontend already adapted.
- **No `users` table on master**: spec FKs reference `users(id)`. Use `user_email VARCHAR(255)` as the canonical key (matches `auth/whitelist` and `user_preferences` shared spec). FKs to `users` are conditional — wrap in `DO $$ ... IF EXISTS ... $$` or skip them entirely on this branch.
- **Existing dashboard `WorkspaceHandler.List`** is a thin stub; rename + replace, don't fork.
- **Holding mutations**: holding workspace must reject any write to master-data-scoped endpoints. Add a usecase guard `IsHoldingReadOnly(wsID)` and return `Forbidden("holding workspaces are read-only")`.
- **Notifications dedup**: must NOT collide with `escalations.dedup` rules in `internal/usecase/escalation/handler.go`. Notifications are visual; escalations are state. Both fire on the same event but write to different tables.

## File checklist

- [ ] migrations 100–108 (.up.sql + .down.sql each)
- [ ] entity: workspace, workspace_member, workspace_invitation, user_preferences, integrations, notification
- [ ] repository: 7 repos + 7 mocks
- [ ] usecase: workspace, workspace_member, preferences, integrations, notification, search (each with `_test.go`)
- [ ] delivery/http/dashboard/{workspace,member,prefs,integrations,notification,search}_handler.go (Swagger annotations on every handler)
- [ ] route.go updates (block above)
- [ ] deps.go updates
- [ ] cmd/server/main.go wiring
- [ ] config.go: `IntegrationsKey`, SMTP block, validation
- [ ] swag regen via `make swag`
- [ ] `make lint && make unit-test` green
- [ ] Conventional-commit history (1 commit per layer is fine)
- [ ] `git push -u origin feat/02-workspace`
