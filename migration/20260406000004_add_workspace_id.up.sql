-- Migration: Add workspace_id to entity tables
-- Version: 20260406000004
-- Description: Adds workspace_id foreign key to all entity tables for multi-tenancy

ALTER TABLE clients ADD COLUMN IF NOT EXISTS workspace_id UUID REFERENCES workspaces(id);
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS workspace_id UUID REFERENCES workspaces(id);
ALTER TABLE client_flags ADD COLUMN IF NOT EXISTS workspace_id UUID REFERENCES workspaces(id);
ALTER TABLE escalations ADD COLUMN IF NOT EXISTS workspace_id UUID REFERENCES workspaces(id);
ALTER TABLE action_log ADD COLUMN IF NOT EXISTS workspace_id UUID REFERENCES workspaces(id);
ALTER TABLE conversation_states ADD COLUMN IF NOT EXISTS workspace_id UUID REFERENCES workspaces(id);
ALTER TABLE cron_log ADD COLUMN IF NOT EXISTS workspace_id UUID REFERENCES workspaces(id);

CREATE INDEX IF NOT EXISTS idx_clients_workspace_id ON clients(workspace_id);
CREATE INDEX IF NOT EXISTS idx_invoices_workspace_id ON invoices(workspace_id);
CREATE INDEX IF NOT EXISTS idx_client_flags_workspace_id ON client_flags(workspace_id);
CREATE INDEX IF NOT EXISTS idx_escalations_workspace_id ON escalations(workspace_id);
CREATE INDEX IF NOT EXISTS idx_action_log_workspace_id ON action_log(workspace_id);
CREATE INDEX IF NOT EXISTS idx_conversation_states_workspace_id ON conversation_states(workspace_id);
CREATE INDEX IF NOT EXISTS idx_cron_log_workspace_id ON cron_log(workspace_id);
