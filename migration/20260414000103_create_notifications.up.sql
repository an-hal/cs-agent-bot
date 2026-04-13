-- Migration: Create notifications table
-- Version: 20260414000103
-- Description: Cross-cutting notification hub for in-app, telegram, email channels.

CREATE TABLE IF NOT EXISTS notifications (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    recipient_email VARCHAR(255) NOT NULL,
    type            VARCHAR(50)  NOT NULL,
    icon            VARCHAR(50)  NOT NULL DEFAULT '',
    message         TEXT         NOT NULL,
    href            VARCHAR(500) NOT NULL DEFAULT '',
    source_feature  VARCHAR(50)  NOT NULL DEFAULT '',
    source_id       VARCHAR(100) NOT NULL DEFAULT '',
    read            BOOLEAN      NOT NULL DEFAULT FALSE,
    read_at         TIMESTAMP,
    telegram_sent   BOOLEAN      NOT NULL DEFAULT FALSE,
    email_sent      BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMP    NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notif_recipient
    ON notifications(workspace_id, recipient_email, read, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notif_unread
    ON notifications(workspace_id, recipient_email)
    WHERE read = FALSE;
CREATE INDEX IF NOT EXISTS idx_notif_created ON notifications(created_at DESC);
