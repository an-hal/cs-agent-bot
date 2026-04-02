-- Migration: Add missing columns to action_log
-- Version: 20250401000005

ALTER TABLE action_log ADD COLUMN IF NOT EXISTS message_id VARCHAR(100);
ALTER TABLE action_log ADD COLUMN IF NOT EXISTS log_notes TEXT;
ALTER TABLE action_log ADD COLUMN IF NOT EXISTS company_name VARCHAR(200);
ALTER TABLE action_log ADD COLUMN IF NOT EXISTS response_classification VARCHAR(30);
ALTER TABLE action_log ADD COLUMN IF NOT EXISTS next_action_triggered VARCHAR(50);

CREATE INDEX IF NOT EXISTS idx_action_log_message_id ON action_log(message_id);
CREATE INDEX IF NOT EXISTS idx_action_log_company_date ON action_log(company_id, (triggered_at::date));
