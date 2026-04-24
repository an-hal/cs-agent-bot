DROP TABLE IF EXISTS workspace_themes;
DROP INDEX IF EXISTS idx_workspaces_holding;
ALTER TABLE IF EXISTS workspaces DROP COLUMN IF EXISTS holding_id;
