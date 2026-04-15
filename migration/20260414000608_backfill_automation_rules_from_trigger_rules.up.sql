-- feat/06-workflow-engine: backfill legacy trigger_rules → automation_rules
-- Maps existing trigger_rules rows (rule_id is VARCHAR PK) to the new table.
-- Only migrates rows that have rule_group and flag_key set.
-- legacy_trigger_rule_id stores the VARCHAR rule_id cast to UUID (via trigger_rules.rule_id).
-- NOTE: trigger_rules.rule_id is VARCHAR, not UUID — we store it in a separate text column
-- and keep legacy_trigger_rule_id NULL until a proper UUID FK can be established.

-- Add a text column to preserve the original string rule_id for reference
ALTER TABLE automation_rules ADD COLUMN IF NOT EXISTS legacy_rule_id_text VARCHAR(100);

INSERT INTO automation_rules (
  workspace_id,
  rule_code,
  trigger_id,
  template_id,
  role,
  phase,
  timing,
  condition,
  stop_if,
  sent_flag,
  channel,
  status,
  legacy_rule_id_text,
  created_at
)
SELECT
  COALESCE(workspace_id::uuid, (SELECT id FROM workspaces WHERE is_holding = FALSE LIMIT 1)),
  tr.rule_id,                               -- rule_code = existing rule_id
  tr.rule_id,                               -- trigger_id = same
  tr.template_id,
  COALESCE(LOWER(SPLIT_PART(tr.rule_group, '-', 1)), 'ae'),  -- derive role from group prefix
  COALESCE(UPPER(tr.rule_group), 'P0'),
  'Migrated',
  COALESCE(tr.condition::text, 'true'),
  '-',
  tr.flag_key,
  'whatsapp',
  CASE WHEN tr.active THEN 'active' ELSE 'paused' END,
  tr.rule_id,
  tr.created_at
FROM trigger_rules tr
WHERE tr.rule_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM automation_rules ar WHERE ar.rule_code = tr.rule_id
      AND ar.workspace_id = COALESCE(tr.workspace_id::uuid, (SELECT id FROM workspaces WHERE is_holding = FALSE LIMIT 1))
  )
ON CONFLICT (workspace_id, rule_code) DO NOTHING;
