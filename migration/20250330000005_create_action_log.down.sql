-- Migration: Drop action_log table
-- Version: 20250330000005
-- Description: Drops the action_log table and its indexes

DROP INDEX IF EXISTS idx_action_log_intent;
DROP INDEX IF EXISTS idx_action_log_channel;
DROP INDEX IF EXISTS idx_action_log_template_id;
DROP INDEX IF EXISTS idx_action_log_trigger_type;
DROP INDEX IF EXISTS idx_action_log_triggered_at;
DROP INDEX IF EXISTS idx_action_log_company_id;

DROP TABLE IF EXISTS action_log;
