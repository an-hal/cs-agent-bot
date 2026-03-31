-- Migration: Create system_config table
-- Version: 20250330000007
-- Description: Creates the system_config table for storing system-wide configuration

CREATE TABLE system_config (
  key        VARCHAR(100) PRIMARY KEY,
  value      TEXT         NOT NULL,
  description TEXT,
  updated_at TIMESTAMP    DEFAULT NOW(),
  updated_by VARCHAR(100)
);

-- Create index for lookups
CREATE INDEX idx_system_config_key ON system_config(key);

-- Insert default configuration values
INSERT INTO system_config (key, value, description) VALUES
  ('cron_enabled', 'true', 'Enable/disable daily cron processing'),
  ('cron_hour', '8', 'Hour to run daily cron (0-23)'),
  ('batch_delay_ms', '300', 'Delay between processing clients in milliseconds'),
  ('max_retries', '3', 'Maximum retry attempts for failed operations'),
  ('wa_timeout_seconds', '30', 'Timeout for WhatsApp API calls'),
  ('halo_api_endpoint', '', 'HaloAI WhatsApp API endpoint'),
  ('telegram_bot_token', '', 'Telegram bot token for notifications'),
  ('escalation_enabled', 'true', 'Enable escalation notifications'),
  ('default_segment', 'SMB', 'Default client segment for new clients');
