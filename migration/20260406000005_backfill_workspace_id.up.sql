-- Migration: Backfill workspace_id based on company_id prefix
-- Version: 20260406000005
-- Description: KK* prefix → kantorku workspace, everything else → dealls

-- Backfill clients first
UPDATE clients SET workspace_id = (
  SELECT id FROM workspaces WHERE slug =
    CASE WHEN company_id LIKE 'KK%' THEN 'kantorku' ELSE 'dealls' END
) WHERE workspace_id IS NULL;

-- Propagate to child tables via company_id
UPDATE invoices SET workspace_id = (
  SELECT workspace_id FROM clients WHERE clients.company_id = invoices.company_id
) WHERE workspace_id IS NULL;

UPDATE client_flags SET workspace_id = (
  SELECT workspace_id FROM clients WHERE clients.company_id = client_flags.company_id
) WHERE workspace_id IS NULL;

UPDATE escalations SET workspace_id = (
  SELECT workspace_id FROM clients WHERE clients.company_id = escalations.company_id
) WHERE workspace_id IS NULL;

UPDATE action_log SET workspace_id = (
  SELECT workspace_id FROM clients WHERE clients.company_id = action_log.company_id
) WHERE workspace_id IS NULL;

UPDATE conversation_states SET workspace_id = (
  SELECT workspace_id FROM clients WHERE clients.company_id = conversation_states.company_id
) WHERE workspace_id IS NULL;

UPDATE cron_log SET workspace_id = (
  SELECT workspace_id FROM clients WHERE clients.company_id = cron_log.company_id
) WHERE workspace_id IS NULL;

-- Make NOT NULL after backfill for core tables
ALTER TABLE clients ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE invoices ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE client_flags ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE conversation_states ALTER COLUMN workspace_id SET NOT NULL;
-- action_log, escalations, cron_log: keep nullable (historical records may lack mapping)
