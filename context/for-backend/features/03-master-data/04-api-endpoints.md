# API Endpoints — Master Data

> **Penamaan resmi: "Master Data"** (bukan "Data Master").
> Tabel DB tetap `master_data` (snake_case). URL path: `/master-data/`.

## Base URL
```
{BACKEND_API_URL}/api/v1
```

All endpoints require `Authorization: Bearer {token}` header.
Workspace-scoped endpoints require `X-Workspace-ID: {uuid}` header.

---

## 1. Master Data CRUD

### GET `/master-data/clients`
List records dengan pagination + filter + search.

```
Query params:
  ?offset=0&limit=20
  &stage=LEAD,PROSPECT              (optional, comma-separated)
  &search=keyword                   (searches company_name, pic_name, company_id)
  &risk_flag=High                   (optional)
  &bot_active=true                  (optional)
  &payment_status=Terlambat         (optional)
  &expiry_within=30                 (optional, days — contract_end within N days)
  &sort_by=updated_at               (optional, default: updated_at)
  &sort_dir=desc                    (optional: asc/desc)

Response 200:
{
  "data": [
    {
      "id": "uuid",
      "company_id": "DE-001",
      "company_name": "PT Dealls Tech",
      "stage": "CLIENT",
      "pic_name": "John",
      "bot_active": true,
      // ... core fields ...
      "custom_fields": {
        "hc_size": 150,
        "nps_score": 8,
        "plan_type": "Enterprise",
        "onboarding_sent": true
      }
    }
  ],
  "meta": {
    "offset": 0,
    "limit": 20,
    "total": 128
  }
}
```

### GET `/master-data/clients/{id}`
Get single record by UUID.

```
Response 200:
{
  "data": { ... full record ... }
}

Response 404:
{
  "error": "Record not found"
}
```

### POST `/master-data/clients`
Create new record.

```json
// Request body:
{
  "company_id": "DE-NEW-001",
  "company_name": "PT New Company",
  "stage": "LEAD",
  "pic_name": "Alice",
  "pic_wa": "628123456789",
  "custom_fields": {
    "hc_size": 50,
    "industry": "Technology"
  }
}

// Response 201:
{
  "data": { "id": "uuid", ... }
}

// Response 409 (duplicate company_id in workspace):
{
  "error": "Company ID DE-NEW-001 already exists in this workspace"
}
```

### PUT `/master-data/clients/{id}`
Partial update (patch merge). `custom_fields` di-MERGE (bukan replace).

```json
// Request body — hanya field yang berubah:
{
  "stage": "PROSPECT",
  "bot_active": true,
  "custom_fields": {
    "nps_score": 9
  }
}
// custom_fields di-merge via PostgreSQL: custom_fields || $1::jsonb

// Response 200:
{
  "data": { "id": "uuid", ... },
  "changed_fields": ["stage", "custom_fields.nps_score"]
}
```

### DELETE `/master-data/clients/{id}`

```
Response 200:
{
  "message": "Deleted",
  "id": "uuid"
}
```

---

## 2. Summary & Stats

### GET `/master-data/stats`
Stat cards untuk halaman Master Data.

```
Response 200:
{
  "total": 128,
  "by_stage": {
    "LEAD": 23,
    "PROSPECT": 15,
    "CLIENT": 85,
    "DORMANT": 5
  },
  "total_revenue": 4750000000,
  "high_risk": 8,
  "overdue_payment": 5,
  "expiring_30d": 12,
  "cross_sell_active": 7,
  "bot_active": 95
}
```

### GET `/master-data/attention`
Records yang butuh perhatian (tab "Perhatian" di frontend).

```
Query: ?offset=0&limit=50&search=keyword

Filter otomatis:
  risk_flag = 'High'
  OR payment_status IN ('Terlambat', 'Belum bayar')
  OR days_to_expiry BETWEEN 0 AND 30

Response 200:
{
  "data": [ ... filtered records ... ],
  "meta": { "total": 25 },
  "summary": {
    "high_risk": 8,
    "overdue": 5,
    "expiring": 12
  }
}
```

---

## 3. Bulk Operations

### POST `/master-data/clients/import`
Bulk import dari Excel/CSV.

