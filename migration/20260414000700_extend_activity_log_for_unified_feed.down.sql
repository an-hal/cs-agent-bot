-- Migration: Extend activity_log for unified feed — rollback
-- Version: 20260414000700

DROP INDEX IF EXISTS idx_al_ws_occurred;
DROP INDEX IF EXISTS idx_al_company_id;
DROP INDEX IF EXISTS idx_al_trigger_id;
DROP INDEX IF EXISTS idx_al_status;
DROP INDEX IF EXISTS idx_al_actor_type;
DROP INDEX IF EXISTS idx_al_actor_email;

ALTER TABLE activity_log DROP COLUMN IF EXISTS target_id;
ALTER TABLE activity_log DROP COLUMN IF EXISTS target_email;
ALTER TABLE activity_log DROP COLUMN IF EXISTS target_name;

ALTER TABLE activity_log DROP COLUMN IF EXISTS note;
ALTER TABLE activity_log DROP COLUMN IF EXISTS bulk_count;
ALTER TABLE activity_log DROP COLUMN IF EXISTS new_values;
ALTER TABLE activity_log DROP COLUMN IF EXISTS previous_values;
ALTER TABLE activity_log DROP COLUMN IF EXISTS changed_fields;
ALTER TABLE activity_log DROP COLUMN IF EXISTS company_name;
ALTER TABLE activity_log DROP COLUMN IF EXISTS company_id;

ALTER TABLE activity_log DROP COLUMN IF EXISTS reply_text;
ALTER TABLE activity_log DROP COLUMN IF EXISTS replied;
ALTER TABLE activity_log DROP COLUMN IF EXISTS channel;
ALTER TABLE activity_log DROP COLUMN IF EXISTS phase;
ALTER TABLE activity_log DROP COLUMN IF EXISTS template_id;
ALTER TABLE activity_log DROP COLUMN IF EXISTS trigger_id;

ALTER TABLE activity_log DROP COLUMN IF EXISTS actor_email;
ALTER TABLE activity_log DROP COLUMN IF EXISTS actor_name;

-- Note: REVOKE is not re-granted here because the initial table did not grant it.
