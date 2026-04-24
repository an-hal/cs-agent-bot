-- Feature 05-messaging: per-reply rejection analysis (Claude-driven).
-- When a client replies with rejection/objection, we log the classifier + analysis
-- so coaching + pattern dashboards can surface common objections.
CREATE TABLE IF NOT EXISTS rejection_analysis_log (
    id                 UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id       UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    master_data_id     UUID         NOT NULL REFERENCES clients(master_id) ON DELETE CASCADE,

    source_channel     VARCHAR(16)  NOT NULL DEFAULT 'wa',  -- wa | email | call_note | meeting
    source_message     TEXT         NOT NULL DEFAULT '',    -- original reply text

    rejection_category VARCHAR(64)  NOT NULL DEFAULT '',    -- 'price' | 'authority' | 'timing' | 'feature' | 'tone' | 'other'
    severity           VARCHAR(8)   NOT NULL DEFAULT 'mid', -- low | mid | high
    analysis_summary   TEXT         NOT NULL DEFAULT '',
    suggested_response TEXT         NOT NULL DEFAULT '',

    analyst            VARCHAR(32)  NOT NULL DEFAULT 'rule', -- 'rule' | 'claude' | 'human'
    analyst_version    VARCHAR(32)  NOT NULL DEFAULT '',

    detected_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rej_analysis_workspace_time
    ON rejection_analysis_log (workspace_id, detected_at DESC);
CREATE INDEX IF NOT EXISTS idx_rej_analysis_master_data
    ON rejection_analysis_log (master_data_id, detected_at DESC);
CREATE INDEX IF NOT EXISTS idx_rej_analysis_category
    ON rejection_analysis_log (workspace_id, rejection_category, detected_at DESC);
