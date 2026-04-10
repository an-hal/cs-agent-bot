-- Rollback: Remove resource_type column from activity_log

DROP INDEX IF EXISTS idx_al_resource_type;

ALTER TABLE activity_log DROP COLUMN IF EXISTS resource_type;
