-- feat/06-workflow-engine: seed 4 default workflows for every non-holding workspace

DO $$
DECLARE
  ws RECORD;
  wf_sdr UUID := '4fc22c98-1e3b-4901-aa86-9f81b33354d2';
  wf_bd  UUID := '0c85261e-277c-4143-93b3-bb6714eaff08';
  wf_ae  UUID := '406e6b25-37f6-4531-aade-aa42df2d52a3';
  wf_cs  UUID := '01400f6a-cdc9-43a0-8409-b96e316bec91';
BEGIN
  FOR ws IN SELECT id FROM workspaces WHERE is_holding = FALSE LOOP

    -- SDR Lead Outreach
    INSERT INTO workflows (id, workspace_id, name, icon, slug, description, status, stage_filter, created_by)
    VALUES (
      gen_random_uuid(),
      ws.id,
      'SDR Lead Outreach',
      '📞',
      wf_sdr::text,
      'Email + WA multi-channel outreach for LEAD and DORMANT records',
      'active',
      ARRAY['LEAD','DORMANT'],
      'system'
    ) ON CONFLICT (workspace_id, slug) DO NOTHING;

    -- BD Deal Closing
    INSERT INTO workflows (id, workspace_id, name, icon, slug, description, status, stage_filter, created_by)
    VALUES (
      gen_random_uuid(),
      ws.id,
      'BD Deal Closing',
      '🤝',
      wf_bd::text,
      'Prospect follow-up and deal closing pipeline',
      'active',
      ARRAY['PROSPECT'],
      'system'
    ) ON CONFLICT (workspace_id, slug) DO NOTHING;

    -- AE Client Lifecycle
    INSERT INTO workflows (id, workspace_id, name, icon, slug, description, status, stage_filter, created_by)
    VALUES (
      gen_random_uuid(),
      ws.id,
      'AE Client Lifecycle',
      '🏆',
      wf_ae::text,
      'Onboarding, check-in, renewal negotiation, invoice, overdue, expansion, cross-sell',
      'active',
      ARRAY['CLIENT'],
      'system'
    ) ON CONFLICT (workspace_id, slug) DO NOTHING;

    -- CS Customer Support
    INSERT INTO workflows (id, workspace_id, name, icon, slug, description, status, stage_filter, created_by)
    VALUES (
      gen_random_uuid(),
      ws.id,
      'CS Customer Support',
      '🎧',
      wf_cs::text,
      'Event-driven customer support and escalation for active clients',
      'active',
      ARRAY['CLIENT'],
      'system'
    ) ON CONFLICT (workspace_id, slug) DO NOTHING;

    -- Seed pipeline tabs for AE workflow
    INSERT INTO pipeline_tabs (workflow_id, tab_key, label, icon, filter, sort_order)
    SELECT wf.id, t.tab_key, t.label, t.icon, t.filter, t.sort_order
    FROM workflows wf
    CROSS JOIN (VALUES
      ('semua',     'Semua Client',   '📋', 'all',       0),
      ('aktif',     'Bot Aktif',      '🟢', 'bot_active',1),
      ('renewal',   'Renewal',        '📅', 'expiry:30', 2),
      ('perhatian', 'Perhatian',      '⚠️',  'risk',      3)
    ) AS t(tab_key, label, icon, filter, sort_order)
    WHERE wf.workspace_id = ws.id AND wf.slug = wf_ae::text
    ON CONFLICT (workflow_id, tab_key) DO NOTHING;

    -- Seed pipeline stats for AE workflow
    INSERT INTO pipeline_stats (workflow_id, stat_key, label, metric, color, border, sort_order)
    SELECT wf.id, s.stat_key, s.label, s.metric, s.color, s.border, s.sort_order
    FROM workflows wf
    CROSS JOIN (VALUES
      ('total',   'Total Client',   'count',           'text-emerald-400', 'border-emerald-500/20', 0),
      ('revenue', 'Total Revenue',  'sum:final_price',  'text-brand-400',   'border-brand-400/20',   1),
      ('renewal', 'Renewal <=30d',  'count:expiry:30',  'text-amber-400',   'border-amber-500/20',   2),
      ('risk',    'Perhatian',      'count:risk',       'text-rose-400',    'border-rose-500/20',    3)
    ) AS s(stat_key, label, metric, color, border, sort_order)
    WHERE wf.workspace_id = ws.id AND wf.slug = wf_ae::text
    ON CONFLICT (workflow_id, stat_key) DO NOTHING;

    -- Seed pipeline columns for AE workflow
    INSERT INTO pipeline_columns (workflow_id, column_key, field, label, width, visible, sort_order)
    SELECT wf.id, c.column_key, c.field, c.label, c.width, c.visible, c.sort_order
    FROM workflows wf
    CROSS JOIN (VALUES
      ('Company_Name',   'company_name',    'Company',   220, TRUE,  0),
      ('Stage',          'stage',           'Stage',      90, TRUE,  1),
      ('Bot_Active',     'bot_active',      'Bot',        60, TRUE,  2),
      ('Risk_Flag',      'risk_flag',       'Risk',       70, TRUE,  3),
      ('Payment_Status', 'payment_status',  'Payment',   100, TRUE,  4),
      ('Days_to_Expiry', 'days_to_expiry',  'Expiry',     80, TRUE,  5),
      ('Final_Price',    'final_price',     'ACV',       120, TRUE,  6)
    ) AS c(column_key, field, label, width, visible, sort_order)
    WHERE wf.workspace_id = ws.id AND wf.slug = wf_ae::text
    ON CONFLICT (workflow_id, column_key) DO NOTHING;

    -- Seed pipeline tabs for SDR workflow
    INSERT INTO pipeline_tabs (workflow_id, tab_key, label, icon, filter, sort_order)
    SELECT wf.id, t.tab_key, t.label, t.icon, t.filter, t.sort_order
    FROM workflows wf
    CROSS JOIN (VALUES
      ('semua',    'Semua Lead',    '📋', 'all',           0),
      ('lead',     'Lead',          '🔵', 'stage:LEAD',    1),
      ('dormant',  'Dormant',       '😴', 'stage:DORMANT', 2),
      ('aktif',    'Bot Aktif',     '🟢', 'bot_active',    3)
    ) AS t(tab_key, label, icon, filter, sort_order)
    WHERE wf.workspace_id = ws.id AND wf.slug = wf_sdr::text
    ON CONFLICT (workflow_id, tab_key) DO NOTHING;

    -- Seed pipeline stats for SDR workflow
    INSERT INTO pipeline_stats (workflow_id, stat_key, label, metric, color, border, sort_order)
    SELECT wf.id, s.stat_key, s.label, s.metric, s.color, s.border, s.sort_order
    FROM workflows wf
    CROSS JOIN (VALUES
      ('total',   'Total Lead',   'count',              'text-emerald-400', 'border-emerald-500/20', 0),
      ('aktif',   'Bot Aktif',    'count:bot_active',   'text-brand-400',   'border-brand-400/20',   1),
      ('lead',    'Lead',         'count:stage:LEAD',   'text-blue-400',    'border-blue-500/20',    2),
      ('dormant', 'Dormant',      'count:stage:DORMANT','text-gray-400',    'border-gray-500/20',    3)
    ) AS s(stat_key, label, metric, color, border, sort_order)
    WHERE wf.workspace_id = ws.id AND wf.slug = wf_sdr::text
    ON CONFLICT (workflow_id, stat_key) DO NOTHING;

  END LOOP;
END $$;
