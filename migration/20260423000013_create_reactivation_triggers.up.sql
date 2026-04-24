-- Feature 03-master-data: reactivation triggers for dormant clients.
-- Configurable rules per workspace that determine when a dormant client should
-- be re-engaged (pricing change, new feature, anniversary, manual).
CREATE TABLE IF NOT EXISTS reactivation_triggers (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,

    code          VARCHAR(64)  NOT NULL,             -- 'price_change' | 'new_feature' | 'anniversary' | 'manual'
    name          VARCHAR(255) NOT NULL,
    description   TEXT         NOT NULL DEFAULT '',
    condition     TEXT         NOT NULL DEFAULT '-',  -- condition DSL (see pkg/conditiondsl)
    template_code VARCHAR(64)  NOT NULL DEFAULT '',   -- message_templates.code to send
    is_active     BOOLEAN      NOT NULL DEFAULT TRUE,

    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_by    VARCHAR(255) NOT NULL DEFAULT '',

    UNIQUE (workspace_id, code)
);

-- Per-client reactivation history — track which triggers have fired for a
-- client so we don't re-send the same thing in a tight window.
CREATE TABLE IF NOT EXISTS reactivation_events (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id   UUID        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    trigger_id     UUID        NOT NULL REFERENCES reactivation_triggers(id) ON DELETE CASCADE,
    master_data_id UUID        NOT NULL REFERENCES clients(master_id) ON DELETE CASCADE,
    fired_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    outcome        VARCHAR(32) NOT NULL DEFAULT 'sent', -- 'sent' | 'skipped' | 'replied' | 'bounced'
    note           TEXT        NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_react_events_master
    ON reactivation_events (master_data_id, fired_at DESC);
CREATE INDEX IF NOT EXISTS idx_react_triggers_active
    ON reactivation_triggers (workspace_id, is_active);
