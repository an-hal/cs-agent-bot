-- Restore bot fields to clients and back-fill from cms before dropping the
-- relocated cms columns. Rebuilds the view to its pre-1100 form.

ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS bot_active               BOOLEAN     DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS sequence_cs              TEXT,
    ADD COLUMN IF NOT EXISTS blacklisted              BOOLEAN     DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS risk_flag_text           VARCHAR(20) NOT NULL DEFAULT 'None',
    ADD COLUMN IF NOT EXISTS owner_telegram_id        VARCHAR(100),
    ADD COLUMN IF NOT EXISTS ae_assigned              BOOLEAN     DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS ae_telegram_id           VARCHAR(100),
    ADD COLUMN IF NOT EXISTS backup_owner_telegram_id VARCHAR(100);

UPDATE clients c SET
    bot_active               = cms.bot_active,
    sequence_cs              = cms.sequence_cs,
    blacklisted              = cms.blacklisted,
    risk_flag_text           = cms.risk_flag,
    owner_telegram_id        = cms.owner_telegram_id,
    ae_assigned              = cms.ae_assigned,
    ae_telegram_id           = cms.ae_telegram_id,
    backup_owner_telegram_id = cms.backup_owner_telegram_id
FROM client_message_state cms WHERE cms.master_id = c.master_id;

ALTER TABLE client_message_state
    DROP COLUMN IF EXISTS blacklisted,
    DROP COLUMN IF EXISTS risk_flag,
    DROP COLUMN IF EXISTS owner_telegram_id,
    DROP COLUMN IF EXISTS ae_assigned,
    DROP COLUMN IF EXISTS ae_telegram_id,
    DROP COLUMN IF EXISTS backup_owner_telegram_id;

-- Rebuild view to the post-800 form (clients-source for blacklisted/risk).
DROP VIEW IF EXISTS master_data;
CREATE OR REPLACE VIEW master_data AS
SELECT
    c.master_id AS id, c.workspace_id, c.company_id, c.company_name, c.stage,
    c.pic_name, c.pic_nickname, c.pic_role, c.pic_wa, c.pic_email,
    c.owner_name, c.owner_wa, c.owner_telegram_id,
    COALESCE(cms.bot_active, TRUE) AS bot_active,
    c.blacklisted,
    COALESCE(cms.sequence_status, 'ACTIVE') AS sequence_status,
    cms.snooze_until, cms.snooze_reason,
    c.risk_flag_text AS risk_flag,
    c.contract_start, c.contract_end, c.contract_months, c.days_to_expiry,
    c.payment_status, c.payment_terms, c.final_price, c.last_payment_date,
    c.billing_period, c.quantity, c.unit_price, c.currency,
    cms.last_interaction_date,
    c.notes, c.custom_fields, c.created_at, c.updated_at
FROM clients c LEFT JOIN client_message_state cms ON cms.master_id = c.master_id;
