-- Migration: Seed workspace-scoped data
-- Version: 20260408000008
-- Description: Seeds clients, invoices, escalations, client_flags, and conversation_states
--              for both 'dealls' and 'kantorku' workspaces.

-- ============================================================
-- CLIENTS — Dealls workspace (8 clients)
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
  checkin_replied,
  workspace_id
) VALUES
-- DL01: High segment, Enterprise, Paid
('DL01', 'PT Maju Bersama', 'Andi Prasetyo', '+6281200000001', 'andi@majubersama.co.id', 'HR Director', '250-500',
 'Bambang Sutrisno', '+6282200000001', '200000001',
 'High', 'Enterprise', 'Net 30',
 '2025-10-01', '2026-10-01', 12, '2025-10-15',
 'Paid', 9, 88,
 TRUE, FALSE,
 'https://quote.example.com/dl01',
 'Pending', 'ACTIVE',
 FALSE, FALSE, NULL, FALSE, FALSE, FALSE,
 (SELECT id FROM workspaces WHERE slug = 'dealls')),

-- DL02: Mid segment, Pro, Pending payment
('DL02', 'CV Teknologi Nusantara', 'Citra Maharani', '+6281200000002', 'citra@teknusantara.co.id', 'Finance Manager', '50-100',
 'Dewi Anggraini', '+6282200000002', '200000002',
 'Mid', 'Pro', 'Net 30',
 '2025-08-01', '2026-08-01', 12, '2025-08-15',
 'Pending', 7, 65,
 TRUE, FALSE,
 'https://quote.example.com/dl02',
 'Pending', 'ACTIVE',
 FALSE, FALSE, NULL, FALSE, FALSE, FALSE,
 (SELECT id FROM workspaces WHERE slug = 'dealls')),

-- DL03: Low segment, Basic, Overdue
('DL03', 'PT Sinar Abadi', 'Eko Purnomo', '+6281200000003', 'eko@sinarabadi.co.id', 'Owner', '10-50',
 'Fitria Handayani', '+6282200000003', '200000003',
 'Low', 'Basic', 'Net 7',
 '2025-06-01', '2026-06-01', 12, '2025-06-15',
 'Overdue', 4, 35,
 TRUE, FALSE,
 NULL,
 'Pending', 'ACTIVE',
 FALSE, FALSE, NULL, FALSE, FALSE, FALSE,
 (SELECT id FROM workspaces WHERE slug = 'dealls')),

-- DL04: High segment, Enterprise, Paid, renewed
('DL04', 'PT Global Sejahtera', 'Gita Permata', '+6281200000004', 'gita@globalsejahtera.co.id', 'VP People', '500+',
 'Hadi Santoso', '+6282200000004', '200000004',
 'High', 'Enterprise', 'Net 30',
 '2025-01-01', '2026-01-01', 12, '2025-01-15',
 'Paid', 9, 92,
 TRUE, FALSE,
 'https://quote.example.com/dl04',
 'Replied', 'ACTIVE',
 TRUE, FALSE, '2026-01-10', FALSE, TRUE, TRUE,
 (SELECT id FROM workspaces WHERE slug = 'dealls')),

-- DL05: Mid segment, Pro, Paid, long-term cross-sell
('DL05', 'PT Karya Digital', 'Indah Lestari', '+6281200000005', 'indah@karyadigital.co.id', 'HR Manager', '100-250',
 'Joko Widodo', '+6282200000005', '200000005',
 'Mid', 'Pro', 'Net 30',
 '2025-04-01', '2027-04-01', 24, '2025-04-15',
 'Paid', 8, 78,
 TRUE, FALSE,
 'https://quote.example.com/dl05',
 'Pending', 'LONGTERM',
 FALSE, FALSE, NULL, FALSE, FALSE, FALSE,
 (SELECT id FROM workspaces WHERE slug = 'dealls')),

