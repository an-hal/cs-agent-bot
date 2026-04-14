-- Migration: Create master_data view over clients
-- Version: 20260414000202
-- Description: A read-only SQL view aliasing the existing `clients` table.
-- Writes go through the master_data usecase which targets `clients` directly.

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
