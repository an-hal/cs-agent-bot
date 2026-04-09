-- Migration: Seed trigger rules
-- Version: 20260409000005
-- Description: Seeds all trigger rules that replicate the existing hardcoded business logic.
-- Each row maps to one if-block in the current trigger evaluators.
-- Conditions use JSON DSL. Variables use {Bracket} syntax in templates.

-- ══════════════════════════════════════════════════════════════════════
-- P0: HEALTH (priority=10)
-- ══════════════════════════════════════════════════════════════════════
INSERT INTO trigger_rules (rule_id, rule_group, priority, sub_priority, condition, action_type, template_id, flag_key, stop_on_fire, active, description) VALUES
('HEALTH_LOW_USAGE', 'HEALTH', 10, 1,
 '{"and":[{"field":"usage_score","op":"lt","value":40},{"flag":"low_usage_msg_sent","op":"not_set"}]}',
 'send_wa', 'LOW_USAGE', 'low_usage_msg_sent', TRUE, TRUE,
 'Send low usage alert when UsageScore < 40'),

('HEALTH_LOW_NPS', 'HEALTH', 10, 2,
 '{"and":[{"field":"nps_score","op":"gt","value":0},{"field":"nps_score","op":"lte","value":5},{"flag":"low_nps_msg_sent","op":"not_set"}]}',
 'send_wa', 'LOW_NPS', 'low_nps_msg_sent', TRUE, TRUE,
 'Send low NPS alert and escalate when NPSScore <= 5'),

-- ESC-003 escalation for low NPS (fires after LOW_NPS message)
('HEALTH_LOW_NPS_ESC', 'HEALTH', 10, 3,
 '{"and":[{"field":"nps_score","op":"gt","value":0},{"field":"nps_score","op":"lte","value":5},{"flag":"low_nps_msg_sent","op":"set"}]}',
 'escalate', NULL, 'low_nps_msg_sent', FALSE, TRUE,
 'Escalate ESC-003 for NPS <= 5');

-- ══════════════════════════════════════════════════════════════════════
-- P0.5: CHECKIN Branch A (ContractMonths >= 9) (priority=20)
-- ══════════════════════════════════════════════════════════════════════
INSERT INTO trigger_rules (rule_id, rule_group, priority, sub_priority, condition, action_type, template_id, flag_key, escalation_id, esc_priority, esc_reason, stop_on_fire, active, description) VALUES
('CHECKIN_A1_FORM', 'CHECKIN', 20, 1,
 '{"and":[{"flag":"checkin_replied","op":"not_set"},{"field":"contract_months","op":"gte","value":9},{"field":"days_to_expiry","op":"lte","value":120},{"flag":"checkin_a1_form_sent","op":"not_set"}]}',
 'send_wa', 'TPL-CHECKIN-FORM', 'checkin_a1_form_sent', NULL, NULL, NULL, TRUE, TRUE,
 'Check-in Branch A: H-120 form (ContractMonths >= 9)'),

('CHECKIN_A1_CALL', 'CHECKIN', 20, 2,
 '{"and":[{"flag":"checkin_replied","op":"not_set"},{"field":"contract_months","op":"gte","value":9},{"field":"days_to_expiry","op":"lte","value":113},{"flag":"checkin_a1_form_sent","op":"set"},{"flag":"checkin_a1_call_sent","op":"not_set"}]}',
 'send_wa', 'TPL-CHECKIN-CALL', 'checkin_a1_call_sent', NULL, NULL, NULL, TRUE, TRUE,
 'Check-in Branch A: H-113 call (7 days after form)'),

('CHECKIN_A2_FORM', 'CHECKIN', 20, 3,
 '{"and":[{"flag":"checkin_replied","op":"not_set"},{"field":"contract_months","op":"gte","value":9},{"field":"days_to_expiry","op":"lte","value":90},{"flag":"checkin_a2_form_sent","op":"not_set"}]}',
 'send_wa', 'TPL-CHECKIN-FORM', 'checkin_a2_form_sent', NULL, NULL, NULL, TRUE, TRUE,
 'Check-in Branch A: H-90 form'),

