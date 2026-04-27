-- Phase 2 — seed 12 additional permission modules per
-- context/for-backend/features/04-team/05-additional-modules.md.
--
-- New module IDs (12):
--   daily_tasks, approvals, workflows, templates, playbooks, activity_log,
--   automation_rules, handoff_sla, haloai_conversations, collections,
--   workspace_audit, workspace_settings
--
-- Spec assumed roles include "BD" / "BD Lead" which DO NOT exist in this DB.
-- Role mapping applied here:
--   AE Officer  ← spec AE  (+ elevated handoff_sla/haloai_conversations,
--                            since DB has no BD/BD Lead role to inherit those)
--   SDR Officer ← spec SDR
--   CS Officer  ← spec CS
--   Manager     ← spec Manager (also absorbs BD Lead-grade rows)
--   Admin/Super Admin/Finance/Viewer ← unchanged from spec
--
-- Idempotent: ON CONFLICT (role_id, workspace_id, module_id) DO NOTHING.
-- For new workspaces created via API after this migration, role_permissions
-- still need to be seeded — currently no in-app seeder exists; that gap is
-- tracked separately and out of scope for Phase 2.

-- ─── Super Admin: 'all' on every new module ──────────────────────────────────
INSERT INTO role_permissions
    (role_id, workspace_id, module_id,
     view_list, view_detail, can_create, can_edit, can_delete, can_export, can_import)
SELECT r.id, w.id, m.module_id,
       'all', TRUE, TRUE, TRUE, TRUE, TRUE, TRUE
FROM roles r
CROSS JOIN workspaces w
CROSS JOIN (VALUES
    ('daily_tasks'), ('approvals'), ('workflows'), ('templates'),
    ('playbooks'), ('activity_log'), ('automation_rules'),
    ('handoff_sla'), ('haloai_conversations'), ('collections'),
    ('workspace_audit'), ('workspace_settings')
) AS m(module_id)
WHERE r.name = 'Super Admin'
ON CONFLICT (role_id, workspace_id, module_id) DO NOTHING;

-- ─── Admin: full within workspace, conservative on system-level modules ─────
INSERT INTO role_permissions
    (role_id, workspace_id, module_id,
     view_list, view_detail, can_create, can_edit, can_delete, can_export, can_import)
SELECT r.id, w.id, m.module_id,
       'true', TRUE,
       -- create
       m.module_id IN ('daily_tasks','approvals','workflows','templates',
                       'playbooks','automation_rules','collections'),
       -- edit
       m.module_id IN ('daily_tasks','approvals','workflows','templates',
                       'playbooks','automation_rules','haloai_conversations',
                       'collections','workspace_settings'),
       -- delete
       m.module_id IN ('daily_tasks','approvals','workflows','templates',
                       'playbooks','automation_rules','haloai_conversations',
                       'collections'),
       -- export
       m.module_id IN ('daily_tasks','templates','playbooks','activity_log',
                       'automation_rules','handoff_sla','haloai_conversations',
                       'collections','workspace_audit'),
       -- import
       m.module_id IN ('daily_tasks','playbooks','collections')
FROM roles r
CROSS JOIN workspaces w
CROSS JOIN (VALUES
    ('daily_tasks'), ('approvals'), ('workflows'), ('templates'),
    ('playbooks'), ('activity_log'), ('automation_rules'),
    ('handoff_sla'), ('haloai_conversations'), ('collections'),
    ('workspace_audit'), ('workspace_settings')
) AS m(module_id)
WHERE r.name = 'Admin'
ON CONFLICT (role_id, workspace_id, module_id) DO NOTHING;

-- ─── Manager: workspace power-user except settings/sensitive ops ────────────
INSERT INTO role_permissions
    (role_id, workspace_id, module_id,
     view_list, view_detail, can_create, can_edit, can_delete, can_export, can_import)
SELECT r.id, w.id, m.module_id,
       CASE WHEN m.module_id = 'workspace_settings' THEN 'false' ELSE 'true' END,
       m.module_id <> 'workspace_settings',
       -- create
       m.module_id IN ('daily_tasks','approvals','templates','automation_rules',
                       'collections'),
       -- edit
       m.module_id IN ('daily_tasks','approvals','templates','playbooks',
                       'automation_rules','haloai_conversations','collections'),
       -- delete
       m.module_id IN ('daily_tasks','approvals','templates','automation_rules',
                       'haloai_conversations','collections'),
       -- export
       m.module_id IN ('daily_tasks','templates','activity_log','automation_rules',
                       'handoff_sla','haloai_conversations','collections',
                       'workspace_audit'),
       -- import
       FALSE