-- DL06: Low segment, Basic, Overdue, rejected
('DL06', 'CV Mandiri Utama', 'Kevin Saputra', '+6281200000006', 'kevin@mandiriutama.co.id', 'Admin', '10-50',
 'Lina Marlina', '+6282200000006', '200000006',
 'Low', 'Basic', 'Net 7',
 '2025-05-01', '2026-05-01', 12, '2025-05-15',
 'Overdue', 2, 18,
 TRUE, FALSE,
 NULL,
 'Replied', 'REJECTED',
 FALSE, TRUE, NULL, FALSE, FALSE, FALSE,
 (SELECT id FROM workspaces WHERE slug = 'dealls')),

-- DL07: Mid segment, Pro, Paid, snoozed
('DL07', 'PT Nusantara Kreatif', 'Maya Putri', '+6281200000007', 'maya@nusantarakreatif.co.id', 'HR Lead', '50-100',
 'Nugroho Adi', '+6282200000007', '200000007',
 'Mid', 'Pro', 'Net 30',
 '2025-09-01', '2026-09-01', 12, '2025-09-15',
 'Paid', 6, 55,
 TRUE, FALSE,
 'https://quote.example.com/dl07',
 'Pending', 'SNOOZED',
 FALSE, FALSE, NULL, TRUE, FALSE, FALSE,
 (SELECT id FROM workspaces WHERE slug = 'dealls')),

-- DL08: High segment, Enterprise, Partial payment
('DL08', 'PT Optima Solusi', 'Putri Rahayu', '+6281200000008', 'putri@optimasolusi.co.id', 'CFO', '250-500',
 'Rahmat Hidayat', '+6282200000008', '200000008',
 'High', 'Enterprise', 'Net 30',
 '2025-07-01', '2026-07-01', 12, '2025-07-15',
 'Partial', 7, 70,
 TRUE, FALSE,
 'https://quote.example.com/dl08',
 'Replied', 'ACTIVE',
 FALSE, FALSE, NULL, FALSE, FALSE, FALSE,
 (SELECT id FROM workspaces WHERE slug = 'dealls'));

-- ============================================================
-- CLIENTS — KantorKu workspace (6 clients)
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
  checkin_replied,
  workspace_id
) VALUES
-- KK01: High segment, Enterprise, Paid
('KK01', 'PT Sakura Indonesia', 'Sari Dewi', '+6281300000001', 'sari@sakuraid.co.id', 'CHRO', '500+',
 'Tono Prasetya', '+6282300000001', '300000001',
 'High', 'Enterprise', 'Net 30',
 '2025-11-01', '2026-11-01', 12, '2025-11-15',
 'Paid', 10, 95,
 TRUE, FALSE,
 'https://quote.example.com/kk01',
 'Replied', 'ACTIVE',
 FALSE, FALSE, NULL, FALSE, TRUE, TRUE,
 (SELECT id FROM workspaces WHERE slug = 'kantorku')),

-- KK02: Mid segment, Pro, Pending
('KK02', 'CV Usaha Muda', 'Umar Faruq', '+6281300000002', 'umar@usahamuda.co.id', 'HR Manager', '50-100',
 'Vina Kusuma', '+6282300000002', '300000002',
 'Mid', 'Pro', 'Net 30',
 '2025-09-01', '2026-09-01', 12, '2025-09-15',
 'Pending', 6, 60,
 TRUE, FALSE,
 'https://quote.example.com/kk02',
 'Pending', 'ACTIVE',
 FALSE, FALSE, NULL, FALSE, FALSE, FALSE,
 (SELECT id FROM workspaces WHERE slug = 'kantorku')),

-- KK03: Low segment, Basic, Overdue
('KK03', 'PT Wahana Teknik', 'Wati Susanti', '+6281300000003', 'wati@wahanateknik.co.id', 'Owner', '10-50',
 'Xavier Tan', '+6282300000003', '300000003',
 'Low', 'Basic', 'Net 7',
 '2025-07-01', '2026-07-01', 12, '2025-07-15',
 'Overdue', 3, 28,
 TRUE, FALSE,
 NULL,
 'Pending', 'ACTIVE',
 FALSE, FALSE, NULL, FALSE, FALSE, FALSE,
 (SELECT id FROM workspaces WHERE slug = 'kantorku')),

