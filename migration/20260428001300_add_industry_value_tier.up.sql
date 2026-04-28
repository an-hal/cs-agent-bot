-- Add the two CRM-core fields the spec lists in Table 1 (clients) but our
-- implementation never had:
--
--   industry    — Retail / F&B / Manufaktur / Jasa / dll. Free text in spec;
--                 we keep VARCHAR(100) for flexibility (workspace can curate
--                 their own industry vocabulary via custom_field_definitions
--                 if they want a controlled list).
--   value_tier  — HIGH / MID / LOW. ACV-based segmentation. Drives AE
--                 assignment + automation aggressiveness in the spec's
--                 "Risk_Flag" auto-rules.

ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS industry   VARCHAR(100),
    ADD COLUMN IF NOT EXISTS value_tier VARCHAR(20);

-- Rebuild master_data view to expose the new columns. Order kept stable so
-- scanMasterData's positional Scan stays in sync.
DROP VIEW IF EXISTS master_data;
CREATE OR REPLACE VIEW master_data AS
SELECT
    c.master_id              AS id,
    c.workspace_id           AS workspace_id,
    c.company_id             AS company_id,
    c.company_name           AS company_name,
    c.stage                  AS stage,
    COALESCE(c.industry, '')   AS industry,
    COALESCE(c.value_tier, '') AS value_tier,
    c.pic_name               AS pic_name,
    c.pic_nickname           AS pic_nickname,
    c.pic_role               AS pic_role,
    c.pic_wa                 AS pic_wa,
    c.pic_email              AS pic_email,
    c.owner_name             AS owner_name,
    c.owner_wa               AS owner_wa,
    COALESCE(cms.owner_telegram_id, '')           AS owner_telegram_id,
    COALESCE(cms.bot_active, TRUE)                AS bot_active,
    COALESCE(cms.blacklisted, FALSE)              AS blacklisted,
    COALESCE(cms.sequence_status, 'ACTIVE')       AS sequence_status,
    cms.snooze_until         AS snooze_until,
    cms.snooze_reason        AS snooze_reason,
    COALESCE(cms.risk_flag, 'None')               AS risk_flag,
    c.contract_start         AS contract_start,
    c.contract_end           AS contract_end,
    c.contract_months        AS contract_months,
    c.days_to_expiry         AS days_to_expiry,
    c.payment_status         AS payment_status,
    c.payment_terms          AS payment_terms,
    c.final_price            AS final_price,
    c.last_payment_date      AS last_payment_date,
    c.billing_period         AS billing_period,
    c.quantity               AS quantity,
    c.unit_price             AS unit_price,
    c.currency               AS currency,
    cms.last_interaction_date AS last_interaction_date,
    c.notes                  AS notes,
    c.custom_fields          AS custom_fields,
    c.created_at             AS created_at,
    c.updated_at             AS updated_at
FROM clients c
LEFT JOIN client_message_state cms ON cms.master_id = c.master_id;
