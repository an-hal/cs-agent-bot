-- Revert NOT NULL constraints
ALTER TABLE conversation_states ALTER COLUMN workspace_id DROP NOT NULL;
ALTER TABLE client_flags ALTER COLUMN workspace_id DROP NOT NULL;
ALTER TABLE invoices ALTER COLUMN workspace_id DROP NOT NULL;
ALTER TABLE clients ALTER COLUMN workspace_id DROP NOT NULL;

-- Clear workspace_id values
UPDATE cron_log SET workspace_id = NULL;
UPDATE conversation_states SET workspace_id = NULL;
UPDATE action_log SET workspace_id = NULL;
UPDATE escalations SET workspace_id = NULL;
UPDATE client_flags SET workspace_id = NULL;
UPDATE invoices SET workspace_id = NULL;
UPDATE clients SET workspace_id = NULL;
