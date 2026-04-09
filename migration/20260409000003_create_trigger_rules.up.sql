-- Migration: Create trigger_rules table
-- Version: 20260409000003
-- Description: Creates the trigger_rules table for dynamic trigger condition evaluation.
-- Each row replaces one hardcoded if-block in the trigger evaluators.

CREATE TABLE trigger_rules (
  rule_id         VARCHAR(50)   PRIMARY KEY,
  rule_group      VARCHAR(30)   NOT NULL,          -- e.g. HEALTH, CHECKIN, NEGOTIATION, INVOICE, OVERDUE, EXPANSION, CROSS_SELL
  priority        INT           NOT NULL DEFAULT 0, -- global ordering: lower = higher priority
  sub_priority    INT           NOT NULL DEFAULT 0, -- ordering within group
  condition       JSONB         NOT NULL,           -- JSON condition expression
  action_type     VARCHAR(30)   NOT NULL,           -- send_wa, send_email, escalate, alert_telegram, create_invoice, skip_and_set_flag
  template_id     VARCHAR(50),                      -- FK to templates (nullable for non-send actions)
  flag_key        VARCHAR(100)  NOT NULL,           -- flag to check/set (e.g. "low_usage_msg_sent")
  escalation_id   VARCHAR(20),                      -- ESC-XXX for escalation actions (nullable)
  esc_priority    VARCHAR(30),                      -- escalation priority (nullable)
  esc_reason      TEXT,                             -- escalation reason (nullable)
  extra_flags     JSONB,                            -- additional flags to set on fire (nullable)
  stop_on_fire    BOOLEAN       NOT NULL DEFAULT TRUE, -- if true, stop evaluating further rules after this fires
  active          BOOLEAN       NOT NULL DEFAULT TRUE,
  description     TEXT,                             -- human-readable description of the rule
  workspace_id    VARCHAR(50),                      -- nullable, for workspace-specific overrides
  created_at      TIMESTAMP     DEFAULT NOW(),
  updated_at      TIMESTAMP     DEFAULT NOW()
);

-- Indexes for ordered rule retrieval
CREATE INDEX idx_trigger_rules_priority ON trigger_rules(priority, sub_priority);
CREATE INDEX idx_trigger_rules_group ON trigger_rules(rule_group);
CREATE INDEX idx_trigger_rules_active ON trigger_rules(active);
CREATE INDEX idx_trigger_rules_workspace ON trigger_rules(workspace_id);
