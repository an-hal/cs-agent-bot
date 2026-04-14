-- Migration: Create email_templates (HTML body, workspace-scoped)
-- Version: 20260414000501

CREATE TABLE IF NOT EXISTS email_templates (
    id              VARCHAR(50) PRIMARY KEY,
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    role            VARCHAR(10)  NOT NULL DEFAULT 'ae',
    category        VARCHAR(30)  NOT NULL,
    status          VARCHAR(20)  NOT NULL DEFAULT 'draft',
    subject         VARCHAR(500) NOT NULL,
    body_html       TEXT         NOT NULL,
    variables       TEXT[]       NOT NULL DEFAULT '{}',
    updated_at      TIMESTAMPTZ,
    updated_by      VARCHAR(255),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT email_templates_role_check   CHECK (role IN ('sdr','bd','ae')),
    CONSTRAINT email_templates_status_check CHECK (status IN ('active','draft','archived')),
    CONSTRAINT email_templates_ws_id_uniq   UNIQUE (workspace_id, id)
);

CREATE INDEX IF NOT EXISTS idx_et_workspace        ON email_templates(workspace_id);
CREATE INDEX IF NOT EXISTS idx_et_workspace_role   ON email_templates(workspace_id, role);
CREATE INDEX IF NOT EXISTS idx_et_workspace_status ON email_templates(workspace_id, status);
CREATE INDEX IF NOT EXISTS idx_et_category         ON email_templates(workspace_id, category);
