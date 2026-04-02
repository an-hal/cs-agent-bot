-- Migration: Create conversation_states table
-- Version: 20250401000001

CREATE TABLE conversation_states (
  company_id               VARCHAR(20) PRIMARY KEY REFERENCES clients(company_id) ON DELETE CASCADE,
  company_name             VARCHAR(200),
  active_flow              VARCHAR(30),
  current_stage            VARCHAR(30),
  last_message_type        VARCHAR(50),
  last_message_date        TIMESTAMP,
  response_status          VARCHAR(20)  DEFAULT 'Pending',
  response_classification  VARCHAR(30),
  attempt_count            SMALLINT     DEFAULT 0,
  cooldown_until           TIMESTAMP,
  bot_active               BOOLEAN      DEFAULT TRUE,
  reason_bot_paused        VARCHAR(200),
  next_scheduled_action    VARCHAR(50),
  next_scheduled_date      TIMESTAMP,
  human_owner_notified     BOOLEAN      DEFAULT FALSE,
  created_at               TIMESTAMP    DEFAULT NOW(),
  updated_at               TIMESTAMP    DEFAULT NOW()
);

CREATE INDEX idx_conv_state_bot_active ON conversation_states(bot_active);
CREATE INDEX idx_conv_state_response_status ON conversation_states(response_status);
