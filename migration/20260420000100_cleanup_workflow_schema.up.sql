-- Phase A: Schema cleanup for workflow engine tech debt

-- 1. Drop broken legacy FK column (trigger_rules.rule_id is VARCHAR, not UUID)
ALTER TABLE automation_rules DROP COLUMN IF EXISTS legacy_trigger_rule_id;

-- 2. Rename legacy_rule_id_text → legacy_rule_id for clarity (no FK, text reference only)
ALTER TABLE automation_rules RENAME COLUMN legacy_rule_id_text TO legacy_rule_id;

-- 3. Make workflows.slug NOT NULL — slug is always generated on insert
UPDATE workflows SET slug = LOWER(REPLACE(name, ' ', '-')) WHERE slug IS NULL;
ALTER TABLE workflows ALTER COLUMN slug SET NOT NULL;

-- 4. Add missing index for rule change log per-rule timeline queries
CREATE INDEX IF NOT EXISTS idx_rcl_rule_edited
  ON rule_change_logs(rule_id, edited_at DESC);
