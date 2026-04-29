-- Restore the pre-multi-stage view (sourcing pic_*/owner_* from clients
-- directly), then drop the contacts table.

DROP VIEW IF EXISTS master_data;
CREATE OR REPLACE VIEW master_data AS
SELECT
    c.master_id AS id, c.workspace_id, c.company_id, c.company_name, c.stage,
    COALESCE(c.industry, '')   AS industry,
    COALESCE(c.value_tier, '') AS value_tier,
    c.pic_name, c.pic_nickname, c.pic_role, c.pic_wa, c.pic_email,
    c.owner_name, c.owner_wa,
    COALESCE(cms.owner_telegram_id, '')           AS owner_telegram_id,
    COALESCE(cms.bot_active, TRUE)                AS bot_active,
    COALESCE(cms.blacklisted, FALSE)              AS blacklisted,
    COALESCE(cms.sequence_status, 'ACTIVE')       AS sequence_status,
    cms.snooze_until, cms.snooze_reason,
    COALESCE(cms.risk_flag, 'None')               AS risk_flag,
    c.contract_start, c.contract_end, c.contract_months, c.days_to_expiry,
    c.payment_status, c.payment_terms, c.final_price, c.last_payment_date,
    c.billing_period, c.quantity, c.unit_price, c.currency,
    cms.last_interaction_date,
    c.notes, c.custom_fields, c.created_at, c.updated_at
FROM clients c LEFT JOIN client_message_state cms ON cms.master_id = c.master_id;

DROP TRIGGER IF EXISTS trg_client_contacts_updated_at ON client_contacts;
DROP FUNCTION IF EXISTS update_client_contacts_updated_at();
DROP INDEX IF EXISTS idx_client_contacts_workspace;
DROP INDEX IF EXISTS idx_client_contacts_lookup;
DROP INDEX IF EXISTS uq_client_contacts_primary;
DROP TABLE IF EXISTS client_contacts;
