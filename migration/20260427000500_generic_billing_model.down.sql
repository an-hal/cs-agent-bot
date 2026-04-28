-- Reverse Phase 5: restore restrictive billing schema.

DROP VIEW IF EXISTS master_data;

-- Drop new billing columns
ALTER TABLE clients DROP CONSTRAINT IF EXISTS chk_clients_billing_period;
ALTER TABLE clients DROP COLUMN IF EXISTS billing_period;
ALTER TABLE clients DROP COLUMN IF EXISTS quantity;
ALTER TABLE clients DROP COLUMN IF EXISTS unit_price;
ALTER TABLE clients DROP COLUMN IF EXISTS currency;

-- Restore NOT NULL on contract_end (any NULL row gets contract_start as fallback)
UPDATE clients SET contract_end = contract_start + INTERVAL '12 months' WHERE contract_end IS NULL;
ALTER TABLE clients ALTER COLUMN contract_end SET NOT NULL;

-- Restore NOT NULL on owner_telegram_id
UPDATE clients SET owner_telegram_id = '' WHERE owner_telegram_id IS NULL;
ALTER TABLE clients ALTER COLUMN owner_telegram_id SET NOT NULL;

-- Restore NOT NULL UNIQUE on pic_wa
DROP INDEX IF EXISTS idx_clients_pic_wa_uniq;
UPDATE clients SET pic_wa = '0000000000-' || master_id::text WHERE pic_wa IS NULL;
ALTER TABLE clients ALTER COLUMN pic_wa SET NOT NULL;
ALTER TABLE clients ADD CONSTRAINT clients_pic_wa_key UNIQUE (pic_wa);

-- Recreate master_data view (without new billing columns)
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
    last_interaction_date            AS last_interaction_date,
    notes                            AS notes,
    custom_fields                    AS custom_fields,
    created_at                       AS created_at,
    updated_at                       AS updated_at
FROM clients;
