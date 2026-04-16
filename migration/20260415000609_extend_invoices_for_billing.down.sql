-- Revert: 20260415000609 — Remove added billing columns from invoices

DROP INDEX IF EXISTS idx_inv_paper_id_ref;
DROP INDEX IF EXISTS idx_inv_days_overdue;
DROP INDEX IF EXISTS idx_inv_workspace_stage;
DROP INDEX IF EXISTS idx_inv_workspace_due;
DROP INDEX IF EXISTS idx_inv_workspace_status;
DROP INDEX IF EXISTS idx_inv_workspace_company;
DROP INDEX IF EXISTS idx_inv_workspace;

ALTER TABLE invoices ALTER COLUMN payment_status SET DEFAULT 'Pending';

ALTER TABLE invoices
  DROP COLUMN IF EXISTS updated_at,
  DROP COLUMN IF EXISTS created_by,
  DROP COLUMN IF EXISTS paper_id_ref,
  DROP COLUMN IF EXISTS paper_id_url,
  DROP COLUMN IF EXISTS last_reminder_date,
  DROP COLUMN IF EXISTS days_overdue,
  DROP COLUMN IF EXISTS payment_date,
  DROP COLUMN IF EXISTS payment_method,
  DROP COLUMN IF EXISTS payment_terms;