('CHECKIN_A2_CALL', 'CHECKIN', 20, 4,
 '{"and":[{"flag":"checkin_replied","op":"not_set"},{"field":"contract_months","op":"gte","value":9},{"field":"days_to_expiry","op":"lte","value":83},{"flag":"checkin_a2_form_sent","op":"set"},{"flag":"checkin_a2_call_sent","op":"not_set"}]}',
 'send_wa', 'TPL-CHECKIN-CALL', 'checkin_a2_call_sent', NULL, NULL, NULL, TRUE, TRUE,
 'Check-in Branch A: H-83 call');

-- ══════════════════════════════════════════════════════════════════════
-- P0.5: CHECKIN Branch B (ContractMonths < 9) (priority=21)
-- ══════════════════════════════════════════════════════════════════════
INSERT INTO trigger_rules (rule_id, rule_group, priority, sub_priority, condition, action_type, template_id, flag_key, escalation_id, esc_priority, esc_reason, stop_on_fire, active, description) VALUES
('CHECKIN_B1_FORM', 'CHECKIN', 21, 1,
 '{"and":[{"flag":"checkin_replied","op":"not_set"},{"field":"contract_months","op":"lt","value":9},{"field":"days_to_expiry","op":"lte","value":90},{"flag":"checkin_b1_form_sent","op":"not_set"}]}',
 'send_wa', 'TPL-CHECKIN-FORM', 'checkin_b1_form_sent', NULL, NULL, NULL, TRUE, TRUE,
 'Check-in Branch B: H-90 form (ContractMonths < 9)'),

('CHECKIN_B1_CALL', 'CHECKIN', 21, 2,
 '{"and":[{"flag":"checkin_replied","op":"not_set"},{"field":"contract_months","op":"lt","value":9},{"field":"days_to_expiry","op":"lte","value":83},{"flag":"checkin_b1_form_sent","op":"set"},{"flag":"checkin_b1_call_sent","op":"not_set"}]}',
 'send_wa', 'TPL-CHECKIN-CALL', 'checkin_b1_call_sent', NULL, NULL, NULL, TRUE, TRUE,
 'Check-in Branch B: H-83 call'),

('CHECKIN_B2_FORM', 'CHECKIN', 21, 3,
 '{"and":[{"flag":"checkin_replied","op":"not_set"},{"field":"contract_months","op":"lt","value":9},{"field":"days_to_expiry","op":"lte","value":60},{"flag":"checkin_b2_form_sent","op":"not_set"}]}',
 'send_wa', 'TPL-CHECKIN-FORM', 'checkin_b2_form_sent', NULL, NULL, NULL, TRUE, TRUE,
 'Check-in Branch B: H-60 form'),

('CHECKIN_B2_CALL', 'CHECKIN', 21, 4,
 '{"and":[{"flag":"checkin_replied","op":"not_set"},{"field":"contract_months","op":"lt","value":9},{"field":"days_to_expiry","op":"lte","value":53},{"flag":"checkin_b2_form_sent","op":"set"},{"flag":"checkin_b2_call_sent","op":"not_set"}]}',
 'send_wa', 'TPL-CHECKIN-CALL', 'checkin_b2_call_sent', NULL, NULL, NULL, TRUE, TRUE,
 'Check-in Branch B: H-53 call');

-- ══════════════════════════════════════════════════════════════════════
-- P1: NEGOTIATION / RENEWAL (priority=30)
-- ══════════════════════════════════════════════════════════════════════

-- REN60: skip if checkin_replied (set ren60+ren45 flags and skip)
INSERT INTO trigger_rules (rule_id, rule_group, priority, sub_priority, condition, action_type, template_id, flag_key, extra_flags, stop_on_fire, active, description) VALUES
('REN60_SKIP_CHECKIN', 'NEGOTIATION', 30, 1,
 '{"and":[{"field":"days_to_expiry","op":"lte","value":60},{"flag":"ren60_sent","op":"not_set"},{"flag":"checkin_replied","op":"set"}]}',
 'skip_and_set_flag', NULL, 'ren60_sent', '{"ren45_sent":true}', FALSE, TRUE,
 'Skip REN60+REN45 when checkin_replied=TRUE (Rule 6)');

