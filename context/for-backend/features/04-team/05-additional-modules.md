# Additional Permission Modules — Phase 2

**Status:** Spec, awaiting BE implementation.
**Owner:** FE (Arief) + BE.
**Tracking:** Sidebar gating in `components/common/Sidebar.tsx` already references these modules; until BE seeds them, `can(<module>, 'view_list')` returns `false` and the items are hidden for every non–Super-Admin role. Super Admin bypasses module checks via `is_system=true` short-circuit (see `02-database-schema.md §1`), so the live UI for Sejutacita is unaffected.

---

## 1. Background

The `role_permissions.module_id` column (`VARCHAR(50)`, see `02-database-schema.md §2`) currently carries 9 values across the seed:

```
ae, analytics, bd, cs, dashboard, data_master, reports, sdr, team
```

The dashboard sidebar surfaces ~30 menu items. Most of those mapped naturally to the 9 modules above, but the following had no module match and were rendered ungated:

| Sidebar group | Item | Was gated by |
|---|---|---|
| Daily Work | Daily Tasks | — |
| Daily Work | Approvals | — |
| Pipeline | Workflow (root) | — |
| Templates | Message Template, Email Template | — |
| Playbooks | (8 docs items) | — |
| Monitoring | Activity Log, Automation Rules, Handoff SLA, HaloAI Conversations | — |
| Master Data | Collections | — |
| Settings | Workspace Audit, Settings | — |

Leaving them ungated means a `viewer` role still sees admin pages like *Workspace Audit* and *Automation Rules*, even though hitting the underlying API endpoints would 403. We want the sidebar to reflect actual capability, so we're proposing **12 new modules** to cover this gap.

## 2. New modules to seed

Add to the `modules` lookup (or wherever module IDs are validated server-side) and to every existing role's `role_permissions` rows.

| `module_id` | Sidebar surface | Default scope philosophy |
|---|---|---|
| `daily_tasks` | Daily Work → Daily Tasks | Personal task list. Every active member should have at least `view_list='true'` + `view_detail=true` for own tasks. |
| `approvals` | Daily Work → Approvals | Workspace queue. Reviewers see `'true'`, requesters see `'self'` (their own pending requests), viewers `'false'`. |
| `workflows` | Pipeline → Workflow (root) | Read-only overview of pipeline definitions. Most roles `'true'`; only admins can `edit`/`create`. |
| `templates` | Templates → Message + Email Template | Single module covers both surfaces. SDR/BD operators `'true'`; admin-only for `edit`/`create`/`delete`. |
| `playbooks` | Playbooks → 8 doc pages | Static reference docs. Default `'true'` for everyone; `edit` reserved to playbook authors (admin-grade). |
| `activity_log` | Monitoring → Activity Log | Audit timeline. Admin/Manager `'all'` or `'true'`; ops roles `'self'` (own actions); viewer `'false'`. |
| `automation_rules` | Monitoring → Automation Rules | Power-user feature. Admin/Manager only; `'false'` for everyone else. |
| `handoff_sla` | Monitoring → Handoff SLA | BD ops dashboard. BD/Manager `'true'`; others `'false'`. Could alternatively be folded into existing `bd` — see §6. |
| `haloai_conversations` | Monitoring → HaloAI Conversations | Inbound WA/AI conversations. SDR/BD ops `'true'`; others `'false'`. |
| `collections` | Master Data → Collections | Saved client filters/cohorts. Mirrors `data_master` defaults. |
| `workspace_audit` | Settings → Workspace Audit | Compliance/audit log. Admin/Manager only. |
| `workspace_settings` | Settings → Settings | Workspace-level configuration (theme, integrations, etc.). Admin only for `edit`; others `'false'` or `'true'` (view). |

**Total after this phase:** 9 + 12 = **21 module IDs**.

> **Note on `Guide` (`/how-to`):** intentionally NOT a module. Product help docs are accessible to every authenticated user regardless of role. Keep ungated on FE, no BE row needed.

