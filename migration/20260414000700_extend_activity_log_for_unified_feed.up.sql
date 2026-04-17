-- Migration: Extend activity_log for unified feed (feat/08-activity-log)
-- Version: 20260414000700
-- Description: Adds spec-required columns to the existing unified activity_log table
--              so a single table backs the three logical log types (bot / data / team).
--              Also hardens CLAUDE.md rule 10 (INSERT-only audit) with REVOKE UPDATE, DELETE.

-- Actor identity (was previously: actor VARCHAR only)
ALTER TABLE activity_log ADD COLUMN IF NOT EXISTS actor_name   VARCHAR(255);
ALTER TABLE activity_log ADD COLUMN IF NOT EXISTS actor_email  VARCHAR(255);

-- Bot category — extra trace fields
ALTER TABLE activity_log ADD COLUMN IF NOT EXISTS trigger_id   VARCHAR(100);
ALTER TABLE activity_log ADD COLUMN IF NOT EXISTS template_id  VARCHAR(100);
ALTER TABLE activity_log ADD COLUMN IF NOT EXISTS phase        VARCHAR(10);
ALTER TABLE activity_log ADD COLUMN IF NOT EXISTS channel      VARCHAR(20);
ALTER TABLE activity_log ADD COLUMN IF NOT EXISTS replied      BOOLEAN DEFAULT FALSE;
ALTER TABLE activity_log ADD COLUMN IF NOT EXISTS reply_text   TEXT;

-- Data mutation category
ALTER TABLE activity_log ADD COLUMN IF NOT EXISTS company_id     VARCHAR(50);
ALTER TABLE activity_log ADD COLUMN IF NOT EXISTS company_name   VARCHAR(255);
ALTER TABLE activity_log ADD COLUMN IF NOT EXISTS changed_fields TEXT[];
ALTER TABLE activity_log ADD COLUMN IF NOT EXISTS previous_values JSONB;
ALTER TABLE activity_log ADD COLUMN IF NOT EXISTS new_values      JSONB;
ALTER TABLE activity_log ADD COLUMN IF NOT EXISTS bulk_count      INT;
ALTER TABLE activity_log ADD COLUMN IF NOT EXISTS note            TEXT;

-- Team category
ALTER TABLE activity_log ADD COLUMN IF NOT EXISTS target_name  VARCHAR(255);
ALTER TABLE activity_log ADD COLUMN IF NOT EXISTS target_email VARCHAR(255);
ALTER TABLE activity_log ADD COLUMN IF NOT EXISTS target_id    VARCHAR(100);

-- Spec indexes
CREATE INDEX IF NOT EXISTS idx_al_actor_email ON activity_log(actor_email);
CREATE INDEX IF NOT EXISTS idx_al_actor_type  ON activity_log(workspace_id, actor_type);
CREATE INDEX IF NOT EXISTS idx_al_status      ON activity_log(workspace_id, status);
CREATE INDEX IF NOT EXISTS idx_al_trigger_id  ON activity_log(workspace_id, trigger_id);
CREATE INDEX IF NOT EXISTS idx_al_company_id  ON activity_log(workspace_id, company_id);
CREATE INDEX IF NOT EXISTS idx_al_ws_occurred ON activity_log(workspace_id, occurred_at DESC);

-- CLAUDE.md rule 10: INSERT-only audit trail.
-- Revoke UPDATE and DELETE from the application role if it exists.
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cs_agent_app') THEN
    REVOKE UPDATE, DELETE ON activity_log FROM cs_agent_app;
  END IF;
END $$;