```
Content-Type: multipart/form-data
Fields:
  file: .xlsx or .csv file
  mode: "add_new" | "update_existing"

"add_new": Tambah semua baris baru. Skip jika company_id sudah ada.
"update_existing": Update baris yang company_id-nya sudah ada. Skip baris baru.

Response 200:
{
  "imported": 45,
  "skipped": 3,
  "errors": [
    { "row": 12, "error": "company_id already exists", "company_id": "DE-001" },
    { "row": 18, "error": "custom_field 'hc_size' must be number, got 'abc'", "company_id": "DE-007", "field": "hc_size" },
    { "row": 22, "error": "required field 'legal_entity' is empty", "company_id": "DE-011", "field": "legal_entity" }
  ],
  "preview": [ ... first 5 imported records ... ]
}
```

**Validation rules (per row, short-circuit on first error per row — but other rows continue):**

1. **Required core fields** — `company_id` and `company_name` must be non-empty. Missing → row rejected.
2. **Enum coercion** — `stage`, `payment_status`, `risk_flag`, `sequence_status` must match allowed values (case-insensitive match then normalized to canonical form). Unknown value → row rejected with the field name.
3. **Date fields** — `contract_start`, `contract_end`, `first_blast_date`, `bd_meeting_date`, `last_payment_date` accept `YYYY-MM-DD` or `DD/MM/YYYY` (Excel serial dates coerced).
4. **Numeric fields** — `contract_months`, `final_price`, `sdr_score`, `bd_score`: non-numeric → row rejected.
5. **Boolean fields** — `bot_active`, `blacklisted`, `renewed`: accept `true`/`false`/`1`/`0`/`yes`/`no` (case-insensitive).
6. **Custom fields** — header name matched against `field_label` first (case-insensitive), then `field_key`. Per value:
   - Coerce to declared `field_type` (number, date, boolean, etc.)
   - Validate against `is_required`, `min_value`/`max_value`, `options`, `regex_pattern`
   - `select` / `multiselect`: value must be in `options` (for multiselect, split by `|` or `,`)
   - On coercion error → row rejected, errors envelope carries `{row, field, error}`
7. **Unknown headers** — headers matching no core field and no custom field definition are **skipped with a warning** (not a hard rejection). Returned in `warnings` array.
8. **Duplicate handling** — determined by `mode`:
   - `add_new`: if `company_id` exists → skip, add to `errors` with `"company_id already exists"`
   - `update_existing`: if `company_id` missing → skip, add to `errors` with `"company_id not found"`

**`warnings` envelope** (new):

```
"warnings": [
  { "row": "header", "message": "Unknown column 'Extra_Col' — skipped" }
]
```

### Security: CSV / Excel formula injection

Backend MUST sanitize every string cell before storing:

- Reject or neutralize values whose first character is one of `= + - @ \t \r` — these are Excel/Sheets formula triggers. The FE does the same (`sanitizeCell` prepends a single quote), but backend must not rely on FE sanitization.
- Use XLSX parsing options that disable formula evaluation: in Go's `excelize`, call `GetCellValue` (not `GetCellFormula`); in Node's `xlsx`, pass `{ cellFormula: false, cellHTML: false }`.
- Enforce upload size limit (FE caps at 10 MB; backend should enforce the same or stricter).
- Validate MIME type by magic bytes, not by `Content-Type` header alone.

Export endpoints MUST also prepend a single quote to any exported string value starting with the formula triggers — otherwise a compromised record can inject formulas into the exported file opened in Excel.

### POST `/master-data/import` (Gap #52 — dedup-aware)

Newer dedup-aware import path. Coexists with legacy `/master-data/clients/import`; new clients SHOULD use this. Quarantines ambiguous rows instead of skipping silently.

**Preview mode** — `?preview=true` performs the dedup pass without writing anything:

```
POST /master-data/import?preview=true
Content-Type: multipart/form-data
Fields:
  file: .xlsx or .csv (≤ 10 MB)

Response 200:
{
  "batch_id": "uuid",                          // reuse on the real POST to confirm
  "total_rows": 50,
  "new_count": 38,
  "duplicates_count": 9,                       // exact composite-key matches
  "quarantined": [
    {
      "row_index": 12,
      "reason": "duplicate_match",
      "matches_existing_id": "uuid-of-master_data-row",
      "row_data": { "company_name": "Acme Corp", "pic_email": "ops@acme.com", "pic_wa": "628177712345", ... }
    },
    {
      "row_index": 19,
      "reason": "ambiguous_dm",
      "matches_existing_id": null,
      "row_data": { ... }
    }
  ],
  "errors": [
    { "row_index": 22, "error": "stage 'XYZ' is not allowed" }
  ]
}
```

**Commit mode** — same endpoint without `?preview=true`. If the request includes `batch_id` from a prior preview, backend reuses the dedup decisions; otherwise it re-runs dedup.

