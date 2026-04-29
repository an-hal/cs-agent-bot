-- Phase 6 cleanup round 2 — finish what migration 800 started.
--
-- The CRM spec (context/new/crm_database_spec.md, Table 1) defines clients as
-- a *portable* CRM-core table; bot-automation flags belong in client_message_state.
-- After Phase 6 (migration 700) and the residual cleanup (800), these columns
-- still leak from bot_state into clients:
--
--   bot_active, sequence_cs            — re-emerged after migration 800
--                                        (cause unknown — possibly an external
--                                        ALTER outside our migration runner)
--   blacklisted                         — bot stop-flag, spec says bot_state
--   risk_flag_text                      — bot-computed, spec says bot_state
--   owner_telegram_id                   — bot alert routing, spec says bot_state
--   ae_assigned, ae_telegram_id,
--   backup_owner_telegram_id            — AE routing for bot escalations
--
-- Plan: backfill the new columns into client_message_state, drop the master_data
-- view, drop the columns from clients, recreate the view sourcing the relocated
-- fields from cms.

-- Step 1 — extend client_message_state with the new columns.
ALTER TABLE client_message_state
    ADD COLUMN IF NOT EXISTS blacklisted              BOOLEAN     NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS risk_flag                VARCHAR(20) NOT NULL DEFAULT 'None',
    ADD COLUMN IF NOT EXISTS owner_telegram_id        VARCHAR(100),
    ADD COLUMN IF NOT EXISTS ae_assigned              BOOLEAN     NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS ae_telegram_id           VARCHAR(100),
    ADD COLUMN IF NOT EXISTS backup_owner_telegram_id VARCHAR(100);

-- Step 2 — backfill from clients. Each is best-effort; missing rows in cms
-- get seeded by the prior split's master_id PK.
UPDATE client_message_state cms SET
    blacklisted              = COALESCE(c.blacklisted, FALSE),
    risk_flag                = COALESCE(c.risk_flag_text, 'None'),
    owner_telegram_id        = c.owner_telegram_id,
    ae_assigned              = COALESCE(c.ae_assigned, FALSE),
    ae_telegram_id           = c.ae_telegram_id,
    backup_owner_telegram_id = c.backup_owner_telegram_id
FROM clients c WHERE cms.master_id = c.master_id;

-- Step 3 — drop the view; we'll rebuild it sourcing from cms.
DROP VIEW IF EXISTS master_data;

-- Step 4 — drop the relocated columns from clients (plus the residual
-- bot_active / sequence_cs that migration 800 was supposed to remove).
ALTER TABLE clients
    DROP COLUMN IF EXISTS bot_active,
    DROP COLUMN IF EXISTS sequence_cs,
    DROP COLUMN IF EXISTS blacklisted,
    DROP COLUMN IF EXISTS risk_flag_text,
    DROP COLUMN IF EXISTS owner_telegram_id,
    DROP COLUMN IF EXISTS ae_assigned,
    DROP COLUMN IF EXISTS ae_telegram_id,
    DROP COLUMN IF EXISTS backup_owner_telegram_id;

-- Step 5 — recreate master_data view. blacklisted / risk_flag /
-- owner_telegram_id now come from cms with COALESCE-on-missing so a row
-- without a cms companion still reads sensibly.
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