INSERT INTO trigger_rules (rule_id, rule_group, priority, sub_priority, condition, action_type, template_id, flag_key, stop_on_fire, active, description) VALUES
('REN60', 'NEGOTIATION', 30, 2,
 '{"and":[{"field":"days_to_expiry","op":"lte","value":60},{"flag":"ren60_sent","op":"not_set"},{"field":"response_status","op":"neq","value":"Replied"}]}',
 'send_wa', 'REN60', 'ren60_sent', TRUE, TRUE,
 'Renewal H-60: Send if no reply and not checkin_replied'),

-- REN45: skip if checkin_replied
('REN45_SKIP_CHECKIN', 'NEGOTIATION', 30, 3,
 '{"and":[{"field":"days_to_expiry","op":"lte","value":45},{"flag":"ren45_sent","op":"not_set"},{"flag":"checkin_replied","op":"set"}]}',
 'skip_and_set_flag', NULL, 'ren45_sent', FALSE, TRUE,
 'Skip REN45 when checkin_replied=TRUE'),

('REN45', 'NEGOTIATION', 30, 4,
 '{"and":[{"field":"days_to_expiry","op":"lte","value":45},{"flag":"ren45_sent","op":"not_set"},{"field":"response_status","op":"neq","value":"Replied"}]}',
 'send_wa', 'REN45', 'ren45_sent', TRUE, TRUE,
 'Renewal H-45: Send if no reply'),

-- REN30: ignores reply status, requires quotation_link
('REN30_BLOCK_NO_QUOTATION', 'NEGOTIATION', 30, 5,
 '{"and":[{"field":"days_to_expiry","op":"lte","value":30},{"flag":"ren30_sent","op":"not_set"},{"field":"quotation_link","op":"is_empty"}]}',
 'alert_telegram', NULL, 'ren30_sent', FALSE, TRUE,
 'REN30 delayed: quotation_link is empty (Rule 10). Alert AE.'),

('REN30', 'NEGOTIATION', 30, 6,
 '{"and":[{"field":"days_to_expiry","op":"lte","value":30},{"flag":"ren30_sent","op":"not_set"},{"field":"quotation_link","op":"not_empty"}]}',
 'send_wa', 'REN30', 'ren30_sent', TRUE, TRUE,
 'Renewal H-30: Send regardless of reply status'),

-- REN15
('REN15', 'NEGOTIATION', 30, 7,
 '{"and":[{"field":"days_to_expiry","op":"lte","value":15},{"flag":"ren15_sent","op":"not_set"}]}',
 'send_wa', 'REN15', 'ren15_sent', TRUE, TRUE,
 'Renewal H-15: Send regardless of reply status'),

-- REN0
('REN0', 'NEGOTIATION', 30, 8,
 '{"and":[{"field":"days_to_expiry","op":"lte","value":0},{"flag":"ren0_sent","op":"not_set"}]}',
 'send_wa', 'REN0', 'ren0_sent', TRUE, TRUE,
 'Renewal H-0: Contract expires today');

-- REN0 escalation for Mid/High segment with no reply
INSERT INTO trigger_rules (rule_id, rule_group, priority, sub_priority, condition, action_type, template_id, flag_key, escalation_id, esc_priority, esc_reason, stop_on_fire, active, description) VALUES
('REN0_ESC_MID_HIGH', 'NEGOTIATION', 30, 9,
 '{"and":[{"field":"days_to_expiry","op":"lte","value":0},{"flag":"ren0_sent","op":"set"},{"field":"response_status","op":"neq","value":"Replied"},{"field":"segment","op":"in","value":["Mid","High"]}]}',
 'escalate', NULL, 'ren0_sent', 'ESC-004', 'P2 High', 'REN0 sent, no reply from Mid/High segment client', FALSE, TRUE,
 'Escalate ESC-004: REN0 no-reply for Mid/High segment');

