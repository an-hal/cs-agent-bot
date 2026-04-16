-- Revert: 20260415000610

DROP INDEX IF EXISTS idx_ili_workspace;
DROP INDEX IF EXISTS idx_ili_invoice;
DROP TABLE IF EXISTS invoice_line_items;
