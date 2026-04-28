-- Phase 6 cleanup: bot_active and sequence_cs were supposed to be dropped from
-- `clients` in migration 700 but are still present (with diverged definitions:
-- nullable/default-FALSE in clients vs NOT NULL/default-TRUE in cms). Reads
-- already go through `master_data` view → `cms.bot_active` and the
-- repository's clientColumns explicitly aliases `cms.bot_active`/`cms.sequence_cs`.
-- No view depends on the residual columns. Safe to drop.
--
-- We rebuild the view defensively in case the residual columns were silently
-- referenced by an older view definition.

DROP VIEW IF EXISTS master_data;

ALTER TABLE clients
    DROP COLUMN IF EXISTS bot_active,
    DROP COLUMN IF EXISTS sequence_cs;

CREATE OR REPLACE VIEW master_data AS
SELECT
    c.master_id              AS id,
    c.workspace_id           AS workspace_id,
    c.company_id             AS company_id,
    c.company_name           AS company_name,
    c.stage                  AS stage,
    c.pic_name               AS pic_name,
    c.pic_nickname           AS pic_nickname,
    c.pic_role               AS pic_role,
    c.pic_wa                 AS pic_wa,
    c.pic_email              AS pic_email,
    c.owner_name             AS owner_name,
    c.owner_wa               AS owner_wa,
    c.owner_telegram_id      AS owner_telegram_id,
    COALESCE(cms.bot_active, TRUE)              AS bot_active,
    c.blacklisted            AS blacklisted,
    COALESCE(cms.sequence_status, 'ACTIVE')     AS sequence_status,
    cms.snooze_until         AS snooze_until,
    cms.snooze_reason        AS snooze_reason,
    c.risk_flag_text         AS risk_flag,
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
