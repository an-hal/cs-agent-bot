-- Adds `source` to master_data_mutations so activity-log views can differentiate
-- dashboard edits vs bot actions vs bulk imports vs API callers (feat/08 §mutation coverage).
ALTER TABLE IF EXISTS master_data_mutations
    ADD COLUMN IF NOT EXISTS source VARCHAR(32) NOT NULL DEFAULT 'dashboard';

-- Backfill any existing rows with a reasonable default.
UPDATE master_data_mutations SET source = 'dashboard' WHERE source IS NULL OR source = '';

CREATE INDEX IF NOT EXISTS idx_master_data_mutations_source
    ON master_data_mutations (workspace_id, source, timestamp DESC);