## 3. Default permission matrix per existing role

Schema reminder:

```
view_list   VARCHAR(5)  -- 'false' | 'true' | 'all'
view_detail BOOLEAN
can_create  BOOLEAN
can_edit    BOOLEAN
can_delete  BOOLEAN
can_export  BOOLEAN
can_import  BOOLEAN
```

Below: `view_list` value · `(c)reate` · `(e)dit` · `(d)elete` · `(x)export` · `(i)mport` — `view_detail` follows `view_list ≠ 'false'`.

| Module | Super Admin | Admin | Manager | BD Lead | SDR | BD | AE | CS | Finance | Viewer |
|---|---|---|---|---|---|---|---|---|---|---|
| `daily_tasks` | all·cedxi | true·cedxi | true·cedx- | true·cedx- | self·ce--- | self·ce--- | self·ce--- | self·ce--- | self·c---- | self·----- |
| `approvals` | all·cedxi | true·cedx- | true·cedx- | true·cedx- | self·c---- | self·c---- | self·c---- | self·c---- | true·c---- | false·----- |
| `workflows` | all·cedxi | true·ced-- | true·----- | true·----- | true·----- | true·----- | true·----- | true·----- | false·----- | false·----- |
| `templates` | all·cedxi | true·cedx- | true·cedx- | true·ced-- | true·-e-x- | true·-e-x- | true·-e-x- | true·-e-x- | false·----- | false·----- |
| `playbooks` | all·cedxi | true·cedxi | true·-e--- | true·-e--- | true·----- | true·----- | true·----- | true·----- | true·----- | true·----- |
| `activity_log` | all·----x- | true·----x- | true·----x- | true·----x- | self·----- | self·----- | self·----- | self·----- | true·----x- | false·----- |
| `automation_rules` | all·cedx- | true·cedx- | true·cedx- | false·----- | false·----- | false·----- | false·----- | false·----- | false·----- | false·----- |
| `handoff_sla` | all·----x- | true·----x- | true·----x- | true·----x- | true·----- | true·----- | false·----- | false·----- | false·----- | false·----- |
| `haloai_conversations` | all·-edx- | true·-edx- | true·-edx- | true·-edx- | true·----- | true·----- | false·----- | true·----- | false·----- | false·----- |
| `collections` | all·cedxi | true·cedxi | true·cedx- | true·cedx- | true·c-dx- | true·c-dx- | true·c-dx- | true·c-dx- | true·----x- | false·----- |
| `workspace_audit` | all·----x- | true·----x- | true·----x- | false·----- | false·----- | false·----- | false·----- | false·----- | true·----x- | false·----- |
| `workspace_settings` | all·-e--- | true·-e--- | false·----- | false·----- | false·----- | false·----- | false·----- | false·----- | false·----- | false·----- |

> Roles I assumed exist: Super Admin, Admin, Manager, BD Lead, SDR, BD, AE, CS, Finance, Viewer. Adjust to match the actual `roles` rows in BE seed. The `is_system=true` Super Admin already short-circuits to `'all'` for every module — these rows can be inserted as belt-and-suspenders or skipped entirely.

## 4. Migration outline

