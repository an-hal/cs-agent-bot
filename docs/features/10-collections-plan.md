# Plan — feat/10-collections

> **Branch base**: `master` &nbsp;&nbsp;|&nbsp;&nbsp; **Migration range**: `20260417001000`–`001099` &nbsp;&nbsp;|&nbsp;&nbsp; **Spec dir**: `~/dealls/project-bumi-dashboard/context/for-backend/features/10-collections/`

## Scope

User-defined generic tables (Airtable/Notion-style) with runtime-mutable schemas. Three core tables (`collections`, `collection_fields`, `collection_records`), record values stored in JSONB, 12 field types supported, per-workspace scoping, checker-maker approval on schema changes, hard limits (50 collections/workspace, 30 fields/collection, 10k records/collection).

**Non-goals** (spec §Non-Goals): cross-collection relations, formula fields, views/saved filters, real-time collab, record version history, workflow triggers from collection data.

**Business invariants:**
- Collections **MUST NOT** be referenced by Workflow Engine, Invoice, or pipeline logic.
- Schema mutations (create/delete collection, add/delete field) go through `approval_requests` with `request_type='collection_schema_change'`. Record CRUD is direct but audit-logged.
- `link_client` is the only one-way bridge to Data Master (`clients`/`master_data`) — Clients never reference Collections.

**Read first**: `01-overview.md`, `02-database-schema.md`, `03-api-endpoints.md`, `04-progress.md`, `00-shared/05-checker-maker.md`, `00-shared/01-filter-dsl.md`.

> **Existing repo has** `approval_requests` (VARCHAR `request_type`), `activity_log`, `pkg/xlsxexport`, `pkg/xlsximport`, squirrel + timeout-ctx repo pattern. **REUSE all of them.**

## Migrations

| # | File | Purpose |
|---|---|---|
| 1000 | `create_collections.{up,down}.sql` | `collections` table per spec §02; `UNIQUE(workspace_id, slug) WHERE deleted_at IS NULL` partial index; FK to `workspaces` ON DELETE CASCADE; `permissions` JSONB |
| 1001 | `create_collection_fields.{up,down}.sql` | `collection_fields` table; `UNIQUE(collection_id, key)`; quoted `"order"`; FK to `collections` ON DELETE CASCADE |
| 1002 | `create_collection_records.{up,down}.sql` | `collection_records` table; GIN index on `data`; partial index on `(collection_id)` WHERE `deleted_at IS NULL`; soft-delete |

No changes to `approval_requests` — `request_type` is already `VARCHAR(64)`.

## Entities

`internal/entity/collection.go`:

```go
type Collection struct {
    ID          string
    WorkspaceID string
    Slug        string
    Name        string
    Description string
    Icon        string
    Permissions map[string]any   // {viewer:[], editor:[], admin:[]}
    CreatedBy   string
    CreatedAt   time.Time
    UpdatedAt   time.Time
    DeletedAt   *time.Time
    RecordCount int              // computed on list
    FieldCount  int              // computed on list
    Fields      []CollectionField
}

type CollectionField struct {
    ID           string
    CollectionID string
    Key          string
    Label        string
    Type         string   // text|textarea|number|boolean|date|datetime|enum|multi_enum|url|email|link_client|file
    Required     bool
    Options      map[string]any   // {choices, min, max, maxLength, accept}
    DefaultValue any
    Order        int
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

type CollectionRecord struct {
    ID           string
    CollectionID string
    Data         map[string]any
    CreatedBy    string
    CreatedByRef *ActorRef         // resolved for list response
    CreatedAt    time.Time
    UpdatedAt    time.Time
    DeletedAt    *time.Time
}
```

`FieldType` constants and `IsValidFieldType(s string) bool` live alongside.

## Repositories

```
internal/repository/
  collection_repo.go          // Collection CRUD — List, GetByID, GetBySlug, Create, Update, SoftDelete, CountByWorkspace
  collection_field_repo.go    // Field CRUD — ListByCollection, GetByID, GetByKey, Create, Update, Delete (hard), CountByCollection
  collection_record_repo.go   // Record CRUD — List(opts), Get, Count, CountByCollection, Create, Update, SoftDelete,
                              //                BulkSoftDelete, BulkUpdate, Distinct(fieldKey, fieldType, filter, limit)
```

