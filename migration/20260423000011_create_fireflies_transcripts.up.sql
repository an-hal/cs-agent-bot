-- Feature 00-shared/07: Fireflies integration. Stores incoming transcript
-- webhook metadata + extraction lifecycle state. Full transcript body is kept
-- in `transcript_text` (can be large; PG TOAST handles it).
CREATE TABLE IF NOT EXISTS fireflies_transcripts (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id      UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,

    fireflies_id      VARCHAR(128) NOT NULL,            -- upstream transcript id
    meeting_title     TEXT         NOT NULL DEFAULT '',
    meeting_date      TIMESTAMPTZ,
    duration_seconds  INT          NOT NULL DEFAULT 0,
    host_email        VARCHAR(255) NOT NULL DEFAULT '',
    participants      JSONB        NOT NULL DEFAULT '[]'::jsonb,

    transcript_text   TEXT         NOT NULL DEFAULT '',
    raw_payload       JSONB        NOT NULL DEFAULT '{}'::jsonb,

    extraction_status VARCHAR(16)  NOT NULL DEFAULT 'pending',
                                                       -- pending | running | succeeded | failed
    extraction_error  TEXT         NOT NULL DEFAULT '',
    extracted_at      TIMESTAMPTZ,
    master_data_id    UUID,                             -- set when extraction is wired to a master_data row

    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    UNIQUE (workspace_id, fireflies_id)
);

CREATE INDEX IF NOT EXISTS idx_ff_transcripts_status
    ON fireflies_transcripts (extraction_status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ff_transcripts_workspace_time
    ON fireflies_transcripts (workspace_id, created_at DESC);
