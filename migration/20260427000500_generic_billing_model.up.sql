-- Phase 5: make `clients` truly generic across SaaS billing models.
-- Current schema assumes monthly subscription with WhatsApp + Telegram
-- as required channels. This migration:
--   1. Relaxes WA / Telegram NOT NULL (some workspaces use email-only / Slack)
--   2. Allows perpetual contracts (contract_end nullable)
--   3. Adds billing_period, quantity, unit_price, currency for cross-business use
-- The master_data view is rebuilt to expose the new columns.

-- ─── Channels: drop NOT NULL + UNIQUE, keep partial UNIQUE for backward compat ──
ALTER TABLE clients ALTER COLUMN pic_wa DROP NOT NULL;
ALTER TABLE clients DROP CONSTRAINT IF EXISTS clients_pic_wa_key;
CREATE UNIQUE INDEX IF NOT EXISTS idx_clients_pic_wa_uniq
    ON clients(pic_wa) WHERE pic_wa IS NOT NULL;

ALTER TABLE clients ALTER COLUMN owner_telegram_id DROP NOT NULL;

-- ─── Perpetual contract support ──
ALTER TABLE clients ALTER COLUMN contract_end DROP NOT NULL;

-- ─── Generic billing fields ──
ALTER TABLE clients ADD COLUMN IF NOT EXISTS billing_period VARCHAR(20)
    NOT NULL DEFAULT 'monthly';
ALTER TABLE clients DROP CONSTRAINT IF EXISTS chk_clients_billing_period;
ALTER TABLE clients ADD CONSTRAINT chk_clients_billing_period
    CHECK (billing_period IN ('monthly', 'quarterly', 'annual', 'one_time', 'perpetual'));

ALTER TABLE clients ADD COLUMN IF NOT EXISTS quantity INT;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS unit_price NUMERIC(12,2);
ALTER TABLE clients ADD COLUMN IF NOT EXISTS currency CHAR(3)
    NOT NULL DEFAULT 'IDR';

-- ─── master_data view: drop + recreate with new columns ──
DROP VIEW IF EXISTS master_data;
CREATE OR REPLACE VIEW master_data AS
SELECT
    master_id                        AS id,
    workspace_id                     AS workspace_id,
    company_id                       AS company_id,
    company_name                     AS company_name,
    stage                            AS stage,
    pic_name                         AS pic_name,
    pic_nickname                     AS pic_nickname,
    pic_role                         AS pic_role,
    pic_wa                           AS pic_wa,
    pic_email                        AS pic_email,
    owner_name                       AS owner_name,
    owner_wa                         AS owner_wa,
    owner_telegram_id                AS owner_telegram_id,
    bot_active                       AS bot_active,
    blacklisted                      AS blacklisted,
    sequence_status                  AS sequence_status,
    snooze_until                     AS snooze_until,
    snooze_reason                    AS snooze_reason,
    risk_flag_text                   AS risk_flag,
    contract_start                   AS contract_start,
    contract_end                     AS contract_end,
    contract_months                  AS contract_months,
    days_to_expiry                   AS days_to_expiry,
    payment_status                   AS payment_status,
    payment_terms                    AS payment_terms,
    final_price                      AS final_price,
    last_payment_date                AS last_payment_date,
    billing_period                   AS billing_period,
    quantity                         AS quantity,
    unit_price                       AS unit_price,
    currency                         AS currency,
    last_interaction_date            AS last_interaction_date,
    notes                            AS notes,
    custom_fields                    AS custom_fields,
    created_at                       AS created_at,
    updated_at                       AS updated_at
FROM clients;
