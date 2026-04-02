-- Migration: Seed Test Clients (Rollback)
-- Version: 20250401000006

-- Delete in reverse dependency order
DELETE FROM invoices WHERE company_id LIKE 'TC%';
DELETE FROM conversation_states WHERE company_id LIKE 'TC%';
DELETE FROM client_flags WHERE company_id LIKE 'TC%';
DELETE FROM action_log WHERE company_id LIKE 'TC%';
DELETE FROM clients WHERE company_id LIKE 'TC%';
