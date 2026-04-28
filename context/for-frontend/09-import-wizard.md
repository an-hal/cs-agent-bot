# OneSchema-Style Import Wizard — FE Implementation Spec

## Context

The backend (`cs-agent-bot`) ships an OneSchema.co-equivalent import API across three phases:

- **Phase A** — schema discovery + header auto-detect with fuzzy match
- **Phase B** — type-driven cell transforms (date / phone / currency / boolean / enum normalization)
- **Phase C** — per-cell inline editing via persistent server-side sessions, no re-upload required

This doc is the FE spec to build the wizard UI in the dashboard (`project-bumi-dashboard`) at:

```
/dashboard/{workspace}/data-master
```

The "Download Template" + "Import" cards already exist; replace the basic upload modal with this multi-step wizard.

---

## UX Flow — 4 Steps

```
┌────────────┐    ┌─────────────┐    ┌──────────────┐    ┌─────────────┐
│ 1. Upload  │ →  │ 2. Map      │ →  │ 3. Validate  │ →  │ 4. Submit   │
│   xlsx     │    │   columns   │    │   + fix cells│    │   for       │
│            │    │             │    │              │    │   approval  │
└────────────┘    └─────────────┘    └──────────────┘    └─────────────┘
```

### Step 1 — Upload

**Visual:** Modal/page with a drag-and-drop zone, accepts `.xlsx` only (≤32 MB). Shows file name + size after drop. Button "Continue" disabled until a file is selected.

**API:** None yet — file is held in memory until Step 2.

**Edge cases:**
- Reject non-xlsx (xls, csv) with a tooltip "Only .xlsx supported"
- Reject >32 MB up front

### Step 2 — Map Columns

**Visual (split panel):**

| Left panel: Source columns (from xlsx) | Right panel: Target schema |
|---|---|
| `[Sheet selector dropdown]` | Section: **Core Fields** (30 cols) |
| `Company ID    ──→  [target dropdown]` | `company_id   (text, required)` |
| `Name          ──→  [target dropdown]` | `company_name (text, required)` |
| `subsType      ──→  [target dropdown]` | ... |
| ... | Section: **Custom Fields** (workspace-specific, N cols) |
|     | `subscription_type (select: trial/paid/...)` |

**Auto-suggest:** On entering Step 2, fire `POST /import/detect` — backend returns suggested mappings keyed by source column. Pre-fill the right column dropdowns. Each row shows confidence:
- Confidence ≥0.9 → green check icon
- 0.5–0.9 → yellow circle with the suggestion still selected
- <0.5 → no suggestion, user must pick or skip

**Sheet selector:** Backend `detect.sheets[]` returns all sheets with their headers + sample rows. Default to `recommended_sheet`. Switching sheet re-renders the source column list.

**Required fields warning:** Show a banner at top: "5 required fields not yet mapped: pic_name, pic_wa, owner_name, …". Updates live as user assigns dropdowns. "Continue" disabled while required-but-unmapped > 0.

**API:** `POST /import/detect` (once on entering step). Schema metadata from `GET /import/schema` (cached at app boot or on wizard open).

### Step 3 — Validate & Fix

**Visual:** Spreadsheet-style preview table.
- Header row: target field labels (in target order, not source order)
- Body rows: parsed values
- Cell coloring:
  - Green = valid
  - Red = cell error (required missing OR transform error)
  - Yellow = warning (e.g., truncated, phone short)
- Each red cell shows a popover with `error_code` + `error_message` and an inline editor (text input pre-filled with original raw value)

Top stats bar:
```
[ 9 total ]  [ 7 new ]  [ 0 duplicate ]  [ 2 invalid ]
```

Submit button **disabled** while `invalid > 0`.

**Inline edit flow:**
1. User clicks a red cell → popover with input
2. User types new value → "Save" button
3. On Save: `PATCH /import/sessions/{id}/cell` with `{row, target_key, value}`
4. Backend re-parses + returns refreshed preview; FE updates that row's cells (and the stats bar)

**API:**
- On entering step: `POST /import/sessions` (multipart with file + mapping + sheet_name)
- Per cell edit: `PATCH /import/sessions/{id}/cell`
- Re-fetch state if needed: `GET /import/sessions/{id}`

### Step 4 — Submit for Approval

