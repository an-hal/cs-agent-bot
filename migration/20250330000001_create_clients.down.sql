-- Migration: Drop clients table
-- Version: 20250330000001
-- Description: Drops the clients table and its indexes

DROP INDEX IF EXISTS idx_clients_blacklisted;
DROP INDEX IF EXISTS idx_clients_renewal_date;
DROP INDEX IF EXISTS idx_clients_contract_end;
DROP INDEX IF EXISTS idx_clients_segment;
DROP INDEX IF EXISTS idx_clients_bot_active;
DROP INDEX IF EXISTS idx_clients_owner_telegram_id;
DROP INDEX IF EXISTS idx_clients_pic_wa;

DROP TABLE IF EXISTS clients;
DROP TABLE IF EXISTS schema_migrations;
