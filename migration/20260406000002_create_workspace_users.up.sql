-- Migration: Create workspace_users table
-- Version: 20260406000002
-- Description: Creates the workspace_users table for workspace access control

CREATE TABLE workspace_users (
  id            UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
  user_email    VARCHAR(100)  NOT NULL,
  workspace_id  UUID          NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  role          VARCHAR(10)   NOT NULL DEFAULT 'viewer',
  created_at    TIMESTAMP     DEFAULT NOW(),
  UNIQUE(user_email, workspace_id)
);

CREATE INDEX idx_workspace_users_email ON workspace_users(user_email);
CREATE INDEX idx_workspace_users_workspace_id ON workspace_users(workspace_id);
