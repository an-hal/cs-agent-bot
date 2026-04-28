-- Phase C of OneSchema-style import: a session row stores a maker's
-- in-progress import (file + mapping + per-cell corrections) so they can fix
-- bad cells in the FE wizard without re-uploading. On Submit the session
-- spawns a normal bulk_import_master_data approval; the file_b64 + final
-- mapping + overrides are baked into the approval payload so the apply
-- step reparses identically.

CREATE TABLE IF NOT EXISTS import_sessions (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id   UUID        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    created_by     VARCHAR(255) NOT NULL,
    status         VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending|submitted|expired

    file_name      VARCHAR(255) NOT NULL,
    file_b64       TEXT         NOT NULL,
    sheet_name     VARCHAR(255),
    mode           VARCHAR(20)  NOT NULL DEFAULT 'add_new',
    mapping        JSONB        NOT NULL DEFAULT '{}'::jsonb,
    -- cell_overrides: {"<row_num>": {"<target_key>": "<corrected_value>"}}
    cell_overrides JSONB        NOT NULL DEFAULT '{}'::jsonb,

    approval_id    UUID,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Sessions auto-expire after 24h to keep the table from holding stale
    -- file_b64 blobs forever; FE should resubmit if the user comes back later.
    expires_at     TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '24 hours')
);

CREATE INDEX IF NOT EXISTS idx_import_sessions_workspace_status
    ON import_sessions (workspace_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_import_sessions_expires
    ON import_sessions (expires_at)
    WHERE status = 'pending';

CREATE OR REPLACE FUNCTION update_import_sessions_updated_at()
RETURNS TRIGGER AS $$
BEGIN NEW.updated_at = NOW(); RETURN NEW; END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS trg_import_sessions_updated_at ON import_sessions;
CREATE TRIGGER trg_import_sessions_updated_at BEFORE UPDATE ON import_sessions
    FOR EACH ROW EXECUTE FUNCTION update_import_sessions_updated_at();
