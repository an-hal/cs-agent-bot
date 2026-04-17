-- Migration: Create master_data_mutations (dashboard edit history)
-- Version: 20260414000204
-- Description: Each row = a dashboard write operation. Distinct from
-- `action_log` (bot/AE audit) and `action_logs` (workflow node traces).

CREATE TABLE IF NOT EXISTS master_data_mutations (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    master_data_id  UUID         NOT NULL,
    company_id      VARCHAR(50),
    company_name    VARCHAR(255),
    action          VARCHAR(50)  NOT NULL,
    actor_email     VARCHAR(255) NOT NULL,
    changed_fields  TEXT[]       NOT NULL DEFAULT '{}',
    previous_values JSONB,
    new_values      JSONB,
    note            TEXT,
    timestamp       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_md_mut_workspace_ts
    ON master_data_mutations(workspace_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_md_mut_master_data
    ON master_data_mutations(master_data_id);
