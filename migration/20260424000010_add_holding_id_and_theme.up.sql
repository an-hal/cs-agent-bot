-- Multi-workspace "holding" hierarchy — one parent workspace groups several
-- operating workspaces. When present, dashboard queries can aggregate across
-- all children via the expand helper in internal/usecase/workspace.
ALTER TABLE IF EXISTS workspaces
    ADD COLUMN IF NOT EXISTS holding_id UUID;

CREATE INDEX IF NOT EXISTS idx_workspaces_holding
    ON workspaces (holding_id) WHERE holding_id IS NOT NULL;

-- Theme settings per workspace — compact JSONB to match FE shape
-- ({primary, accent, mode, sidebar_collapsed, logo_url, ...}).
CREATE TABLE IF NOT EXISTS workspace_themes (
    workspace_id UUID         PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
    theme        JSONB        NOT NULL DEFAULT '{}'::jsonb,
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_by   VARCHAR(255) NOT NULL DEFAULT ''
);
