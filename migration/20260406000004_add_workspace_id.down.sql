DROP INDEX IF EXISTS idx_cron_log_workspace_id;
DROP INDEX IF EXISTS idx_conversation_states_workspace_id;
DROP INDEX IF EXISTS idx_action_log_workspace_id;
DROP INDEX IF EXISTS idx_escalations_workspace_id;
DROP INDEX IF EXISTS idx_client_flags_workspace_id;
DROP INDEX IF EXISTS idx_invoices_workspace_id;
DROP INDEX IF EXISTS idx_clients_workspace_id;

ALTER TABLE cron_log DROP COLUMN IF EXISTS workspace_id;
ALTER TABLE conversation_states DROP COLUMN IF EXISTS workspace_id;
ALTER TABLE action_log DROP COLUMN IF EXISTS workspace_id;
ALTER TABLE escalations DROP COLUMN IF EXISTS workspace_id;
ALTER TABLE client_flags DROP COLUMN IF EXISTS workspace_id;
ALTER TABLE invoices DROP COLUMN IF EXISTS workspace_id;
ALTER TABLE clients DROP COLUMN IF EXISTS workspace_id;
