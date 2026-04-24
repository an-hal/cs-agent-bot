DROP INDEX IF EXISTS idx_invoices_has_termin;
ALTER TABLE IF EXISTS invoices DROP COLUMN IF EXISTS termin_breakdown;