**Visual:** Summary card:
```
File:     kantorku.xlsx
Sheet:    company
Mapping:  12 columns mapped
Rows:     9 ready to import (mode: add new)
Bot:      will be enabled by default
```

Big button "Submit for Approval". On click → `POST /import/sessions/{id}/submit`. Returns approval object. Show toast: "Import submitted; pending approval. Approver: …". Close wizard.

(Approval flow is owned by the existing approval queue UI — not part of this wizard.)

---

## Backend API Contracts

> Base URL: `${PROXY_BASE}/api` (FE proxy strips `/api/v1` prefixes; verify against your `proxy.ts`).
> All endpoints require `Authorization: Bearer <jwt>` and `X-Workspace-ID: <uuid>`.

### Type definitions (TypeScript)

```ts
// ─── Schema ────────────────────────────────────────────────────────

export interface ImportFieldDef {
  key: string;
  label: string;
  type: 'text' | 'number' | 'date' | 'boolean' | 'enum'
      | 'select' | 'phone' | 'email' | 'currency'
      | 'money' | 'percentage' | 'multi_select' | 'url';
  required: boolean;
  options?: string[];
  description?: string;
  is_custom: boolean;
}

export interface ImportSchema {
  core_fields: ImportFieldDef[];
  custom_fields: ImportFieldDef[];
}

// ─── Detect ────────────────────────────────────────────────────────

export interface ImportDetectSheet {
  name: string;
  headers: string[];
  sample_rows: string[][]; // first 3 rows
  row_count: number;
}

export interface MappingSuggestion {
  source_column: string;
  target_key?: string; // empty when confidence <0.5
  confidence: number;  // 0.0–1.0
}

export interface ImportDetectResult {
  sheets: ImportDetectSheet[];
  recommended_sheet: string;
  suggested_mapping: MappingSuggestion[];
  unmapped_target_keys: string[];
}

// ─── Preview / Cell errors ─────────────────────────────────────────

export interface ImportPreviewRow {
  row: number; // 1-based; row 1 is header, data starts at row 2
  status: 'new' | 'duplicate' | 'invalid';
  company_id?: string;
  company_name?: string;
  existing_id?: string;
  error?: string;
}

export interface CellError {
  row: number;
  column: string;       // source header
  target_key?: string;
  source_value: string;
  error_code: 'required' | 'invalid_date' | 'invalid_number'
            | 'invalid_boolean' | 'invalid_enum' | 'invalid_phone'
            | 'invalid_email' | 'invalid_currency' | 'too_long';
  error_message: string;
}

export interface ImportPreview {
  mode: 'add_new' | 'update_existing';
  total_rows: number;
  new: number;
  duplicates: number;
  invalid: number;
  rows: ImportPreviewRow[];
  cell_errors?: CellError[];
}

// ─── Session ───────────────────────────────────────────────────────

export interface ImportSession {
  id: string;
  workspace_id: string;
  created_by: string;
  status: 'pending' | 'submitted' | 'expired';
  file_name: string;
  sheet_name: string;
  mode: 'add_new' | 'update_existing';
  mapping: Record<string, string>;          // source_column → target_key
  cell_overrides: Record<string, Record<string, string>>; // {"<row>": {target_key: corrected_value}}
  approval_id?: string;
  created_at: string;
  updated_at: string;
  expires_at: string;
}

export interface SessionPreview {
  session: ImportSession;
  preview: ImportPreview;
}

export interface PatchCellInput {
  row: number;        // ≥2 (row 1 is header)
  target_key: string;
  value: string;      // raw string; backend transforms it
}

// ─── Approval ──────────────────────────────────────────────────────

export interface ApprovalRequest {
  id: string;
  workspace_id: string;
  request_type: 'bulk_import_master_data';
  description: string;
  payload: Record<string, unknown>;
  status: 'pending' | 'approved' | 'rejected' | 'expired';
  maker_email: string;
  maker_at: string;
  expires_at: string;
}
```

### Endpoint reference

| Method | Path | Body | Returns |
|---|---|---|---|
| `GET` | `/master-data/import/schema` | – | `ImportSchema` |
| `POST` | `/master-data/import/detect` | `multipart: file` | `ImportDetectResult` |
| `POST` | `/master-data/import/sessions` | `multipart: file, mode, sheet_name, mapping (JSON string)` | `SessionPreview` |
| `GET` | `/master-data/import/sessions/{id}` | – | `SessionPreview` |
| `PATCH` | `/master-data/import/sessions/{id}/cell` | `PatchCellInput` (JSON) | `SessionPreview` |
| `POST` | `/master-data/import/sessions/{id}/submit` | – | `ApprovalRequest` |

