-- Migrate invoice reminder and overdue sent-flags from clients (dropped) to client_flags.
-- pre14_sent / pre7_sent / pre3_sent: H-14 / H-7 / H-3 invoice reminders
-- post1_sent / post4_sent / post8_sent / post15_sent: overdue day +1/+4/+8/+15 reminders
ALTER TABLE client_flags
    ADD COLUMN IF NOT EXISTS pre14_sent  boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS pre7_sent   boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS pre3_sent   boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS post1_sent  boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS post4_sent  boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS post8_sent  boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS post15_sent boolean NOT NULL DEFAULT false;