```
POST /master-data/import
Content-Type: multipart/form-data
Fields:
  file:        .xlsx or .csv
  batch_id:    uuid                            (optional — from preview)
  on_duplicate: "quarantine" | "skip"          (default: quarantine)

Response 200:
{
  "batch_id":          "uuid",
  "imported":          38,
  "skipped":           0,
  "quarantined_count": 9,                      // rows pushed into import_quarantine
  "errors":            [ ... ]
}
```

Composite dedup key + quarantine schema: see `02-database-schema.md §2d`. SLA 48h on quarantine review.

### GET `/import-quarantine`

List pending quarantine rows for human review. Workspace-scoped.

```
Query params:
  ?status=pending              (default: pending; allowed: pending, accepted, rejected, expired)
  &batch_id=uuid               (optional — filter to one upload)
  &offset=0&limit=50

Response 200:
{
  "data": [
    {
      "id":             "uuid",
      "batch_id":       "uuid",
      "row_index":      12,
      "row_data":       { "company_name": "Acme Corp", "pic_email": "ops@acme.com", ... },
      "dedup_match_id": "uuid",                 // null if reason != duplicate_match
      "reason":         "duplicate_match",
      "status":         "pending",
      "created_at":     "2026-04-22T03:14:00Z",
      "age_hours":      8.5
    }
  ],
  "meta": { "offset": 0, "limit": 50, "total": 9 },
  "summary": {
    "pending":   9,
    "over_sla":  0                              // pending AND age_hours > 48
  }
}
```

### PUT `/import-quarantine/{id}/accept`

Promote the quarantined row to `master_data`. If `dedup_match_id` is set, reviewer can choose merge vs new:

```json
// Request body:
{
  "action_on_match": "merge_into_existing" | "create_new",
  "reviewer_note":   "confirmed different entity — same DM"
}

// Response 200:
{
  "id":               "uuid",                   // quarantine row id
  "status":           "accepted",
  "master_data_id":   "uuid",                   // resulting master_data row
  "action_taken":     "create_new",
  "reviewed_by":      "arief@dealls.com",
  "reviewed_at":      "2026-04-22T11:00:00Z"
}
```

### PUT `/import-quarantine/{id}/reject`

Discard the quarantined row.

```json
// Request body:
{ "reviewer_note": "spam — fake company" }

// Response 200:
{
  "id":          "uuid",
  "status":     "rejected",
  "reviewed_by": "arief@dealls.com",
  "reviewed_at": "2026-04-22T11:01:00Z"
}
```

> Both `accept` and `reject` emit one `audit_logs` row. Cron `triggerImportQuarantineSLA` flips rows older than 48h to `status='expired'` and escalates per `01-auth/04-security.md §2d`.

---

### GET `/master-data/clients/export`
Export semua data sebagai Excel.

```
Response headers:
  Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
  Content-Disposition: attachment; filename="master-data-export-2026-04-12.xlsx"

Kolom = core fields + custom field definitions (sorted by sort_order)
Custom field columns diberi prefix [Custom] di header agar jelas.
```

### GET `/master-data/clients/template`
Download import template Excel. ← **PENTING: belum ada di spec sebelumnya**

```
Response headers:
  Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
  Content-Disposition: attachment; filename="master-data-import-template.xlsx"

Sheet 1 "Template":
  - Header row dengan semua kolom (core + custom fields untuk workspace ini)
  - 1 baris contoh dengan example values
  - Kolom Company_ID dan Company_Name di-highlight kuning (required)

Sheet 2 "Reference":
  - Daftar allowed values per kolom:
    Stage: LEAD, PROSPECT, CLIENT, DORMANT
    Payment_Status: Lunas, Menunggu, Belum bayar, Terlambat
    Risk_Flag: High, Mid, Low
    [Custom select fields]: options dari custom_field_definitions
```

---

## 4. Custom Field Definitions

### GET `/master-data/field-definitions`
List semua custom field definitions untuk workspace.

```
Response 200:
{
  "data": [
    {
      "id": "uuid",
      "field_key": "hc_size",
      "field_label": "Jumlah Karyawan",
      "field_type": "number",
      "is_required": true,
      "default_value": null,
      "placeholder": "e.g. 150",
      "description": "Jumlah total karyawan perusahaan",
      "options": null,
      "min_value": 1,
      "max_value": 100000,
      "sort_order": 1,
      "visible_in_table": true,
      "column_width": 100
    },
    {
      "id": "uuid",
      "field_key": "plan_type",
      "field_label": "Plan",
      "field_type": "select",
      "is_required": false,
      "options": ["Basic", "Mid", "Enterprise"],
      "sort_order": 2,
      "visible_in_table": true,
      "column_width": 90
    }
  ]
}
```

