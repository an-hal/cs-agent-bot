-- Migration: Seed Test Clients for Testing All Trigger Cases
-- Version: 20250401000006
-- Description: Seeds 10 test clients covering every trigger phase (P0-P5)
--   TC01: Health Risk — low usage
--   TC02: Health Risk — low NPS + escalation ESC-003
--   TC03: Check-in Branch A (contract_months=12, H-120 zone)
--   TC04: Check-in Branch B (contract_months=6, H-58 ≈ B2 form zone)
--   TC05: Renewal Negotiation — REN60 zone
--   TC06: Invoice Payment — PRE14/PRE7/PRE3 zone
--   TC07: Overdue POST1/POST4 zone
--   TC08: Overdue POST15 + ESC-001 escalation
--   TC09: Expansion — NPS survey + cross-sell (90-day active)
--   TC10: Cross-sell long-term rotation
-- All dates anchored to TODAY = 2026-04-01 for reproducibility.

-- ============================================================
-- CLIENTS
-- ============================================================
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
  cross_sell_rejected, cross_sell_interested,
  checkin_replied
) VALUES

-- TC01: Health Risk — low usage (Usage 25, NPS 7, Paid, DTE > 60)
('TC01', 'PT Alpha Sehat', 'Andi Wijaya', '+6281000000001', 'andi@alpha.co.id', 'HR Manager', '50-100',
 'Siti Rahayu', '+6282000000001', '100000001',
 'Mid', 'Pro', 'Net 30',
 '2025-06-01', '2027-06-01', 12, '2025-07-01',
 'Paid', 7, 25,
 TRUE, FALSE,
 'https://quote.example.com/tc01',
 'Pending', 'ACTIVE',
 FALSE, FALSE, NULL, FALSE, FALSE, FALSE),

-- TC02: Health Risk — low NPS (NPS 3, Usage 60, Paid, triggers ESC-003)
('TC02', 'PT Beta Jaya', 'Budi Santoso', '+6281000000002', 'budi@beta.co.id', 'CEO', '10-50',
 'Rina Wati', '+6282000000002', '100000002',
 'Mid', 'Basic', 'Net 30',
 '2025-08-01', '2026-08-01', 12, '2025-09-01',
 'Paid', 3, 60,
 TRUE, FALSE,
 'https://quote.example.com/tc02',
 'Pending', 'ACTIVE',
 FALSE, FALSE, NULL, FALSE, FALSE, FALSE),

-- TC03: Check-in Branch A — contract_months=12, DTE ~118 (H-120 zone)
('TC03', 'PT Gamma Maju', 'Citra Dewi', '+6281000000003', 'citra@gamma.co.id', 'HR Director', '100-250',
 'Hendra K.', '+6282000000003', '100000003',
 'High', 'Enterprise', 'Net 30',
 '2025-07-29', '2026-07-27', 12, '2025-08-01',
 'Paid', 8, 72,
 TRUE, FALSE,
 'https://quote.example.com/tc03',
 'Pending', 'ACTIVE',
 FALSE, FALSE, NULL, FALSE, FALSE, FALSE),

-- TC04: Check-in Branch B — contract_months=6, DTE ~58 (H-60 zone, Branch B step 3)
('TC04', 'CV Delta Kencana', 'Dimas Pratama', '+6281000000004', 'dimas@delta.co.id', 'Owner', '10-50',
 'Lina Susanti', '+6282000000004', '100000004',
 'Low', 'Basic', 'Net 7',
 '2025-10-01', '2026-05-29', 6, '2025-10-15',
 'Paid', 6, 55,
 TRUE, FALSE,
 'https://quote.example.com/tc04',
 'Pending', 'ACTIVE',
 FALSE, FALSE, NULL, FALSE, FALSE, FALSE),

-- TC05: Renewal Negotiation — DTE ~62 (REN60 zone, Branch A)
('TC05', 'PT Epsilon Tech', 'Eka Putri', '+6281000000005', 'eka@epsilon.co.id', 'HR Manager', '50-100',
 'Fajar Nugroho', '+6282000000005', '100000005',
 'Mid', 'Pro', 'Net 30',
 '2025-06-03', '2026-06-02', 12, '2025-07-01',
 'Paid', 8, 80,
 TRUE, FALSE,
 'https://quote.example.com/tc05',
 'Pending', 'ACTIVE',
 FALSE, FALSE, NULL, FALSE, FALSE, FALSE),

