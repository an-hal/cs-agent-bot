-- Migration: Create action_log table
-- Version: 20250330000005
-- Description: Creates the action_log table for tracking all bot actions and responses

CREATE TABLE action_log (
  id                    BIGSERIAL    PRIMARY KEY,
  company_id            VARCHAR(20)  REFERENCES clients(company_id) ON DELETE SET NULL,
  triggered_at          TIMESTAMP    DEFAULT NOW(),
  trigger_type          VARCHAR(50)  NOT NULL,
  template_id           VARCHAR(50),
  message_id            VARCHAR(100),
  channel               VARCHAR(10)  DEFAULT 'WA',
  sent_to_wa            VARCHAR(20),
  message_sent          BOOLEAN,
  status                VARCHAR(20),
  intent                VARCHAR(30),
  response_received     TEXT,
  next_action_triggered VARCHAR(50),
  by_human              VARCHAR(100),
  notes                 TEXT
);

-- Create indexes for queries and analytics
CREATE INDEX idx_action_log_company_id ON action_log(company_id);
CREATE INDEX idx_action_log_triggered_at ON action_log(triggered_at);
CREATE INDEX idx_action_log_trigger_type ON action_log(trigger_type);
CREATE INDEX idx_action_log_template_id ON action_log(template_id);
CREATE INDEX idx_action_log_channel ON action_log(channel);
CREATE INDEX idx_action_log_intent ON action_log(intent);

-- Revoke update/delete on action_log to maintain audit trail
DO $$
BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'cs_agent_app') THEN
    REVOKE UPDATE, DELETE ON action_log FROM cs_agent_app;
  END IF;
END $$;
