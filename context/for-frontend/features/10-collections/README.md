# feat/10 — Collections

User-defined generic tables + field schemas + records. Think "mini Airtable"
scoped per workspace.

## Status

**✅ 90%** — CRUD + field validation + checker-maker for schema changes +
bulk records. Data migration engine on schema change still deferred.

## Scale limits

- Max 50 collections per workspace
- Max 30 fields per collection
- Max 10,000 records per collection
- Max 500 distinct values in a single distinct-query

## Collections CRUD

```
GET    /collections                             # list all
POST   /collections                             # create (goes through approval)
GET    /collections/{id}                        # detail incl. fields
PATCH  /collections/{id}                        # rename / update metadata
DELETE /collections/{id}                        # soft delete (goes through approval)
```

Create body:
```json
{
  "slug": "products",
  "name": "Products",
  "description": "Product catalog",
  "icon": "box",
  "permissions": {}
}
```

POST + DELETE return `202 Accepted` with an `ApprovalRequest` (type
`collection_schema_change`). Apply via:
```
POST /approvals/{id}/apply
```
or the per-feature alias:
```
POST /collections/approvals/{approval_id}/approve
```

## Field schema

```
POST   /collections/{id}/fields                         # add field (approval-gated)
PATCH  /collections/{id}/fields/{field_id}              # update (non-destructive changes only)
DELETE /collections/{id}/fields/{field_id}              # remove (approval-gated)
```

Add field body:
```json
{
  "key": "price",                         // snake_case; must be unique in collection
  "label": "Price (IDR)",
  "type": "number",                       // see types below
  "required": true,
  "options": {"min": 0, "max": 99999999},
  "default_value": null,
  "order": 1
}
```

### Supported field types

| Type | Notes |
|---|---|
| `text` | Short string |
| `textarea` | Long string |
| `number` | Float; `options.min/max` supported |
| `boolean` | true/false |
| `date` | `YYYY-MM-DD` |
| `datetime` | RFC3339 |
| `enum` | Single choice; `options.choices: [...]` required |
| `multi_enum` | Array of choices; `options.choices: [...]` |
| `url` | Validated scheme + host |
| `email` | Validated with RFC5322 parser |
| `link_client` | References `master_data.id`; validated in workspace |
| `file` | Opaque string (URL/ref); BE doesn't store blobs |

## Records

```
GET    /collections/{id}/records?limit=50&offset=0
GET    /collections/{id}/records/distinct?field=category
POST   /collections/{id}/records
PATCH  /collections/{id}/records/{record_id}
DELETE /collections/{id}/records/{record_id}
POST   /collections/{id}/records/bulk                  # batch ops
```

Create record body:
```json
{
  "data": {
    "price": 150000,
    "category": "subscription",
    "active": true
  }
}
```

BE validates `data` against the collection's field schema:
- Required fields must be present + non-null
- Type match (string for text/email/url, number for number, bool, array for multi_enum, etc.)
- `enum`/`multi_enum` values must be in `options.choices`
- `number` respects `options.min/max`
- `date` must match `YYYY-MM-DD`
- `datetime` must be RFC3339
- `url` must have scheme + host
- `email` must parse per RFC5322
- `link_client` value must exist in `master_data` in this workspace

On validation error: `422 VALIDATION_ERROR` with `errors` map keyed by field.

### Bulk ops
```
POST /collections/{id}/records/bulk
{
  "ops": [
    {"op": "create", "data": {...}},
    {"op": "update", "id": "uuid", "data": {...}},
    {"op": "delete", "id": "uuid"}
  ]
}
```

Atomic per-row (one op fails → skip + return error in the per-row result).

### Distinct values (filter builder)
```
GET /collections/{id}/records/distinct?field=category
→ {"data": ["subscription", "one-off", "addon"]}
```

Capped at `MaxDistinctValues = 500`.

## Filter DSL for list queries

Query params support the shared filter DSL (see `pkg/filterdsl`):

```
GET /collections/{id}/records?filter=category:eq:subscription&filter=price:gte:100000&sort=-updated_at
```

Operators: `eq`, `neq`, `gt`, `gte`, `lt`, `lte`, `in`, `between`, `contains`.

## FE UX

**Collection list page** (left sidebar):
- Grid of collection cards (icon + name + record count)
- "New collection" button → approval flow
- Click card → records table

**Records table** (datagrid style):
- Columns match field schema (order, width per user_preferences)
- Inline edit for simple fields
- Row detail drawer for complex records
- Filter bar with DSL chips
- Bulk select + delete/update

**Schema editor** (collection settings):
- Drag-to-reorder fields
- Field type picker with options UI (choices editor for enum, min/max for number)
- Add/delete fields → approval flow
- Readonly changelog from `collection_schema_change` approvals

**Data migration on schema change:**
Currently not automatic — BE doesn't migrate records when field `type`
changes. FE should:
- Block destructive type changes (e.g. text → number) in UI, OR
- Show a warning + require admin confirmation, then rely on future BE
  support (or write a one-shot cleanup script).