-- KK04: Mid segment, Pro, Paid, converted
('KK04', 'PT Yudha Perkasa', 'Yoga Pratama', '+6281300000004', 'yoga@yudhaperkasa.co.id', 'People Ops', '100-250',
 'Zahra Amira', '+6282300000004', '300000004',
 'Mid', 'Pro', 'Net 30',
 '2025-03-01', '2026-03-01', 12, '2025-03-15',
 'Paid', 8, 82,
 TRUE, FALSE,
 'https://quote.example.com/kk04',
 'Replied', 'CONVERTED',
 TRUE, FALSE, '2026-02-20', FALSE, TRUE, TRUE,
 (SELECT id FROM workspaces WHERE slug = 'kantorku')),

-- KK05: High segment, Enterprise, Paid
('KK05', 'PT Aksara Bangsa', 'Aldi Firmansyah', '+6281300000005', 'aldi@aksarabangsa.co.id', 'HR Director', '250-500',
 'Bella Safitri', '+6282300000005', '300000005',
 'High', 'Enterprise', 'Net 30',
 '2025-12-01', '2026-12-01', 12, '2025-12-15',
 'Paid', 9, 90,
 TRUE, FALSE,
 'https://quote.example.com/kk05',
 'Pending', 'ACTIVE',
 FALSE, FALSE, NULL, FALSE, FALSE, FALSE,
 (SELECT id FROM workspaces WHERE slug = 'kantorku')),

-- KK06: Low segment, Basic, bot paused
('KK06', 'CV Citra Mandala', 'Cindy Wijaya', '+6281300000006', 'cindy@citramandala.co.id', 'Admin', '10-50',
 'Deni Saputra', '+6282300000006', '300000006',
 'Low', 'Basic', 'Net 7',
 '2025-08-01', '2026-08-01', 12, '2025-08-15',
 'Paid', 5, 40,
 FALSE, FALSE,
 NULL,
 'Pending', 'SNOOZED',
 FALSE, FALSE, NULL, FALSE, FALSE, FALSE,
 (SELECT id FROM workspaces WHERE slug = 'kantorku'));


-- ============================================================
-- INVOICES — Dealls workspace
-- ============================================================
INSERT INTO invoices (
  invoice_id, company_id, issue_date, due_date, amount, payment_status,
  paid_at, amount_paid, reminder_count, collection_stage,
  notes, link_invoice, workspace_id
) VALUES
('INV-DL01-001', 'DL01', '2025-12-01', '2025-12-31', 15000000.00, 'Paid',
 '2025-12-28 10:00:00', 15000000.00, 0, 'Closed',
 'Paid on time', 'https://inv.example.com/dl01-001',
 (SELECT id FROM workspaces WHERE slug = 'dealls')),

('INV-DL01-002', 'DL01', '2026-03-01', '2026-03-31', 15000000.00, 'Paid',
 '2026-03-29 14:00:00', 15000000.00, 0, 'Closed',
 '', 'https://inv.example.com/dl01-002',
 (SELECT id FROM workspaces WHERE slug = 'dealls')),

('INV-DL02-001', 'DL02', '2026-03-01', '2026-04-15', 8500000.00, 'Pending',
 NULL, 0, 1, 'Stage 0 — Pre-due',
 'Reminder sent', 'https://inv.example.com/dl02-001',
 (SELECT id FROM workspaces WHERE slug = 'dealls')),

('INV-DL03-001', 'DL03', '2026-02-01', '2026-03-01', 3200000.00, 'Overdue',
 NULL, 0, 4, 'Stage 3 — Urgency',
 'Multiple reminders sent, no response', 'https://inv.example.com/dl03-001',
 (SELECT id FROM workspaces WHERE slug = 'dealls')),

