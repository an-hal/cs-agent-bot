-- Migration: Create action_logs (plural) for workflow node execution traces.
-- Version: 20260414000203
-- Description: Distinct from existing singular `action_log` (bot/AE audit).
-- This table records each workflow node run: read snapshot, write snapshot, status.

CREATE TABLE IF NOT EXISTS action_logs (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    master_data_id  UUID         NOT NULL,
    trigger_id      VARCHAR(100) NOT NULL,
    template_id     VARCHAR(100),
    status          VARCHAR(20)  NOT NULL,
    channel         VARCHAR(20),
    phase           VARCHAR(10),
    fields_read     JSONB,
    fields_written  JSONB,
    replied         BOOLEAN      NOT NULL DEFAULT FALSE,
    conversation_id VARCHAR(100),
    timestamp       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_action_logs_workspace
    ON action_logs(workspace_id);
CREATE INDEX IF NOT EXISTS idx_action_logs_master_data
    ON action_logs(master_data_id);
CREATE INDEX IF NOT EXISTS idx_action_logs_trigger
    ON action_logs(trigger_id);
CREATE INDEX IF NOT EXISTS idx_action_logs_workspace_ts
    ON action_logs(workspace_id, timestamp DESC);
