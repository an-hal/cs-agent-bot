DROP INDEX IF EXISTS idx_ar_resource;
ALTER TABLE approval_requests DROP COLUMN IF EXISTS resource_id;
