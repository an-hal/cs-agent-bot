-- Revert Phase A schema cleanup

-- 4. Drop added index
DROP INDEX IF EXISTS idx_rcl_rule_edited;

-- 3. Allow NULL slug again
ALTER TABLE workflows ALTER COLUMN slug DROP NOT NULL;

-- 2. Rename back
ALTER TABLE automation_rules RENAME COLUMN legacy_rule_id TO legacy_rule_id_text;

-- 1. Re-add legacy FK column
ALTER TABLE automation_rules ADD COLUMN legacy_trigger_rule_id VARCHAR(50) REFERENCES trigger_rules(rule_id) ON DELETE SET NULL;
