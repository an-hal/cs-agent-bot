-- Feature 00-shared/08: PDP compliance. One row per subject-erasure request
-- (SAR / right-to-be-forgotten). Admin reviews + approves → cron processor
-- scrubs PII in downstream tables. Retention policy is a separate table so
-- admins can configure data-class lifespans per workspace.

-- Erasure requests (subject-access / right-to-delete).
CREATE TABLE IF NOT EXISTS pdp_erasure_requests (
    id             UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id   UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,

    subject_email  VARCHAR(255) NOT NULL,   -- subject of the request (data owner)
    subject_kind   VARCHAR(32)  NOT NULL DEFAULT 'contact',
                                             -- 'contact' | 'employee' | 'lead' | 'user'
    requester      VARCHAR(255) NOT NULL,   -- who submitted (admin email)
    reason         TEXT         NOT NULL DEFAULT '',
    scope          JSONB        NOT NULL DEFAULT '[]'::jsonb,
                                             -- list of targeted tables/resources
    status         VARCHAR(16)  NOT NULL DEFAULT 'pending',
                                             -- pending | approved | executed | rejected | expired
    rejection_reason TEXT       NOT NULL DEFAULT '',

    reviewed_by    VARCHAR(255),
    reviewed_at    TIMESTAMPTZ,
    executed_at    TIMESTAMPTZ,
    execution_summary JSONB     NOT NULL DEFAULT '{}'::jsonb,
                                             -- {tables: {master_data: 3, action_log: 127}, scrubbed_at: ...}
    expires_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW() + INTERVAL '30 days',

    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pdp_erasure_status
    ON pdp_erasure_requests (workspace_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_pdp_erasure_subject
    ON pdp_erasure_requests (workspace_id, subject_email);

-- Retention policies — per-workspace, per-table data lifespan + action.
CREATE TABLE IF NOT EXISTS pdp_retention_policies (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,

    data_class      VARCHAR(64)  NOT NULL,     -- 'action_log' | 'master_data' | 'invoices' | 'fireflies_transcripts'
    retention_days  INT          NOT NULL,     -- <=0 means keep forever
    action          VARCHAR(16)  NOT NULL DEFAULT 'delete',
                                               -- delete | anonymize | archive
    is_active       BOOLEAN      NOT NULL DEFAULT TRUE,

    last_run_at     TIMESTAMPTZ,
    last_run_rows   INT          NOT NULL DEFAULT 0,

    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_by      VARCHAR(255) NOT NULL DEFAULT '',

    UNIQUE (workspace_id, data_class)
);

CREATE INDEX IF NOT EXISTS idx_pdp_retention_active
    ON pdp_retention_policies (workspace_id, is_active);
