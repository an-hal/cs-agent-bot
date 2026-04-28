-- Reverse Phase 6: re-add 17 columns to clients, restore data from
-- client_message_state, drop the new table.

ALTER TABLE clients ADD COLUMN IF NOT EXISTS bot_active             BOOLEAN     DEFAULT TRUE;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS sequence_cs            VARCHAR(20) DEFAULT 'ACTIVE';
ALTER TABLE clients ADD COLUMN IF NOT EXISTS sequence_status        VARCHAR(20) DEFAULT 'ACTIVE';
ALTER TABLE clients ADD COLUMN IF NOT EXISTS response_status        VARCHAR(20) DEFAULT 'Pending';
ALTER TABLE clients ADD COLUMN IF NOT EXISTS snooze_until           DATE;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS snooze_reason          TEXT;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS pre14_sent             BOOLEAN     DEFAULT FALSE;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS pre7_sent              BOOLEAN     DEFAULT FALSE;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS pre3_sent              BOOLEAN     DEFAULT FALSE;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS post1_sent             BOOLEAN     DEFAULT FALSE;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS post4_sent             BOOLEAN     DEFAULT FALSE;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS post8_sent             BOOLEAN     DEFAULT FALSE;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS post15_sent            BOOLEAN     DEFAULT FALSE;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS checkin_replied        BOOLEAN     DEFAULT FALSE;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS feature_update_sent    BOOLEAN     DEFAULT FALSE;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS last_interaction_date  DATE;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS days_since_cs_last_sent SMALLINT  DEFAULT 0;

UPDATE clients c SET
    bot_active              = cms.bot_active,
    sequence_cs             = cms.sequence_cs,
    sequence_status         = cms.sequence_status,
    response_status         = cms.response_status,
    snooze_until            = cms.snooze_until,
    snooze_reason           = cms.snooze_reason,
    pre14_sent              = cms.pre14_sent,
    pre7_sent               = cms.pre7_sent,
    pre3_sent               = cms.pre3_sent,
    post1_sent              = cms.post1_sent,
    post4_sent              = cms.post4_sent,
    post8_sent              = cms.post8_sent,
    post15_sent             = cms.post15_sent,
    checkin_replied         = cms.checkin_replied,
    feature_update_sent     = cms.feature_update_sent,
    last_interaction_date   = cms.last_interaction_date,
    days_since_cs_last_sent = cms.days_since_cs_last_sent
FROM client_message_state cms
WHERE cms.master_id = c.master_id;

DROP VIEW IF EXISTS master_data;
DROP TABLE IF EXISTS client_message_state;

CREATE OR REPLACE VIEW master_data AS
SELECT
    master_id AS id, workspace_id, company_id, company_name, stage,
    pic_name, pic_nickname, pic_role, pic_wa, pic_email,
    owner_name, owner_wa, owner_telegram_id,
    bot_active, blacklisted,
    sequence_status, snooze_until, snooze_reason,
    risk_flag_text AS risk_flag,
    contract_start, contract_end, contract_months, days_to_expiry,
    payment_status, payment_terms, final_price, last_payment_date,
    billing_period, quantity, unit_price, currency,
    last_interaction_date, notes, custom_fields, created_at, updated_at
FROM clients;