-- ══════════════════════════════════════════════════════════════════════
-- P2: INVOICE (priority=40)
-- ══════════════════════════════════════════════════════════════════════
INSERT INTO trigger_rules (rule_id, rule_group, priority, sub_priority, condition, action_type, template_id, flag_key, stop_on_fire, active, description) VALUES
-- Create invoice at H-30 if none exists
('INVOICE_CREATE', 'INVOICE', 40, 1,
 '{"and":[{"field":"conv_should_send","op":"is_true"},{"field":"days_to_expiry","op":"lte","value":30},{"field":"has_active_invoice","op":"is_false"}]}',
 'create_invoice', NULL, 'invoice_created', FALSE, TRUE,
 'Create invoice at H-30 if none exists (requires quotation_link)'),

-- Pre-due reminders
('PAY_PRE14', 'INVOICE', 40, 2,
 '{"and":[{"field":"conv_should_send","op":"is_true"},{"field":"has_active_invoice","op":"is_true"},{"field":"days_to_expiry","op":"lte","value":14},{"field":"pre14_sent","op":"is_false"}]}',
 'send_wa', 'TPL-PAY-PRE14', 'pre14_sent', TRUE, TRUE,
 'Invoice reminder H-14'),

('PAY_PRE7', 'INVOICE', 40, 3,
 '{"and":[{"field":"conv_should_send","op":"is_true"},{"field":"has_active_invoice","op":"is_true"},{"field":"days_to_expiry","op":"lte","value":7},{"field":"pre7_sent","op":"is_false"}]}',
 'send_wa', 'TPL-PAY-PRE7', 'pre7_sent', TRUE, TRUE,
 'Invoice reminder H-7'),

('PAY_PRE3', 'INVOICE', 40, 4,
 '{"and":[{"field":"conv_should_send","op":"is_true"},{"field":"has_active_invoice","op":"is_true"},{"field":"days_to_expiry","op":"lte","value":3},{"field":"pre3_sent","op":"is_false"}]}',
 'send_wa', 'TPL-PAY-PRE3', 'pre3_sent', TRUE, TRUE,
 'Invoice reminder H-3');

-- ══════════════════════════════════════════════════════════════════════
-- P3: OVERDUE (priority=50)
-- ══════════════════════════════════════════════════════════════════════
INSERT INTO trigger_rules (rule_id, rule_group, priority, sub_priority, condition, action_type, template_id, flag_key, escalation_id, esc_priority, esc_reason, stop_on_fire, active, description) VALUES
('PAY_POST1', 'OVERDUE', 50, 1,
 '{"and":[{"field":"conv_should_send","op":"is_true"},{"field":"is_payment_overdue","op":"is_true"},{"field":"days_past_due","op":"gte","value":1},{"field":"post1_sent","op":"is_false"}]}',
 'send_wa', 'TPL-PAY-POST1', 'post1_sent', NULL, NULL, NULL, TRUE, TRUE,
 'Overdue reminder D+1'),

('PAY_POST4', 'OVERDUE', 50, 2,
 '{"and":[{"field":"conv_should_send","op":"is_true"},{"field":"is_payment_overdue","op":"is_true"},{"field":"days_past_due","op":"gte","value":4},{"field":"post4_sent","op":"is_false"}]}',
 'send_wa', 'TPL-PAY-POST4', 'post4_sent', NULL, NULL, NULL, TRUE, TRUE,
 'Overdue reminder D+4'),

('PAY_POST8', 'OVERDUE', 50, 3,
 '{"and":[{"field":"conv_should_send","op":"is_true"},{"field":"is_payment_overdue","op":"is_true"},{"field":"days_past_due","op":"gte","value":8},{"field":"post8_sent","op":"is_false"}]}',
 'send_wa', 'TPL-PAY-POST8', 'post8_sent', NULL, NULL, NULL, TRUE, TRUE,
 'Overdue reminder D+8'),

