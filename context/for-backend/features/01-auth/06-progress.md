# Auth — Implementation Progress

## 2026-04-23 — Workspace + role security sync

### FE (shipped)
- RBAC `can()` chokepoint added — single permission gate for all UI actions (`lib/rbac/can.ts`)
- `useActor()` hook surfaces current user's role, division, workspace tier, and permission set to components
- Settings → System Config tab (Admin-only) — surfaces escalation severity matrix + workspace isolation tier toggles
- Workspace Audit Log viewer page (Admin-only) — read-only feed of cross-workspace access events

### Backend spec (documented, implementation pending)
- §2c Workspace Data Isolation added in `01-overview.md` — 3-tier holding/operating model, query-layer enforcement contract, `audit_logs.workspace_access` table schema
- §2d Role-Specific Escalation Severity added in `01-overview.md` — 5-row matrix (BD / SDR / AE / CS / GUARD) with severity tiers read from `system_config`
- Cross-references: see `02-database-schema.md` for `audit_logs.workspace_access` columns; `03-api-endpoints.md` for `GET /audit-logs/workspace-access`

### Open dependencies (backend)
- Implement holding ↔ operating tier joins at query layer (every workspace-scoped read must respect tier)
- Create `audit_logs.workspace_access` table + write path (every cross-workspace read logged)
- Wire escalation severity reading from `system_config` table — currently FE reads hardcoded matrix, must fetch from backend
- `can()` chokepoint needs server-side mirror for API authorization (FE check is convenience only, not enforcement)

### Cross-refs
- FE permission contract: `lib/rbac/can.ts`, `hooks/useActor.ts`
- Audit log viewer: `app/dashboard/[workspace]/settings/audit-log/page.tsx`
- Backend gap doc: `claude/for-backend/01-auth/gap-2c-workspace-isolation.md`, `gap-2d-escalation-severity.md`