('INV-DL06-001', 'DL06', '2026-01-15', '2026-02-15', 1800000.00, 'Overdue',
 NULL, 0, 5, 'Stage 3 — Urgency',
 'Client rejected renewal', 'https://inv.example.com/dl06-001',
 (SELECT id FROM workspaces WHERE slug = 'dealls')),

('INV-DL08-001', 'DL08', '2026-03-01', '2026-04-01', 18000000.00, 'Partial',
 NULL, 9000000.00, 2, 'Stage 1 — Gentle',
 'Partial payment received', 'https://inv.example.com/dl08-001',
 (SELECT id FROM workspaces WHERE slug = 'dealls'));


-- ============================================================
-- INVOICES — KantorKu workspace
-- ============================================================
INSERT INTO invoices (
  invoice_id, company_id, issue_date, due_date, amount, payment_status,
  paid_at, amount_paid, reminder_count, collection_stage,
  notes, link_invoice, workspace_id
) VALUES
('INV-KK01-001', 'KK01', '2026-02-01', '2026-03-01', 20000000.00, 'Paid',
 '2026-02-25 09:00:00', 20000000.00, 0, 'Closed',
 'Early payment', 'https://inv.example.com/kk01-001',
 (SELECT id FROM workspaces WHERE slug = 'kantorku')),

('INV-KK02-001', 'KK02', '2026-03-15', '2026-04-15', 7500000.00, 'Pending',
 NULL, 0, 0, 'Stage 0 — Pre-due',
 '', 'https://inv.example.com/kk02-001',
 (SELECT id FROM workspaces WHERE slug = 'kantorku')),

('INV-KK03-001', 'KK03', '2026-01-01', '2026-02-01', 2500000.00, 'Overdue',
 NULL, 0, 6, 'Stage 3 — Urgency',
 'Escalated to management', 'https://inv.example.com/kk03-001',
 (SELECT id FROM workspaces WHERE slug = 'kantorku')),

('INV-KK04-001', 'KK04', '2026-02-01', '2026-03-01', 9000000.00, 'Paid',
 '2026-02-28 16:00:00', 9000000.00, 0, 'Closed',
 '', 'https://inv.example.com/kk04-001',
 (SELECT id FROM workspaces WHERE slug = 'kantorku'));


-- ============================================================
-- ESCALATIONS — Dealls workspace
-- ============================================================
INSERT INTO escalations (
  esc_id, company_id, trigger_condition, priority, status,
  triggered_at, resolved_at, resolved_by, notified_party, notes,
  workspace_id
) VALUES
('ESC-001', 'DL03', 'Invoice overdue >15 days', 'P1 Critical', 'Open',
 '2026-03-20 08:00:00', NULL, '', 'owner,ae',
 'Client unresponsive to payment reminders',
 (SELECT id FROM workspaces WHERE slug = 'dealls')),

('ESC-006', 'DL06', 'Angry client detected', 'P0 Emergency', 'Resolved',
 '2026-02-28 10:00:00', '2026-03-02 14:00:00', 'bambang@dealls.com', 'owner,ae,management',
 'Client expressed frustration about service quality',
 (SELECT id FROM workspaces WHERE slug = 'dealls')),

('ESC-003', 'DL08', 'Low NPS score detected', 'P2 High', 'Open',
 '2026-03-25 09:00:00', NULL, '', 'owner',
 'NPS dropped from 7 to 4 after partial payment issue',
 (SELECT id FROM workspaces WHERE slug = 'dealls')),

('ESC-007', 'DL02', 'Payment claim dispute', 'P1 Critical', 'Open',
 '2026-04-01 11:00:00', NULL, '', 'owner,finance',
 'Client claims payment was made but not reflected',
 (SELECT id FROM workspaces WHERE slug = 'dealls'));


-- ============================================================
-- ESCALATIONS — KantorKu workspace
-- ============================================================
INSERT INTO escalations (
  esc_id, company_id, trigger_condition, priority, status,
  triggered_at, resolved_at, resolved_by, notified_party, notes,
  workspace_id
) VALUES
('ESC-001', 'KK03', 'Invoice overdue >15 days', 'P1 Critical', 'Open',
 '2026-02-20 08:00:00', NULL, '', 'owner,ae',
 'Client unreachable for payment follow-up',
 (SELECT id FROM workspaces WHERE slug = 'kantorku')),

