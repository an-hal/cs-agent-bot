-- Migration: 20260415000612 — Create invoice_sequences table
-- Provides atomic per-workspace per-year sequential ID generation.
-- Usage: INSERT ... ON CONFLICT DO UPDATE SET last_seq = last_seq + 1 RETURNING last_seq
-- Result: 42 → format as 'INV-DE-2026-042'
-- No FK to workspaces: avoids cascade issues if workspace is soft-deleted.

CREATE TABLE IF NOT EXISTS invoice_sequences (
  workspace_id UUID  NOT NULL,
  year         INT   NOT NULL,
  last_seq     INT   NOT NULL DEFAULT 0,

  PRIMARY KEY (workspace_id, year)
);