All responses follow the standard envelope:

```ts
interface StandardResponse<T> {
  requestId: string;
  traceId: string;
  status: 'success' | 'error';
  message: string;
  data: T;
  errorCode?: string;
}
```

---

## Component Architecture (Next.js + React + TypeScript)

```
components/import-wizard/
├── ImportWizard.tsx          # Top-level: step machine, modal/page shell
├── steps/
│   ├── Step1Upload.tsx        # Drag-drop, file validation
│   ├── Step2Map.tsx           # Source ↔ target mapping UI
│   ├── Step3Validate.tsx      # Spreadsheet preview, inline edit
│   └── Step4Submit.tsx        # Final summary + submit button
├── parts/
│   ├── ColumnMappingRow.tsx   # One source column row in Step 2
│   ├── TargetFieldDropdown.tsx # Searchable dropdown of unused target keys
│   ├── CellEditPopover.tsx    # Per-cell error popover + inline editor
│   ├── PreviewTable.tsx       # Virtualized spreadsheet (use @tanstack/react-virtual for 1000+ rows)
│   ├── PreviewStatsBar.tsx    # 4 numeric stats + filter chips
│   └── SheetSelector.tsx      # Tab-style sheet picker
├── hooks/
│   ├── useImportSchema.ts     # SWR(`/import/schema`) — long cache
│   ├── useImportDetect.ts     # mutation: POST /detect
│   ├── useImportSession.ts    # SWR(`/import/sessions/{id}`) + mutate helpers
│   └── useImportWizard.ts     # zustand store: file, step, mapping, sessionId
├── lib/
│   ├── import-api.ts          # Typed fetch wrappers
│   ├── mapping-helpers.ts     # validateRequiredMapped(), unmappedTargets()
│   └── error-codes.ts         # error_code → human label/icon
└── types.ts                   # All TS interfaces above
```

### State machine (zustand)

```ts
import create from 'zustand';

type Step = 1 | 2 | 3 | 4;

interface WizardState {
  step: Step;
  file: File | null;
  schema: ImportSchema | null;
  detectResult: ImportDetectResult | null;
  selectedSheet: string;
  mapping: Record<string, string>; // source → target
  mode: 'add_new' | 'update_existing';
  sessionId: string | null;
  preview: ImportPreview | null;

  setFile(f: File): void;
  setMappingFor(source: string, target: string | null): void;
  setSelectedSheet(name: string): void;
  setMode(m: 'add_new' | 'update_existing'): void;
  goNext(): void;
  goPrev(): void;
  reset(): void;
}

export const useImportWizard = create<WizardState>((set, get) => ({
  step: 1,
  file: null,
  schema: null,
  detectResult: null,
  selectedSheet: '',
  mapping: {},
  mode: 'add_new',
  sessionId: null,
  preview: null,
  // ... setters
}));
```

### API wrapper (`lib/import-api.ts`)

```ts
import { fetchJSON, fetchMultipart } from '@/lib/api'; // existing project helpers

export const importApi = {
  schema: () =>
    fetchJSON<ImportSchema>('/master-data/import/schema'),

  detect: (file: File) => {
    const fd = new FormData();
    fd.append('file', file);
    return fetchMultipart<ImportDetectResult>('/master-data/import/detect', fd);
  },

  createSession: (input: {
    file: File;
    mode: string;
    sheet_name: string;
    mapping: Record<string, string>;
  }) => {
    const fd = new FormData();
    fd.append('file', input.file);
    fd.append('mode', input.mode);
    fd.append('sheet_name', input.sheet_name);
    fd.append('mapping', JSON.stringify(input.mapping));
    return fetchMultipart<SessionPreview>('/master-data/import/sessions', fd);
  },

  getSession: (id: string) =>
    fetchJSON<SessionPreview>(`/master-data/import/sessions/${id}`),

  patchCell: (id: string, body: PatchCellInput) =>
    fetchJSON<SessionPreview>(`/master-data/import/sessions/${id}/cell`, {
      method: 'PATCH',
      body: JSON.stringify(body),
      headers: { 'Content-Type': 'application/json' },
    }),

  submitSession: (id: string) =>
    fetchJSON<ApprovalRequest>(`/master-data/import/sessions/${id}/submit`, {
      method: 'POST',
    }),
};
```