('PAY_POST15', 'OVERDUE', 50, 4,
 '{"and":[{"field":"conv_should_send","op":"is_true"},{"field":"is_payment_overdue","op":"is_true"},{"field":"days_past_due","op":"gte","value":15},{"field":"post15_sent","op":"is_false"}]}',
 'send_wa', 'TPL-PAY-POST15', 'post15_sent', NULL, NULL, NULL, TRUE, TRUE,
 'Overdue reminder D+15'),

-- D+15 escalation
('OVERDUE_ESC_D15', 'OVERDUE', 50, 5,
 '{"and":[{"field":"is_payment_overdue","op":"is_true"},{"field":"days_past_due","op":"gte","value":15}]}',
 'escalate', NULL, 'post15_sent', 'ESC-001', 'P1 Critical', 'Invoice overdue D+15+', TRUE, TRUE,
 'Escalate ESC-001: Invoice overdue D+15, AE takes over');

-- ══════════════════════════════════════════════════════════════════════
-- P4: EXPANSION - NPS + Referral (priority=60)
-- ══════════════════════════════════════════════════════════════════════
INSERT INTO trigger_rules (rule_id, rule_group, priority, sub_priority, condition, action_type, template_id, flag_key, stop_on_fire, active, description) VALUES
('NPS1', 'EXPANSION', 60, 1,
 '{"and":[{"field":"activation_date_set","op":"is_true"},{"field":"days_since_activation","op":"gte","value":15},{"flag":"nps1_sent","op":"not_set"}]}',
 'send_wa', 'NPS1', 'nps1_sent', TRUE, TRUE,
 'NPS survey #1 at D+15 post-activation'),

('NPS2', 'EXPANSION', 60, 2,
 '{"and":[{"field":"activation_date_set","op":"is_true"},{"field":"days_since_activation","op":"gte","value":45},{"flag":"nps2_sent","op":"not_set"}]}',
 'send_wa', 'NPS2', 'nps2_sent', TRUE, TRUE,
 'NPS survey #2 at D+45 post-activation'),

('NPS3', 'EXPANSION', 60, 3,
 '{"and":[{"field":"activation_date_set","op":"is_true"},{"field":"days_since_activation","op":"gte","value":60},{"flag":"nps3_sent","op":"not_set"}]}',
 'send_wa', 'NPS3', 'nps3_sent', TRUE, TRUE,
 'NPS survey #3 at D+60 post-activation'),

('REFERRAL', 'EXPANSION', 60, 4,
 '{"and":[{"flag":"nps_replied","op":"set"},{"field":"nps_score","op":"gte","value":8},{"flag":"referral_sent_this_cycle","op":"not_set"}]}',
 'send_wa', 'REFERRAL', 'referral_sent_this_cycle', TRUE, TRUE,
 'Referral request: only if NPSReplied=TRUE and NPSScore >= 8');

-- ══════════════════════════════════════════════════════════════════════
-- P5: CROSS-SELL 90-day sequence (priority=70)
-- ══════════════════════════════════════════════════════════════════════
INSERT INTO trigger_rules (rule_id, rule_group, priority, sub_priority, condition, action_type, template_id, flag_key, stop_on_fire, active, description) VALUES
('CS_H7', 'CROSS_SELL', 70, 1,
 '{"and":[{"field":"activation_date_set","op":"is_true"},{"field":"cross_sell_rejected","op":"is_false"},{"field":"cross_sell_interested","op":"is_false"},{"not":{"and":[{"flag":"nps_replied","op":"set"},{"field":"nps_score","op":"lt","value":8}]}},{"flag":"feature_update_sent","op":"not_set"},{"or":[{"field":"sequence_cs","op":"eq","value":"ACTIVE"},{"field":"sequence_cs","op":"is_empty"}]},{"field":"days_since_activation","op":"gte","value":30},{"flag":"cs_h7","op":"not_set"}]}',
 'send_wa', 'CS_H7', 'cs_h7', TRUE, TRUE,
 'Cross-sell Day 7 (D+30 post-activation)'),

