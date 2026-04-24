-- Partial payment support: termin_breakdown is a JSONB array of Termin rows
-- (termin_number, amount, due_date, status, paid_at, payment_method, ...).
-- Empty default so existing single-payment invoices work unchanged.
ALTER TABLE IF EXISTS invoices
    ADD COLUMN IF NOT EXISTS termin_breakdown JSONB NOT NULL DEFAULT '[]'::jsonb;

-- Index only when non-empty so the majority of single-payment invoices don't
-- pay for the index maintenance.
CREATE INDEX IF NOT EXISTS idx_invoices_has_termin
    ON invoices ((jsonb_array_length(termin_breakdown) > 0))
    WHERE jsonb_array_length(termin_breakdown) > 0;
