# Project Progress Summary

> **Last updated: 2026-04-21 (workflow UX + security hardening)**
> **Overall: ~40% complete** (FE ~80% · BE ~1%)
> **FE ~80% done · BE ~1% done** (backend work not started beyond invoice proxy)

## Shared Specs (cross-cutting)

| File | Description |
|------|-------------|
| `features/00-shared/01-filter-dsl.md` | Filter & metric DSL syntax + Go parser (used by pipeline tabs, stats, query) |
| `features/00-shared/02-user-preferences.md` | User preferences table + API (theme, column visibility, UI settings) |
| `features/00-shared/03-html-sanitization.md` | HTML sanitization rules for email templates (allowed tags, styles, Go library) |
| `features/00-shared/04-integrations.md` | External service config: HaloAI (WA), Telegram, SMTP, Paper.id + webhook endpoints |
| `features/00-shared/05-checker-maker.md` | Approval system: **10 high-risk operations**, DB schema, API, Go service (includes `collection_schema_change`) |

## Per-Feature Status

| # | Feature | Overall | Frontend | Backend | Notes |
|---|---------|---------|----------|---------|-------|
| 01 | [Auth](features/01-auth/06-progress.md) | **68%** | 85% | 0% | Uses external `ms-auth-proxy.up.railway.app`. Whitelist gate works. |
| 02 | [Workspace](features/02-workspace/06-progress.md) | **40%** | 85% | 0% | Includes notifications + workspace validation middleware (slug→UUID redirect, `allowed_workspaces` cookie). |
| 03 | [Master Data](features/03-master-data/07-progress.md) | **45%** | 85% | 0% | **Custom Fields page** now complete (`/custom-fields`). DataSource abstraction + client-mapper added. `master_data` schema extended with `pipeline_status`, `sdr_score`, `bd_score`, `verdict_sdr`, `location`, `first_blast_date`, `bd_meeting_date`. Dedicated spec at `03-master-data/08-custom-fields-page.md`. |
| 04 | [Team](features/04-team/04-progress.md) | **15%** | 35% | 0% | RBAC matrix UI functional. |
| 05 | [Messaging](features/05-messaging/04-progress.md) | **35%** | 70% | 0% | Templates CRUD + TipTap editor. |
| 06 | [Workflow Engine](features/06-workflow-engine/06-progress.md) | **35%** | 80% | 0% | React Flow canvas, 27 node types, Excel-linked specs. |
| 07 | [Invoices](features/07-invoices/04-progress.md) | **40%** | 82% | 5% | Only feature with partial backend integration (proxy to `DASHBOARD_API_URL`). Invoices page currently lives inside Data Master / client drawer; no dedicated `/invoices` route. |
| 08 | [Activity Log](features/08-activity-log/05-progress.md) | **45%** | 80% | 0% | Unified bot/human feed, 30+ mock entries. |
| 09 | [Analytics & Reports](features/09-analytics-reports/03-progress.md) | **45%** | 80% | 0% | **Forecast endpoint** (`/analytics/funnel-forecast`) now specified — blends current-pipeline math + 6-month regression + per-owner breakdown. |
| 10 | [Collections](features/10-collections/04-progress.md) | **38%** | 75% | 0% | **Feature flag-gated** (`NEXT_PUBLIC_FEATURES_COLLECTIONS`). Pages + CollectionBuilder + CollectionTable + CollectionRecordDrawer complete. Schema change approval via `collection_schema_change`. |
| | **TOTAL** | **~38%** | **~75%** | **~1%** | |

## What Changed — Business Layer Sync (latest pass)

`context/for-business/` is the authoritative source for business intent. Ten business-layer
concepts were propagated into `context/for-backend/` to close backend contract gaps:

### New specs

- `features/06-workflow-engine/07-manual-flows.md` (**NEW, 383 LOC**) — GUARD manual-flow queue:
  `manual_action_queue` table, 20-flow inventory, API (list / detail / mark-sent / skip), Telegram
  + dashboard contracts, Go `ManualActionService` skeleton. Source: `GUARD-Manual-Flows-CEO-Brief.md`.

### Schema additions

- `features/03-master-data/02-database-schema.md` + `DATABASE-FULL-SCHEMA.md` — BANTS scoring
  columns: `bants_budget/authority/need/timing/sentiment` (SMALLINT 0-3), `bants_score`
  (DECIMAL 3,2), `bants_percentage` (SMALLINT 0-100), `bants_classification` (HOT/WARM/COLD),
  `buying_intent` (high/low), + override audit trail (`intent_overridden/original/reason/by/at`).
  New indexes `idx_dm_buying_intent`, `idx_dm_bants_score`.

### API specs

- `features/06-workflow-engine/04-api-endpoints.md` — **§2.5 Template Resolution**:
  `POST /workflows/{id}/resolve-template` with 4-step priority (renewal → intent → legacy
  branches → default), low-intent BD sequence shortening (skip D12/D14/D21), Go service skeleton.
