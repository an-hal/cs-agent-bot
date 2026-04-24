-- Feature 00-shared/04: per-workspace integration credentials
-- Stores API keys / tokens for HaloAI (WA), Telegram, Paper.id, SMTP per workspace.
-- NOTE: Values in `config` are stored as-is in this iteration; encryption-at-rest
-- is deferred to a follow-up. FE MUST treat secret fields as write-only — read
-- returns a redacted view (see repository/handler).
CREATE TABLE IF NOT EXISTS workspace_integrations (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    provider     VARCHAR(64)  NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    config       JSONB        NOT NULL DEFAULT '{}'::jsonb,
    is_active    BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_by   VARCHAR(255) NOT NULL DEFAULT '',
    updated_by   VARCHAR(255) NOT NULL DEFAULT '',
    UNIQUE (workspace_id, provider)
);

CREATE INDEX IF NOT EXISTS idx_ws_integrations_active
    ON workspace_integrations (workspace_id, is_active);
