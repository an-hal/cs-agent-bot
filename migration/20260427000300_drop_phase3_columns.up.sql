-- Phase 3: drop 8 high-usage columns and migrate data into custom_fields.
-- Goal: complete the move toward a generic CRM core.
--
-- Columns dropped:
--   1. cross_sell_rejected     (bot lifecycle flag, varies per workspace)
--   2. cross_sell_interested
--   3. cross_sell_resume_date
--   4. renewed                  (lifecycle flag)
--   5. rejected                 (lifecycle flag)
--   6. quotation_link           (sales doc URL)
--   7. segment                  (workspace label, varies per CRM)
--   8. risk_flag                (BOOLEAN, deprecated — replaced by risk_flag_text)
--
-- Note: risk_flag (boolean) is dropped without migration because the
-- VARCHAR variant `risk_flag_text` already supplants it. The boolean form
-- has been dead code since migration 20260414000200 added the text column.

UPDATE clients
SET custom_fields = COALESCE(
        (SELECT jsonb_object_agg(key, value)
         FROM (
             VALUES
                 ('cross_sell_rejected', to_jsonb(cross_sell_rejected)),
                 ('cross_sell_interested', to_jsonb(cross_sell_interested)),
                 ('cross_sell_resume_date', to_jsonb(cross_sell_resume_date)),
                 ('renewed', to_jsonb(renewed)),
                 ('rejected', to_jsonb(rejected)),
                 ('quotation_link', to_jsonb(quotation_link)),
                 ('segment', to_jsonb(segment))
         ) AS kv(key, value)
         WHERE value IS NOT NULL AND value <> 'null'::jsonb
           AND NOT (custom_fields ? key)),
        '{}'::jsonb
    ) || custom_fields
WHERE
    cross_sell_rejected     IS NOT NULL
 OR cross_sell_interested   IS NOT NULL
 OR cross_sell_resume_date  IS NOT NULL
 OR renewed                  IS NOT NULL
 OR rejected                 IS NOT NULL
 OR quotation_link           IS NOT NULL
 OR segment                  IS NOT NULL;

-- The master_data view references `renewed` (and previously also `risk_flag`,
-- but that column was already replaced by `risk_flag_text` in migration
-- 20260414000200). Drop the view, drop the columns, then recreate the view
-- without the dropped columns. Callers that need the moved fields read them
-- from `custom_fields` JSONB instead.
DROP VIEW IF EXISTS master_data;

ALTER TABLE clients
    DROP COLUMN IF EXISTS cross_sell_rejected,
    DROP COLUMN IF EXISTS cross_sell_interested,
    DROP COLUMN IF EXISTS cross_sell_resume_date,
    DROP COLUMN IF EXISTS renewed,
    DROP COLUMN IF EXISTS rejected,
    DROP COLUMN IF EXISTS quotation_link,
    DROP COLUMN IF EXISTS segment,
    DROP COLUMN IF EXISTS risk_flag;

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
