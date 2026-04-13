-- Migration: Create workspace_invitations table
-- Version: 20260414000102
-- Description: Pending invitations with URL-safe token and 7-day default expiry.

CREATE TABLE IF NOT EXISTS workspace_invitations (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    email         VARCHAR(255) NOT NULL,
    role          VARCHAR(20)  NOT NULL DEFAULT 'member',
    invite_token  VARCHAR(100) NOT NULL UNIQUE,
    status        VARCHAR(20)  NOT NULL DEFAULT 'pending',
    invited_by    VARCHAR(255) NOT NULL,
    accepted_at   TIMESTAMP,
    expires_at    TIMESTAMP    NOT NULL,
    created_at    TIMESTAMP    NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wi_token     ON workspace_invitations(invite_token);
CREATE INDEX IF NOT EXISTS idx_wi_workspace ON workspace_invitations(workspace_id);
CREATE INDEX IF NOT EXISTS idx_wi_email     ON workspace_invitations(email);
CREATE INDEX IF NOT EXISTS idx_wi_status    ON workspace_invitations(status, expires_at);

CREATE UNIQUE INDEX IF NOT EXISTS uq_wi_pending
    ON workspace_invitations(workspace_id, email)
    WHERE status = 'pending';