-- TC06: Invoice Payment — DTE ~10 (PRE14/PRE7/PRE3 zone)
('TC06', 'PT Zeta Mandiri', 'Fiona Lim', '+6281000000006', 'fiona@zeta.co.id', 'Finance Manager', '50-100',
 'Giovanni T.', '+6282000000006', '100000006',
 'Mid', 'Pro', 'Net 30',
 '2025-04-11', '2026-04-11', 12, '2025-05-01',
 'Pending', 7, 65,
 TRUE, FALSE,
 'https://quote.example.com/tc06',
 'Pending', 'ACTIVE',
 FALSE, FALSE, NULL, FALSE, FALSE, FALSE),

-- TC07: Overdue — DTE -5 (5 days past due, POST1/POST4 zone)
('TC07', 'PT Eta Prima', 'Gunawan Hadi', '+6281000000007', 'gunawan@eta.co.id', 'COO', '10-50',
 'Helen S.', '+6282000000007', '100000007',
 'Low', 'Basic', 'Net 7',
 '2025-04-06', '2026-03-27', 12, '2025-05-01',
 'Overdue', 4, 40,
 TRUE, FALSE,
 'https://quote.example.com/tc07',
 'Pending', 'ACTIVE',
 FALSE, FALSE, NULL, FALSE, FALSE, FALSE),

-- TC08: Overdue POST15 — DTE -20 (20 days past due, triggers ESC-001)
('TC08', 'PT Theta Global', 'Irwan Setiawan', '+6281000000008', 'irwan@theta.co.id', 'Director', '100-250',
 'Julia Hartono', '+6282000000008', '100000008',
 'High', 'Enterprise', 'Net 30',
 '2025-03-12', '2026-03-12', 12, '2025-04-01',
 'Overdue', 2, 30,
 TRUE, FALSE,
 'https://quote.example.com/tc08',
 'Pending', 'ACTIVE',
 FALSE, FALSE, NULL, FALSE, FALSE, FALSE),

-- TC09: Expansion — NPS survey + Cross-sell (90-day active, DSA ~45)
('TC09', 'PT Iota Inovasi', 'Joko Susanto', '+6281000000009', 'joko@iota.co.id', 'CTO', '50-100',
 'Kartika W.', '+6282000000009', '100000009',
 'Mid', 'Pro', 'Net 30',
 '2026-01-01', '2027-01-01', 12, '2026-02-15',
 'Paid', 9, 85,
 TRUE, FALSE,
 'https://quote.example.com/tc09',
 'Pending', 'ACTIVE',
 FALSE, FALSE, NULL, FALSE, FALSE, FALSE),

-- TC10: Cross-sell long-term (DSA ~100, already completed 90-day sequence)
('TC10', 'PT Kappa Digital', 'Lina Marlina', '+6281000000010', 'lina@kappa.co.id', 'VP HR', '250-500',
 'Michael T.', '+6282000000010', '100000010',
 'High', 'Enterprise', 'Net 30',
 '2025-06-01', '2027-06-01', 24, '2025-07-01',
 'Paid', 8, 90,
 TRUE, FALSE,
 'https://quote.example.com/tc10',
 'Pending', 'LONGTERM',
 FALSE, FALSE, NULL, FALSE, FALSE, FALSE),

-- TC11: Blacklisted — should be skipped entirely by GetAll
('TC11', 'PT Lambda Hitam', 'Mira Sari', '+6281000000011', 'mira@lambda.co.id', 'HR', '10-50',
 'Nina K.', '+6282000000011', '100000011',
 'Low', 'Basic', 'Net 7',
 '2025-01-01', '2026-01-01', 12, '2025-02-01',
 'Overdue', 1, 10,
 TRUE, TRUE,
 NULL,
 'Pending', 'ACTIVE',
 FALSE, FALSE, NULL, FALSE, FALSE, FALSE),

