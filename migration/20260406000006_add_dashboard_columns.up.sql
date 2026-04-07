-- Migration: Add missing dashboard columns
-- Version: 20260406000006
-- Description: Adds columns needed by the dashboard that don't exist yet

-- Invoices
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS notes TEXT;
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS link_invoice VARCHAR(500);
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS last_reminder_date TIMESTAMP;

-- Action Log
ALTER TABLE action_log ADD COLUMN IF NOT EXISTS reply_timestamp TIMESTAMP;
ALTER TABLE action_log ADD COLUMN IF NOT EXISTS reply_text TEXT;
ALTER TABLE action_log ADD COLUMN IF NOT EXISTS ae_notified BOOLEAN DEFAULT FALSE;
