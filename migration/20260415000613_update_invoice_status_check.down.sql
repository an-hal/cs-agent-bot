-- Revert: 20260415000613
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS chk_invoice_collection_stage;
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS chk_invoice_payment_status;
