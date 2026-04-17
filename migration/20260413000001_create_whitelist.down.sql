-- Migration: Drop whitelist table
-- Version: 20260413000001

DROP INDEX IF EXISTS idx_whitelist_active;
DROP INDEX IF EXISTS idx_whitelist_email;
DROP TABLE IF EXISTS whitelist;
