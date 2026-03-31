-- Migration: Create cron_log table
-- Version: 20250330000006
-- Description: Creates the cron_log table for tracking daily cron job processing status

CREATE TABLE cron_log (
  id            SERIAL      PRIMARY KEY,
  run_date      DATE        NOT NULL,
  company_id    VARCHAR(20) NOT NULL REFERENCES clients(company_id) ON DELETE CASCADE,
  status        VARCHAR(20) DEFAULT 'pending',
  processed_at  TIMESTAMP,
  error_message TEXT,
  CONSTRAINT uq_cron_log_run UNIQUE (run_date, company_id)
);

-- Create indexes for efficient lookups
CREATE INDEX idx_cron_log_run_date ON cron_log(run_date);
CREATE INDEX idx_cron_log_status ON cron_log(status);
CREATE INDEX idx_cron_log_company_id ON cron_log(company_id);
CREATE INDEX idx_cron_log_processed_at ON cron_log(processed_at);
