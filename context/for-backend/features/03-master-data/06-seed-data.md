# Seed Data — Default Custom Fields per Workspace

## Dealls (ATS/Recruitment Product)

```sql
INSERT INTO custom_field_definitions (workspace_id, field_key, field_label, field_type, is_required, options, sort_order, visible_in_table, column_width) VALUES
-- Engagement
('{ws_dealls}', 'hc_size',         'Headcount',          'number',  true,  NULL, 1, true, 80),
('{ws_dealls}', 'industry',        'Industry',           'text',    false, NULL, 2, true, 120),
('{ws_dealls}', 'plan_type',       'Plan',               'select',  false, '["Basic","Mid","Enterprise"]', 3, true, 90),
('{ws_dealls}', 'value_tier',      'Value Tier',         'select',  false, '["High","Mid","Low"]', 4, true, 80),
-- NPS
('{ws_dealls}', 'nps_score',       'NPS Score',          'number',  false, NULL, 5, true, 70),
('{ws_dealls}', 'nps_score_1',     'NPS Score 1',        'number',  false, NULL, 6, false, 70),
('{ws_dealls}', 'nps_score_2',     'NPS Score 2',        'number',  false, NULL, 7, false, 70),
('{ws_dealls}', 'nps_replied',     'NPS Replied',        'boolean', false, NULL, 8, false, 60),
('{ws_dealls}', 'usage_score',     'Usage Score',        'number',  false, NULL, 9, true, 80),
-- Sent flags (automation tracking)
('{ws_dealls}', 'onboarding_sent',           'Onboarding Sent',            'boolean', false, NULL, 20, false, 60),
('{ws_dealls}', 'ob_checkin_sent',           'OB Check-in Sent',           'boolean', false, NULL, 21, false, 60),
('{ws_dealls}', 'ob_usage_sent',             'OB Usage Sent',              'boolean', false, NULL, 22, false, 60),
('{ws_dealls}', 'nps1_sent',                 'NPS1 Sent',                  'boolean', false, NULL, 23, false, 60),
('{ws_dealls}', 'nps1_fu_sent',              'NPS1 FU Sent',               'boolean', false, NULL, 24, false, 60),
('{ws_dealls}', 'cs_awareness_sent',         'Cross-sell Awareness Sent',  'boolean', false, NULL, 25, false, 60),
('{ws_dealls}', 'warmup_form_sent',          'Warmup Form Sent',           'boolean', false, NULL, 26, false, 60),
('{ws_dealls}', 'warmup_call_sent',          'Warmup Call Sent',           'boolean', false, NULL, 27, false, 60),
('{ws_dealls}', 'checkin_replied',           'Check-in Replied',           'boolean', false, NULL, 28, false, 60),
('{ws_dealls}', 'cross_sell_interested',     'Cross-sell Interested',      'boolean', false, NULL, 29, false, 60),
('{ws_dealls}', 'cross_sell_rejected',       'Cross-sell Rejected',        'boolean', false, NULL, 30, false, 60),
('{ws_dealls}', 'referral_sent',             'Referral Sent',              'boolean', false, NULL, 31, false, 60),
('{ws_dealls}', 'ren90_sent',                'REN90 Sent',                 'boolean', false, NULL, 32, false, 60),
('{ws_dealls}', 'ren60_sent',                'REN60 Sent',                 'boolean', false, NULL, 33, false, 60),
('{ws_dealls}', 'ren52_sent',                'REN52 Sent',                 'boolean', false, NULL, 34, false, 60),
('{ws_dealls}', 'ren45_sent',                'REN45 Sent',                 'boolean', false, NULL, 35, false, 60);
-- ... (add more as needed from Excel spec)
```

## KantorKu (HRIS Product)

```sql
-- Same structure but different custom fields relevant to HRIS:
INSERT INTO custom_field_definitions (workspace_id, field_key, field_label, field_type, is_required, options, sort_order, visible_in_table, column_width) VALUES
('{ws_kantorku}', 'hc_size',         'Jumlah Karyawan',    'number',  true,  NULL, 1, true, 100),
('{ws_kantorku}', 'industry',        'Industri',           'text',    false, NULL, 2, true, 120),
('{ws_kantorku}', 'plan_type',       'Plan',               'select',  false, '["Basic","Pro","Enterprise"]', 3, true, 90),
('{ws_kantorku}', 'current_system',  'Sistem HR Saat Ini', 'text',    false, NULL, 4, true, 120),
('{ws_kantorku}', 'contract_value',  'Nilai Kontrak',      'number',  false, NULL, 5, true, 120),
('{ws_kantorku}', 'nps_score',       'NPS Score',          'number',  false, NULL, 6, true, 70),
('{ws_kantorku}', 'usage_score',     'Usage Score',        'number',  false, NULL, 7, true, 80);
-- ... sent flags same pattern as Dealls
```

## Contoh Custom Fields untuk Industri Lain

### Logistik
```sql
('{ws_logistik}', 'fleet_size',     'Jumlah Armada',      'number', true, NULL, 1, true, 100),
('{ws_logistik}', 'route_count',    'Jumlah Rute',        'number', false, NULL, 2, true, 80),
('{ws_logistik}', 'avg_delivery',   'Avg Delivery/Day',   'number', false, NULL, 3, true, 100),
('{ws_logistik}', 'coverage_area',  'Area Coverage',      'select', false, '["Jabodetabek","Jawa","Nasional","Internasional"]', 4, true, 120);
```

### Retail/F&B
```sql
('{ws_retail}', 'store_count',      'Jumlah Outlet',      'number', true, NULL, 1, true, 100),
('{ws_retail}', 'monthly_revenue',  'Revenue Bulanan',    'number', false, NULL, 2, true, 120),
('{ws_retail}', 'pos_system',       'Sistem POS',         'text',   false, NULL, 3, true, 120),
('{ws_retail}', 'franchise',        'Franchise?',         'boolean', false, NULL, 4, true, 80);
```

Ini menunjukkan bahwa dengan JSONB custom fields, **industri apapun bisa pakai Master Data yang sama** tanpa perlu ALTER TABLE.
