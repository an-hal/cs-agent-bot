# Collections — API Endpoints

All endpoints require `X-Workspace-ID` header and valid `auth_session` cookie. Scoping is always per-workspace.

## Collection CRUD

### `GET /collections`
List all collections in current workspace.

**Response:**
```json
{
  "data": [
    {
      "id": "uuid",
      "slug": "internal-events",
      "name": "Internal Events",
      "icon": "🎉",
      "record_count": 42,
      "field_count": 6,
      "created_at": "2026-04-14T10:00:00Z"
    }
  ]
}
```

### `GET /collections/{id}`
Get collection meta + full field schema.

**Response:**
```json
{
  "id": "uuid",
  "slug": "internal-events",
  "name": "Internal Events",
  "description": "...",
  "icon": "🎉",
  "fields": [
    {
      "id": "uuid",
      "key": "title",
      "label": "Event Title",
      "type": "text",
      "required": true,
      "options": { "maxLength": 128 },
      "order": 1
    }
  ],
  "permissions": { "viewer": [], "editor": [], "admin": [] }
}
```

### `POST /collections`
Create new collection. **Requires approval (checker-maker).**

**Request:**
```json
{
  "slug": "internal-events",
  "name": "Internal Events",
  "description": "Team events and all-hands",
  "icon": "🎉",
  "fields": [
    { "key": "title", "label": "Event Title", "type": "text", "required": true, "options": { "maxLength": 128 }, "order": 1 }
  ]
}
```

**Response:** `201 Created` — returns approval request ID if approval needed, or created collection if auto-approved.

### `PATCH /collections/{id}`
Update collection meta (name, description, icon, permissions). **Requires approval if schema (fields) changes.**

### `DELETE /collections/{id}`
Soft delete. **Requires approval (checker-maker).**

---

## Field CRUD

### `POST /collections/{id}/fields`
Add new field to existing collection. **Requires approval.**

**Request:**
```json
{
  "key": "category",
  "label": "Category",
  "type": "enum",
  "required": true,
  "options": { "choices": ["A", "B", "C"] },
  "order": 4
}
```

### `PATCH /collections/{id}/fields/{field_id}`
Rename label, reorder, change required flag. **Type change NOT allowed** — must delete + recreate.

### `DELETE /collections/{id}/fields/{field_id}`
Remove field. **Requires approval.** Removes the key from all existing records' `data` JSONB.

---

## Record CRUD

Low-risk — **no approval needed**, but audit-logged.

### `GET /collections/{id}/records`
List records with pagination, sort, filter.

**Query params:**
- `limit` (default 50, max 500)
- `offset`
- `sort` — field key + direction, e.g. `created_at:desc`, `data.title:asc`. Backend MUST support sort on **any declared field key** (via JSONB path on `data`) plus the built-in `created_at` / `updated_at`. Sort semantics per field `type`:
  - `number` → numeric
  - `boolean` → false < true
  - `date` / `datetime` → chronological
  - all other types → case-insensitive lex
- `filter` — Filter DSL (reuse `00-shared/01-filter-dsl.md`), evaluated against JSONB `data`. Frontend filter dropdowns emit `in` predicates (`data.category in ["A","B"]`), so backend MUST support `in` on any field key. For `date`/`datetime`, the UI collapses values to `YYYY-MM-DD` and emits `data.created_on prefix "2026-04-15"` (prefix match on ISO date).
- `search` — full-text over all text fields

**Response:**
```json
{
  "data": [
    {
      "id": "uuid",
      "data": { "title": "Q2 All-Hands", "date": "2026-05-15", "category": "All-hands" },
      "created_by": { "id": "uuid", "name": "Arief" },
      "created_at": "...",
      "updated_at": "..."
    }
  ],
  "meta": { "total": 42, "limit": 50, "offset": 0 }
}
```

### `GET /collections/{id}/records/distinct`
Returns distinct values per field — used by the frontend to populate per-column filter dropdowns when the field has no predefined `choices` (i.e. `text`, `number`, `date`, `url`, `email`, `link_client`, or `multi_enum` without schema choices).

**Query params:**
- `field` — required, the field `key` to collect
- `limit` — default 100, max 500 (hard cap to avoid DOM explosion on free-text columns)
- `filter` — optional Filter DSL, so the distinct set narrows with the user's other active filters

**Response:**
```json
{
  "field": "category",
  "values": ["Training", "All-hands", "Townhall"],
  "truncated": false
}
```

**Semantics:**
- Non-empty values only (skip `null`, `""`, empty arrays).
- `multi_enum` → flatten array items before collecting distinct.
- `date`/`datetime` → collapse to date-only (`YYYY-MM-DD`) so one day = one bucket.
- `number` → sort numerically ascending; other types → sort lex ascending.
- `truncated: true` when the unique set exceeded `limit` (UI shows "showing first N" hint).

**Why a dedicated endpoint:** MVP frontend currently derives distinct values client-side from the currently loaded page of records, which is fine for small collections (<500) but breaks once pagination kicks in — a dropdown built from page 1 will not include values that only exist on page 5. This endpoint fixes that at the source.

### `POST /collections/{id}/records`
Create new record. Backend validates `data` against `collection_fields` schema.

**Request:**
```json
{
  "data": { "title": "Q2 All-Hands", "date": "2026-05-15", "category": "All-hands", "attendees": 42 }
}
```

**Validation errors:**
```json
{
  "error": "validation_failed",
  "fields": {
    "title": "required",
    "category": "not in allowed choices"
  }
}
```

### `PATCH /collections/{id}/records/{record_id}`
Partial update. Only keys provided in request body are updated.

### `DELETE /collections/{id}/records/{record_id}`
Soft delete.

### `POST /collections/{id}/records/bulk`
Bulk operations:
```json
{ "op": "delete", "ids": ["uuid1", "uuid2"] }
{ "op": "update", "ids": ["uuid1"], "data": { "category": "Training" } }
```

---

## Import / Export

### `POST /collections/{id}/import`
Multipart file upload. Supports `.xlsx` and `.csv`. Header row must match field keys exactly.

**Query:** `mode=add_new | update_existing | replace_all`

**Response:** dry-run preview first, then actual apply with confirmation token.

### `GET /collections/{id}/export?format=xlsx|csv|json`
Download current records as file. Field schema included as first sheet (xlsx only).

---

## Schema Validation Rules (backend enforces)

1. All required fields must be present in record data.
2. Field values must match declared type:
   - `number` → valid number, within `min/max`
   - `date` → ISO 8601
   - `enum` → must be in `options.choices`
   - `url`/`email` → regex validation
   - `link_client` → must reference existing `clients.id` in same workspace
3. Unknown keys in record `data` → reject (strict mode).
4. String length limits from `options.maxLength`.

---

## Rate Limiting

- Schema writes (collection/field CRUD): 10 req/min per user
- Record CRUD: 120 req/min per user
- Bulk ops: 5 req/min per user
- Import: 2 req/min per user
