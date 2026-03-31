-- Migration: Create client_flags table
-- Version: 20250330000003
-- Description: Creates the client_flags table for tracking message sent flags and automation state

CREATE TABLE client_flags (
  company_id                VARCHAR(20)  PRIMARY KEY REFERENCES clients(company_id) ON DELETE CASCADE,
  -- Renewal reminders
  ren60_sent                BOOLEAN DEFAULT FALSE,
  ren45_sent                BOOLEAN DEFAULT FALSE,
  ren30_sent                BOOLEAN DEFAULT FALSE,
  ren15_sent                BOOLEAN DEFAULT FALSE,
  ren0_sent                 BOOLEAN DEFAULT FALSE,
  -- Check-in Branch A
  checkin_a1_form_sent      BOOLEAN DEFAULT FALSE,
  checkin_a1_call_sent      BOOLEAN DEFAULT FALSE,
  checkin_a2_form_sent      BOOLEAN DEFAULT FALSE,
  checkin_a2_call_sent      BOOLEAN DEFAULT FALSE,
  -- Check-in Branch B
  checkin_b1_form_sent      BOOLEAN DEFAULT FALSE,
  checkin_b1_call_sent      BOOLEAN DEFAULT FALSE,
  checkin_b2_form_sent      BOOLEAN DEFAULT FALSE,
  checkin_b2_call_sent      BOOLEAN DEFAULT FALSE,
  -- Check-in state
  checkin_replied           BOOLEAN DEFAULT FALSE,
  -- NPS surveys
  nps1_sent                 BOOLEAN DEFAULT FALSE,
  nps2_sent                 BOOLEAN DEFAULT FALSE,
  nps3_sent                 BOOLEAN DEFAULT FALSE,
  nps_replied               BOOLEAN DEFAULT FALSE,
  -- Referral program
  referral_sent_this_cycle  BOOLEAN DEFAULT FALSE,
  -- Quotation tracking
  quotation_acknowledged    BOOLEAN DEFAULT FALSE,
  -- Health monitoring
  low_usage_msg_sent        BOOLEAN DEFAULT FALSE,
  low_nps_msg_sent          BOOLEAN DEFAULT FALSE,
  -- Cross-sell sequence (high touch)
  cs_h7                     BOOLEAN DEFAULT FALSE,
  cs_h14                    BOOLEAN DEFAULT FALSE,
  cs_h21                    BOOLEAN DEFAULT FALSE,
  cs_h30                    BOOLEAN DEFAULT FALSE,
  cs_h45                    BOOLEAN DEFAULT FALSE,
  cs_h60                    BOOLEAN DEFAULT FALSE,
  cs_h75                    BOOLEAN DEFAULT FALSE,
  cs_h90                    BOOLEAN DEFAULT FALSE,
  -- Cross-sell sequence (low touch)
  cs_lt1                    BOOLEAN DEFAULT FALSE,
  cs_lt2                    BOOLEAN DEFAULT FALSE,
  cs_lt3                    BOOLEAN DEFAULT FALSE
);

-- Create index for lookups
CREATE INDEX idx_client_flags_company_id ON client_flags(company_id);
