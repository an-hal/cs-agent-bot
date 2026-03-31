-- Migration: Drop escalations table
-- Version: 20250330000004
-- Description: Drops the escalations table and its indexes

DROP INDEX IF EXISTS idx_escalations_triggered_at;
DROP INDEX IF EXISTS idx_escalations_esc_id;
DROP INDEX IF EXISTS idx_escalations_priority;
DROP INDEX IF EXISTS idx_escalations_status;
DROP INDEX IF EXISTS idx_escalations_company_id;

DROP TABLE IF EXISTS escalations;
