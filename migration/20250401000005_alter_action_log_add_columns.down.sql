DROP INDEX IF EXISTS idx_action_log_company_date;
DROP INDEX IF EXISTS idx_action_log_message_id;
ALTER TABLE action_log DROP COLUMN IF EXISTS next_action_triggered;
ALTER TABLE action_log DROP COLUMN IF EXISTS response_classification;
ALTER TABLE action_log DROP COLUMN IF EXISTS company_name;
ALTER TABLE action_log DROP COLUMN IF EXISTS log_notes;
ALTER TABLE action_log DROP COLUMN IF EXISTS message_id;
