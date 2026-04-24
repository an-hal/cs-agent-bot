# Team Management & RBAC — Implementation Progress

## 2026-04-23 — RBAC dev tooling sync

### FE (shipped)
- Role Switcher dev panel — lets devs impersonate any role/division for QA without re-login (gated to non-prod env)
- RBAC permission matrix view — surfaces full module × action × role grid; reads from `lib/rbac/can.ts` chokepoint
- Both feed off `useActor()` from 01-auth (single source of truth for current actor)

### Backend spec (no new sections this round)
- No new spec changes — backend implementation is the gap, not the spec
- Role/division resolution must read from session token claims (added in 01-auth §2c context)

### Open dependencies (backend)
- Implement role/division resolution from session — backend must populate `actor.role` + `actor.division` from team_members join on every request
- Implement `can()` chokepoint server-side — every protected endpoint must call permission middleware (#28 below)
- Without this, FE `can()` is decorative — API is still reachable by any authenticated user

### Cross-refs
- FE chokepoint: `lib/rbac/can.ts`, `hooks/useActor.ts` (shared with 01-auth)
- Role Switcher: `components/dev/RoleSwitcher.tsx`
- See `01-auth/06-progress.md` 2026-04-23 for the broader workspace+role security context

---

> **Overall: 12% complete** (5/43 items done or partial)
> - Frontend/BFF: 30% done (3 done + 2 partial)
> - Backend (Go): 0% done (31 items not started)
> - Optional improvements: 0% done (5 items)

---

## DONE — Frontend/BFF ✅ (3 items)

| # | Item | File | Notes |
|---|------|------|-------|
| 1 | Team page UI with member table | `app/dashboard/[workspace]/team/page.tsx` | Ant Design table listing members with role, status, workspaces, department |
| 2 | Role & permission matrix UI | `app/dashboard/[workspace]/team/page.tsx` | Full permission grid per module per workspace, role detail drawer |
| 3 | Mock roles with permission builders | `app/dashboard/[workspace]/team/page.tsx` | 8 roles: Super Admin, Admin (per ws), Manager, AE/SDR/CS Officer, Finance, Viewer — all with correct permission matrices |

## PARTIAL ⚠️ (2 items)

| # | Item | What's Done | What's Missing |
|---|------|-------------|----------------|
| 4 | Member management UI | Add/edit/deactivate member modals exist in page | **All data is mock/hardcoded** — no API calls. Comment says "in prod this would hit /api/team-activity". No fetch(), no backend proxy. |
| 5 | Role CRUD UI | Create/edit role forms with permission matrix editor | **All in-memory** — useState only. Changes are lost on page refresh. No API integration. |

## NOT DONE — Backend (Go) Required 🔴 (31 items)

### Critical (blocks real team management)

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 6 | `roles` table | 02-database-schema | id, name, description, color, bg_color, is_system. System roles cannot be deleted. |
| 7 | `role_permissions` table | 02-database-schema | role_id + workspace_id + module_id + 7 permission flags (view_list as VARCHAR for 'all' scope) |
| 8 | `team_members` table | 02-database-schema | user_id (nullable for pending), name, email, role_id, status, department, invite_token |
| 9 | `member_workspace_assignments` table | 02-database-schema | member_id + workspace_id mapping (1 member can be in multiple workspaces) |
| 10 | `role_workspace_scope` table | 02-database-schema | Defines which workspaces a role is available in |

### High Priority — Member Endpoints

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 11 | GET `/team/members` | 03-api-endpoints | List with status/role/search filters, pagination, summary counts |
| 12 | GET `/team/members/{id}` | 03-api-endpoints | Single member detail with role + workspaces |
| 13 | POST `/team/members/invite` | 03-api-endpoints | Create pending member, generate invite_token, send email |
| 14 | PUT `/team/members/{id}` | 03-api-endpoints | Update name, department, avatar_color |
| 15 | PUT `/team/members/{id}/role` | 03-api-endpoints | Change role. Cannot change Super Admin unless you are Super Admin. |
| 16 | PUT `/team/members/{id}/status` | 03-api-endpoints | Activate/deactivate member with reason |
| 17 | PUT `/team/members/{id}/workspaces` | 03-api-endpoints | Update workspace assignments |
| 18 | POST `/team/members/{id}/reset-password` | 03-api-endpoints | Trigger password reset email |
| 19 | DELETE `/team/members/{id}` | 03-api-endpoints | Remove member. Cannot delete self or Super Admin (unless you are one). |

### High Priority — Role Endpoints

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 20 | GET `/team/roles` | 03-api-endpoints | List roles with member_count, workspace scope |
| 21 | GET `/team/roles/{id}` | 03-api-endpoints | Role detail with full permission matrix (per workspace, per module) |
| 22 | POST `/team/roles` | 03-api-endpoints | Create custom role with permission matrix |
| 23 | PUT `/team/roles/{id}` | 03-api-endpoints | Update role metadata (name, description, color, workspace scope) |
| 24 | PUT `/team/roles/{id}/permissions` | 03-api-endpoints | Update permission matrix per module per workspace |
| 25 | DELETE `/team/roles/{id}` | 03-api-endpoints | Delete custom role. Blocked if has active members. System roles protected. |

### Medium Priority — Permission Middleware

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 26 | GET `/team/permissions/check` | 03-api-endpoints | Check if current user has specific action on specific module |
| 27 | GET `/team/permissions/me` | 03-api-endpoints | Full permission matrix for current user in current workspace |
| 28 | Permission enforcement middleware | 03-api-endpoints | `RequirePermission(module, action)` middleware for all API endpoints |
| 29 | Super Admin protection logic | 01-overview | Only Super Admin can manage other Super Admins |

### Medium Priority — Invitation Flow

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 30 | Invitation email sending | 01-overview | Send invite link with token to new member's email |
| 31 | POST `/auth/accept-invite` | 01-overview | Validate invite_token, set password, activate member, link user_id |
| 32 | Invite token expiry check | 02-database-schema | invite_expires validation on accept |

### Low Priority — Activity

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 33 | `team_activity_logs` table | 03-api-endpoints | Log all team actions: invite, role change, deactivate, etc. |
| 34 | GET `/team/activity` | 03-api-endpoints | Activity feed with action/actor/timestamp filters |

### Low Priority — Seed Data

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 35 | Seed system roles | 02-database-schema | Super Admin, Admin, Manager, AE Officer, SDR Officer, CS Officer, Finance, Viewer |
| 36 | Seed default permissions | 02-database-schema | Permission matrix for each system role per workspace per module |

### Checker-Maker Approval

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 44 | Checker-maker for POST `/team/members/invite` | 03-api-endpoints | Require approval (type: `invite_member`) before executing member invitation |
| 45 | Checker-maker for DELETE `/team/members/{id}` | 03-api-endpoints | Require approval (type: `remove_member`) before executing member removal |
| 46 | Checker-maker for PUT `/team/roles/{id}/permissions` | 03-api-endpoints | Require approval (type: `change_permission`) before applying permission changes |

## NOT DONE — Optional Frontend Improvements 🟡 (5 items)

| # | Item | Priority | Description |
|---|------|----------|-------------|
| 37 | BFF proxy routes for team API | Critical | Create `/api/team/members`, `/api/team/roles` proxy routes (none exist today) |
| 38 | Replace mock data with API fetch | Critical | Team page currently has zero API calls — all useState with hardcoded data |
| 39 | Permission-based UI rendering | High | Use `/team/permissions/me` to conditionally show/hide buttons and modules |
| 40 | Invitation accept page | Medium | Frontend page at `/invite?token=...` for accepting workspace invitations |
| 41 | Activity feed via backend | Low | Replace simulated polling comment with real `/team/activity` endpoint |
| 42 | Member workspace badge | Low | Show workspace assignments with live data instead of mock arrays |
| 43 | Role permission diff on save | Low | Show what changed before saving permission matrix updates |

---

## Recommended Implementation Order (Backend)

```
Week 1: #6 roles + #7 role_permissions + #35-36 seed system roles + default permissions
Week 2: #8 team_members + #9 member_workspace_assignments + #10 role_workspace_scope
Week 3: #11-12 GET members + #20-21 GET roles + #27 GET permissions/me
Week 4: #13 invite + #14-17 member updates + #18-19 reset/delete
Week 5: #22-25 role CRUD + #26 permission check + #28-29 middleware
Week 6: #30-32 invitation flow + #33-34 activity logs
```

## Dependency Chain

```
roles ──→ role_permissions ──→ permission middleware ──→ all protected endpoints
  │              │
  │              └──→ seed default permissions
  │
  └──→ team_members ──→ GET/POST/PUT/DELETE members
  │        │
  │        └──→ member_workspace_assignments ──→ workspace assignment mgmt
  │        │
  │        └──→ invite flow ──→ accept-invite ──→ activate member
  │
  └──→ role_workspace_scope ──→ role visibility per workspace
  │
  └──→ team_activity_logs ──→ activity feed
```

## Key Finding: No Backend Integration

The team page (`app/dashboard/[workspace]/team/page.tsx`) is **100% mock data**:
- All 8 roles are hardcoded with `buildWsPolicy()` helper functions
- All 27 members are hardcoded in `INITIAL_MEMBERS` array
- No `fetch()` calls exist anywhere in the file
- No `/api/team/` routes exist in the project
- The only API reference is a comment: `"in prod this would hit /api/team-activity"`
- All state management is via `useState` — changes lost on refresh

This means the frontend UI is a **functional prototype** but requires both backend endpoints AND BFF proxy routes before it can work with real data.
