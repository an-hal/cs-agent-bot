# feat/03 — Master Data (Clients)

Workspace-scoped CRUD for clients + custom fields + import/export + stage
transitions + reactivation + BD→AE discovery handoff.

## Status

**✅ 100%** — full CRUD, preview dedup, update_existing import, reactivation,
handoff, mutation logging, custom fields, approval for delete.

## Core CRUD

```
GET    /master-data/clients?stage=client&limit=50&offset=0
POST   /master-data/clients                           # create
GET    /master-data/clients/{id}                      # get by id (master_data.id UUID)
PUT    /master-data/clients/{id}                      # patch (bot-restricted fields enforced)
DELETE /master-data/clients/{id}                      # soft-delete via approval (returns 202 + approval_id)
POST   /master-data/clients/{id}/transition           # stage transition (direct or gated)
POST   /master-data/query                             # flexible filter DSL
GET    /master-data/stats
GET    /master-data/attention                         # clients needing attention (low NPS, overdue, etc.)
GET    /master-data/mutations?limit=100&since=2026-04-01
```

Filter chips on `/clients`:
- `stage` — `lead|prospect|client|dormant`
- `search` — fuzzy over name + company_id
- `risk_flag` — `None|Low|Mid|High`
- `bot_active` — `true|false`
- `payment_status`
- `expiry_within` — days to expiry

## Import / Export

### Preview (dedup, no writes)
```
POST /master-data/clients/import/preview
Content-Type: multipart/form-data
Fields: file (xlsx), mode (add_new|update_existing)
```

Response:
```json
{
  "mode": "add_new",
  "total_rows": 50,
  "new": 42,
  "duplicates": 5,
  "invalid": 3,
  "rows": [
    {"row": 2, "status": "new", "company_id": "ACME-001", "company_name": "Acme"},
    {"row": 3, "status": "duplicate", "company_id": "BETA-001", "existing_id": "uuid"},
    {"row": 4, "status": "invalid", "error": "company_id and company_name are required"}
  ]
}
```

FE shows this in a confirmation modal before the user submits the real import.

### Commit import (creates approval)
```
POST /master-data/clients/import
Content-Type: multipart/form-data
Fields: file (xlsx), mode (add_new|update_existing)
```

Returns 202 with an `ApprovalRequest` (type `bulk_import_master_data`).
Another user must approve + apply — for bulk_import, the application endpoint
is per-feature (not via central dispatcher because rows are too large for
JSONB payload):

```
POST /data-master/import/commit/{approval_id}
Content-Type: multipart/form-data
Fields: file (xlsx)                  # re-upload the same file for the payload
```

### Export / template
```
GET /master-data/clients/export      # current workspace as xlsx
GET /master-data/clients/template    # template xlsx for import
```

**CSV injection guard:** Export + import both sanitize cells starting with
`=|+|-|@|\t|\r` by prepending `'`.

## Custom fields

Per-workspace user-defined fields stored in `master_data.custom_fields` JSONB.

```
GET    /master-data/field-definitions
POST   /master-data/field-definitions
PUT    /master-data/field-definitions/reorder
PUT    /master-data/field-definitions/{id}
DELETE /master-data/field-definitions/{id}
```

Field def shape — similar to collections (see
[../10-collections/](../10-collections/)).

## Stage transition

Two paths: direct (works today) or gated (via approval — new).

### Direct
```
POST /master-data/clients/{id}/transition
{
  "new_stage": "client",
  "reason": "first payment confirmed",
  "updates": {},
  "custom_field_updates": {"paid_amount": 12000000}
}
```

### Gated (via approval)
```
POST /master-data/clients/{id}/transition-request
→ returns ApprovalRequest (type stage_transition)

# Another user approves via central dispatcher:
POST /approvals/{approval_id}/apply
```

## BD→AE handoff (auto)

When a stage transition moves `prospect → client`, BE auto-copies 30 BD
discovery fields from `custom_fields.*` into `custom_fields.ae_*`. Non-
destructive: only fills AE keys that are empty/missing.

Field map (excerpt):
```
hc_size                 → ae_hc_size
industry                → ae_industry
current_pain_point      → ae_pain_point
competitor_mentioned    → ae_competitor
dm_name, dm_role, ...   → ae_dm_*
bants_*                 → ae_bants_*
last_fireflies_id       → ae_last_fireflies_id
expected_close_date     → ae_expected_close_date
(… 30 total — see master_data/handoff.go)
```

FE: after transition, refetch the client to see the populated AE fields.

## Reactivation

See [../00-shared/10-reactivation.md](../00-shared/10-reactivation.md).

## Mutation log

Every write (create, edit, delete, stage_transition, reactivate, handoff)
appends a `master_data_mutations` row tagged with `source`
(`dashboard|bot|import|api|reactivation|handoff`).

```
GET /master-data/mutations?since=2026-04-20&limit=100
```

FE uses this for:
- Per-client timeline ("who changed what when")
- Activity feed (also available in unified form at `/activity-log/feed`)

## Data model

See [../../05-data-models.md](../../05-data-models.md#client--masterdata)
for the full `MasterData` shape (40+ fields + custom_fields JSONB).
