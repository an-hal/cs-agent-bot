-- Migration: Create custom_field_definitions table
-- Version: 20260414000201

CREATE TABLE IF NOT EXISTS custom_field_definitions (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id     UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    field_key        VARCHAR(50)  NOT NULL,
    field_label      VARCHAR(100) NOT NULL,
    field_type       VARCHAR(20)  NOT NULL,
    is_required      BOOLEAN      NOT NULL DEFAULT FALSE,
    default_value    TEXT,
    placeholder      VARCHAR(200),
    description      TEXT,
    options          JSONB,
    min_value        NUMERIC,
    max_value        NUMERIC,
    regex_pattern    VARCHAR(255),
    sort_order       INT          NOT NULL DEFAULT 0,
    visible_in_table BOOLEAN      NOT NULL DEFAULT TRUE,
    column_width     INT          NOT NULL DEFAULT 120,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, field_key),
    CHECK (field_type IN ('text','number','date','boolean','select','url','email'))
);

CREATE INDEX IF NOT EXISTS idx_cfd_workspace
    ON custom_field_definitions(workspace_id);
CREATE INDEX IF NOT EXISTS idx_cfd_workspace_sort
    ON custom_field_definitions(workspace_id, sort_order);

DROP TRIGGER IF EXISTS trg_cfd_updated_at ON custom_field_definitions;
CREATE TRIGGER trg_cfd_updated_at
    BEFORE UPDATE ON custom_field_definitions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();