All per-workspace scoping enforced: collection_repo joins on workspace_id; field and record ops take a pre-validated `CollectionID` that the usecase resolved against workspace first.

## Usecases

`internal/usecase/collection/`:

- `usecase.go` — Interface: full public surface (collection CRUD, field CRUD, record CRUD + bulk, distinct)
- `schema_validator.go` — `ValidateRecordData(fields []CollectionField, data map[string]any, strict bool) (fieldErrs map[string]string, err error)` — handles every field type, required check, enum choice set, numeric range, URL/email regex, maxLength, strict unknown-key rejection
- `filter.go` — Translate filter DSL to JSONB SQL (`data->>'field_key'`). Only predicates used by UI: `in`, `=`, `!=`, `prefix` (for date ISO collapse). Rejects unknown field keys (loads schema first).
- `sort.go` — per-type ORDER BY on JSONB path with correct casts (`::numeric`, `::boolean`, `::date`). Built-in `created_at`/`updated_at` use real columns. Unknown keys rejected.
- `distinct.go` — per-type bucketing: flatten `multi_enum`, collapse `date`/`datetime` to `YYYY-MM-DD`, numeric sort for `number`, lex for the rest. Hard cap limit=500.
- `approval.go` — Build approval payloads (create_collection, delete_collection, add_field, delete_field, update_meta_and_fields) and dispatch `ApplyCollectionSchemaChange(ctx, workspaceID, approvalID, checkerEmail)` to execute atomically.
- `import_export.go` — xlsx/csv dry-run + confirm-token apply; enforce 10k cap.

Hard limits enforced in usecase: `CountByWorkspace < 50` before Create, `CountByCollection(fields) < 30` before AddField, `CountByCollection(records) + incoming < 10000` before insert.

## Handlers

`internal/delivery/http/dashboard/collection_handler.go` + `collection_record_handler.go` (split for file-size hygiene).

Follows `custom_field_handler.go` pattern: swag annotations, `StandardSuccess`, `apperror`-based error paths.

## Routes

```go
col := api.Group("/collections")
col.Handle(http.MethodGet,    "",                            wsRequired(jwtAuth(collectionH.List)))
col.Handle(http.MethodPost,   "",                            wsRequired(jwtAuth(collectionH.Create)))          // 202 approval
col.Handle(http.MethodGet,    "/{id}",                       wsRequired(jwtAuth(collectionH.Get)))
col.Handle(http.MethodPatch,  "/{id}",                       wsRequired(jwtAuth(collectionH.Update)))          // 200 or 202
col.Handle(http.MethodDelete, "/{id}",                       wsRequired(jwtAuth(collectionH.Delete)))          // 202
col.Handle(http.MethodPost,   "/{id}/fields",                wsRequired(jwtAuth(collectionH.AddField)))        // 202
col.Handle(http.MethodPatch,  "/{id}/fields/{field_id}",     wsRequired(jwtAuth(collectionH.UpdateField)))     // 200
col.Handle(http.MethodDelete, "/{id}/fields/{field_id}",     wsRequired(jwtAuth(collectionH.DeleteField)))     // 202
col.Handle(http.MethodPost,   "/{id}/approvals/{approval_id}/approve", wsRequired(jwtAuth(collectionH.ApplyApproval)))

col.Handle(http.MethodGet,    "/{id}/records",               wsRequired(jwtAuth(recordH.List)))
col.Handle(http.MethodGet,    "/{id}/records/distinct",      wsRequired(jwtAuth(recordH.Distinct)))
col.Handle(http.MethodPost,   "/{id}/records",               wsRequired(jwtAuth(recordH.Create)))
col.Handle(http.MethodPatch,  "/{id}/records/{record_id}",   wsRequired(jwtAuth(recordH.Update)))
col.Handle(http.MethodDelete, "/{id}/records/{record_id}",   wsRequired(jwtAuth(recordH.Delete)))
col.Handle(http.MethodPost,   "/{id}/records/bulk",          wsRequired(jwtAuth(recordH.Bulk)))
```