### POST `/master-data/field-definitions`
Tambah custom field baru.

```json
{
  "field_key": "nps_score",
  "field_label": "NPS Score",
  "field_type": "number",
  "is_required": false,
  "min_value": 0,
  "max_value": 10,
  "placeholder": "0-10",
  "sort_order": 3,
  "visible_in_table": true,
  "column_width": 80
}

// Response 201
// Response 409 jika field_key sudah ada di workspace ini
```

### PUT `/master-data/field-definitions/{id}`
Update definisi (label, required, options, dll).
**`field_key` TIDAK boleh diubah** setelah creation (akan break existing data).

### DELETE `/master-data/field-definitions/{id}`
Hapus definisi. Data di `custom_fields` JSONB **TIDAK dihapus** — orphan keys di-ignore oleh frontend.

### PUT `/master-data/field-definitions/reorder`
Reorder semua field definitions sekaligus.

```json
{
  "order": [
    { "id": "uuid-1", "sort_order": 0 },
    { "id": "uuid-2", "sort_order": 1 },
    { "id": "uuid-3", "sort_order": 2 }
  ]
}
```

---

## 5. Stage Transition (workflow engine)

### POST `/master-data/clients/{id}/transition`
Atomic stage change + extra field updates. Dipakai oleh workflow engine saat handoff.

```json
{
  "new_stage": "PROSPECT",
  "updates": {
    "sequence_status": "ACTIVE",
    "bot_active": true
  },
  "custom_field_updates": {
    "qualified_at": "2026-04-12T10:00:00Z",
    "qualified_by": "SDR-Budi"
  },
  "trigger_id": "SDR_QUALIFY_HANDOFF",
  "reason": "HC >= 50, role match, interest signal"
}

// Response 200:
{
  "data": { "id": "uuid", "stage": "PROSPECT", ... },
  "previous_stage": "LEAD",
  "action_log_id": "uuid"
}
```

---

## 6. Flexible Query (workflow condition evaluation)

### POST `/master-data/query`
Query records berdasarkan conditions. Dipakai cron job untuk evaluasi workflow nodes.

```json
{
  "conditions": [
    { "field": "stage", "op": "=", "value": "CLIENT" },
    { "field": "bot_active", "op": "=", "value": true },
    { "field": "custom_fields.nps_score", "op": ">=", "value": 8 },
    { "field": "custom_fields.onboarding_sent", "op": "=", "value": false },
    { "field": "days_to_expiry", "op": "between", "value": [0, 35] }
  ],
  "limit": 100
}

// Response 200:
{
  "data": [ ... matching records ... ],
  "meta": { "total": 23 }
}
```

SQL yang di-generate:
```sql
SELECT * FROM master_data
WHERE workspace_id = $1
  AND stage = 'CLIENT'
  AND bot_active = TRUE
  AND (custom_fields->>'nps_score')::numeric >= 8
  AND (custom_fields->>'onboarding_sent')::boolean = FALSE
  AND days_to_expiry BETWEEN 0 AND 35
LIMIT 100
```

---

## 7. Mutation Log

### GET `/master-data/mutations`
Riwayat perubahan data (DataMutationFeed di frontend).

```
Query: ?limit=50&since=2026-04-01T00:00:00Z

Response 200:
{
  "data": [
    {
      "id": "uuid",
      "action": "edit_client",
      "actor": "arief@dealls.com",
      "timestamp": "2026-04-12T10:30:00Z",
      "company_id": "DE-001",
      "company_name": "PT Dealls Tech",
      "changed_fields": ["stage", "custom_fields.nps_score"],
      "previous_values": { "stage": "LEAD", "custom_fields.nps_score": null },
      "new_values": { "stage": "PROSPECT", "custom_fields.nps_score": 8 }
    },
    {
      "id": "uuid",
      "action": "import_bulk",
      "actor": "budi@kantorku.id",
      "timestamp": "2026-04-12T09:00:00Z",
      "count": 45,
      "note": "Import Excel — mode: add_new"
    }
  ]
}
```

---

## 7b. Churn Reactivation (Gap #54)

State machine, fields, and cron logic: see `02-database-schema.md §2e`.

### GET `/clients/{id}/reactivation-history`

Read-only audit trail of every reactivation state transition for one client. Available to all roles with read access on the workspace.

