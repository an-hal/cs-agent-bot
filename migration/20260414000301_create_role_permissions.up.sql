-- Feature 04: role_permissions — permission matrix per role per workspace per module.

CREATE TABLE IF NOT EXISTS role_permissions (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id      UUID        NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    workspace_id UUID        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    module_id    VARCHAR(50) NOT NULL,

    -- view_list has 3 possible values: 'false', 'true', 'all'
    view_list    VARCHAR(5)  NOT NULL DEFAULT 'false',
    view_detail  BOOLEAN     NOT NULL DEFAULT FALSE,
    can_create   BOOLEAN     NOT NULL DEFAULT FALSE,
    can_edit     BOOLEAN     NOT NULL DEFAULT FALSE,
    can_delete   BOOLEAN     NOT NULL DEFAULT FALSE,
    can_export   BOOLEAN     NOT NULL DEFAULT FALSE,
    can_import   BOOLEAN     NOT NULL DEFAULT FALSE,

    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(role_id, workspace_id, module_id)
);

CREATE INDEX IF NOT EXISTS idx_rp_role      ON role_permissions(role_id);
CREATE INDEX IF NOT EXISTS idx_rp_workspace ON role_permissions(workspace_id);
CREATE INDEX IF NOT EXISTS idx_rp_role_ws   ON role_permissions(role_id, workspace_id);
