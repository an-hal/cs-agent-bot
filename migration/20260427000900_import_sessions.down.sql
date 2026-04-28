DROP TRIGGER IF EXISTS trg_import_sessions_updated_at ON import_sessions;
DROP FUNCTION IF EXISTS update_import_sessions_updated_at();
DROP INDEX IF EXISTS idx_import_sessions_expires;
DROP INDEX IF EXISTS idx_import_sessions_workspace_status;
DROP TABLE IF EXISTS import_sessions;
