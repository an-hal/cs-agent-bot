-- Migration: Minimal approval_requests scaffold for checker-maker.
-- Version: 20260414000206
-- NOTE: This is a minimal local scaffold owned temporarily by feat/03-master-data
-- so the master_data delete + bulk-import flows compile and pass tests. Feature
-- 04 (team & approvals) will own the full table; this migration is additive
-- (IF NOT EXISTS) and only creates a small subset of columns. Feature 04 may
-- ALTER this table to add additional columns.

CREATE TABLE IF NOT EXISTS approval_requests (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id     UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    request_type     VARCHAR(64)  NOT NULL,
    description      TEXT         NOT NULL DEFAULT '',
    payload          JSONB        NOT NULL DEFAULT '{}'::jsonb,
    status           VARCHAR(20)  NOT NULL DEFAULT 'pending',
    maker_email      VARCHAR(255) NOT NULL,
    maker_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    checker_email    VARCHAR(255),
    checker_at       TIMESTAMPTZ,
    rejection_reason TEXT,
    expires_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW() + INTERVAL '72 hours',
    applied_at       TIMESTAMPTZ,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ar_workspace_status
    ON approval_requests(workspace_id, status);
CREATE INDEX IF NOT EXISTS idx_ar_workspace_created
    ON approval_requests(workspace_id, created_at DESC);
