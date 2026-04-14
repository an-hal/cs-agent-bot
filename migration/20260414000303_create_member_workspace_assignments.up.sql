-- Feature 04: member_workspace_assignments — many-to-many member <-> workspace.

CREATE TABLE IF NOT EXISTS member_workspace_assignments (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    member_id    UUID        NOT NULL REFERENCES team_members(id) ON DELETE CASCADE,
    workspace_id UUID        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    assigned_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    assigned_by  UUID,
    UNIQUE(member_id, workspace_id)
);

CREATE INDEX IF NOT EXISTS idx_mwa_member    ON member_workspace_assignments(member_id);
CREATE INDEX IF NOT EXISTS idx_mwa_workspace ON member_workspace_assignments(workspace_id);
