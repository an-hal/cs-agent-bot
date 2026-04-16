-- Migration: Extend escalations table — rollback
-- Version: 20260414000701

DROP INDEX IF EXISTS idx_esc_ws_status;
DROP INDEX IF EXISTS idx_esc_master_data;
DROP INDEX IF EXISTS idx_esc_assigned_to;
DROP INDEX IF EXISTS idx_esc_severity;

ALTER TABLE escalations DROP COLUMN IF EXISTS workspace_id;
ALTER TABLE escalations DROP COLUMN IF EXISTS master_data_id;
ALTER TABLE escalations DROP COLUMN IF EXISTS notified_at;
ALTER TABLE escalations DROP COLUMN IF EXISTS notified_via;
ALTER TABLE escalations DROP COLUMN IF EXISTS resolution_note;
ALTER TABLE escalations DROP COLUMN IF EXISTS assigned_to;
ALTER TABLE escalations DROP COLUMN IF EXISTS severity;