- `features/09-analytics-reports/02-api-endpoints.md` — **§5 KPI Endpoints**: 14 per-metric
  endpoints (SDR 4 / BD 4 / AE 6) + `/analytics/kpi/bundle?role=` batch, Redis 15min cache,
  formulas matching `context/claude/13-kpi-metrics-and-targets.md`.

### Reconciliations

- `features/08-activity-log/04-escalation.md` — renamed `severity` → `priority`
  (P0/P1/P2), added exact-threshold trigger table (7 rows: NPS≤5, overdue≥30d, usage<20%+30d,
  angry/reject reply, expired contract, cross-sell 2× reject, 3× missed check-in), status
  machine, notification fan-out contract. Also patched `05-progress.md` item #32-33 to match.
- `features/06-workflow-engine/05-cron-engine.md` — added AE Phase Reference Table (P0–P6 window
  + timing basis), Operating Constraints (working-day / send window / max touches / stop-resume),
  Handoff Data Carry-Through Contract (4 transitions with required/optional fields + side effects).

### Known discrepancy (needs decision)

- `bants_percentage` typed `SMALLINT` in schema (per gap-fill spec) vs. `DECIMAL(4,1)` in
  `context/for-business/10-BUYING-INTENT-MECHANISM.md` (line 563). Decide in next review
  whether percentage should be decimal (e.g. 56.7) or integer (e.g. 57). Leaning integer —
  FE rounds for display anyway.

---

## What Changed Since 2026-04-21 (latest — workflow UX + security hardening)

### Frontend additions
- **Workflow node editor UX (non-technical users)** — Indonesian builders replace raw SQL-ish inputs:
  - `TimingBuilder` (H/J/M/D unit + Setelah/Sebelum + optional range)
  - `ConditionBuilder` (21-field catalog + 7 operators in Indonesian)
  - `StopIfBuilder` (8-preset OR-joined checklist)
  - `Template Pesan` → dropdown from `ALL_TEMPLATES` (was free-text ID)
  - `Sent Flag` auto-derive from label + hidden in "Pengaturan Lanjutan"
  - Data Master READ/WRITE → chip rendering with Indonesian labels (`COLUMN_LABELS_ID`, 40+ columns)
  - Variable chips translate `[Company_Name]` → "Nama Perusahaan"
  - Advanced collapsibles hide Trigger ID / Template ID / Sent Flag under "Referensi Teknis"
  - All category labels translated: Pemicu / Syarat / Aksi / Kontrol Alur
- **Security hardening (FIX-1 through FIX-14)**:
  - `requireWorkspaceScope()` helper — no more `?company=` query param
  - Middleware strips client-sent `X-Workspace-ID`, re-derives from referer
  - Full-length HMAC signature (was `.slice(0, 16)` → 64-bit)
  - CSP + Permissions-Policy + HSTS in `next.config.ts`
  - Rate limits on `/export`, `/import`, `/search`
  - CSV formula injection protection (`sanitizeCell` + `cellFormula: false`)
  - `clearSessionStorage()` clears all sensitive keys on logout
  - Cross-tab storage listeners in 4 contexts
  - Zod schemas expanded (ClientRecord + 10 ApprovalRequest payload types + Collection)
  - Integration credentials: UI warning banner + clear on logout (backend vault is BLOCKED dep)
  - ESLint `no-restricted-imports` blocks direct `@/lib/mock-data*` imports
- **LOC refactor** — 14 monolithic files split into namespaced subfolders (`_chapters/`, `_tabs/`, `_parts/`, `_sections/`, `_modals/`, `_drawers/`, `header/`). Top files dropped from 2379 LOC (how-to) and 1519 LOC (team) down to 89 and 208 respectively. Zero behavior change; 111/111 tests pass.

### Backend spec updates (this session)
- `features/01-auth/01-overview.md`, `03-api-endpoints.md`, `04-security.md` — HMAC full 64-char (no slice); new "2b. Workspace Scope Enforcement" section mandating backend re-validate `X-Workspace-ID` against session claims
- `features/03-master-data/04-api-endpoints.md` — CSV/Excel formula injection protection contract (both import + export)
- `features/06-workflow-engine/01-overview.md` — note on dual timing formats from FE
- `features/06-workflow-engine/05-cron-engine.md` — **new sections**: "Timing Format Parser (dual-format)" (Indonesian + legacy) + "DSL Field Catalog (FE contract)" (21-field reference table)
- `PROGRESS-SUMMARY.md` — this refresh

---

## What Changed 2026-04-12 → 2026-04-21

### Frontend additions (first wave)

