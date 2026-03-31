-- Migration: Create invoices table
-- Version: 20250330000002
-- Description: Creates the invoices table for tracking client billing and payment reminders

CREATE TABLE invoices (
  invoice_id       VARCHAR(30)    PRIMARY KEY,
  company_id       VARCHAR(20)    NOT NULL REFERENCES clients(company_id) ON DELETE CASCADE,
  issue_date       DATE           NOT NULL,
  due_date         DATE           NOT NULL,
  amount           DECIMAL(12,2)  NOT NULL,
  payment_status   VARCHAR(20)    DEFAULT 'Pending',
  paid_at          TIMESTAMP,
  amount_paid      DECIMAL(12,2),
  reminder_count   SMALLINT       DEFAULT 0,
  collection_stage VARCHAR(30)    DEFAULT 'Stage 0 — Pre-due',
  pre14_sent       BOOLEAN        DEFAULT FALSE,
  pre7_sent        BOOLEAN        DEFAULT FALSE,
  pre3_sent        BOOLEAN        DEFAULT FALSE,
  post1_sent       BOOLEAN        DEFAULT FALSE,
  post4_sent       BOOLEAN        DEFAULT FALSE,
  post8_sent       BOOLEAN        DEFAULT FALSE,
  created_at       TIMESTAMP      DEFAULT NOW()
);

-- Create indexes for common queries
CREATE INDEX idx_invoices_company_id ON invoices(company_id);
CREATE INDEX idx_invoices_payment_status ON invoices(payment_status);
CREATE INDEX idx_invoices_due_date ON invoices(due_date);
CREATE INDEX idx_invoices_collection_stage ON invoices(collection_stage);
CREATE INDEX idx_invoices_issue_date ON invoices(issue_date);
