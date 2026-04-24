# Collections — Database Schema

## Tables

### `collections`

Meta table — 1 row per user-defined table.

```sql
CREATE TABLE collections (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  slug            VARCHAR(64) NOT NULL,           -- url-safe, unique per workspace
  name            VARCHAR(128) NOT NULL,          -- display name
  description     TEXT,
  icon            VARCHAR(8),                     -- emoji
  permissions     JSONB NOT NULL DEFAULT '{}',    -- { viewer: [], editor: [], admin: [] }
  created_by      UUID NOT NULL REFERENCES users(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at      TIMESTAMPTZ,

  UNIQUE (workspace_id, slug)
);

CREATE INDEX idx_collections_workspace ON collections(workspace_id) WHERE deleted_at IS NULL;
```

### `collection_fields`

Schema definition — 1 row per field per collection.

```sql
CREATE TABLE collection_fields (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  collection_id   UUID NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
  key             VARCHAR(64) NOT NULL,           -- snake_case, unique per collection
  label           VARCHAR(128) NOT NULL,
  type            VARCHAR(32) NOT NULL,           -- text|textarea|number|boolean|date|datetime|enum|multi_enum|url|email|link_client|file
  required        BOOLEAN NOT NULL DEFAULT false,
  options         JSONB NOT NULL DEFAULT '{}',    -- type-specific: { choices: [...], min, max, maxLength, accept }
  default_value   JSONB,
  "order"         INTEGER NOT NULL DEFAULT 0,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE (collection_id, key)
);

CREATE INDEX idx_fields_collection ON collection_fields(collection_id, "order");
```

### `collection_records`

Actual data — 1 row per record. Values stored as JSONB keyed by field `key`.

```sql
CREATE TABLE collection_records (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  collection_id   UUID NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
  data            JSONB NOT NULL DEFAULT '{}',    -- { field_key: value, ... }
  created_by      UUID NOT NULL REFERENCES users(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_records_collection ON collection_records(collection_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_records_data_gin ON collection_records USING GIN (data);
```

## Example Data

**Collection "Internal Events":**

```sql
INSERT INTO collections (workspace_id, slug, name, icon)
VALUES ('<ws-dealls>', 'internal-events', 'Internal Events', '🎉');

INSERT INTO collection_fields (collection_id, key, label, type, required, options, "order") VALUES
  ('<c1>', 'title',      'Event Title',  'text',     true, '{"maxLength": 128}', 1),
  ('<c1>', 'date',       'Date',         'date',     true, '{}',                 2),
  ('<c1>', 'location',   'Location',     'text',     false,'{}',                 3),
  ('<c1>', 'category',   'Category',     'enum',     true, '{"choices":["All-hands","Training","Social","Launch"]}', 4),
  ('<c1>', 'attendees',  'Attendees',    'number',   false,'{"min":0}',          5),
  ('<c1>', 'organizer',  'Organizer',    'link_client', false, '{}',              6);

INSERT INTO collection_records (collection_id, data, created_by) VALUES
  ('<c1>', '{"title":"Q2 All-Hands","date":"2026-05-15","category":"All-hands","attendees":42}', '<user>');
```

## Constraints & Validation

- `slug` must match `^[a-z0-9-]{3,64}$`
- `field.key` must match `^[a-z][a-z0-9_]{0,63}$`
- `field.type` must be in allowed enum (backend enforces)
- `record.data` must only contain keys defined in `collection_fields` for that collection (validated at write time)
- Required fields must be non-null at insert

## Audit

All schema changes (create/alter/delete collection, add/remove/alter field) are logged to `activity_log` with `entity_type='collection'`, `entity_id=<collection.id>`.

Record CRUD operations also logged with `entity_type='collection_record'`.

## Soft Delete

Both `collections` and `collection_records` use `deleted_at` soft delete. Hard delete requires admin + approval.
