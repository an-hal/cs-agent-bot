DROP INDEX IF EXISTS idx_master_data_mutations_source;
ALTER TABLE IF EXISTS master_data_mutations DROP COLUMN IF EXISTS source;
