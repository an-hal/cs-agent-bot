-- Migration: 20260415000613 — Add CHECK constraints for payment_status and collection_stage
-- Run AFTER migration 609 which normalises existing rows to Indonesian status names.
-- Uses NOT VALID so the constraint is enforced on future rows immediately but does not
-- scan existing rows (avoids table lock on large tables). Run VALIDATE CONSTRAINT separately
-- during a low-traffic window if full validation is needed.

ALTER TABLE invoices
  ADD CONSTRAINT chk_invoice_payment_status
    CHECK (payment_status IN ('Lunas', 'Menunggu', 'Belum bayar', 'Terlambat')) NOT VALID;

ALTER TABLE invoices
  ADD CONSTRAINT chk_invoice_collection_stage
    CHECK (collection_stage IN (
      'Stage 0 — Pre-due',
      'Stage 1 — Soft',
      'Stage 2 — Firm',
      'Stage 3 — Urgency',
      'Stage 4 — Escalate',
      'Closed'
    )) NOT VALID;