Import/export deferred to post-MVP to stay within scope; skeletons included with TODO markers.

## Approval Integration

- Maker creates collection → handler calls `collectionUC.RequestCreate(...)` → writes `approval_requests(request_type='collection_schema_change', payload={op:'create_collection', ...}, status='pending')` and returns 202 + `approval_id`.
- Checker calls `POST /api/collections/{id}/approvals/{approval_id}/approve` (or a future central approve endpoint) → `ApplyCollectionSchemaChange(ctx, wsID, approvalID, checkerEmail)` dispatches on `payload.op` and executes the mutation + marks approval approved + writes `activity_log`. Matches the `invoice.ApplyMarkPaid` pattern established by feat/07.

## Validation Strategy

- `slug` regex: `^[a-z0-9-]{3,64}$`
- `field.key` regex: `^[a-z][a-z0-9_]{0,63}$`
- `field.type` must be in the whitelist (12 types)
- `record.data` keys MUST be a subset of declared `field.key`s (strict mode)
- Required fields present; enum choices honored; numeric in range; URL/email regex; `link_client` FK check against `master_data` table in same workspace

## Test Matrix

**Unit (`internal/usecase/collection/*_test.go`):**

| File | Covers |
|---|---|
| `schema_validator_test.go` | All 12 field types × required/optional × positive/negative cases; unknown-key rejection |
| `filter_test.go` | `in`, `=`, `prefix`; unknown key rejected |
| `sort_test.go` | per-type cast; created_at/updated_at fallback |
| `distinct_test.go` | multi_enum flatten, date collapse, truncated flag |
| `usecase_test.go` | hard limits (51st collection, 31st field, 10001st record); slug uniqueness; approval path for schema writes; audit log called on record CRUD; workspace isolation |
| `approval_test.go` | ApplyCollectionSchemaChange dispatch for all 5 op values |

**Handler (`internal/delivery/http/dashboard/collection_handler_test.go`):**

- 400 missing body
- 202 on create/delete collection + delete field (approval flow)
- 200 on record CRUD + field meta-only update
- 404 on unknown collection id
- 422-style validation-error JSON on schema mismatch

**Target:** 80%+ on new usecase + handler files.

## Risks

| Level | Risk | Mitigation |
|---|---|---|
| HIGH | SQL injection via JSONB field-key interpolation in sort/filter/distinct | Always load + validate field keys against schema before building SQL; whitelist `created_at`/`updated_at`; no raw user string enters SQL fragments |
| HIGH | Central approval endpoint not yet wired (feat/04 scope) | Expose per-feature `ApplyCollectionSchemaChange` same as `ApplyMarkPaid`; add a feature-local approve endpoint to prove the flow end-to-end |
| MED | 10k hard limit on bulk insert | Count-before-insert guard in `usecase.BulkCreateRecords`; reject preview if overflow |
| MED | `link_client` FK check cost on bulk | Batch-lookup client IDs in one query per import/bulk op; document in code |
| LOW | Distinct endpoint DoS on free-text | Hard cap limit=500, `truncated=true` in response |
| LOW | Slug collision across soft-deleted collections | Partial unique index `WHERE deleted_at IS NULL` (migration handles) |

## Definition of Done

- [ ] Branch `feat/10-collections` pushed; PR-ready
- [ ] `docs/features/10-collections-plan.md` present (this file)
- [ ] 3 migrations up + down
- [ ] 16 endpoints implemented with swag annotations
- [ ] Schema-change endpoints return 202 with `approval_id`; approve flow applies atomically
- [ ] Record CRUD strict-validates against schema
- [ ] Per-workspace scoping in every query
- [ ] Hard limits enforced (50 / 30 / 10k)
- [ ] `make lint && make unit-test && make swag` all green
- [ ] 80%+ coverage on new usecase code
- [ ] Frontend context docs at `context/for-frontend/features/10-collections/`