---

## Implementation Checklist (in order)

1. **Types + API client**
   - [ ] Create `components/import-wizard/types.ts` with all interfaces above
   - [ ] Create `components/import-wizard/lib/import-api.ts` with typed fetch wrappers
   - [ ] Verify all 6 endpoints reachable through your existing FE proxy (`proxy.ts`)

2. **Wizard shell + state**
   - [ ] `ImportWizard.tsx` with step state machine, "Back / Continue" buttons, modal/page wrapper matching design system
   - [ ] zustand store (or your existing pattern) for step/file/mapping/sessionId

3. **Step 1 — Upload**
   - [ ] Drag-drop zone (use `react-dropzone` or your existing pattern)
   - [ ] xlsx + size validation
   - [ ] On Continue → fetch schema (cached) + advance to Step 2

4. **Step 2 — Map columns**
   - [ ] Call `detect()` once on enter — store result in store
   - [ ] Render source headers (left), target field list grouped by core/custom (right)
   - [ ] Pre-fill mapping from `detect.suggested_mapping`
   - [ ] `TargetFieldDropdown` with searchable list, hides already-used target keys
   - [ ] `SheetSelector` shows all `detect.sheets` with row count
   - [ ] Banner: "X required fields not mapped" — disable Continue when X>0
   - [ ] On Continue → call `createSession()`, advance to Step 3

5. **Step 3 — Validate + fix**
   - [ ] `PreviewTable` virtualized (use `@tanstack/react-virtual` for >100 rows)
   - [ ] Render cells with green/yellow/red status from `cell_errors`
   - [ ] `CellEditPopover`: click red cell → show input + "Save" → call `patchCell()` → update store from response
   - [ ] `PreviewStatsBar`: total/new/duplicate/invalid + filter chips ("show only invalid")
   - [ ] Submit button disabled while `preview.invalid > 0`

6. **Step 4 — Submit**
   - [ ] Summary card with file/sheet/mapping count/row count/mode
   - [ ] On Submit → `submitSession()` → toast + close wizard
   - [ ] Show approval ID + link to approval queue page

7. **Edge cases / polish**
   - [ ] Session expiry — `expires_at` is 24h after creation; if 401/404 on existing sessionId, fall back to Step 1 with toast
   - [ ] Network error during patchCell → optimistic update with rollback
   - [ ] Multiple cell edits queued → debounce 250ms or batch
   - [ ] Resume mode — if user navigates away mid-wizard, store sessionId in localStorage; on next visit show "Resume in-progress import"

---

## Acceptance Criteria

A user with 1 xlsx file (any column names, any sheet name, locale-ID dates/currencies) should be able to complete an import in **≤4 clicks past upload**:

```
1. Drop xlsx               → wizard opens at Step 2 with auto-detected mapping
2. (review/adjust mapping) → Continue
3. (review preview)        → fix any red cells inline (no re-upload)
4. Submit                  → approval created, awaits approver
```

The wizard MUST handle:
- xlsx with sheet names other than "Template Import"
- Headers like `subsType`, `SuperAdminHandphone` (no exact match to template)
- Excel scientific-notation phones (`6.28898E+12`)
- Multi-format dates (`27/01/2026`, `2026-01-15 10:30:00`)
- Locale-ID currency (`Rp 12.500.000`)
- Indonesian booleans (`iya`, `tidak`)
- Bad cells fixable inline (no re-upload)

---

## Implementation references

- Backend usecase: `internal/usecase/master_data/import_schema.go`, `import_session.go`
- Backend parser: `internal/pkg/xlsximport/mapping_parser.go`, `transforms.go`
- Backend handlers: `internal/delivery/http/dashboard/master_data_handler.go` (search `Import`)
- Migrations: `migration/20260427000900_import_sessions.up.sql`

For dev testing, the synthetic xlsx at `/tmp/phaseb_test.xlsx` (3 rows, 1 with intentionally bad cells) was used to validate the full Phase A+B+C flow. Use it as your FE smoke test fixture.
