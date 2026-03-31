-- Migration: Drop cron_log table
-- Version: 20250330000006
-- Description: Drops the cron_log table and its indexes

DROP INDEX IF EXISTS idx_cron_log_processed_at;
DROP INDEX IF EXISTS idx_cron_log_company_id;
DROP INDEX IF EXISTS idx_cron_log_status;
DROP INDEX IF EXISTS idx_cron_log_run_date;

DROP TABLE IF EXISTS cron_log;
