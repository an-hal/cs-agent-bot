-- Migration: Drop system_config table
-- Version: 20250330000007
-- Description: Drops the system_config table and its indexes

DROP INDEX IF EXISTS idx_system_config_key;
DROP TABLE IF EXISTS system_config;