('ESC-005', 'KK03', 'High-value churn risk', 'P2 High', 'Resolved',
 '2026-02-25 10:00:00', '2026-03-05 16:00:00', 'sari@kantorku.com', 'owner,management',
 'Churn risk mitigated after personal meeting',
 (SELECT id FROM workspaces WHERE slug = 'kantorku')),

('ESC-002', 'KK02', 'Objection raised during renewal', 'P2 High', 'Open',
 '2026-03-28 14:00:00', NULL, '', 'owner,ae',
 'Price objection during renewal discussion',
 (SELECT id FROM workspaces WHERE slug = 'kantorku'));


-- ============================================================
-- CLIENT FLAGS — Dealls workspace clients
-- ============================================================
INSERT INTO client_flags (
  company_id,
  ren60_sent, ren45_sent, ren30_sent, ren15_sent, ren0_sent,
  checkin_a1_form_sent, checkin_a1_call_sent, checkin_a2_form_sent, checkin_a2_call_sent,
  checkin_b1_form_sent, checkin_b1_call_sent, checkin_b2_form_sent, checkin_b2_call_sent,
  checkin_replied,
  nps1_sent, nps2_sent, nps3_sent, nps_replied,
  referral_sent_this_cycle, quotation_acknowledged,
  low_usage_msg_sent, low_nps_msg_sent,
  cs_h7, cs_h14, cs_h21, cs_h30, cs_h45, cs_h60, cs_h75, cs_h90,
  cs_lt1, cs_lt2, cs_lt3,
  workspace_id
) VALUES
('DL01', FALSE,FALSE,FALSE,FALSE,FALSE, TRUE,TRUE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, TRUE, TRUE,TRUE,FALSE,FALSE, FALSE,FALSE, FALSE,FALSE, TRUE,TRUE,TRUE,FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE, (SELECT id FROM workspaces WHERE slug = 'dealls')),
('DL02', FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE, TRUE,FALSE,FALSE,FALSE, FALSE,FALSE, FALSE,FALSE, FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE, (SELECT id FROM workspaces WHERE slug = 'dealls')),
('DL03', TRUE,TRUE,TRUE,TRUE,TRUE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE, FALSE,FALSE, FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE, (SELECT id FROM workspaces WHERE slug = 'dealls')),
('DL04', TRUE,TRUE,TRUE,TRUE,TRUE, TRUE,TRUE,TRUE,TRUE, FALSE,FALSE,FALSE,FALSE, TRUE, TRUE,TRUE,TRUE,TRUE, TRUE,TRUE, FALSE,FALSE, TRUE,TRUE,TRUE,TRUE,TRUE,TRUE,TRUE,TRUE, TRUE,TRUE,FALSE, (SELECT id FROM workspaces WHERE slug = 'dealls')),
('DL05', FALSE,FALSE,FALSE,FALSE,FALSE, TRUE,TRUE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE, TRUE,TRUE,TRUE,FALSE, FALSE,FALSE, FALSE,FALSE, TRUE,TRUE,TRUE,TRUE,TRUE,TRUE,TRUE,TRUE, FALSE,FALSE,FALSE, (SELECT id FROM workspaces WHERE slug = 'dealls')),
('DL06', TRUE,TRUE,TRUE,TRUE,TRUE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE, FALSE,FALSE, FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE, (SELECT id FROM workspaces WHERE slug = 'dealls')),
('DL07', FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE, FALSE,FALSE, TRUE,TRUE,TRUE,TRUE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE, (SELECT id FROM workspaces WHERE slug = 'dealls')),
('DL08', FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE, TRUE,FALSE,FALSE,FALSE, FALSE,FALSE, FALSE,TRUE, FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE, (SELECT id FROM workspaces WHERE slug = 'dealls'));


