-- Migration: Create template_edit_logs (INSERT-only audit trail)
-- Version: 20260414000503

CREATE TABLE IF NOT EXISTS template_edit_logs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    template_id     VARCHAR(50)  NOT NULL,
    template_type   VARCHAR(10)  NOT NULL,
    field           VARCHAR(50)  NOT NULL,
    old_value       TEXT,
    new_value       TEXT,
    edited_by       VARCHAR(255) NOT NULL,
    edited_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT template_edit_logs_type_check CHECK (template_type IN ('message','email'))
);

CREATE INDEX IF NOT EXISTS idx_tel_workspace   ON template_edit_logs(workspace_id);
CREATE INDEX IF NOT EXISTS idx_tel_template    ON template_edit_logs(template_id);
CREATE INDEX IF NOT EXISTS idx_tel_edited_at   ON template_edit_logs(workspace_id, edited_at DESC);
CREATE INDEX IF NOT EXISTS idx_tel_template_ts ON template_edit_logs(template_id, edited_at DESC);
