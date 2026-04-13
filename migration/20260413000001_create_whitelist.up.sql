-- Migration: Create whitelist table
-- Version: 20260413000001
-- Description: Creates the whitelist table for dashboard access gating.
--              Auth itself is delegated to ms-auth-proxy; this table only
--              determines which authenticated emails may access the dashboard.

CREATE TABLE whitelist (
  id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
  email       VARCHAR(255) NOT NULL,
  is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
  added_by    VARCHAR(255),
  notes       TEXT,
  created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

  CONSTRAINT uq_whitelist_email UNIQUE (email)
);

CREATE INDEX idx_whitelist_email ON whitelist(LOWER(email));
CREATE INDEX idx_whitelist_active ON whitelist(is_active);
