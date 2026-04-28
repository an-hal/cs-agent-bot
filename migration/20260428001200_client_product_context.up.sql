-- Per spec Option B (context/new/crm_database_spec.md, "Product Custom Fields"):
-- a key-value sidecar for product-specific fields that the spec wants OUT of
-- the portable clients table.
--
-- This migration only creates the EMPTY table. Actual data migration of
-- existing HRIS-style fields (contract_*, payment_*, final_price,
-- last_payment_date, activation_date) out of clients into this table is
-- intentionally deferred — they are heavily referenced by dashboard queries,
-- the renewal pipeline, and FE filters. Moving them is a separate, larger
-- refactor (likely 30+ file changes).
--
-- For now, treat this table as the future home of:
--   - non-HRIS tenants' product context (e.g., Job Portal active_job_slots,
--     Retail monthly_transaction_vol)
--   - HRIS-but-extra fields not in clients (e.g., current_system,
--     first_time_discount_pct, next_discount_pct)
--
-- Tenants writing to this table set product_type so the same
-- (workspace_id, master_id) row can hold different fields for different
-- product flavors. field_value is TEXT to keep typing flexible — callers
-- coerce via the same Transform pipeline used by custom_fields.

CREATE TABLE IF NOT EXISTS client_product_context (
    workspace_id UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    master_id    UUID         NOT NULL REFERENCES clients(master_id) ON DELETE CASCADE,
    product_type VARCHAR(20)  NOT NULL,             -- HRIS | JOB_PORTAL | RETAIL | ...
    field_key    VARCHAR(100) NOT NULL,             -- e.g. hc_size, active_job_slots
    field_value  TEXT,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (master_id, product_type, field_key)
);

CREATE INDEX IF NOT EXISTS idx_cpc_workspace_product
    ON client_product_context (workspace_id, product_type);

CREATE OR REPLACE FUNCTION update_cpc_updated_at()
RETURNS TRIGGER AS $$
BEGIN NEW.updated_at = NOW(); RETURN NEW; END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS trg_cpc_updated_at ON client_product_context;
CREATE TRIGGER trg_cpc_updated_at BEFORE UPDATE ON client_product_context
    FOR EACH ROW EXECUTE FUNCTION update_cpc_updated_at();
