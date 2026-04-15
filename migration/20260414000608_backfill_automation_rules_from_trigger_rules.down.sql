-- Remove backfilled rows (identified by legacy_rule_id_text being set)
DELETE FROM automation_rules WHERE legacy_rule_id_text IS NOT NULL;
ALTER TABLE automation_rules DROP COLUMN IF EXISTS legacy_rule_id_text;