-- TC12: Bot paused — should be skipped by GetAll (bot_active=false)
('TC12', 'PT Muara Dorman', 'Oscar Pratama', '+6281000000012', 'oscar@muara.co.id', 'Admin', '10-50',
 'Prita Dewi', '+6282000000012', '100000012',
 'Mid', 'Pro', 'Net 30',
 '2025-09-01', '2026-09-01', 12, '2025-10-01',
 'Paid', 7, 70,
 FALSE, FALSE,
 'https://quote.example.com/tc12',
 'Pending', 'ACTIVE',
 FALSE, FALSE, NULL, FALSE, FALSE, FALSE),

-- TC13: Already renewed — cycle should reset
('TC13', 'PT Nusantara Baru', 'Putri Ayu', '+6281000000013', 'putri@nusantara.co.id', 'HR Lead', '50-100',
 'Rudi Hermawan', '+6282000000013', '100000013',
 'Mid', 'Pro', 'Net 30',
 '2025-01-01', '2026-01-01', 12, '2025-02-01',
 'Paid', 9, 88,
 TRUE, FALSE,
 'https://quote.example.com/tc13',
 'Pending', 'ACTIVE',
 TRUE, FALSE, '2026-01-15', FALSE, FALSE, FALSE),

-- TC14: Rejected renewal
('TC14', 'PT Omega Churn', 'Qori Ismail', '+6281000000014', 'qori@omega.co.id', 'Manager', '10-50',
 'Sandra P.', '+6282000000014', '100000014',
 'Low', 'Basic', 'Net 7',
 '2025-04-01', '2026-04-01', 12, '2025-05-01',
 'Overdue', 2, 15,
 TRUE, FALSE,
 NULL,
 'Pending', 'ACTIVE',
 FALSE, TRUE, NULL, FALSE, FALSE, FALSE),

-- TC15: Cross-sell rejected
('TC15', 'PT Pi Data', 'Raka Aditya', '+6281000000015', 'raka@pi.co.id', 'HR', '10-50',
 'Tina Mulyadi', '+6282000000015', '100000015',
 'Low', 'Basic', 'Net 7',
 '2026-01-01', '2027-01-01', 12, '2026-02-01',
 'Paid', 6, 50,
 TRUE, FALSE,
 'https://quote.example.com/tc15',
 'Pending', 'REJECTED',
 FALSE, FALSE, NULL, TRUE, FALSE, FALSE);


-- ============================================================
-- CLIENT FLAGS
-- ============================================================
INSERT INTO client_flags (
  company_id,
  ren60_sent, ren45_sent, ren30_sent, ren15_sent, ren0_sent,
  checkin_a1_form_sent, checkin_a1_call_sent, checkin_a2_form_sent, checkin_a2_call_sent,
  checkin_b1_form_sent, checkin_b1_call_sent, checkin_b2_form_sent, checkin_b2_call_sent,
  checkin_replied,
  nps1_sent, nps2_sent, nps3_sent, nps_replied,
  referral_sent_this_cycle,
  quotation_acknowledged,
  low_usage_msg_sent, low_nps_msg_sent,
  cs_h7, cs_h14, cs_h21, cs_h30, cs_h45, cs_h60, cs_h75, cs_h90,
  cs_lt1, cs_lt2, cs_lt3
) VALUES

-- TC01: No flags sent yet — health triggers should fire
('TC01', FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE, FALSE,FALSE, FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE),

-- TC02: No flags sent yet — low NPS trigger should fire
('TC02', FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE, FALSE,FALSE, FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE),

-- TC03: No check-in flags sent — A1 form should fire at DTE 118
('TC03', FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE, FALSE,FALSE, FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE),

-- TC04: B1 form & call sent, B2 form NOT sent — B2 form should fire at DTE 58
('TC04', FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, TRUE,TRUE,FALSE,FALSE, FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE, FALSE,FALSE, FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE),

-- TC05: No renewal flags — REN60 should fire at DTE 62
('TC05', FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE, FALSE,FALSE, FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE),

-- TC06: REN30 sent, no payment flags — PRE14 should fire at DTE 10
('TC06', FALSE,FALSE,TRUE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE, FALSE,FALSE, FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE),

-- TC07: REN0 sent, POST1 sent — POST4 should fire at overdue D+5
('TC07', TRUE,TRUE,TRUE,TRUE,TRUE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE, FALSE,FALSE, FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE),