-- ============================================================
-- CLIENT FLAGS — KantorKu workspace clients
-- ============================================================
INSERT INTO client_flags (
  company_id,
  ren60_sent, ren45_sent, ren30_sent, ren15_sent, ren0_sent,
  checkin_a1_form_sent, checkin_a1_call_sent, checkin_a2_form_sent, checkin_a2_call_sent,
  checkin_b1_form_sent, checkin_b1_call_sent, checkin_b2_form_sent, checkin_b2_call_sent,
  checkin_replied,
  nps1_sent, nps2_sent, nps3_sent, nps_replied,
  referral_sent_this_cycle, quotation_acknowledged,
  low_usage_msg_sent, low_nps_msg_sent,
  cs_h7, cs_h14, cs_h21, cs_h30, cs_h45, cs_h60, cs_h75, cs_h90,
  cs_lt1, cs_lt2, cs_lt3,
  workspace_id
) VALUES
('KK01', FALSE,FALSE,FALSE,FALSE,FALSE, TRUE,TRUE,TRUE,TRUE, FALSE,FALSE,FALSE,FALSE, TRUE, TRUE,TRUE,TRUE,TRUE, TRUE,FALSE, FALSE,FALSE, TRUE,TRUE,TRUE,TRUE,TRUE,TRUE,TRUE,TRUE, FALSE,FALSE,FALSE, (SELECT id FROM workspaces WHERE slug = 'kantorku')),
('KK02', FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE, FALSE,FALSE, FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE, (SELECT id FROM workspaces WHERE slug = 'kantorku')),
('KK03', TRUE,TRUE,TRUE,TRUE,TRUE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE, TRUE,FALSE, FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE, (SELECT id FROM workspaces WHERE slug = 'kantorku')),
('KK04', TRUE,TRUE,TRUE,TRUE,TRUE, TRUE,TRUE,TRUE,TRUE, FALSE,FALSE,FALSE,FALSE, TRUE, TRUE,TRUE,TRUE,TRUE, FALSE,TRUE, FALSE,FALSE, TRUE,TRUE,TRUE,TRUE,TRUE,TRUE,TRUE,TRUE, TRUE,FALSE,FALSE, (SELECT id FROM workspaces WHERE slug = 'kantorku')),
('KK05', FALSE,FALSE,FALSE,FALSE,FALSE, TRUE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE, TRUE,FALSE,FALSE,FALSE, FALSE,FALSE, FALSE,FALSE, TRUE,TRUE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE, (SELECT id FROM workspaces WHERE slug = 'kantorku')),
('KK06', FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE,FALSE, FALSE, FALSE,FALSE,FALSE,FALSE, FALSE,FALSE, FALSE,FALSE, FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE,FALSE, FALSE,FALSE,FALSE, (SELECT id FROM workspaces WHERE slug = 'kantorku'));


