-- Down migration: revert workspace extensions

DROP TRIGGER IF EXISTS trg_ws_updated_at ON workspaces;
DROP INDEX IF EXISTS idx_workspaces_settings;
DROP INDEX IF EXISTS idx_workspaces_is_active;

ALTER TABLE workspaces
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS is_active,
    DROP COLUMN IF EXISTS settings;

ALTER TABLE workspaces
    ALTER COLUMN slug TYPE VARCHAR(20);