-- TC08: All renewal sent, POST1/POST4/POST8 sent — POST15 should fire at overdue D+20
('TC08', TRUE,TRUE,TRUE,TRUE,TRUE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE, FALSE,FALSE, FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE),

-- TC09: NPS1 sent, NPS2 NOT sent — NPS2 should fire (DSA ~45). No CS flags sent yet.
('TC09', FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE, TRUE,FALSE,FALSE,FALSE, FALSE,FALSE, FALSE,FALSE, FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE),

-- TC10: All 90-day CS flags sent, CS_H90=TRUE — should trigger LT1 in long-term rotation
('TC10', FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE, FALSE,FALSE, TRUE,TRUE,TRUE,TRUE,TRUE,TRUE,TRUE,TRUE, FALSE,FALSE,FALSE),

-- TC11-TC15: Minimal flags (testing skip conditions)
('TC11', FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE, FALSE,FALSE, FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE),
('TC12', FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE, FALSE,FALSE, FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE),
('TC13', TRUE,TRUE,TRUE,TRUE,TRUE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, TRUE, TRUE,TRUE,TRUE,TRUE, FALSE,FALSE, FALSE,FALSE, TRUE,TRUE,TRUE,TRUE,TRUE,TRUE,TRUE,TRUE, FALSE,FALSE,FALSE),
('TC14', TRUE,TRUE,TRUE,TRUE,TRUE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE, FALSE,FALSE, FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE),
('TC15', FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE, TRUE,FALSE,FALSE,TRUE, FALSE,FALSE, FALSE,FALSE, TRUE,TRUE,TRUE,TRUE,TRUE,TRUE,TRUE,TRUE, FALSE,FALSE,FALSE);


-- ============================================================
-- CONVERSATION STATES
-- ============================================================
INSERT INTO conversation_states (
  company_id, company_name,
  active_flow, current_stage, last_message_type, last_message_date,
  response_status, response_classification,
  attempt_count, cooldown_until,
  bot_active, reason_bot_paused,
  next_scheduled_action, next_scheduled_date,
  human_owner_notified
) VALUES

-- TC01: Idle — no active flow, health should trigger
('TC01', 'PT Alpha Sehat', NULL, NULL, NULL, NULL, 'Pending', NULL, 0, NULL, TRUE, NULL, NULL, NULL, FALSE),

-- TC02: Idle — no active flow, low NPS should trigger
('TC02', 'PT Beta Jaya', NULL, NULL, NULL, NULL, 'Pending', NULL, 0, NULL, TRUE, NULL, NULL, NULL, FALSE),

-- TC03: Check-in flow starting
('TC03', 'PT Gamma Maju', 'CHECKIN', 'CheckIn_A1_Form', NULL, NULL, 'Pending', NULL, 0, NULL, TRUE, NULL, NULL, NULL, FALSE),

-- TC04: Check-in B1 done, awaiting B2
('TC04', 'CV Delta Kencana', 'CHECKIN', 'CheckIn_B1_Call', 'TPL-CHECKIN-CALL', '2026-03-18 09:00:00', 'Pending', NULL, 1, NULL, TRUE, NULL, 'CheckIn_B2_Form', NULL, FALSE),

-- TC05: Renewal flow starting
('TC05', 'PT Epsilon Tech', 'RENEWAL', 'Renewal_REN60', NULL, NULL, 'Pending', NULL, 0, NULL, TRUE, NULL, NULL, NULL, FALSE),

-- TC06: Invoice payment flow active
('TC06', 'PT Zeta Mandiri', 'INVOICE', 'PAY_PRE14', 'REN30', '2026-03-12 09:00:00', 'Pending', NULL, 1, NULL, TRUE, NULL, NULL, NULL, FALSE),

-- TC07: Overdue — POST1 sent, POST4 should fire
('TC07', 'PT Eta Prima', 'OVERDUE', 'Overdue_POST1', 'TPL-PAY-POST1', '2026-03-28 09:00:00', 'Pending', NULL, 2, NULL, TRUE, NULL, NULL, NULL, FALSE),

-- TC08: Overdue — POST8 sent, POST15 should fire + ESC-001
('TC08', 'PT Theta Global', 'OVERDUE', 'Overdue_POST8', 'TPL-PAY-POST8', '2026-03-19 09:00:00', 'Pending', 'Objection — price', 4, NULL, TRUE, NULL, NULL, NULL, FALSE),

