-- Revert: 20260415000611

GRANT UPDATE, DELETE ON payment_logs TO PUBLIC;

DROP INDEX IF EXISTS idx_pl_timestamp;
DROP INDEX IF EXISTS idx_pl_event_type;
DROP INDEX IF EXISTS idx_pl_invoice;
DROP INDEX IF EXISTS idx_pl_workspace;
DROP TABLE IF EXISTS payment_logs;
