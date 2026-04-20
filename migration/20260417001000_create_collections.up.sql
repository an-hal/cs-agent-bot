-- collections: meta table for user-defined generic tables (feat/10).
-- Per spec ~/dealls/project-bumi-dashboard/context/for-backend/features/10-collections/02-database-schema.md.
-- Admins may create collections at runtime (with approval). Scoping is per-workspace.
CREATE TABLE IF NOT EXISTS collections (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    slug            VARCHAR(64) NOT NULL,
    name            VARCHAR(128) NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    icon            VARCHAR(16) NOT NULL DEFAULT '',
    permissions     JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by      TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ
);

-- Active collections have unique slug per workspace (soft-deleted rows freed).
CREATE UNIQUE INDEX IF NOT EXISTS idx_collections_ws_slug_active
    ON collections (workspace_id, slug)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_collections_workspace
    ON collections (workspace_id)
    WHERE deleted_at IS NULL;