('CS_H14', 'CROSS_SELL', 70, 2,
 '{"and":[{"field":"activation_date_set","op":"is_true"},{"field":"cross_sell_rejected","op":"is_false"},{"field":"cross_sell_interested","op":"is_false"},{"not":{"and":[{"flag":"nps_replied","op":"set"},{"field":"nps_score","op":"lt","value":8}]}},{"flag":"feature_update_sent","op":"not_set"},{"or":[{"field":"sequence_cs","op":"eq","value":"ACTIVE"},{"field":"sequence_cs","op":"is_empty"}]},{"field":"days_since_activation","op":"gte","value":37},{"flag":"cs_h14","op":"not_set"}]}',
 'send_wa', 'CS_H14', 'cs_h14', TRUE, TRUE,
 'Cross-sell Day 14 (D+37)'),

('CS_H21', 'CROSS_SELL', 70, 3,
 '{"and":[{"field":"activation_date_set","op":"is_true"},{"field":"cross_sell_rejected","op":"is_false"},{"field":"cross_sell_interested","op":"is_false"},{"not":{"and":[{"flag":"nps_replied","op":"set"},{"field":"nps_score","op":"lt","value":8}]}},{"flag":"feature_update_sent","op":"not_set"},{"or":[{"field":"sequence_cs","op":"eq","value":"ACTIVE"},{"field":"sequence_cs","op":"is_empty"}]},{"field":"days_since_activation","op":"gte","value":44},{"flag":"cs_h21","op":"not_set"}]}',
 'send_wa', 'CS_H21', 'cs_h21', TRUE, TRUE,
 'Cross-sell Day 21 (D+44)'),

('CS_H30', 'CROSS_SELL', 70, 4,
 '{"and":[{"field":"activation_date_set","op":"is_true"},{"field":"cross_sell_rejected","op":"is_false"},{"field":"cross_sell_interested","op":"is_false"},{"not":{"and":[{"flag":"nps_replied","op":"set"},{"field":"nps_score","op":"lt","value":8}]}},{"flag":"feature_update_sent","op":"not_set"},{"or":[{"field":"sequence_cs","op":"eq","value":"ACTIVE"},{"field":"sequence_cs","op":"is_empty"}]},{"field":"days_since_activation","op":"gte","value":52},{"flag":"cs_h30","op":"not_set"}]}',
 'send_wa', 'CS_H30', 'cs_h30', TRUE, TRUE,
 'Cross-sell Day 30 (D+52)'),

('CS_H45', 'CROSS_SELL', 70, 5,
 '{"and":[{"field":"activation_date_set","op":"is_true"},{"field":"cross_sell_rejected","op":"is_false"},{"field":"cross_sell_interested","op":"is_false"},{"not":{"and":[{"flag":"nps_replied","op":"set"},{"field":"nps_score","op":"lt","value":8}]}},{"flag":"feature_update_sent","op":"not_set"},{"or":[{"field":"sequence_cs","op":"eq","value":"ACTIVE"},{"field":"sequence_cs","op":"is_empty"}]},{"field":"days_since_activation","op":"gte","value":60},{"flag":"cs_h45","op":"not_set"}]}',
 'send_wa', 'CS_H45', 'cs_h45', TRUE, TRUE,
 'Cross-sell Day 45 (D+60)'),

('CS_H60', 'CROSS_SELL', 70, 6,
 '{"and":[{"field":"activation_date_set","op":"is_true"},{"field":"cross_sell_rejected","op":"is_false"},{"field":"cross_sell_interested","op":"is_false"},{"not":{"and":[{"flag":"nps_replied","op":"set"},{"field":"nps_score","op":"lt","value":8}]}},{"flag":"feature_update_sent","op":"not_set"},{"or":[{"field":"sequence_cs","op":"eq","value":"ACTIVE"},{"field":"sequence_cs","op":"is_empty"}]},{"field":"days_since_activation","op":"gte","value":67},{"flag":"cs_h60","op":"not_set"}]}',
 'send_wa', 'CS_H60', 'cs_h60', TRUE, TRUE,
 'Cross-sell Day 60 (D+67)'),