-- TC09: Expansion — NPS1 done, NPS2 next
('TC09', 'PT Iota Inovasi', 'EXPANSION', 'NPS_Survey_1', 'NPS1', '2026-03-17 09:00:00', 'Pending', NULL, 1, NULL, TRUE, NULL, 'NPS_Survey_2', NULL, FALSE),

-- TC10: Cross-sell long-term
('TC10', 'PT Kappa Digital', 'CROSS_SELL', 'CS_H90', 'CS_H90', '2026-02-20 09:00:00', 'Pending', NULL, 8, NULL, TRUE, NULL, 'CS_LT1', NULL, FALSE),

-- TC11: Blacklisted — bot paused with reason
('TC11', 'PT Lambda Hitam', NULL, NULL, NULL, NULL, 'Pending', NULL, 0, NULL, FALSE, 'Blacklisted', NULL, NULL, FALSE),

-- TC12: Bot paused by escalation
('TC12', 'PT Muara Dorman', NULL, NULL, NULL, NULL, 'Pending', NULL, 0, NULL, FALSE, 'Escalated to human', NULL, NULL, TRUE),

-- TC13: Renewed — clean state
('TC13', 'PT Nusantara Baru', NULL, NULL, NULL, NULL, 'Pending', NULL, 0, NULL, TRUE, NULL, NULL, NULL, FALSE),

-- TC14: Rejected — cycle ended
('TC14', 'PT Omega Churn', 'RENEWAL', 'Renewal_REN0', 'REN0', '2026-03-25 09:00:00', 'Replied', 'Reject — not interested', 3, NULL, FALSE, 'Client rejected renewal', NULL, NULL, TRUE),

-- TC15: Cross-sell rejected
('TC15', 'PT Pi Data', 'CROSS_SELL', 'CS_H45', 'CS_H45', '2026-03-10 09:00:00', 'Replied', 'Reject — not interested', 5, NULL, TRUE, NULL, NULL, NULL, FALSE);


-- ============================================================
-- INVOICES
-- ============================================================
INSERT INTO invoices (
  invoice_id, company_id, issue_date, due_date, amount, payment_status, paid_at, amount_paid, reminder_count, collection_stage,
  pre14_sent, pre7_sent, pre3_sent, post1_sent, post4_sent, post8_sent
) VALUES

-- TC06: Active invoice, due in ~10 days — PRE14 should fire
('INV-TC06-001', 'TC06', '2026-03-11', '2026-04-11', 5500000.00, 'Pending', NULL, NULL, 0, 'Stage 0 — Pre-due', FALSE,FALSE,FALSE, FALSE,FALSE,FALSE),

-- TC07: Overdue by ~5 days — POST4 should fire
('INV-TC07-001', 'TC07', '2026-02-25', '2026-03-27', 2200000.00, 'Overdue', NULL, NULL, 2, 'Stage 2 — Firm', FALSE,FALSE,FALSE, TRUE,FALSE,FALSE),

-- TC08: Overdue by ~20 days — POST15 + ESC-001 should fire
('INV-TC08-001', 'TC08', '2026-02-10', '2026-03-12', 18000000.00, 'Overdue', NULL, NULL, 4, 'Stage 3 — Urgency', FALSE,FALSE,FALSE, TRUE,TRUE,TRUE),

-- TC01: Paid invoice (historical, for reference)
('INV-TC01-001', 'TC01', '2025-11-01', '2025-12-01', 5500000.00, 'Paid', '2025-11-28 10:30:00', 5500000.00, 0, 'Closed', FALSE,FALSE,FALSE, FALSE,FALSE,FALSE),

-- TC02: Paid invoice (historical)
('INV-TC02-001', 'TC02', '2025-11-01', '2025-12-01', 3200000.00, 'Paid', '2025-12-01 14:00:00', 3200000.00, 0, 'Closed', FALSE,FALSE,FALSE, FALSE,FALSE,FALSE),

-- TC14: Overdue invoice for rejected client
('INV-TC14-001', 'TC14', '2026-02-25', '2026-03-25', 1800000.00, 'Overdue', NULL, NULL, 3, 'Stage 3 — Urgency', FALSE,FALSE,FALSE, TRUE,TRUE,FALSE);
