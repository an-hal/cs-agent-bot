-- Migration: Create escalations table
-- Version: 20250330000004
-- Description: Creates the escalations table for tracking priority alerts to AE/BD team

CREATE TABLE escalations (
  id                    SERIAL       PRIMARY KEY,
  esc_id                VARCHAR(10)  NOT NULL,
  company_id            VARCHAR(20)  NOT NULL REFERENCES clients(company_id) ON DELETE CASCADE,
  trigger_condition     TEXT,
  priority              VARCHAR(20),
  notified_party        TEXT,
  telegram_message_sent TEXT,
  status                VARCHAR(20)  DEFAULT 'Open',
  triggered_at          TIMESTAMP    DEFAULT NOW(),
  resolved_at           TIMESTAMP,
  resolved_by           VARCHAR(100),
  notes                 TEXT,
  CONSTRAINT uq_open_escalation UNIQUE (esc_id, company_id, status)
    DEFERRABLE INITIALLY DEFERRED
);

-- Create indexes for common queries
CREATE INDEX idx_escalations_company_id ON escalations(company_id);
CREATE INDEX idx_escalations_status ON escalations(status);
CREATE INDEX idx_escalations_priority ON escalations(priority);
CREATE INDEX idx_escalations_esc_id ON escalations(esc_id);
CREATE INDEX idx_escalations_triggered_at ON escalations(triggered_at);
