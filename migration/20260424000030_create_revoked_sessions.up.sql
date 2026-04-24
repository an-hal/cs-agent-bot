-- Session/token revocation list — admin revokes a session (JWT jti) and
-- the auth middleware rejects any future request presenting that jti until
-- it naturally expires. Rows auto-expire via `expires_at` so the table stays
-- bounded.
CREATE TABLE IF NOT EXISTS revoked_sessions (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID         REFERENCES workspaces(id) ON DELETE CASCADE,
    jti          VARCHAR(128) NOT NULL,
    user_email   VARCHAR(255) NOT NULL DEFAULT '',
    reason       TEXT         NOT NULL DEFAULT '',
    revoked_by   VARCHAR(255) NOT NULL DEFAULT '',
    revoked_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    expires_at   TIMESTAMPTZ  NOT NULL,
    UNIQUE (jti)
);

CREATE INDEX IF NOT EXISTS idx_revoked_sessions_jti
    ON revoked_sessions (jti);
CREATE INDEX IF NOT EXISTS idx_revoked_sessions_user
    ON revoked_sessions (user_email, revoked_at DESC);
-- Index to support cleanup of expired rows.
CREATE INDEX IF NOT EXISTS idx_revoked_sessions_expires
    ON revoked_sessions (expires_at);
