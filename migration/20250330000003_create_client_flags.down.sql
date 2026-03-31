-- Migration: Drop client_flags table
-- Version: 20250330000003
-- Description: Drops the client_flags table and its indexes

DROP INDEX IF EXISTS idx_client_flags_company_id;
DROP TABLE IF EXISTS client_flags;