- **`/approvals` page** — dedicated checker-maker queue (stat cards, status tabs, search, detail modal, self-approval block)
- **`/custom-fields` page** — full CRUD for custom field definitions (6 field types)
- **`/collections/[slug]` + CollectionBuilder / CollectionTable / CollectionRecordDrawer** — user-defined ad-hoc tables
- **`ForecastView` component** — SDR/BD funnel projection + 6-month regression + per-owner breakdown
- **`FilterBar` component** — Owner / Company Size / Industry / Location multi-select
- **`ExportImportButtons` component** — CSV export/import with preview modal (RFC 4180 compliant)
- **DataSource abstraction** (`lib/api/data-source.ts`) — single seam for mock↔HTTP swap via `NEXT_PUBLIC_DATA_SOURCE`
- **Client mapper** (`lib/api/client-mapper.ts`) — PascalCase ↔ snake_case bridge
- **Filter DSL client** (`lib/filter-dsl-client.ts`) — serialize `FilterState` → backend filter DSL
- **Mock enrichment** (`lib/mock-data-enrich.ts`) — adds `sdr_score`, `bd_score`, `verdict_sdr`, `pipeline_status`, `location`, `first_blast_date`, `bd_meeting_date`
- **Historical stats + regression** (`lib/mock-historical.ts`) — 6-month synthetic + linear regression helpers
- **Custom fields store** (`lib/custom-fields-store.ts`) — localStorage-backed, backend-ready shape
- **Proxy middleware hardening** — slug→UUID redirect, workspace param validation, `allowed_workspaces` enforcement, `X-Workspace-ID` forwarding
- **New `ApprovalType`**: `collection_schema_change` (10 total)

### Backend spec updates (this session)

- `00-shared/05-checker-maker.md` — added `collection_schema_change` enum value, operations row, payload example, Go `applyApproval` case, handler example, service wiring
- `03-master-data/02-database-schema.md` — added 7 new columns to `master_data` + 4 new indexes; `custom_field_definitions.field_type` note updated to include `multiselect`
- `03-master-data/04-api-endpoints.md` — added 8 validation rules for CSV import + `warnings` envelope
- `03-master-data/08-custom-fields-page.md` — **NEW** dedicated spec for the `/custom-fields` page (full CRUD + value handling + CSV import integration)
- `09-analytics-reports/02-api-endpoints.md` — added `GET /analytics/funnel-forecast` endpoint (current pipeline + regression + blended + per-owner)
- `DATABASE-FULL-SCHEMA.md` — synced `approval_request_type` enum, `master_data` columns/indexes, added full **Feature: 10-collections** section (tables + validation rules + schema-change migration behavior)

## Backend Implementation Order (unchanged)

```
Phase 1 — Foundation (Week 1-4)
├── 01-auth: users + sessions + whitelist tables, /auth/me, /auth/refresh
├── 02-workspace: workspaces + members tables, CRUD, settings
├── 03-master-data: master_data + custom_field_definitions tables, full CRUD
└── 04-team: roles + permissions + members tables, RBAC middleware

Phase 2 — Core Business (Week 5-8)
├── 05-messaging: message_templates + email_templates tables, CRUD, render
├── 06-workflow-engine: workflows + nodes + edges tables, canvas save, cron engine
└── 07-invoices: invoices + payment_logs tables, Paper.id webhook, collection stages

Phase 3 — Observability (Week 9-10)
├── 08-activity-log: unified feed, auto-recording, escalation triggers
└── 09-analytics-reports: aggregation endpoints, caching, export

Phase 4 — Extensions (Week 11+)
└── 10-collections: collections + collection_fields + collection_records tables
    — tied to approvals (collection_schema_change)
```

## Dependency Chain

```
01-auth ──→ 02-workspace ──→ 03-master-data ──→ 04-team
                                    │
                              ┌─────┼──────┐
                              ▼     ▼      ▼
                         05-msg  06-wf  07-inv
                              │     │      │
                              └─────┼──────┘
                                    ▼
                              08-activity-log
                                    │
                                    ▼
                              09-analytics
                                    │
                                    ▼
                              10-collections (optional)
```

## What's Blocking Progress

1. **No Go backend repo yet** — 40+ spec files ready, zero Go code
2. **No PostgreSQL schema deployed** — 25+ tables defined in specs, none created
3. **Frontend uses mock data** — 2,500+ LOC of mock data in client bundle (mitigated by DataSource abstraction — flipping `NEXT_PUBLIC_DATA_SOURCE=http` will switch once backend is live)
4. **No real API integration** — only workspace list + invoice list proxy to backend

## Files Reference

Each feature folder contains:
- `01-overview.md` — Architecture, data flow
- `02-database-schema.md` — PostgreSQL CREATE TABLE + indexes
- `03-api-endpoints.md` — Full REST API with request/response examples
- `03-golang-models.md` — Go structs + repository (auth, master-data, workflow only)
- `04-security.md` / `05-cron-engine.md` / similar — Feature-specific extras
- `*-progress.md` — This tracking file (DONE / PARTIAL / NOT DONE)

Cross-reference docs in `context/claude/` describe the FE codebase state (updated 2026-04-21).
