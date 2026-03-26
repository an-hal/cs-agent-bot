-- Drop indexes
DROP INDEX IF EXISTS idx_examples_deleted_at;
DROP INDEX IF EXISTS idx_examples_status;

-- Drop examples table
DROP TABLE IF EXISTS examples;
