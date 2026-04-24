-- Feature 01-auth §2c + 08-activity-log: audit trail for cross-workspace access.
-- Every time a user accesses a workspace they are not a direct member of
-- (holding/admin flow), we record who/what/when for compliance.
CREATE TABLE IF NOT EXISTS audit_logs_workspace_access (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    actor_email  VARCHAR(255) NOT NULL,
    access_kind  VARCHAR(32)  NOT NULL,            -- 'read' | 'write' | 'admin'
    resource     VARCHAR(128) NOT NULL DEFAULT '', -- e.g. 'master_data', 'invoice', 'team_member'
    resource_id  VARCHAR(255) NOT NULL DEFAULT '',
    ip_address   VARCHAR(64)  NOT NULL DEFAULT '',
    user_agent   TEXT         NOT NULL DEFAULT '',
    reason       TEXT         NOT NULL DEFAULT '', -- free-form context
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_wsaccess_workspace_time
    ON audit_logs_workspace_access (workspace_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_wsaccess_actor_time
    ON audit_logs_workspace_access (actor_email, created_at DESC);
