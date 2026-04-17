-- collection_records: dynamic data rows. All values live in `data` JSONB keyed by field.key.
-- Per spec ~/dealls/project-bumi-dashboard/context/for-backend/features/10-collections/02-database-schema.md.
CREATE TABLE IF NOT EXISTS collection_records (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    collection_id   UUID NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    data            JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by      TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ
);

-- Hot path: list active records per collection, newest first.
CREATE INDEX IF NOT EXISTS idx_collection_records_collection
    ON collection_records (collection_id, created_at DESC)
    WHERE deleted_at IS NULL;

-- GIN index supports JSONB containment + existence filters (spec §Scale Assumptions).
CREATE INDEX IF NOT EXISTS idx_collection_records_data_gin
    ON collection_records USING GIN (data);
