-- Action Log
ALTER TABLE action_log DROP COLUMN IF EXISTS ae_notified;
ALTER TABLE action_log DROP COLUMN IF EXISTS reply_text;
ALTER TABLE action_log DROP COLUMN IF EXISTS reply_timestamp;

-- Invoices
ALTER TABLE invoices DROP COLUMN IF EXISTS last_reminder_date;
ALTER TABLE invoices DROP COLUMN IF EXISTS link_invoice;
ALTER TABLE invoices DROP COLUMN IF EXISTS notes;
