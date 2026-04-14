-- Migration: Create message_templates (WA + Telegram, workspace-scoped)
-- Version: 20260414000500

CREATE TABLE IF NOT EXISTS message_templates (
    id              VARCHAR(50) PRIMARY KEY,
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    trigger_id      VARCHAR(100) NOT NULL,
    phase           VARCHAR(10)  NOT NULL,
    phase_label     VARCHAR(50)  NOT NULL,
    channel         VARCHAR(20)  NOT NULL DEFAULT 'whatsapp',
    role            VARCHAR(10)  NOT NULL DEFAULT 'ae',
    category        VARCHAR(30)  NOT NULL,
    action          VARCHAR(255) NOT NULL,
    timing          VARCHAR(100) NOT NULL,
    condition       TEXT         NOT NULL DEFAULT '',
    message         TEXT         NOT NULL,
    variables       TEXT[]       NOT NULL DEFAULT '{}',
    stop_if         TEXT,
    sent_flag       VARCHAR(100) NOT NULL DEFAULT '',
    priority        VARCHAR(10),
    updated_at      TIMESTAMPTZ,
    updated_by      VARCHAR(255),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT message_templates_channel_check CHECK (channel IN ('whatsapp','telegram')),
    CONSTRAINT message_templates_role_check    CHECK (role IN ('sdr','bd','ae')),
    CONSTRAINT message_templates_ws_trigger_channel_uniq UNIQUE (workspace_id, trigger_id, channel)
);

CREATE INDEX IF NOT EXISTS idx_mt_workspace       ON message_templates(workspace_id);
CREATE INDEX IF NOT EXISTS idx_mt_workspace_role  ON message_templates(workspace_id, role);
CREATE INDEX IF NOT EXISTS idx_mt_workspace_phase ON message_templates(workspace_id, phase);
CREATE INDEX IF NOT EXISTS idx_mt_trigger         ON message_templates(workspace_id, trigger_id);
CREATE INDEX IF NOT EXISTS idx_mt_category        ON message_templates(workspace_id, category);
CREATE INDEX IF NOT EXISTS idx_mt_channel         ON message_templates(workspace_id, channel);
