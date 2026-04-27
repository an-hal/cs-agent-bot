-- Reverse Phase 3: re-add 8 columns. Best-effort restore from custom_fields.

ALTER TABLE clients ADD COLUMN IF NOT EXISTS cross_sell_rejected    BOOLEAN DEFAULT FALSE;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS cross_sell_interested  BOOLEAN DEFAULT FALSE;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS cross_sell_resume_date DATE;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS renewed                BOOLEAN DEFAULT FALSE;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS rejected               BOOLEAN DEFAULT FALSE;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS quotation_link         VARCHAR(500);
ALTER TABLE clients ADD COLUMN IF NOT EXISTS segment                VARCHAR(10);
ALTER TABLE clients ADD COLUMN IF NOT EXISTS risk_flag              BOOLEAN DEFAULT FALSE;

UPDATE clients SET
    cross_sell_rejected   = COALESCE((custom_fields->>'cross_sell_rejected')::boolean, cross_sell_rejected),
    cross_sell_interested = COALESCE((custom_fields->>'cross_sell_interested')::boolean, cross_sell_interested),
    renewed               = COALESCE((custom_fields->>'renewed')::boolean, renewed),
    rejected              = COALESCE((custom_fields->>'rejected')::boolean, rejected),
    quotation_link        = COALESCE(custom_fields->>'quotation_link', quotation_link),
    segment               = COALESCE(custom_fields->>'segment', segment);

-- Recreate master_data view including the restored `renewed` column.
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
    renewed                          AS renewed,
    last_interaction_date            AS last_interaction_date,
    notes                            AS notes,
    custom_fields                    AS custom_fields,
    created_at                       AS created_at,
    updated_at                       AS updated_at
FROM clients;
