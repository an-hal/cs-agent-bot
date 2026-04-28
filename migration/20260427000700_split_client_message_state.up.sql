-- Phase 6: extract bot/CS message-delivery state from `clients` into a
-- dedicated `client_message_state` table. `clients` becomes pure CRM data;
-- `client_message_state` holds bot interaction state (1:1 by master_id).
--
-- Columns moved (17): bot_active, sequence_cs, sequence_status,
-- response_status, snooze_until, snooze_reason, pre14_sent..post15_sent (7),
-- checkin_replied, feature_update_sent, last_interaction_date,
-- days_since_cs_last_sent.
--
-- The master_data view is rebuilt as a LEFT JOIN so consumers still see
-- those fields. The `clients`-as-bot-domain repository uses the same view
-- (or a similar JOIN) to keep entity.Client backward-compatible.

CREATE TABLE IF NOT EXISTS client_message_state (
    master_id    UUID PRIMARY KEY REFERENCES clients(master_id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,

    bot_active             BOOLEAN     NOT NULL DEFAULT TRUE,
    sequence_cs            VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    sequence_status        VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    response_status        VARCHAR(20) NOT NULL DEFAULT 'Pending',
    snooze_until           DATE,
    snooze_reason          TEXT,

    pre14_sent  BOOLEAN NOT NULL DEFAULT FALSE,
    pre7_sent   BOOLEAN NOT NULL DEFAULT FALSE,
    pre3_sent   BOOLEAN NOT NULL DEFAULT FALSE,
    post1_sent  BOOLEAN NOT NULL DEFAULT FALSE,
    post4_sent  BOOLEAN NOT NULL DEFAULT FALSE,
    post8_sent  BOOLEAN NOT NULL DEFAULT FALSE,
    post15_sent BOOLEAN NOT NULL DEFAULT FALSE,

    checkin_replied         BOOLEAN  NOT NULL DEFAULT FALSE,
    feature_update_sent     BOOLEAN  NOT NULL DEFAULT FALSE,
    last_interaction_date   DATE,
    days_since_cs_last_sent SMALLINT NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cms_workspace ON client_message_state(workspace_id);
CREATE INDEX IF NOT EXISTS idx_cms_sequence_cs ON client_message_state(sequence_cs);
CREATE INDEX IF NOT EXISTS idx_cms_bot_active ON client_message_state(bot_active);

-- Sync existing data from clients into client_message_state (1:1).
INSERT INTO client_message_state (
    master_id, workspace_id,
    bot_active, sequence_cs, sequence_status, response_status,
    snooze_until, snooze_reason,
    pre14_sent, pre7_sent, pre3_sent,
    post1_sent, post4_sent, post8_sent, post15_sent,
    checkin_replied, feature_update_sent,
    last_interaction_date, days_since_cs_last_sent
)
SELECT
    master_id, workspace_id,
    COALESCE(bot_active, TRUE),
    COALESCE(sequence_cs, 'ACTIVE'),
    COALESCE(sequence_status, 'ACTIVE'),
    COALESCE(response_status, 'Pending'),
    snooze_until,
    snooze_reason,
    COALESCE(pre14_sent, FALSE), COALESCE(pre7_sent, FALSE), COALESCE(pre3_sent, FALSE),
    COALESCE(post1_sent, FALSE), COALESCE(post4_sent, FALSE),
    COALESCE(post8_sent, FALSE), COALESCE(post15_sent, FALSE),
    COALESCE(checkin_replied, FALSE),
    COALESCE(feature_update_sent, FALSE),
    last_interaction_date,
    COALESCE(days_since_cs_last_sent, 0)
FROM clients
ON CONFLICT (master_id) DO NOTHING;

-- Updated-at trigger so cron writes bump the timestamp.
CREATE OR REPLACE FUNCTION update_cms_updated_at()
RETURNS TRIGGER AS $$
BEGIN NEW.updated_at = NOW(); RETURN NEW; END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS trg_cms_updated_at ON client_message_state;
CREATE TRIGGER trg_cms_updated_at BEFORE UPDATE ON client_message_state
    FOR EACH ROW EXECUTE FUNCTION update_cms_updated_at();

-- Rebuild master_data view: clients (CRM core) LEFT JOIN message_state.
DROP VIEW IF EXISTS master_data;

ALTER TABLE clients
    DROP COLUMN IF EXISTS bot_active,
    DROP COLUMN IF EXISTS sequence_cs,
    DROP COLUMN IF EXISTS sequence_status,
    DROP COLUMN IF EXISTS response_status,
    DROP COLUMN IF EXISTS snooze_until,
    DROP COLUMN IF EXISTS snooze_reason,
    DROP COLUMN IF EXISTS pre14_sent,
    DROP COLUMN IF EXISTS pre7_sent,
    DROP COLUMN IF EXISTS pre3_sent,
    DROP COLUMN IF EXISTS post1_sent,
    DROP COLUMN IF EXISTS post4_sent,
    DROP COLUMN IF EXISTS post8_sent,
    DROP COLUMN IF EXISTS post15_sent,
    DROP COLUMN IF EXISTS checkin_replied,
    DROP COLUMN IF EXISTS feature_update_sent,
    DROP COLUMN IF EXISTS last_interaction_date,
    DROP COLUMN IF EXISTS days_since_cs_last_sent;

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
