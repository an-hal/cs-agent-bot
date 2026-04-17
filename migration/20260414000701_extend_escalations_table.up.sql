-- Migration: Extend escalations table for activity-log feature
-- Version: 20260414000701
-- Description: Adds severity, assigned_to, resolution_note, notified_via, notified_at,
--              master_data_id, trigger_id, reason columns per spec 04-escalation.

ALTER TABLE escalations ADD COLUMN IF NOT EXISTS severity        VARCHAR(10);
ALTER TABLE escalations ADD COLUMN IF NOT EXISTS assigned_to     VARCHAR(100);
ALTER TABLE escalations ADD COLUMN IF NOT EXISTS resolution_note TEXT;
ALTER TABLE escalations ADD COLUMN IF NOT EXISTS notified_via    VARCHAR(20);
ALTER TABLE escalations ADD COLUMN IF NOT EXISTS notified_at     TIMESTAMPTZ;
ALTER TABLE escalations ADD COLUMN IF NOT EXISTS master_data_id  VARCHAR(100);
ALTER TABLE escalations ADD COLUMN IF NOT EXISTS workspace_id    UUID;

CREATE INDEX IF NOT EXISTS idx_esc_severity      ON escalations(severity);
CREATE INDEX IF NOT EXISTS idx_esc_assigned_to   ON escalations(assigned_to);
CREATE INDEX IF NOT EXISTS idx_esc_master_data   ON escalations(master_data_id);
CREATE INDEX IF NOT EXISTS idx_esc_ws_status     ON escalations(workspace_id, status);