```sql
-- 1. (No DDL needed.) `module_id` is VARCHAR(50); new IDs are valid as-is.

-- 2. Seed missing rows for every (role × workspace) pair. Pseudocode below;
--    actual migration should iterate over roles + workspaces and use the
--    per-role defaults from §3. Keep idempotent so re-running is safe.
INSERT INTO role_permissions (
  role_id, workspace_id, module_id,
  view_list, view_detail, can_create, can_edit, can_delete, can_export, can_import
)
SELECT r.id, w.id, m.module_id,
       m.view_list, m.view_detail, m.can_create, m.can_edit, m.can_delete, m.can_export, m.can_import
FROM roles r
CROSS JOIN workspaces w
CROSS JOIN (VALUES
  ('daily_tasks',          'true', true, true, true, false, false, false),
  ('approvals',            'true', true, true, false, false, false, false),
  ('workflows',            'true', true, false, false, false, false, false),
  ('templates',            'true', true, false, true, false, true, false),
  ('playbooks',            'true', true, false, false, false, false, false),
  ('activity_log',         'true', true, false, false, false, true, false),
  ('automation_rules',     'false', false, false, false, false, false, false),
  ('handoff_sla',          'false', false, false, false, false, false, false),
  ('haloai_conversations', 'false', false, false, false, false, false, false),
  ('collections',          'true', true, true, true, true, true, true),
  ('workspace_audit',      'false', false, false, false, false, false, false),
  ('workspace_settings',   'false', false, false, false, false, false, false)
) AS m(module_id, view_list, view_detail, can_create, can_edit, can_delete, can_export, can_import)
ON CONFLICT (role_id, workspace_id, module_id) DO NOTHING;

-- 3. Override per-role defaults from §3 with a follow-up UPDATE (or do it
--    in the application-layer seeding script — Go side typically owns this).

-- 4. (Optional) Add a CHECK constraint to prevent typos:
-- ALTER TABLE role_permissions
--   ADD CONSTRAINT chk_module_id CHECK (module_id IN (
--     'ae','analytics','bd','cs','dashboard','data_master','reports','sdr','team',
--     'daily_tasks','approvals','workflows','templates','playbooks','activity_log',
--     'automation_rules','handoff_sla','haloai_conversations','collections',
--     'workspace_audit','workspace_settings'
--   ));
```

**Defaults above are SAFE-MODE** (everyone reads except where role obviously has access). The per-role table in §3 is what the seed script should produce.

## 5. API contract — no changes needed

`GET /api/team/permissions/me` already returns:

```json
{
  "data": {
    "role": { "id": "...", "name": "...", "is_system": true, ... },
    "workspace_id": "...",
    "permissions": {
      "<module_id>": {
        "view_list": "all" | "true" | "false",
        "view_detail": true,
        "create": true, "edit": true, "delete": true,
        "export": true, "import": true
      }
    }
  }
}
```

After this migration, the `permissions` map will simply include 12 more keys. FE handles unknown/missing modules safely (returns `false` from `can()`), so partial rollout is OK — the BE can land modules incrementally without breaking the dashboard.

## 6. Open questions

1. **Fold `handoff_sla` into existing `bd`?** The Handoff SLA dashboard is BD-operational. If you'd rather not add a new module, change the FE side: `{ label: 'Handoff SLA', ..., permModule: 'bd' }`. I went with a separate module because non-BD roles (Manager, Finance) sometimes need SLA visibility without all of BD.
2. **Folder `templates` vs split (`message_templates` + `email_templates`)?** I chose unified `templates`. If editorial control differs (e.g. only specific roles can edit email templates because they're customer-facing), split them.
3. **`is_system=true` short-circuit in BE.** Super Admin currently returns `view_list='all'` for everything, derived from `is_system`. Confirm: will the BE keep that behavior for new modules too? If yes, no per-role row needed for Super Admin. If no, add explicit `'all'` rows in the seed.
4. **`Guide` (`/how-to`).** Confirm we don't want this gated. If product help should be role-restricted (it's not currently), add a `help_docs` module.

## 7. Acceptance criteria

- [ ] `INSERT INTO role_permissions` populates 12 new modules × N roles × M workspaces.
- [ ] `GET /api/team/permissions/me` returns 21 module keys (was 9) for any active workspace.
- [ ] Sidebar items in `Sidebar.tsx` resolve via `can('<new-module>', 'view_list')` after FE PR lands — verify with the role-impersonation curl: `curl ... -H 'X-Dev-Roles: viewer'`.
- [ ] No regression for existing 9 modules.
- [ ] CHECK constraint (if added) accepts all 21 IDs and rejects unknown values.
