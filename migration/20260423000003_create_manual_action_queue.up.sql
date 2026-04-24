-- Feature 06-workflow-engine/07: manual action queue (GUARD)
-- Twenty flows per role require human composition at relationship-critical
-- moments. The bot inserts a row here with a suggested draft and context;
-- the human composes + sends, then marks the row sent/skipped.
CREATE TABLE IF NOT EXISTS manual_action_queue (
    id                 UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id       UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    master_data_id     UUID         NOT NULL REFERENCES clients(master_id) ON DELETE CASCADE,

    trigger_id         VARCHAR(64)  NOT NULL,
    flow_category      VARCHAR(32)  NOT NULL,
    role               VARCHAR(8)   NOT NULL,
    assigned_to_user   VARCHAR(255) NOT NULL,

    suggested_draft    TEXT         NOT NULL DEFAULT '',
    context_summary    JSONB        NOT NULL DEFAULT '{}'::jsonb,

    status             VARCHAR(16)  NOT NULL DEFAULT 'pending',
    priority           VARCHAR(4)   NOT NULL DEFAULT 'P2',

    due_at             TIMESTAMPTZ  NOT NULL,
    sent_at            TIMESTAMPTZ,
    sent_channel       VARCHAR(16),
    actual_message     TEXT,
    skipped_reason     TEXT,

    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_maq_workspace_status
    ON manual_action_queue (workspace_id, status);
CREATE INDEX IF NOT EXISTS idx_maq_assigned_pending
    ON manual_action_queue (assigned_to_user, status) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_maq_due_at
    ON manual_action_queue (due_at) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_maq_master_data
    ON manual_action_queue (master_data_id);
