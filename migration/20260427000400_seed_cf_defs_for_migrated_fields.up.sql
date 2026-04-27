-- Seed custom_field_definitions for keys that exist in clients.custom_fields
-- but lack a def. Triggered by phases 1-3 which migrated 18 columns into the
-- JSONB without registering them as workspace-level definitions.
--
-- Strategy: only seed defs for (workspace × key) pairs that already have
-- data — avoids polluting workspaces with defs they don't use. Run as many
-- times as you like; ON CONFLICT DO NOTHING keeps it idempotent.

WITH known_specs AS (
    SELECT * FROM (VALUES
        ('first_time_discount_pct',  'First-Time Discount %',     'number',  NULL,                          0::numeric, 100::numeric, 110, true),
        ('next_discount_pct_manual', 'Next Discount % (Manual)',  'number',  NULL,                          0::numeric, 100::numeric, 111, true),
        ('quotation_link_expires',   'Quotation Link Expires',    'date',    NULL,                          NULL::numeric, NULL::numeric, 130, true),
        ('bd_prospect_id',           'BD Prospect ID',            'text',    NULL,                          NULL::numeric, NULL::numeric, 140, false),
        ('renewal_date',             'Renewal Date',              'date',    NULL,                          NULL::numeric, NULL::numeric, 150, true),
        ('usage_score_avg_30d',      'Usage Score (Avg 30d)',     'number',  NULL,                          0::numeric, 100::numeric, 160, true),
        ('wa_undeliverable',         'WhatsApp Undeliverable',    'boolean', NULL,                          NULL::numeric, NULL::numeric, 170, false),
        ('churn_reason',             'Churn Reason',              'text',    NULL,                          NULL::numeric, NULL::numeric, 180, true),
        ('hc_size',                  'HC Size',                   'text',    NULL,                          NULL::numeric, NULL::numeric, 50,  true),
        ('plan_type',                'Plan Type',                 'text',    NULL,                          NULL::numeric, NULL::numeric, 60,  true),
        ('nps_score',                'NPS Score',                 'number',  NULL,                          0::numeric, 10::numeric,  70,  true),
        ('usage_score',              'Usage Score',               'number',  NULL,                          0::numeric, 100::numeric, 80,  true),
        ('cross_sell_rejected',      'Cross-Sell Rejected',       'boolean', NULL,                          NULL::numeric, NULL::numeric, 200, false),
        ('cross_sell_interested',    'Cross-Sell Interested',     'boolean', NULL,                          NULL::numeric, NULL::numeric, 201, false),
        ('cross_sell_resume_date',   'Cross-Sell Resume Date',    'date',    NULL,                          NULL::numeric, NULL::numeric, 202, false),
        ('renewed',                  'Renewed',                   'boolean', NULL,                          NULL::numeric, NULL::numeric, 220, true),
        ('rejected',                 'Rejected',                  'boolean', NULL,                          NULL::numeric, NULL::numeric, 221, true),
        ('quotation_link',           'Quotation Link',            'url',     NULL,                          NULL::numeric, NULL::numeric, 240, true),
        ('segment',                  'Segment',                   'select',  '["High","Mid","Low"]'::jsonb, NULL::numeric, NULL::numeric, 90,  true)
    ) AS t(field_key, field_label, field_type, options, min_value, max_value, sort_order, visible_in_table)
),
keys_in_use AS (
    SELECT DISTINCT c.workspace_id, k.key AS field_key
    FROM clients c
    CROSS JOIN LATERAL jsonb_object_keys(c.custom_fields) k(key)
    WHERE c.custom_fields <> '{}'::jsonb
)
INSERT INTO custom_field_definitions (
    workspace_id,
    field_key,
    field_label,
    field_type,
    is_required,
    options,
    min_value,
    max_value,
    sort_order,
    visible_in_table,
    column_width
)
SELECT
    ku.workspace_id,
    ks.field_key,
    ks.field_label,
    ks.field_type,
    FALSE,
    ks.options,
    ks.min_value,
    ks.max_value,
    ks.sort_order,
    ks.visible_in_table,
    140
FROM keys_in_use ku
JOIN known_specs ks USING (field_key)
ON CONFLICT (workspace_id, field_key) DO NOTHING;
