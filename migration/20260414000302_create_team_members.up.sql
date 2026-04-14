-- Feature 04: team_members — members of the dashboard.
-- user_id is nullable (no FK): email is the authoritative identity on this branch.

CREATE TABLE IF NOT EXISTS team_members (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID,
    name            VARCHAR(255) NOT NULL,
    email           VARCHAR(255) NOT NULL UNIQUE,
    initials        VARCHAR(5)   NOT NULL DEFAULT '',
    role_id         UUID         NOT NULL REFERENCES roles(id),
    status          VARCHAR(10)  NOT NULL DEFAULT 'pending',
    department      VARCHAR(100) NOT NULL DEFAULT '',
    avatar_color    VARCHAR(7)   NOT NULL DEFAULT '',
    invite_token    VARCHAR(255),
    invite_expires  TIMESTAMPTZ,
    invited_by      UUID,
    joined_at       TIMESTAMPTZ,
    last_active_at  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tm_email  ON team_members(email);
CREATE INDEX IF NOT EXISTS idx_tm_role   ON team_members(role_id);
CREATE INDEX IF NOT EXISTS idx_tm_status ON team_members(status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tm_invite_token ON team_members(invite_token) WHERE invite_token IS NOT NULL;
