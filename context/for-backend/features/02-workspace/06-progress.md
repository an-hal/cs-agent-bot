# Workspace — Implementation Progress

## 2026-04-23 — Holding/operating sync

### FE (shipped)
- Workspace switching now respects active holding ↔ operating context (no stale data after switch)
- Workspace Audit Log page surfaces cross-workspace access events (Admin-only)

### Backend spec (documented, implementation pending)
- Cross-references added to `01-auth/01-overview.md §2c` for the 3-tier holding/operating isolation model
- This feature inherits the `audit_logs.workspace_access` table + query-layer tier enforcement defined in 01-auth

### Open dependencies (backend)
- Workspace member management API (#27-30 below) still pending — required before role assignment can be persisted
- Holding aggregation queries (#34) need to honor tier model: holding workspaces read across `member_ids` but log each cross-tier access
- Workspace switching audit trail (#35) overlaps with `audit_logs.workspace_access` — implement together

### Cross-refs
- See `01-auth/06-progress.md` 2026-04-23 entry for tier model details
- FE switch: `contexts/CompanyContext.tsx` (already lists holding `member_ids`)

---

> **Overall: 36% complete** (16/45 items done or partial)
> - Frontend/BFF: 80% done (13 done + 3 partial)
> - Backend (Go): 0% done (24 items not started — includes notifications)
> - Optional improvements: 0% done (5 items)
>
> **Note**: Notifications system spec added at `07-notifications.md` (cross-cutting feature).

---

## DONE — Frontend/BFF ✅ (13 items)

| # | Item | File | Notes |
|---|------|------|-------|
| 1 | List workspaces API proxy | `app/api/workspaces/route.ts` | Extracts cookie, forwards to backend, returns `{ data: [...] }` |
| 2 | Workspace service (httpClient) | `lib/api/workspace.service.ts` | `getWorkspaces(token)` with DASHBOARD_API_URL base |
| 3 | CompanyProvider context | `contexts/CompanyContext.tsx` | Fetches workspaces on mount, splits regular vs holdings, persists active |
| 4 | Slug-to-UUID redirect (middleware) | `proxy.ts` | Static SLUG_TO_UUID map, 301 redirect before auth check |
| 5 | UUID-based URL routing | `proxy.ts` | `/dashboard/{uuid}/...` as primary URL strategy |
| 6 | Workspace switching (client-side) | `components/common/CompanySwitcher.tsx` | Dropdown, navigate to new UUID path, persist to localStorage |
| 7 | Holding workspace support | `contexts/CompanyContext.tsx` | `is_holding` flag, `member_ids` UUID-to-slug resolution |
| 8 | findByParam resolution | `contexts/CompanyContext.tsx` | Matches by UUID, slug, or id — used in page [workspace] params |
| 9 | Theme palette generator | `contexts/CompanyContext.tsx` | `hexToHsl` / `hslToHex` / `generateBrandPalette` — full HSL algorithm |
| 10 | CSS custom property injection | `contexts/CompanyContext.tsx` | `applyBrandCSSVars()` — 11 CSS variables including `--brand-rgb` |
| 11 | Per-workspace theme selection | `app/dashboard/[workspace]/settings/page.tsx` | 9 presets (amethyst..sakura), stored in localStorage `workspace_themes` |
| 12 | Dark/light mode toggle | `app/dashboard/[workspace]/settings/page.tsx` | Toggle via ThemeContext, persisted in localStorage `theme` |
| 13 | Settings page — workspace tab | `app/dashboard/[workspace]/settings/page.tsx` | Lists all workspaces with UUID, slug, plan, type, holding members |

## PARTIAL ⚠️ (3 items)

| # | Item | What's Done | What's Missing |
|---|------|-------------|----------------|
| 14 | Global search proxy | `app/api/search/route.ts` proxies to backend, parallel clients + invoices | Spec wants `templates` type too; no `types` filter param; no `match_field` in response |
| 15 | Slug-to-UUID mapping | Static map for 3 known slugs in `proxy.ts` | No dynamic resolution for new workspaces created via API |
| 16 | 401 handling on workspace fetch | Calls `handleSessionExpired()` on dashboard pages | No retry/refresh token flow; relies on cookie expiry only |

## NOT DONE — Backend (Go) Required 🔴 (19 items)

### Critical (blocks other features)

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 17 | `workspaces` table | 02-database-schema | id, slug, name, logo, color, plan, is_holding, member_ids, settings JSONB |
| 18 | `workspace_members` table | 02-database-schema | user_id + workspace_id + role + permissions JSONB, unique constraint |
| 19 | `workspace_settings` table | 02-database-schema | Key-value settings per workspace (theme_preset, timezone, etc.) |
| 20 | `workspace_invitations` table | 02-database-schema | invite_token, status (pending/accepted/expired/revoked), expires_at |
| 21 | GET `/workspaces` (backend) | 03-api-endpoints | Query workspaces via workspace_members join, return with auth check |
| 22 | X-Workspace-ID validation | 01-overview | Backend must validate header exists, user has access, filter all queries |

### High Priority

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 23 | GET `/workspaces/{id}` | 03-api-endpoints | Single workspace with members list + stats |
| 24 | POST `/workspaces` | 03-api-endpoints | Create workspace, auto-add creator as owner in workspace_members |
| 25 | PUT `/workspaces/{id}` | 03-api-endpoints | Partial update (name, color, settings merge). Slug immutable. |
| 26 | DELETE `/workspaces/{id}` | 03-api-endpoints | Soft-delete (is_active=false). Owner only. |
| 27 | GET `/workspaces/{id}/members` | 03-api-endpoints | List members with role, permissions, joined_at |
| 28 | POST `/workspaces/{id}/members/invite` | 03-api-endpoints | Send invitation, create pending record |
| 29 | PUT `/workspaces/{id}/members/{member_id}` | 03-api-endpoints | Update member role. Owner only. |
| 30 | DELETE `/workspaces/{id}/members/{member_id}` | 03-api-endpoints | Remove member. Owner only. Cannot remove self. |

### Medium Priority

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 31 | GET `/workspaces/{id}/settings` | 03-api-endpoints | All settings for workspace |
| 32 | PUT `/workspaces/{id}/settings` | 03-api-endpoints | Merge-update workspace settings |
| 33 | GET `/search` (backend) | 04-global-search | Full-text search across clients + invoices + templates, parallel queries |
| 34 | Holding query expansion | 01-overview | `WHERE workspace_id IN (member_ids)` for holding workspaces |

### Low Priority

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 35 | POST `/workspaces/{id}/switch` | 03-api-endpoints | Optional audit trail for workspace switching |

## NOT DONE — Optional Frontend Improvements 🟡 (5 items)

| # | Item | Priority | Description |
|---|------|----------|-------------|
| 36 | GET/PUT `/workspaces/{id}/theme` | Medium | Server-side theme persistence for cross-device sync (currently localStorage only) |
| 37 | `user_preferences` table | Medium | Backend storage: user_id + workspace_id + theme_id + dark_mode |
| 38 | Dynamic slug-to-UUID resolution | Medium | Fetch mapping from API instead of static SLUG_TO_UUID in proxy.ts |
| 39 | Search: template type support | Low | Add message_templates search to global search endpoint |
| 40 | Search: pg_trgm fuzzy indexes | Low | GIN trigram indexes for fuzzy search performance |

### Notifications (see `07-notifications.md`)

| # | Item | Priority | Description |
|---|------|----------|-------------|
| 41 | `notifications` table | High | DB table for in-app notifications with type, message, href, read status |
| 42 | GET `/notifications` | High | List notifications for current user (cursor pagination, unread filter) |
| 43 | GET `/notifications/count` | High | Quick unread count for badge (polled every 30s) |
| 44 | PUT `/notifications/{id}/read` | High | Mark single notification as read |
| 45 | PUT `/notifications/read-all` | High | Mark all notifications as read |
| 46 | POST `/notifications` (internal) | High | Create notification from other services (workflow, invoices, escalation, team) |
| 47 | Telegram integration | Medium | Send alert notifications via Telegram Bot API to team leads |
| 48 | Notification service in Go | High | `notifService.Create()` called by workflow engine, invoices, escalation, team |

### Checker-Maker Approval

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 49 | Checker-maker for PUT `/integrations` (API key changes) | 03-api-endpoints | Require approval (type: `change_integration`) when API keys or secrets change |

---

## Recommended Implementation Order (Backend)

```
Week 1: #17 workspaces + #18 workspace_members + #21 GET /workspaces + #22 X-Workspace-ID validation
Week 2: #23 GET /workspaces/{id} + #24 POST + #25 PUT + #26 DELETE workspaces
Week 3: #27-30 member management + #20 workspace_invitations
Week 4: #31-32 settings + #33 search + #34 holding query expansion
Later:  #19 workspace_settings table + #35 switch audit + #36-40 optional
```

## Dependency Chain

```
workspaces ──→ workspace_members ──→ GET /workspaces ──→ all workspace-scoped endpoints
  │                    │
  │                    └──→ X-Workspace-ID validation (middleware)
  │                              │
  │                              └──→ holding query expansion
  │
  └──→ workspace_settings ──→ GET/PUT settings
  │
  └──→ workspace_invitations ──→ member invite flow
```
