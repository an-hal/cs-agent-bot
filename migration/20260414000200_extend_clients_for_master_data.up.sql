-- Migration: Extend clients table for Master Data feature (feat/03-master-data)
-- Version: 20260414000200
-- Description:
--   Adds the columns required by the Master Data API on top of the existing
--   `clients` table. The bot's P0-P5 cron logic continues to use `clients`
--   directly. We never drop or rename existing columns; we ONLY add.
--
--   Notes:
--   - `clients` has a VARCHAR primary key (company_id). The Master Data spec
--     expects a UUID `id`. We add a synthetic `master_id UUID` column with a
--     default and unique index. The `master_data` view (migration 202) aliases
--     this as `id`.
--   - `clients.risk_flag` already exists as BOOLEAN. We add a new
--     `risk_flag_text` column (string: High|Mid|Low|None) for the spec.
--     The legacy boolean is left untouched.
--   - `sequence_status` is added separately from the existing `sequence_cs`
--     so we do not break the bot CS state machine.
--   - All ALTERs are idempotent (IF NOT EXISTS) so re-runs are safe.

ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS master_id UUID NOT NULL DEFAULT gen_random_uuid();

CREATE UNIQUE INDEX IF NOT EXISTS idx_clients_master_id ON clients(master_id);

ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS stage VARCHAR(20) NOT NULL DEFAULT 'LEAD';

ALTER TABLE clients
    ADD CONSTRAINT clients_stage_chk
    CHECK (stage IN ('LEAD','PROSPECT','CLIENT','DORMANT')) NOT VALID;

ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS pic_nickname VARCHAR(100);

ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS sequence_status VARCHAR(20) DEFAULT 'ACTIVE';

ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS snooze_until DATE;

ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS snooze_reason TEXT;

ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS risk_flag_text VARCHAR(10) NOT NULL DEFAULT 'None';

ALTER TABLE clients
    ADD CONSTRAINT clients_risk_flag_text_chk
    CHECK (risk_flag_text IN ('High','Mid','Low','None')) NOT VALID;

ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS days_to_expiry INT;

ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS custom_fields JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- updated_at trigger (idempotent)
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_clients_updated_at ON clients;
CREATE TRIGGER trg_clients_updated_at
    BEFORE UPDATE ON clients
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

-- Indexes for Master Data queries
CREATE INDEX IF NOT EXISTS idx_clients_workspace_stage
    ON clients(workspace_id, stage);
CREATE INDEX IF NOT EXISTS idx_clients_workspace_bot_active
    ON clients(workspace_id, bot_active);
CREATE INDEX IF NOT EXISTS idx_clients_workspace_company_name
    ON clients(workspace_id, company_name);
CREATE INDEX IF NOT EXISTS idx_clients_workspace_contract_end
    ON clients(workspace_id, contract_end);
CREATE INDEX IF NOT EXISTS idx_clients_workspace_payment_status
    ON clients(workspace_id, payment_status);
CREATE INDEX IF NOT EXISTS idx_clients_custom_fields_gin
    ON clients USING GIN(custom_fields);
