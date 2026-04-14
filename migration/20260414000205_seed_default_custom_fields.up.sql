-- Migration: Seed default custom field definitions for the `dealls` workspace.
-- Version: 20260414000205
-- Idempotent via ON CONFLICT (workspace_id, field_key).

INSERT INTO custom_field_definitions
    (workspace_id, field_key, field_label, field_type, is_required, options, sort_order, visible_in_table, column_width)
SELECT w.id, t.field_key, t.field_label, t.field_type, t.is_required, t.options, t.sort_order, t.visible_in_table, t.column_width
FROM workspaces w
CROSS JOIN (VALUES
    ('hc_size',          'Headcount',                 'number',  TRUE,  NULL::JSONB,                                  1,  TRUE,  80),
    ('industry',         'Industry',                  'text',    FALSE, NULL::JSONB,                                  2,  TRUE,  120),
    ('plan_type',        'Plan',                      'select',  FALSE, '["Basic","Mid","Enterprise"]'::JSONB,        3,  TRUE,  90),
    ('value_tier',       'Value Tier',                'select',  FALSE, '["High","Mid","Low"]'::JSONB,                4,  TRUE,  80),
    ('nps_score',        'NPS Score',                 'number',  FALSE, NULL::JSONB,                                  5,  TRUE,  70),
    ('nps_score_1',      'NPS Score 1',               'number',  FALSE, NULL::JSONB,                                  6,  FALSE, 70),
    ('nps_score_2',      'NPS Score 2',               'number',  FALSE, NULL::JSONB,                                  7,  FALSE, 70),
    ('nps_replied',      'NPS Replied',               'boolean', FALSE, NULL::JSONB,                                  8,  FALSE, 60),
    ('usage_score',      'Usage Score',               'number',  FALSE, NULL::JSONB,                                  9,  TRUE,  80),
    ('onboarding_sent',  'Onboarding Sent',           'boolean', FALSE, NULL::JSONB,                                  20, FALSE, 60),
    ('ob_checkin_sent',  'OB Check-in Sent',          'boolean', FALSE, NULL::JSONB,                                  21, FALSE, 60),
    ('ob_usage_sent',    'OB Usage Sent',             'boolean', FALSE, NULL::JSONB,                                  22, FALSE, 60),
    ('nps1_sent',        'NPS1 Sent',                 'boolean', FALSE, NULL::JSONB,                                  23, FALSE, 60),
    ('nps1_fu_sent',     'NPS1 FU Sent',              'boolean', FALSE, NULL::JSONB,                                  24, FALSE, 60),
    ('cs_awareness_sent','Cross-sell Awareness Sent', 'boolean', FALSE, NULL::JSONB,                                  25, FALSE, 60),
    ('checkin_replied',  'Check-in Replied',          'boolean', FALSE, NULL::JSONB,                                  28, FALSE, 60)
) AS t(field_key, field_label, field_type, is_required, options, sort_order, visible_in_table, column_width)
WHERE w.slug = 'dealls'
ON CONFLICT (workspace_id, field_key) DO NOTHING;
