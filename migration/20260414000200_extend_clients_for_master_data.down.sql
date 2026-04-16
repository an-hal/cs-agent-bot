-- Down: revert columns added by 20260414000200.
-- We do not drop the trigger or updated_at column because other features
-- may now rely on them. We only drop columns we added.

DROP INDEX IF EXISTS idx_clients_custom_fields_gin;
DROP INDEX IF EXISTS idx_clients_workspace_payment_status;
DROP INDEX IF EXISTS idx_clients_workspace_contract_end;
DROP INDEX IF EXISTS idx_clients_workspace_company_name;
DROP INDEX IF EXISTS idx_clients_workspace_bot_active;
DROP INDEX IF EXISTS idx_clients_workspace_stage;
DROP INDEX IF EXISTS idx_clients_master_id;

ALTER TABLE clients DROP CONSTRAINT IF EXISTS clients_risk_flag_text_chk;
ALTER TABLE clients DROP CONSTRAINT IF EXISTS clients_stage_chk;

ALTER TABLE clients DROP COLUMN IF EXISTS custom_fields;
ALTER TABLE clients DROP COLUMN IF EXISTS days_to_expiry;
ALTER TABLE clients DROP COLUMN IF EXISTS risk_flag_text;
ALTER TABLE clients DROP COLUMN IF EXISTS snooze_reason;
ALTER TABLE clients DROP COLUMN IF EXISTS snooze_until;
ALTER TABLE clients DROP COLUMN IF EXISTS sequence_status;
ALTER TABLE clients DROP COLUMN IF EXISTS pic_nickname;
ALTER TABLE clients DROP COLUMN IF EXISTS stage;
ALTER TABLE clients DROP COLUMN IF EXISTS master_id;
