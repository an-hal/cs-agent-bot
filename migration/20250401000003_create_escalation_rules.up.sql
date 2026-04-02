-- Migration: Create escalation_rules table
-- Version: 20250401000003

CREATE TABLE escalation_rules (
  esc_id             VARCHAR(10) PRIMARY KEY,
  name               VARCHAR(200) NOT NULL,
  trigger_condition  TEXT,
  priority           VARCHAR(20) NOT NULL,
  telegram_msg       TEXT,
  active             BOOLEAN DEFAULT TRUE,
  created_at         TIMESTAMP DEFAULT NOW(),
  updated_at         TIMESTAMP DEFAULT NOW()
);
