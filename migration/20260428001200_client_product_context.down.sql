DROP TRIGGER IF EXISTS trg_cpc_updated_at ON client_product_context;
DROP FUNCTION IF EXISTS update_cpc_updated_at();
DROP INDEX IF EXISTS idx_cpc_workspace_product;
DROP TABLE IF EXISTS client_product_context;
