package repository

import (
	"context"
	"fmt"
)

// SeedDefaultsForWorkspace inserts the default role × module permission matrix
// for a single workspace. Mirrors the per-role logic in migrations
// 20260414000305 (legacy 9 modules) and 20260426000100 (Phase 2 12 modules),
// scoped to the given workspaceID.
//
// Idempotent: ON CONFLICT (role_id, workspace_id, module_id) DO NOTHING. Safe
// to call after workspace creation; safe to re-run on existing workspaces too.
//
// Roles seeded: Super Admin, Admin, Manager, AE Officer, SDR Officer,
// CS Officer, Finance, Viewer (matches roles seed in migration 20260414000300).
func (r *rolePermissionRepo) SeedDefaultsForWorkspace(ctx context.Context, workspaceID string) error {
	ctx, span := r.tracer.Start(ctx, "role_permission.repository.SeedDefaultsForWorkspace")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, seedRolePermissionsSQL, workspaceID); err != nil {
		return fmt.Errorf("seed role_permissions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit seed: %w", err)
	}
	return nil
}

// seedRolePermissionsSQL is the consolidated per-role permission matrix for
// all 21 modules (9 legacy + 12 Phase 2), parameterized by $1 (workspace_id).
//
// Source of truth: migration files 20260414000305 + 20260426000100. When those
// per-role rules change, update this SQL in lockstep — there is no automated
// drift check.
const seedRolePermissionsSQL = `
WITH ws AS (SELECT $1::UUID AS id), modules_legacy AS (
    SELECT m FROM (VALUES
        ('dashboard'), ('analytics'), ('reports'),
        ('ae'), ('sdr'), ('bd'), ('cs'),
        ('data_master'), ('team')
    ) AS t(m)
), modules_phase2 AS (
    SELECT m FROM (VALUES
        ('daily_tasks'), ('approvals'), ('workflows'), ('templates'),
        ('playbooks'), ('activity_log'), ('automation_rules'),
        ('handoff_sla'), ('haloai_conversations'), ('collections'),
        ('workspace_audit'), ('workspace_settings')
    ) AS t(m)
)
INSERT INTO role_permissions
    (role_id, workspace_id, module_id,
     view_list, view_detail, can_create, can_edit, can_delete, can_export, can_import)

-- ─── Legacy 9 modules (mirrors 20260414000305) ──────────────────────────────

-- Super Admin
SELECT r.id, ws.id, ml.m,
       'all', TRUE, TRUE, TRUE, TRUE, TRUE, TRUE
FROM roles r CROSS JOIN ws CROSS JOIN modules_legacy ml
WHERE r.name = 'Super Admin'
UNION ALL
-- Admin
SELECT r.id, ws.id, ml.m,
       'true', TRUE, TRUE, TRUE,
       CASE WHEN ml.m = 'team' THEN FALSE ELSE TRUE END,
       TRUE,
       CASE WHEN ml.m IN ('data_master','ae','sdr','bd','cs') THEN TRUE ELSE FALSE END
FROM roles r CROSS JOIN ws CROSS JOIN modules_legacy ml
WHERE r.name = 'Admin'
UNION ALL
-- Manager
SELECT r.id, ws.id, ml.m,
       CASE WHEN ml.m = 'team' THEN 'false' ELSE 'true' END,
       ml.m <> 'team',
       ml.m <> 'team',
       ml.m <> 'team',
       FALSE,
       ml.m <> 'team',
       FALSE
FROM roles r CROSS JOIN ws CROSS JOIN modules_legacy ml
WHERE r.name = 'Manager'
UNION ALL
-- AE Officer
SELECT r.id, ws.id, ml.m,
       CASE WHEN ml.m = 'team' THEN 'false' ELSE 'true' END,
       ml.m <> 'team',
       ml.m IN ('ae','data_master'),
       ml.m IN ('ae','data_master'),
       FALSE,
       ml.m IN ('ae','data_master'),
       FALSE
FROM roles r CROSS JOIN ws CROSS JOIN modules_legacy ml
WHERE r.name = 'AE Officer'
UNION ALL
-- SDR Officer
SELECT r.id, ws.id, ml.m,
       CASE WHEN ml.m = 'team' THEN 'false' ELSE 'true' END,
       ml.m <> 'team',
       ml.m IN ('sdr','data_master'),
       ml.m IN ('sdr','data_master'),
       FALSE,
       ml.m IN ('sdr','data_master'),
       FALSE
FROM roles r CROSS JOIN ws CROSS JOIN modules_legacy ml
WHERE r.name = 'SDR Officer'
UNION ALL
-- CS Officer
SELECT r.id, ws.id, ml.m,
       CASE WHEN ml.m = 'team' THEN 'false' ELSE 'true' END,
       ml.m <> 'team',
       ml.m IN ('cs','data_master'),
       ml.m IN ('cs','data_master'),
       FALSE,
       ml.m IN ('cs','data_master'),
       FALSE
FROM roles r CROSS JOIN ws CROSS JOIN modules_legacy ml
WHERE r.name = 'CS Officer'
UNION ALL
-- Finance: read-only across reports/analytics/data_master/ae/sdr/bd/cs
SELECT r.id, ws.id, ml.m,
       CASE WHEN ml.m IN ('team') THEN 'false' ELSE 'true' END,
       ml.m <> 'team',
       FALSE, FALSE, FALSE,
       ml.m IN ('reports','analytics','data_master','ae','sdr','bd','cs'),
       FALSE
FROM roles r CROSS JOIN ws CROSS JOIN modules_legacy ml
WHERE r.name = 'Finance'
UNION ALL
-- Viewer: read-only minimal
SELECT r.id, ws.id, ml.m,
       CASE WHEN ml.m IN ('team') THEN 'false' ELSE 'true' END,
       ml.m <> 'team',
       FALSE, FALSE, FALSE, FALSE, FALSE
FROM roles r CROSS JOIN ws CROSS JOIN modules_legacy ml
WHERE r.name = 'Viewer'

-- ─── Phase 2 — 12 new modules (mirrors 20260426000100) ──────────────────────
UNION ALL
-- Super Admin (phase 2)
SELECT r.id, ws.id, mp.m,
       'all', TRUE, TRUE, TRUE, TRUE, TRUE, TRUE
FROM roles r CROSS JOIN ws CROSS JOIN modules_phase2 mp
WHERE r.name = 'Super Admin'
UNION ALL
-- Admin (phase 2)
SELECT r.id, ws.id, mp.m,
       'true', TRUE,
       mp.m IN ('daily_tasks','approvals','workflows','templates','playbooks','automation_rules','collections'),
       mp.m IN ('daily_tasks','approvals','workflows','templates','playbooks','automation_rules','haloai_conversations','collections','workspace_settings'),
       mp.m IN ('daily_tasks','approvals','workflows','templates','playbooks','automation_rules','haloai_conversations','collections'),
       mp.m IN ('daily_tasks','templates','playbooks','activity_log','automation_rules','handoff_sla','haloai_conversations','collections','workspace_audit'),
       mp.m IN ('daily_tasks','playbooks','collections')
FROM roles r CROSS JOIN ws CROSS JOIN modules_phase2 mp
WHERE r.name = 'Admin'
UNION ALL
-- Manager (phase 2)
SELECT r.id, ws.id, mp.m,
       CASE WHEN mp.m = 'workspace_settings' THEN 'false' ELSE 'true' END,
       mp.m <> 'workspace_settings',
       mp.m IN ('daily_tasks','approvals','templates','automation_rules','collections'),
       mp.m IN ('daily_tasks','approvals','templates','playbooks','automation_rules','haloai_conversations','collections'),
       mp.m IN ('daily_tasks','approvals','templates','automation_rules','haloai_conversations','collections'),
       mp.m IN ('daily_tasks','templates','activity_log','automation_rules','handoff_sla','haloai_conversations','collections','workspace_audit'),
       FALSE
FROM roles r CROSS JOIN ws CROSS JOIN modules_phase2 mp
WHERE r.name = 'Manager'
UNION ALL
-- AE Officer (phase 2) — absorbs BD/BD-Lead operator-grade rows
SELECT r.id, ws.id, mp.m,
       CASE mp.m
           WHEN 'daily_tasks'        THEN 'self'
           WHEN 'approvals'          THEN 'self'
           WHEN 'activity_log'       THEN 'self'
           WHEN 'automation_rules'   THEN 'false'
           WHEN 'workspace_audit'    THEN 'false'
           WHEN 'workspace_settings' THEN 'false'
           ELSE 'true'
       END,
       mp.m NOT IN ('automation_rules','workspace_audit','workspace_settings'),
       mp.m IN ('daily_tasks','approvals','collections'),
       mp.m IN ('daily_tasks','templates','haloai_conversations'),
       mp.m IN ('haloai_conversations','collections'),
       mp.m IN ('templates','handoff_sla','haloai_conversations','collections'),
       FALSE
FROM roles r CROSS JOIN ws CROSS JOIN modules_phase2 mp
WHERE r.name = 'AE Officer'
UNION ALL
-- SDR Officer (phase 2)
SELECT r.id, ws.id, mp.m,
       CASE mp.m
           WHEN 'daily_tasks'        THEN 'self'
           WHEN 'approvals'          THEN 'self'
           WHEN 'activity_log'       THEN 'self'
           WHEN 'automation_rules'   THEN 'false'
           WHEN 'workspace_audit'    THEN 'false'
           WHEN 'workspace_settings' THEN 'false'
           ELSE 'true'
       END,
       mp.m NOT IN ('automation_rules','workspace_audit','workspace_settings'),
       mp.m IN ('daily_tasks','approvals','collections'),
       mp.m IN ('daily_tasks','templates'),
       mp.m IN ('collections'),
       mp.m IN ('templates','collections'),
       FALSE
FROM roles r CROSS JOIN ws CROSS JOIN modules_phase2 mp
WHERE r.name = 'SDR Officer'
UNION ALL
-- CS Officer (phase 2)
SELECT r.id, ws.id, mp.m,
       CASE mp.m
           WHEN 'daily_tasks'        THEN 'self'
           WHEN 'approvals'          THEN 'self'
           WHEN 'activity_log'       THEN 'self'
           WHEN 'automation_rules'   THEN 'false'
           WHEN 'handoff_sla'        THEN 'false'
           WHEN 'workspace_audit'    THEN 'false'
           WHEN 'workspace_settings' THEN 'false'
           ELSE 'true'
       END,
       mp.m NOT IN ('automation_rules','handoff_sla','workspace_audit','workspace_settings'),
       mp.m IN ('daily_tasks','approvals','collections'),
       mp.m IN ('daily_tasks','templates'),
       mp.m IN ('collections'),
       mp.m IN ('templates','collections'),
       FALSE
FROM roles r CROSS JOIN ws CROSS JOIN modules_phase2 mp
WHERE r.name = 'CS Officer'
UNION ALL
-- Finance (phase 2)
SELECT r.id, ws.id, mp.m,
       CASE mp.m
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
       mp.m IN ('daily_tasks','approvals','playbooks','activity_log','collections','workspace_audit'),
       mp.m IN ('daily_tasks','approvals'),
       FALSE,
       FALSE,
       mp.m IN ('activity_log','collections','workspace_audit'),
       FALSE
FROM roles r CROSS JOIN ws CROSS JOIN modules_phase2 mp
WHERE r.name = 'Finance'
UNION ALL
-- Viewer (phase 2)
SELECT r.id, ws.id, mp.m,
       CASE mp.m
           WHEN 'daily_tasks' THEN 'self'
           WHEN 'playbooks'   THEN 'true'
           ELSE 'false'
       END,
       mp.m IN ('daily_tasks','playbooks'),
       FALSE, FALSE, FALSE, FALSE, FALSE
FROM roles r CROSS JOIN ws CROSS JOIN modules_phase2 mp
WHERE r.name = 'Viewer'

ON CONFLICT (role_id, workspace_id, module_id) DO NOTHING;
`
