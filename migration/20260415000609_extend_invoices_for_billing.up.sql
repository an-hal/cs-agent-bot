-- Migration: 20260415000609 — Extend invoices table for full billing feature
-- Adds columns required by feat/07-invoices spec that are missing from the original schema.
-- Uses ADD COLUMN IF NOT EXISTS throughout for idempotency.

-- ══ Add missing billing columns ══
ALTER TABLE invoices
  ADD COLUMN IF NOT EXISTS payment_terms     INT            DEFAULT 30,
  ADD COLUMN IF NOT EXISTS payment_method    VARCHAR(100),
  ADD COLUMN IF NOT EXISTS payment_date      DATE,
  ADD COLUMN IF NOT EXISTS days_overdue      INT            NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS last_reminder_date DATE,
  ADD COLUMN IF NOT EXISTS paper_id_url      TEXT,
  ADD COLUMN IF NOT EXISTS paper_id_ref      VARCHAR(100),
  ADD COLUMN IF NOT EXISTS created_by        VARCHAR(255),
  ADD COLUMN IF NOT EXISTS updated_at        TIMESTAMPTZ    DEFAULT NOW();

-- ══ Migrate existing payment_status values to Indonesian names ══
-- The original table defaulted to English status names ('Pending', 'Paid', 'Overdue').
-- Spec requires Indonesian: Belum bayar, Menunggu, Lunas, Terlambat.
-- This migration normalises existing rows before the CHECK constraint (migration 613) is added.
UPDATE invoices SET payment_status = 'Lunas'       WHERE payment_status IN ('Paid', 'paid');
UPDATE invoices SET payment_status = 'Menunggu'    WHERE payment_status IN ('Pending', 'pending');
UPDATE invoices SET payment_status = 'Terlambat'   WHERE payment_status IN ('Overdue', 'overdue');
UPDATE invoices SET payment_status = 'Belum bayar' WHERE payment_status IN ('Unpaid', 'unpaid');

-- Change default for new rows
ALTER TABLE invoices ALTER COLUMN payment_status SET DEFAULT 'Belum bayar';

-- ══ Add missing indexes (spec §2 indexes) ══
CREATE INDEX IF NOT EXISTS idx_inv_workspace         ON invoices(workspace_id);
CREATE INDEX IF NOT EXISTS idx_inv_workspace_company ON invoices(workspace_id, company_id);
CREATE INDEX IF NOT EXISTS idx_inv_workspace_status  ON invoices(workspace_id, payment_status);
CREATE INDEX IF NOT EXISTS idx_inv_workspace_due     ON invoices(workspace_id, due_date);
CREATE INDEX IF NOT EXISTS idx_inv_workspace_stage   ON invoices(workspace_id, collection_stage);
CREATE INDEX IF NOT EXISTS idx_inv_days_overdue      ON invoices(workspace_id, days_overdue) WHERE days_overdue > 0;
CREATE INDEX IF NOT EXISTS idx_inv_paper_id_ref      ON invoices(paper_id_ref) WHERE paper_id_ref IS NOT NULL;
