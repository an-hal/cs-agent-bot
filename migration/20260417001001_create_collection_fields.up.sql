-- collection_fields: schema definition per collection. One row per field.
-- Per spec ~/dealls/project-bumi-dashboard/context/for-backend/features/10-collections/02-database-schema.md.
CREATE TABLE IF NOT EXISTS collection_fields (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    collection_id   UUID NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    key             VARCHAR(64) NOT NULL,
    label           VARCHAR(128) NOT NULL,
    -- One of: text|textarea|number|boolean|date|datetime|enum|multi_enum|url|email|link_client|file.
    -- Enforced at usecase layer (not via PG enum to allow future additions without migration).
    type            VARCHAR(32) NOT NULL,
    required        BOOLEAN NOT NULL DEFAULT false,
    options         JSONB NOT NULL DEFAULT '{}'::jsonb,
    default_value   JSONB,
    "order"         INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (collection_id, key)
);

CREATE INDEX IF NOT EXISTS idx_collection_fields_collection
    ON collection_fields (collection_id, "order");
