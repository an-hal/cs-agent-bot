-- Feature 00-shared/02: user-preferences
-- Per-user, per-workspace JSONB prefs store (theme, sidebar state, column
-- visibility, feed interval, etc.). Keyed by (workspace_id, user_email, namespace).
CREATE TABLE IF NOT EXISTS user_preferences (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_email    VARCHAR(255) NOT NULL,
    namespace     VARCHAR(128) NOT NULL,
    value         JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, user_email, namespace)
);

CREATE INDEX IF NOT EXISTS idx_user_prefs_user
    ON user_preferences (workspace_id, user_email);
