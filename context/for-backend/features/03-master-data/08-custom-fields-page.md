# Custom Fields Page — Full CRUD

Dedicated spec for the `/dashboard/{workspace}/custom-fields` page.

## Overview

Workspace admins define extra columns on the client record without a DB migration. Values live in `master_data.custom_fields` JSONB; definitions live in `custom_field_definitions` (already specified in `02-database-schema.md`).

FE source: `app/dashboard/[workspace]/custom-fields/page.tsx` + `lib/custom-fields-store.ts`.

## Supported Field Types

Must match the FE `FieldType` union exactly (otherwise the UI rejects server responses).

| Type | FE control | Stored JSONB shape | Notes |
|------|------------|--------------------|-------|
| `text` | `Input` | `string` | `regex_pattern` optional |
| `number` | `Input type="number"` | `number` | `min_value` / `max_value` optional |
| `boolean` | `Switch` | `boolean` | — |
| `date` | `DatePicker` | `string` ISO `YYYY-MM-DD` | — |
| `select` | `Select` (single) | `string` | `options` required |
| `multiselect` | `Select mode="multiple"` | `string[]` | `options` required |

> `text`/`number` can carry `options: []` but it's ignored. Only `select`/`multiselect` MUST have ≥1 option.

## API Endpoints

All scoped to `/{workspace_id}`. Auth required. Owner/admin role for mutations.

### `GET /master-data/field-definitions`

Returns all custom fields for the active workspace, ordered by `sort_order`.

```
Response 200:
{
  "data": [
    {
      "id": "uuid",
      "workspace_id": "uuid",
      "field_key": "legal_entity",
      "field_label": "Legal Entity",
      "field_type": "text",
      "is_required": false,
      "default_value": null,
      "description": "Registered company legal entity name",
      "options": null,
      "min_value": null,
      "max_value": null,
      "regex_pattern": null,
      "sort_order": 0,
      "visible_in_table": true,
      "column_width": 160,
      "created_at": "2026-04-21T10:00:00Z",
      "updated_at": "2026-04-21T10:00:00Z"
    }
  ]
}
```

### `POST /master-data/field-definitions`

Create a new custom field.

```
Request:
{
  "field_key": "legal_entity",           // snake_case, ^[a-z][a-z0-9_]*$, required
  "field_label": "Legal Entity",         // required
  "field_type": "text",                  // required, one of supported types
  "is_required": false,
  "default_value": null,
  "description": "Registered company legal entity name",
  "options": null,                       // required iff type = select|multiselect
  "visible_in_table": true
}

Response 201: (full field definition)
Response 400: { "error": "field_key must be snake_case (^[a-z][a-z0-9_]*$)" }
Response 409: { "error": "field_key already exists in this workspace" }
```

**Validation:**
- `field_key` unique per workspace (DB constraint already in 02-database-schema.md)
- `field_key` must match `^[a-z][a-z0-9_]*$` — enforced both FE and BE
- Reserved keys (rejected): any name that collides with a core `master_data` column (`company_id`, `stage`, `payment_status`, etc.) — backend maintains a denylist
- `field_key` is IMMUTABLE after creation (it's the JSONB key holding values). Rename via delete+recreate would lose existing values.

### `PATCH /master-data/field-definitions/{id}`

Update mutable attributes. `field_key` cannot be changed.

```
Request (all fields optional):
{
  "field_label": "Legal Entity Name",
  "is_required": true,
  "description": "...",
  "options": ["Option A", "Option B"],   // only for select/multiselect
  "visible_in_table": true,
  "column_width": 200
}

Response 200: (full updated definition)
Response 400: { "error": "field_key is immutable" }
```

**Type-change rule:** Changing `field_type` is **not allowed** via PATCH because existing values in `custom_fields` JSONB may not coerce (e.g. `text` → `number` breaks non-numeric values). To change type, delete the field and create a new one with a different key.

### `DELETE /master-data/field-definitions/{id}`

Soft-delete via `deleted_at` (add this column to the schema) — existing values in `master_data.custom_fields[field_key]` are **preserved** (not purged). Re-creating a field with the same key will resurface them.

```
Response 204: (no body)
```

**Hard-delete** is a separate operation (`DELETE /master-data/field-definitions/{id}?hard=true`) and requires an approval request (`bulk_import_master_data`-style — affects all rows). Out of scope for v1.

### `PUT /master-data/field-definitions/order`

Bulk reorder. FE sends the full ordered list of IDs; backend updates `sort_order` in one query.

```
Request:
{
  "ordered_ids": ["uuid-1", "uuid-2", "uuid-3"]
}

Response 200: { "updated": 3 }
```

## Value Handling During Record CRUD

When the backend receives a `POST` or `PATCH` on `/master-data/clients`:

1. Accept any keys in the request body's `custom_fields` object.
2. For each key present:
   - Look up the definition in `custom_field_definitions` (cached per workspace).
   - Coerce the value to the declared type (`number` ↔ float/int, `date` ↔ ISO string, `boolean` ↔ bool).
   - Validate against `is_required`, `min_value`/`max_value`, `options`, `regex_pattern`.
   - Reject the whole request on first validation error (not partial save).
3. Keys not in definitions are **rejected** (400). This prevents silent drift.
4. Required fields missing in `POST` → 400; in `PATCH` → preserve existing value.

## CSV Import Integration

When a row is imported via `POST /master-data/clients/import`:

1. Header names are matched against `field_label` first (case-insensitive), then `field_key`.
2. Unmatched headers surface as warnings in the import preview response — not rejected, just skipped.
3. Each value goes through the same coercion/validation as direct CRUD.
4. Errors accumulate per-row: `{ row: 5, field: "hc_size", error: "must be number, got 'abc'" }`. Partial success is allowed — valid rows are inserted even if others fail.

See `04-api-endpoints.md` `POST /master-data/clients/import` for the response envelope.

## UI Considerations (for backend alignment)

- FE reorders via up/down arrows calling `PUT /master-data/field-definitions/order` after each move. Debounced 300ms on the client.
- FE's `visible_in_table` toggle controls whether the column appears in Data Master. BE can skip returning non-visible fields in the `GET /master-data/clients` list response to save bandwidth — but custom_fields JSONB still contains them for the detail drawer.
- FE shows a confirmation modal on delete mentioning "existing data preserved". If backend hard-deletes by default, change the modal copy.

## Progress Mapping

FE as of 2026-04-21:
- Page UI: ✅ complete (`app/dashboard/[workspace]/custom-fields/page.tsx`)
- Store: ✅ complete (`lib/custom-fields-store.ts` — localStorage, backend-ready shape)
- Backend: ❌ not started (this spec is the contract)
