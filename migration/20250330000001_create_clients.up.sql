-- Migration: Create clients table
-- Version: 20250330000001
-- Description: Creates the main clients table for storing HRIS SaaS client information

CREATE TABLE IF NOT EXISTS schema_migrations (
    version VARCHAR(14) PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE clients (
  company_id               VARCHAR(20)    PRIMARY KEY,
  company_name             VARCHAR(200)   NOT NULL,
  pic_name                 VARCHAR(100)   NOT NULL,
  pic_wa                   VARCHAR(20)    NOT NULL UNIQUE,
  pic_email                VARCHAR(100),
  pic_role                 VARCHAR(100),
  hc_size                  VARCHAR(20),
  owner_name               VARCHAR(100)   NOT NULL,
  owner_wa                 VARCHAR(20),
  owner_telegram_id        VARCHAR(30)    NOT NULL,
  backup_owner_telegram_id VARCHAR(30),
  ae_assigned              BOOLEAN        DEFAULT FALSE,
  ae_telegram_id           VARCHAR(30),
  segment                  VARCHAR(10),
  plan_type                VARCHAR(50),
  payment_terms            VARCHAR(20),
  contract_start           DATE           NOT NULL,
  contract_end             DATE           NOT NULL,
  contract_months          SMALLINT       NOT NULL,
  activation_date          DATE           NOT NULL,
  first_time_discount_pct  DECIMAL(5,2),
  next_discount_pct_manual DECIMAL(5,2),
  final_price              DECIMAL(12,2),
  quotation_link           VARCHAR(500),
  quotation_link_expires   DATE,
  renewal_date             DATE,
  bd_prospect_id           VARCHAR(50),
  notes                    TEXT,
  payment_status           VARCHAR(20)    DEFAULT 'Paid',
  last_payment_date        DATE,
  nps_score                SMALLINT,
  usage_score              SMALLINT,
  usage_score_avg_30d      SMALLINT,
  last_interaction_date    DATE,
  risk_flag                BOOLEAN        DEFAULT FALSE,
  bot_active               BOOLEAN        DEFAULT TRUE,
  blacklisted              BOOLEAN        DEFAULT FALSE,
  wa_undeliverable         BOOLEAN        DEFAULT FALSE,
  response_status          VARCHAR(20)    DEFAULT 'Pending',
  renewed                  BOOLEAN        DEFAULT FALSE,
  rejected                 BOOLEAN        DEFAULT FALSE,
  churn_reason             VARCHAR(200),
  sequence_cs              VARCHAR(20)    DEFAULT 'ACTIVE',
  cross_sell_rejected      BOOLEAN        DEFAULT FALSE,
  cross_sell_interested    BOOLEAN        DEFAULT FALSE,
  cross_sell_resume_date   DATE,
  days_since_cs_last_sent  SMALLINT       DEFAULT 0,
  feature_update_sent      BOOLEAN        DEFAULT FALSE,
  created_at               TIMESTAMP      DEFAULT NOW(),
  CONSTRAINT chk_renewal_date CHECK (renewed = FALSE OR renewal_date IS NOT NULL)
);

-- Create indexes for common queries
CREATE INDEX idx_clients_pic_wa ON clients(pic_wa);
CREATE INDEX idx_clients_owner_telegram_id ON clients(owner_telegram_id);
CREATE INDEX idx_clients_bot_active ON clients(bot_active);
CREATE INDEX idx_clients_segment ON clients(segment);
CREATE INDEX idx_clients_contract_end ON clients(contract_end);
CREATE INDEX idx_clients_renewal_date ON clients(renewal_date);
CREATE INDEX idx_clients_blacklisted ON clients(blacklisted);
