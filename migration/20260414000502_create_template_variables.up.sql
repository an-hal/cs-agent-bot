-- Migration: Create template_variables (variable catalog per workspace)
-- Version: 20260414000502

CREATE TABLE IF NOT EXISTS template_variables (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    variable_key    VARCHAR(100) NOT NULL,
    display_label   VARCHAR(200) NOT NULL,
    source_type     VARCHAR(30)  NOT NULL,
    source_field    VARCHAR(200),
    description     TEXT,
    example_value   VARCHAR(500),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT template_variables_source_check CHECK (
        source_type IN ('master_data_core','master_data_custom','invoice','computed','workspace_config','generated')
    ),
    CONSTRAINT template_variables_ws_key_uniq UNIQUE (workspace_id, variable_key)
);

CREATE INDEX IF NOT EXISTS idx_tv_workspace ON template_variables(workspace_id);