FROM roles r
CROSS JOIN workspaces w
CROSS JOIN (VALUES
    ('daily_tasks'), ('approvals'), ('workflows'), ('templates'),
    ('playbooks'), ('activity_log'), ('automation_rules'),
    ('handoff_sla'), ('haloai_conversations'), ('collections'),
    ('workspace_audit'), ('workspace_settings')
) AS m(module_id)
WHERE r.name = 'Manager'
ON CONFLICT (role_id, workspace_id, module_id) DO NOTHING;

-- ─── AE Officer: ops-grade access, absorbs BD/BD-Lead permissions ───────────
INSERT INTO role_permissions
    (role_id, workspace_id, module_id,
     view_list, view_detail, can_create, can_edit, can_delete, can_export, can_import)
SELECT r.id, w.id, m.module_id,
       CASE m.module_id
           WHEN 'daily_tasks'          THEN 'self'
           WHEN 'approvals'            THEN 'self'
           WHEN 'activity_log'         THEN 'self'
           WHEN 'automation_rules'     THEN 'false'
           WHEN 'workspace_audit'      THEN 'false'
           WHEN 'workspace_settings'   THEN 'false'
           ELSE 'true'
       END,
       m.module_id NOT IN ('automation_rules','workspace_audit','workspace_settings'),
       -- create
       m.module_id IN ('daily_tasks','approvals','collections'),
       -- edit
       m.module_id IN ('daily_tasks','templates','haloai_conversations'),
       -- delete
       m.module_id IN ('haloai_conversations','collections'),
       -- export
       m.module_id IN ('templates','handoff_sla','haloai_conversations','collections'),
       -- import
       FALSE
FROM roles r
CROSS JOIN workspaces w
CROSS JOIN (VALUES
    ('daily_tasks'), ('approvals'), ('workflows'), ('templates'),
    ('playbooks'), ('activity_log'), ('automation_rules'),
    ('handoff_sla'), ('haloai_conversations'), ('collections'),
    ('workspace_audit'), ('workspace_settings')
) AS m(module_id)
WHERE r.name = 'AE Officer'
ON CONFLICT (role_id, workspace_id, module_id) DO NOTHING;

-- ─── SDR Officer: similar to AE but no haloai create/edit ──────────────────
INSERT INTO role_permissions
    (role_id, workspace_id, module_id,
     view_list, view_detail, can_create, can_edit, can_delete, can_export, can_import)
SELECT r.id, w.id, m.module_id,
       CASE m.module_id
           WHEN 'daily_tasks'          THEN 'self'
           WHEN 'approvals'            THEN 'self'
           WHEN 'activity_log'         THEN 'self'
           WHEN 'automation_rules'     THEN 'false'
           WHEN 'workspace_audit'      THEN 'false'
           WHEN 'workspace_settings'   THEN 'false'
           ELSE 'true'
       END,
       m.module_id NOT IN ('automation_rules','workspace_audit','workspace_settings'),
       m.module_id IN ('daily_tasks','approvals','collections'),
       m.module_id IN ('daily_tasks','templates'),
       m.module_id IN ('collections'),
       m.module_id IN ('templates','collections'),
       FALSE
FROM roles r
CROSS JOIN workspaces w
CROSS JOIN (VALUES
    ('daily_tasks'), ('approvals'), ('workflows'), ('templates'),
    ('playbooks'), ('activity_log'), ('automation_rules'),
    ('handoff_sla'), ('haloai_conversations'), ('collections'),
    ('workspace_audit'), ('workspace_settings')
) AS m(module_id)
WHERE r.name = 'SDR Officer'
ON CONFLICT (role_id, workspace_id, module_id) DO NOTHING;

-- ─── CS Officer: ops view; haloai 'true' but read-only; no handoff_sla ─────
INSERT INTO role_permissions
    (role_id, workspace_id, module_id,
     view_list, view_detail, can_create, can_edit, can_delete, can_export, can_import)
