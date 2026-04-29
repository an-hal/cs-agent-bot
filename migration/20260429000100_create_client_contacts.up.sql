-- Multi-stage PIC: see context/new/multi-stage-pic-spec.md
--
-- Replaces the single (clients.pic_*, clients.owner_*) model with a per-stage
-- contact table so a client moving LEAD → PROSPECT → CLIENT can rotate both
-- the internal owner (SDR → BD → AE) and the client-side PIC (Owner → HR →
-- Finance) without overwriting history.

CREATE TABLE IF NOT EXISTS client_contacts (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    master_id    UUID NOT NULL REFERENCES clients(master_id) ON DELETE CASCADE,

    stage        VARCHAR(20)  NOT NULL,
    kind         VARCHAR(20)  NOT NULL CHECK (kind IN ('internal', 'client_side')),
    role         VARCHAR(100),

    name         VARCHAR(255) NOT NULL,
    wa           VARCHAR(20),
    email        VARCHAR(100),
    telegram_id  VARCHAR(100),

    is_primary   BOOLEAN     NOT NULL DEFAULT TRUE,
    notes        TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Exactly one primary per (client, stage, kind). Backups (is_primary=FALSE)
-- can have multiple rows. Partial unique index because Postgres allows
-- multiple FALSE values in a regular UNIQUE.
CREATE UNIQUE INDEX IF NOT EXISTS uq_client_contacts_primary
    ON client_contacts (master_id, stage, kind)
    WHERE is_primary = TRUE;

CREATE INDEX IF NOT EXISTS idx_client_contacts_lookup
    ON client_contacts (master_id, stage, kind);
CREATE INDEX IF NOT EXISTS idx_client_contacts_workspace
    ON client_contacts (workspace_id);

CREATE OR REPLACE FUNCTION update_client_contacts_updated_at()
RETURNS TRIGGER AS $$
BEGIN NEW.updated_at = NOW(); RETURN NEW; END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS trg_client_contacts_updated_at ON client_contacts;
CREATE TRIGGER trg_client_contacts_updated_at BEFORE UPDATE ON client_contacts
    FOR EACH ROW EXECUTE FUNCTION update_client_contacts_updated_at();

-- ── Backfill ────────────────────────────────────────────────────────
-- Seed one client_side contact (from clients.pic_*) and one internal
-- contact (from clients.owner_* + cms.owner_telegram_id) per existing
-- client at its current stage. Skip rows where the source is NULL/empty.

INSERT INTO client_contacts (
    workspace_id, master_id, stage, kind, role,
    name, wa, email, telegram_id, is_primary
)
SELECT
    c.workspace_id,
    c.master_id,
    c.stage,
    'client_side',
    NULLIF(c.pic_role, ''),
    c.pic_name,
    NULLIF(c.pic_wa, ''),
    NULLIF(c.pic_email, ''),
    NULL,
    TRUE
FROM clients c
WHERE c.pic_name IS NOT NULL AND c.pic_name <> ''
ON CONFLICT DO NOTHING;

INSERT INTO client_contacts (
    workspace_id, master_id, stage, kind, role,
    name, wa, email, telegram_id, is_primary
)
SELECT
    c.workspace_id,
    c.master_id,
    c.stage,
    'internal',
    'AE',
    c.owner_name,
    NULLIF(c.owner_wa, ''),
    NULL,
    NULLIF(cms.owner_telegram_id, ''),
    TRUE
FROM clients c
LEFT JOIN client_message_state cms ON cms.master_id = c.master_id
WHERE c.owner_name IS NOT NULL AND c.owner_name <> ''
ON CONFLICT DO NOTHING;

-- ── Rebuild master_data view ────────────────────────────────────────
-- pic_* and owner_*/owner_telegram_id now sourced from client_contacts at
-- the client's CURRENT stage via LATERAL JOIN. Clients.pic_*/owner_* still
-- exist (deprecated) for legacy bot upsert paths and Migration 002 backfill
-- safety; FE/dashboard reads the view, so no caller change is required.

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
    COALESCE(cc_pic.name, c.pic_name, '')   AS pic_name,
    COALESCE(cc_pic.role, c.pic_role, '')   AS pic_nickname, -- kept for view back-compat; spec calls this pic_role
    COALESCE(cc_pic.role, c.pic_role, '')   AS pic_role,
    COALESCE(cc_pic.wa, c.pic_wa, '')       AS pic_wa,
    COALESCE(cc_pic.email, c.pic_email, '') AS pic_email,
    COALESCE(cc_own.name, c.owner_name, '') AS owner_name,
    COALESCE(cc_own.wa, c.owner_wa, '')     AS owner_wa,
    COALESCE(cc_own.telegram_id, cms.owner_telegram_id, '') AS owner_telegram_id,
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
LEFT JOIN client_message_state cms ON cms.master_id = c.master_id
LEFT JOIN LATERAL (
    SELECT name, role, wa, email FROM client_contacts
    WHERE master_id = c.master_id AND stage = c.stage AND kind = 'client_side' AND is_primary = TRUE
    LIMIT 1
) cc_pic ON TRUE
LEFT JOIN LATERAL (
    SELECT name, role, wa, telegram_id FROM client_contacts
    WHERE master_id = c.master_id AND stage = c.stage AND kind = 'internal' AND is_primary = TRUE
    LIMIT 1
) cc_own ON TRUE;
