-- Migration: Create workspace_members table
-- Version: 20260414000101
-- Description: Many-to-many user/workspace relationship with role and permissions.
-- Note: user_email is the canonical identity (no users table exists yet).

CREATE TABLE IF NOT EXISTS workspace_members (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_email    VARCHAR(255) NOT NULL,
    user_name     VARCHAR(255) NOT NULL DEFAULT '',
    role          VARCHAR(20)  NOT NULL DEFAULT 'member',
    permissions   JSONB        NOT NULL DEFAULT '{}'::jsonb,
    is_active     BOOLEAN      NOT NULL DEFAULT TRUE,
    invited_at    TIMESTAMP,
    joined_at     TIMESTAMP    DEFAULT NOW(),
    invited_by    VARCHAR(255),
    created_at    TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP    NOT NULL DEFAULT NOW(),
    UNIQUE(workspace_id, user_email)
);

CREATE INDEX IF NOT EXISTS idx_wm_workspace   ON workspace_members(workspace_id);
CREATE INDEX IF NOT EXISTS idx_wm_user        ON workspace_members(user_email);
CREATE INDEX IF NOT EXISTS idx_wm_user_active ON workspace_members(user_email, is_active);
CREATE INDEX IF NOT EXISTS idx_wm_role        ON workspace_members(workspace_id, role);

DROP TRIGGER IF EXISTS trg_wm_updated_at ON workspace_members;
CREATE TRIGGER trg_wm_updated_at
    BEFORE UPDATE ON workspace_members
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