```
Path: /clients/{id}/reactivation-history
Query: ?limit=50

Response 200:
{
  "client_id":        "uuid",
  "company_name":     "PT Example",
  "current_status":   "pending",                         // mirrors master_data.reactivation_status
  "attempts_used":    2,
  "attempts_max":     3,
  "last_reactivation_at": "2026-04-15T09:00:00Z",
  "history": [
    {
      "id":               "uuid",
      "trigger_id":       "CHURN_REACTIVATION",
      "from_status":      null,
      "to_status":        "pending",
      "attempts":         1,
      "days_since_churn": 14,
      "reason":           "first attempt — initial outreach scheduled",
      "actor":            "cron:triggerChurnReactivation",
      "timestamp":        "2026-04-01T09:00:00Z"
    },
    {
      "id":               "uuid",
      "trigger_id":       "CHURN_REACTIVATION",
      "from_status":      "pending",
      "to_status":        "pending",
      "attempts":         2,
      "days_since_churn": 28,
      "reason":           "no response — second attempt",
      "actor":            "cron:triggerChurnReactivation",
      "timestamp":        "2026-04-15T09:00:00Z"
    }
  ]
}

Response 404:
{ "error": "Client not found in this workspace" }
```

Rows are read from `action_logs` filtered by `master_data_id = $id AND trigger_id = 'CHURN_REACTIVATION'`.

### POST `/clients/{id}/trigger-reactivation` (Lead-only)

Manually start or restart a reactivation cycle. Requires Lead role on the workspace (BD Lead, AE Lead, or Account Manager).

```json
// Request body:
{
  "reason":         "client signaled re-engagement on LinkedIn",
  "reset_attempts": false                                // optional — true to zero out reactivation_attempts
}

// Response 200:
{
  "id":                    "uuid",
  "reactivation_status":   "pending",
  "reactivation_attempts": 1,                            // or unchanged if reset_attempts=false
  "last_reactivation_at":  "2026-04-22T11:30:00Z",
  "action_log_id":         "uuid"
}

// Response 403 (non-Lead caller):
{ "error": "Lead role required", "required_role": "BD_LEAD|AE_LEAD|ACCOUNT_MANAGER" }

// Response 409 (already terminal state):
{ "error": "Client is in terminal state 'closed_permanently' — cannot retrigger" }
```

Backend MUST:
- Verify caller's role from session claims (NOT from request body).
- Refuse if `reactivation_status IN ('closed_permanently', 'recycled_to_sdr')` unless `reset_attempts=true`.
- Emit one `action_logs` row with `trigger_id='CHURN_REACTIVATION'` and `actor=<caller_email>`.

---

## 8. Holding View (aggregated)

### GET `/master-data/clients?holding=true`
Kalau workspace adalah holding (is_holding=true), ambil data dari semua member workspaces.

```
Query: ?holding=true

Backend logic:
  1. Check workspace.is_holding = true
  2. Get member_ids dari workspace table
  3. Query master_data WHERE workspace_id IN (member_ids)
  4. Add "workspace_name" field ke setiap record untuk display

Response sama dengan GET /master-data/clients, tapi data merged dari semua member.
```

---

## Checker-Maker Approval Required

The following endpoints require approval before execution.
See `00-shared/05-checker-maker.md` for the full approval system spec.

### DELETE `/master-data/clients/{id}` → Approval Required

Instead of executing directly, create an approval request:

```
POST /approvals
{
  "request_type": "delete_client",
  "payload": {
    "client_id": "uuid",
    "company_name": "PT Example",
    "company_id": "DE-001"
  }
}
```

When approved, the system executes the actual delete.

**Approval payload format:**
| Field | Type | Description |
|-------|------|-------------|
| `client_id` | UUID | The master_data record to delete |
| `company_name` | string | Display name for the approval reviewer |
| `company_id` | string | Company ID for reference |

### POST `/master-data/clients/import` → Approval Required

Instead of executing directly, create an approval request:

```
POST /approvals
{
  "request_type": "bulk_import",
  "payload": {
    "file_name": "import-april-2026.xlsx",
    "mode": "add_new",
    "row_count": 45,
    "preview": [ ... first 5 rows ... ]
  }
}
```

When approved, the system executes the actual import with the uploaded file.

**Approval payload format:**
| Field | Type | Description |
|-------|------|-------------|
| `file_name` | string | Original uploaded file name |
| `mode` | string | `"add_new"` or `"update_existing"` |
| `row_count` | number | Total rows in the file |
| `preview` | array | First 5 rows for reviewer to inspect |

> **Note:** The uploaded file must be stored temporarily (e.g., S3 or local storage) until the approval is resolved. The `payload` should include a `file_ref` pointing to the stored file.
