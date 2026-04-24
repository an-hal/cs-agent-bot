-- Feature 00-shared/11: BD coaching pipeline. Peer-review coaching sessions
-- between a lead and a BD, tied to a claude_extractions row or an
-- activity/transcript. Supports coaching score + feedback notes.
CREATE TABLE IF NOT EXISTS coaching_sessions (
    id                  UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id        UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,

    bd_email            VARCHAR(255) NOT NULL,
    coach_email         VARCHAR(255) NOT NULL,
    master_data_id      UUID,                         -- prospect context (optional)
    claude_extraction_id UUID,                        -- extraction this coaching critiques (optional)

    session_type        VARCHAR(32)  NOT NULL DEFAULT 'peer_review',
                                                     -- peer_review | self_review | manager_review
    session_date        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    -- Scoring (1–5)
    bants_clarity_score      INT,
    discovery_depth_score    INT,
    tone_fit_score           INT,
    next_step_clarity_score  INT,
    overall_score            NUMERIC(5,2),

    strengths           TEXT         NOT NULL DEFAULT '',
    improvements        TEXT         NOT NULL DEFAULT '',
    action_items        TEXT         NOT NULL DEFAULT '',

    status              VARCHAR(16)  NOT NULL DEFAULT 'draft',
                                                     -- draft | submitted | reviewed
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_coaching_bd_time
    ON coaching_sessions (workspace_id, bd_email, session_date DESC);
CREATE INDEX IF NOT EXISTS idx_coaching_coach
    ON coaching_sessions (workspace_id, coach_email, session_date DESC);
CREATE INDEX IF NOT EXISTS idx_coaching_status
    ON coaching_sessions (workspace_id, status);