SELECT r.id, w.id, m.module_id,
       CASE m.module_id
           WHEN 'daily_tasks'          THEN 'self'
           WHEN 'approvals'            THEN 'self'
           WHEN 'activity_log'         THEN 'self'
           WHEN 'automation_rules'     THEN 'false'
           WHEN 'handoff_sla'          THEN 'false'
           WHEN 'workspace_audit'      THEN 'false'
           WHEN 'workspace_settings'   THEN 'false'
           ELSE 'true'
       END,
       m.module_id NOT IN ('automation_rules','handoff_sla','workspace_audit','workspace_settings'),
       m.module_id IN ('daily_tasks','approvals','collections'),
       m.module_id IN ('daily_tasks','templates'),
       m.module_id IN ('collections'),
       m.module_id IN ('templates','collections'),
       FALSE
FROM roles r
CROSS JOIN workspaces w
CROSS JOIN (VALUES
    ('daily_tasks'), ('approvals'), ('workflows'), ('templates'),
    ('playbooks'), ('activity_log'), ('automation_rules'),
    ('handoff_sla'), ('haloai_conversations'), ('collections'),
    ('workspace_audit'), ('workspace_settings')
) AS m(module_id)
WHERE r.name = 'CS Officer'
ON CONFLICT (role_id, workspace_id, module_id) DO NOTHING;

-- ─── Finance: read-only on everything except templates/automation/haloai ──
INSERT INTO role_permissions
    (role_id, workspace_id, module_id,
     view_list, view_detail, can_create, can_edit, can_delete, can_export, can_import)
SELECT r.id, w.id, m.module_id,
       CASE m.module_id
           WHEN 'daily_tasks'          THEN 'self'
           WHEN 'approvals'            THEN 'true'
           WHEN 'workflows'            THEN 'false'
           WHEN 'templates'            THEN 'false'
           WHEN 'playbooks'            THEN 'true'
           WHEN 'activity_log'         THEN 'true'
           WHEN 'automation_rules'     THEN 'false'
           WHEN 'handoff_sla'          THEN 'false'
           WHEN 'haloai_conversations' THEN 'false'
           WHEN 'collections'          THEN 'true'
           WHEN 'workspace_audit'      THEN 'true'
           WHEN 'workspace_settings'   THEN 'false'
       END,
       m.module_id IN ('daily_tasks','approvals','playbooks','activity_log',
                       'collections','workspace_audit'),
       m.module_id IN ('daily_tasks','approvals'),
       FALSE,
       FALSE,
       m.module_id IN ('activity_log','collections','workspace_audit'),
       FALSE
FROM roles r
CROSS JOIN workspaces w
CROSS JOIN (VALUES
    ('daily_tasks'), ('approvals'), ('workflows'), ('templates'),
    ('playbooks'), ('activity_log'), ('automation_rules'),
    ('handoff_sla'), ('haloai_conversations'), ('collections'),
    ('workspace_audit'), ('workspace_settings')
) AS m(module_id)
WHERE r.name = 'Finance'
ON CONFLICT (role_id, workspace_id, module_id) DO NOTHING;

-- ─── Viewer: lock everything off except daily_tasks/playbooks read ────────
INSERT INTO role_permissions
    (role_id, workspace_id, module_id,
     view_list, view_detail, can_create, can_edit, can_delete, can_export, can_import)
SELECT r.id, w.id, m.module_id,
       CASE m.module_id
           WHEN 'daily_tasks' THEN 'self'
           WHEN 'playbooks'   THEN 'true'
           ELSE 'false'
       END,
       m.module_id IN ('daily_tasks','playbooks'),
       FALSE, FALSE, FALSE, FALSE, FALSE
FROM roles r
CROSS JOIN workspaces w
CROSS JOIN (VALUES
    ('daily_tasks'), ('approvals'), ('workflows'), ('templates'),
    ('playbooks'), ('activity_log'), ('automation_rules'),
    ('handoff_sla'), ('haloai_conversations'), ('collections'),
    ('workspace_audit'), ('workspace_settings')
) AS m(module_id)
WHERE r.name = 'Viewer'
ON CONFLICT (role_id, workspace_id, module_id) DO NOTHING;