-- ============================================================
-- CONVERSATION STATES — Dealls workspace clients
-- ============================================================
INSERT INTO conversation_states (
  company_id, company_name,
  active_flow, current_stage, last_message_type, last_message_date,
  response_status, response_classification,
  attempt_count, cooldown_until,
  bot_active, reason_bot_paused,
  next_scheduled_action, next_scheduled_date,
  human_owner_notified, workspace_id
) VALUES
('DL01', 'PT Maju Bersama', 'CROSS_SELL', 'CS_H21', 'CS_H21', '2026-03-25 09:00:00', 'Pending', NULL, 3, NULL, TRUE, NULL, 'CS_H30', NULL, FALSE, (SELECT id FROM workspaces WHERE slug = 'dealls')),
('DL02', 'CV Teknologi Nusantara', 'INVOICE', 'PAY_PRE14', NULL, NULL, 'Pending', NULL, 0, NULL, TRUE, NULL, NULL, NULL, FALSE, (SELECT id FROM workspaces WHERE slug = 'dealls')),
('DL03', 'PT Sinar Abadi', 'OVERDUE', 'Overdue_POST15', 'TPL-PAY-POST15', '2026-03-18 09:00:00', 'Pending', NULL, 6, NULL, TRUE, NULL, NULL, NULL, TRUE, (SELECT id FROM workspaces WHERE slug = 'dealls')),
('DL04', 'PT Global Sejahtera', NULL, NULL, NULL, NULL, 'Replied', 'POSITIVE', 0, NULL, TRUE, NULL, NULL, NULL, FALSE, (SELECT id FROM workspaces WHERE slug = 'dealls')),
('DL05', 'PT Karya Digital', 'CROSS_SELL', 'CS_H90', 'CS_H90', '2026-03-10 09:00:00', 'Pending', NULL, 8, NULL, TRUE, NULL, 'CS_LT1', NULL, FALSE, (SELECT id FROM workspaces WHERE slug = 'dealls')),
('DL06', 'CV Mandiri Utama', 'RENEWAL', 'Renewal_REN0', 'REN0', '2026-03-01 09:00:00', 'Replied', 'Reject — not interested', 5, NULL, FALSE, 'Client rejected renewal', NULL, NULL, TRUE, (SELECT id FROM workspaces WHERE slug = 'dealls')),
('DL07', 'PT Nusantara Kreatif', 'CROSS_SELL', 'CS_H30', 'CS_H30', '2026-03-15 09:00:00', 'Pending', 'Reject — not interested', 4, '2026-04-15 00:00:00', TRUE, NULL, NULL, NULL, FALSE, (SELECT id FROM workspaces WHERE slug = 'dealls')),
('DL08', 'PT Optima Solusi', 'INVOICE', 'PAY_PRE7', 'TPL-PAY-PRE7', '2026-03-28 09:00:00', 'Replied', 'PAID', 2, NULL, TRUE, NULL, NULL, NULL, FALSE, (SELECT id FROM workspaces WHERE slug = 'dealls'));


-- ============================================================
-- CONVERSATION STATES — KantorKu workspace clients
-- ============================================================
INSERT INTO conversation_states (
  company_id, company_name,
  active_flow, current_stage, last_message_type, last_message_date,
  response_status, response_classification,
  attempt_count, cooldown_until,
  bot_active, reason_bot_paused,
  next_scheduled_action, next_scheduled_date,
  human_owner_notified, workspace_id
) VALUES
('KK01', 'PT Sakura Indonesia', NULL, NULL, NULL, NULL, 'Replied', 'POSITIVE', 0, NULL, TRUE, NULL, NULL, NULL, FALSE, (SELECT id FROM workspaces WHERE slug = 'kantorku')),
('KK02', 'CV Usaha Muda', 'RENEWAL', 'Renewal_REN60', NULL, NULL, 'Pending', NULL, 0, NULL, TRUE, NULL, NULL, NULL, FALSE, (SELECT id FROM workspaces WHERE slug = 'kantorku')),
('KK03', 'PT Wahana Teknik', 'OVERDUE', 'Overdue_POST8', 'TPL-PAY-POST8', '2026-03-01 09:00:00', 'Pending', 'OBJECTION_PRICE', 4, NULL, TRUE, NULL, NULL, NULL, TRUE, (SELECT id FROM workspaces WHERE slug = 'kantorku')),
('KK04', 'PT Yudha Perkasa', NULL, NULL, NULL, NULL, 'Replied', 'POSITIVE', 0, NULL, TRUE, NULL, NULL, NULL, FALSE, (SELECT id FROM workspaces WHERE slug = 'kantorku')),
('KK05', 'PT Aksara Bangsa', 'CHECKIN', 'CheckIn_A1_Form', NULL, NULL, 'Pending', NULL, 0, NULL, TRUE, NULL, NULL, NULL, FALSE, (SELECT id FROM workspaces WHERE slug = 'kantorku')),
('KK06', 'CV Citra Mandala', NULL, NULL, NULL, NULL, 'Pending', NULL, 0, NULL, FALSE, 'Snoozed by user', NULL, NULL, FALSE, (SELECT id FROM workspaces WHERE slug = 'kantorku'));
