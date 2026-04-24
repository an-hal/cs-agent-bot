-- Feature 00-shared/06: Claude extraction pipeline. Stores the result of
-- 2-stage Claude calls (extraction + BANTS scoring) against a source document
-- (usually a Fireflies transcript). One row per extraction attempt so we can
-- retry + compare versions.
CREATE TABLE IF NOT EXISTS claude_extractions (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,

    source_type     VARCHAR(32)  NOT NULL,   -- 'fireflies' | 'manual_note' | 'email'
    source_id       VARCHAR(128) NOT NULL,   -- FK-ish ref (e.g. fireflies_transcripts.id)
    master_data_id  UUID,                    -- target master_data row (nullable until matched)

    -- Stage 1: field extraction
    extracted_fields   JSONB    NOT NULL DEFAULT '{}'::jsonb,
    extraction_prompt  VARCHAR(64) NOT NULL DEFAULT '',
    extraction_model   VARCHAR(64) NOT NULL DEFAULT '',

    -- Stage 2: BANTS scoring
    bants_budget       INT,
    bants_authority    INT,
    bants_need         INT,
    bants_timing       INT,
    bants_sentiment    INT,
    bants_score        NUMERIC(5,2),
    bants_classification VARCHAR(16),        -- 'hot' | 'warm' | 'cold'
    buying_intent      VARCHAR(16),          -- 'high' | 'medium' | 'low'
    coaching_notes     TEXT         NOT NULL DEFAULT '',

    -- Lifecycle
    status             VARCHAR(16)  NOT NULL DEFAULT 'pending',
                                              -- pending | running | succeeded | failed | superseded
    error_message      TEXT         NOT NULL DEFAULT '',
    prompt_tokens      INT          NOT NULL DEFAULT 0,
    completion_tokens  INT          NOT NULL DEFAULT 0,
    latency_ms         INT          NOT NULL DEFAULT 0,

    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_claude_extractions_source
    ON claude_extractions (workspace_id, source_type, source_id);
CREATE INDEX IF NOT EXISTS idx_claude_extractions_status
    ON claude_extractions (status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_claude_extractions_master_data
    ON claude_extractions (master_data_id) WHERE master_data_id IS NOT NULL;
