-- Migration: Create workspaces table
-- Version: 20260406000001
-- Description: Creates the workspaces table for multi-tenancy support

CREATE TABLE workspaces (
  id          UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
  slug        VARCHAR(20)   NOT NULL UNIQUE,
  name        VARCHAR(100)  NOT NULL,
  logo        VARCHAR(10),
  color       VARCHAR(7),
  plan        VARCHAR(30),
  is_holding  BOOLEAN       DEFAULT FALSE,
  member_ids  UUID[],
  created_at  TIMESTAMP     DEFAULT NOW()
);

CREATE INDEX idx_workspaces_slug ON workspaces(slug);
CREATE INDEX idx_workspaces_is_holding ON workspaces(is_holding);