('CS_H75', 'CROSS_SELL', 70, 7,
 '{"and":[{"field":"activation_date_set","op":"is_true"},{"field":"cross_sell_rejected","op":"is_false"},{"field":"cross_sell_interested","op":"is_false"},{"not":{"and":[{"flag":"nps_replied","op":"set"},{"field":"nps_score","op":"lt","value":8}]}},{"flag":"feature_update_sent","op":"not_set"},{"or":[{"field":"sequence_cs","op":"eq","value":"ACTIVE"},{"field":"sequence_cs","op":"is_empty"}]},{"field":"days_since_activation","op":"gte","value":75},{"flag":"cs_h75","op":"not_set"}]}',
 'send_wa', 'CS_H75', 'cs_h75', TRUE, TRUE,
 'Cross-sell Day 75 (D+75)'),

('CS_H90', 'CROSS_SELL', 70, 8,
 '{"and":[{"field":"activation_date_set","op":"is_true"},{"field":"cross_sell_rejected","op":"is_false"},{"field":"cross_sell_interested","op":"is_false"},{"not":{"and":[{"flag":"nps_replied","op":"set"},{"field":"nps_score","op":"lt","value":8}]}},{"flag":"feature_update_sent","op":"not_set"},{"or":[{"field":"sequence_cs","op":"eq","value":"ACTIVE"},{"field":"sequence_cs","op":"is_empty"}]},{"field":"days_since_activation","op":"gte","value":90},{"flag":"cs_h90","op":"not_set"}]}',
 'send_wa', 'CS_H90', 'cs_h90', TRUE, TRUE,
 'Cross-sell Day 90 (D+90)');

-- ══════════════════════════════════════════════════════════════════════
-- P5: CROSS-SELL Long-term rotation (priority=71)
-- ══════════════════════════════════════════════════════════════════════
INSERT INTO trigger_rules (rule_id, rule_group, priority, sub_priority, condition, action_type, template_id, flag_key, stop_on_fire, active, description) VALUES
('CS_LT1', 'CROSS_SELL', 71, 1,
 '{"and":[{"field":"activation_date_set","op":"is_true"},{"field":"cross_sell_rejected","op":"is_false"},{"field":"cross_sell_interested","op":"is_false"},{"not":{"and":[{"flag":"nps_replied","op":"set"},{"field":"nps_score","op":"lt","value":8}]}},{"flag":"feature_update_sent","op":"not_set"},{"field":"sequence_cs","op":"eq","value":"LONGTERM"},{"flag":"cs_lt1","op":"not_set"}]}',
 'send_wa', 'CS_LT1', 'cs_lt1', TRUE, TRUE,
 'Cross-sell long-term #1'),

('CS_LT2', 'CROSS_SELL', 71, 2,
 '{"and":[{"field":"activation_date_set","op":"is_true"},{"field":"cross_sell_rejected","op":"is_false"},{"field":"cross_sell_interested","op":"is_false"},{"not":{"and":[{"flag":"nps_replied","op":"set"},{"field":"nps_score","op":"lt","value":8}]}},{"flag":"feature_update_sent","op":"not_set"},{"field":"sequence_cs","op":"eq","value":"LONGTERM"},{"flag":"cs_lt2","op":"not_set"}]}',
 'send_wa', 'CS_LT2', 'cs_lt2', TRUE, TRUE,
 'Cross-sell long-term #2'),

('CS_LT3', 'CROSS_SELL', 71, 3,
 '{"and":[{"field":"activation_date_set","op":"is_true"},{"field":"cross_sell_rejected","op":"is_false"},{"field":"cross_sell_interested","op":"is_false"},{"not":{"and":[{"flag":"nps_replied","op":"set"},{"field":"nps_score","op":"lt","value":8}]}},{"flag":"feature_update_sent","op":"not_set"},{"field":"sequence_cs","op":"eq","value":"LONGTERM"},{"flag":"cs_lt3","op":"not_set"}]}',
 'send_wa', 'CS_LT3', 'cs_lt3', TRUE, TRUE,
 'Cross-sell long-term #3 (resets LT flags after fire)');
