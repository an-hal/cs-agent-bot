-- Roll back Phase 2 module seed by removing the 12 new module IDs from
-- role_permissions. Existing 9 modules are untouched.
DELETE FROM role_permissions
WHERE module_id IN (
    'daily_tasks',
    'approvals',
    'workflows',
    'templates',
    'playbooks',
    'activity_log',
    'automation_rules',
    'handoff_sla',
    'haloai_conversations',
    'collections',
    'workspace_audit',
    'workspace_settings'
);
