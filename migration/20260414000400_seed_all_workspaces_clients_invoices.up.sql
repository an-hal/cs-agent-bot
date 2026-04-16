-- Migration: Seed clients and invoices for every non-holding workspace
-- Version: 20260414000400
-- Description: Creates 5 scenario clients per workspace (covering P0-P3 trigger phases)
--              and matching invoices for renewal/invoice/overdue scenarios.
--              Idempotent — safe to re-run via ON CONFLICT DO NOTHING.
--              All dates are CURRENT_DATE relative so trigger zones stay consistent.

-- ============================================================
-- CLIENTS — one batch per scenario, cross-joined with every workspace
-- ============================================================
-- Scenarios:
--   01 Healthy Paid      (P0 baseline, NPS 9, Usage 85, Paid)
--   02 Low NPS Risk      (P0 health risk -> ESC-003, NPS 3, Paid)
--   03 Renewal H-45      (P1 renewal zone, contract_end = today+45)
--   04 Invoice PRE7      (P2 pre-due reminder, contract_end = today+7)
--   05 Overdue POST4     (P3 overdue recovery, contract_end = today-4)

WITH ws AS (
  SELECT id, slug, ROW_NUMBER() OVER (ORDER BY slug) AS ws_idx
  FROM workspaces
  WHERE is_holding = FALSE
),
scenarios(n, label, pay_status, nps, usage, c_start_off, c_end_off) AS (
  VALUES
    (1, 'Healthy Paid',  'Paid',    9::SMALLINT, 85::SMALLINT, -90,  275),
    (2, 'Low NPS Risk',  'Paid',    3::SMALLINT, 70::SMALLINT, -120, 245),
    (3, 'Renewal H-45',  'Paid',    7::SMALLINT, 75::SMALLINT, -320, 45),
    (4, 'Invoice PRE7',  'Pending', 7::SMALLINT, 60::SMALLINT, -358, 7),
    (5, 'Overdue POST4', 'Overdue', 5::SMALLINT, 50::SMALLINT, -369, -4)
)
INSERT INTO clients (
  company_id, company_name, pic_name, pic_wa, pic_email, pic_role, hc_size,
  owner_name, owner_wa, owner_telegram_id,
  segment, plan_type, payment_terms,
  contract_start, contract_end, contract_months, activation_date,
  payment_status, nps_score, usage_score,
  bot_active, blacklisted,
  quotation_link,
  response_status, sequence_cs,
  renewed, rejected, renewal_date,
  cross_sell_rejected, cross_sell_interested, checkin_replied,
  workspace_id
)
SELECT
  'SEED-' || UPPER(LEFT(ws.slug, 10)) || '-' || LPAD(s.n::text, 2, '0'),
  'Seed ' || INITCAP(ws.slug) || ' ' || s.label,
  'PIC ' || INITCAP(ws.slug) || ' ' || s.n,
  '+6289' || LPAD((ws.ws_idx * 100 + s.n)::text, 9, '0'),
  'pic' || s.n || '@seed-' || ws.slug || '.test',
  'HR Manager',
  '50-100',
  'Owner ' || INITCAP(ws.slug) || ' ' || s.n,
  '+6288' || LPAD((ws.ws_idx * 100 + s.n)::text, 9, '0'),
  '9' || LPAD((ws.ws_idx * 100 + s.n)::text, 8, '0'),
  CASE WHEN s.n IN (1, 3) THEN 'High' WHEN s.n IN (2, 4) THEN 'Mid' ELSE 'Low' END,
  CASE WHEN s.n = 1 THEN 'Enterprise' WHEN s.n IN (2, 3, 4) THEN 'Pro' ELSE 'Basic' END,
  'Net 30',
  CURRENT_DATE + s.c_start_off,
  CURRENT_DATE + s.c_end_off,
  12::SMALLINT,
  CURRENT_DATE + s.c_start_off + 14,
  s.pay_status, s.nps, s.usage,
  TRUE, FALSE,
  'https://quote.example.com/seed-' || ws.slug || '-' || s.n,
  'Pending', 'ACTIVE',
  FALSE, FALSE, NULL,
  FALSE, FALSE, FALSE,
  ws.id
FROM ws
CROSS JOIN scenarios s
ON CONFLICT (company_id) DO NOTHING;

-- ============================================================
-- INVOICES — only for scenarios 03, 04, 05
-- ============================================================
INSERT INTO invoices (
  invoice_id, company_id, issue_date, due_date, amount,
  payment_status, collection_stage, workspace_id
)
SELECT
  'INV-' || c.company_id,
  c.company_id,
  CASE
    WHEN c.company_id LIKE 'SEED-%-03' THEN c.contract_end - 30
    WHEN c.company_id LIKE 'SEED-%-04' THEN c.contract_end - 14
    WHEN c.company_id LIKE 'SEED-%-05' THEN c.contract_end - 14
  END,
  c.contract_end,
  5000000.00,
  CASE
    WHEN c.company_id LIKE 'SEED-%-05' THEN 'Overdue'
    ELSE 'Pending'
  END,
  CASE
    WHEN c.company_id LIKE 'SEED-%-05' THEN 'Stage 2 — Post-due'
    ELSE 'Stage 0 — Pre-due'
  END,
  c.workspace_id
FROM clients c
WHERE c.company_id LIKE 'SEED-%'
  AND (c.company_id LIKE '%-03' OR c.company_id LIKE '%-04' OR c.company_id LIKE '%-05')
ON CONFLICT (invoice_id) DO NOTHING;
