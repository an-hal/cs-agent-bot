-- Migration: Extend workspaces table with settings, is_active, updated_at
-- Version: 20260414000100
-- Description: Adds JSONB settings, soft-delete flag, and updated_at timestamp

ALTER TABLE workspaces
    ADD COLUMN IF NOT EXISTS settings   JSONB     NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS is_active  BOOLEAN   NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP NOT NULL DEFAULT NOW();

-- Widen slug to 50 chars (was 20)
ALTER TABLE workspaces
    ALTER COLUMN slug TYPE VARCHAR(50);

CREATE INDEX IF NOT EXISTS idx_workspaces_is_active ON workspaces(is_active);
CREATE INDEX IF NOT EXISTS idx_workspaces_settings  ON workspaces USING GIN(settings);

-- Shared trigger function used by all workspace tables
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_ws_updated_at ON workspaces;
CREATE TRIGGER trg_ws_updated_at
    BEFORE UPDATE ON workspaces
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
