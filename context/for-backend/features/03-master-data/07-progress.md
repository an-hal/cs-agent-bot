# Master Data — Implementation Progress

## 2026-04-23 — BD/SDR/AE field & dedup sync

### FE (shipped)
- SDR pre-call section in client form (call notes, intent, next-step plan)
- Meeting Tracking section (scheduled/held/no-show counters, last meeting summary)
- Quotation Status field with chip rendering
- BD Discovery panel in client drawer (30+ qualification fields)
- BD→AE Inheritance pill on cards — shows which AE fields auto-populated from BD discovery
- Multi-invoice tab in drawer (lists all invoices for client with termin breakdown)
- Skipped lead dedup badge — flags records quarantined during import
- SDR segment chips (Hot / Warm / Cold) + H/L intent chip
- Rejection category badge on rejected leads (11-cat taxonomy from messaging)

### Backend spec (documented, implementation pending)
- §2b BD→AE Handoff Fields added in `01-overview.md` — field-mapping table for inheritance contract
- §2c BD Discovery Field Additions in `01-overview.md` — 30+ qualification fields with types/validation
- §2d Import Dedup with quarantine in `01-overview.md` — duplicate detection rules + quarantine workflow state machine
- §2e Churn Reactivation State Machine in `01-overview.md` — dormant → reactivated transition triggers
- API endpoints added in `04-api-endpoints.md`: `POST /master-data/clients/import/preview`, `POST /master-data/clients/import/quarantine/{id}/review`, `POST /master-data/clients/{id}/reactivate`

### Open dependencies (backend)
- Implement Claude extraction integration for unstructured BD discovery notes → structured fields
- Build dedup quarantine workflow (preview → human review → approve/reject → commit)
- Implement reactivation cron — scans dormant clients against reactivation triggers
- Add BD discovery columns to `master_data` schema (or migrate to `custom_field_definitions`)

### Cross-refs
- FE form: `app/dashboard/[workspace]/data-master/page.tsx`, `components/features/ClientDrawer.tsx`
- Rejection taxonomy lives in `05-messaging/01-overview.md` (Inbound Reply Classification)
- Backend gap doc: `claude/for-backend/03-master-data/gap-2b-handoff.md`, `gap-2c-discovery.md`, `gap-2d-dedup.md`, `gap-2e-reactivation.md`

---

> **Overall: 35% complete** (16/46 items done or partial)
> - Frontend/BFF: 70% done (12 done + 4 partial)
> - Backend (Go): 0% done (25 items not started)
> - Optional improvements: 0% done (5 items)

---

## DONE — Frontend/BFF ✅ (12 items)

| # | Item | File | Notes |
|---|------|------|-------|
| 1 | Client list proxy (backend) | `app/api/data-master/clients/route.ts` | Proxies to backend via `getClients()` with X-Workspace-ID, offset, limit, search |
| 2 | Client service (httpClient) | `lib/api/client.service.ts` | `getClients(token, wsUuid, offset, limit, search)` via httpClient |
| 3 | Client list proxy (mock/legacy) | `app/api/clients/route.ts` | GET + POST using in-memory client-store, Zod validation |
| 4 | In-memory client store | `app/api/_lib/client-store.ts` | CRUD on mock data per company slug, holding = merged dealls+kantorku |
| 5 | Import from Excel | `app/api/clients/import/route.ts` | Multipart upload, XLSX parse, add_new/update_existing modes, mutation log |
| 6 | Export to Excel | `app/api/clients/export/route.ts` | Full Excel export with column widths, per-company data |
| 7 | Import template download | `app/api/clients/template/route.ts` | 2-sheet Excel: Template + Reference values |
| 8 | DataMasterTable component | `components/features/DataMasterTable.tsx` | Ant Design table with column toggle, filters, sort, pagination, holding badge |
| 9 | Data Master page | `app/dashboard/[workspace]/data-master/page.tsx` | Full page with search, tabs, stats cards, import/export, edit modal |
| 10 | Mutation log tracking | `app/api/_lib/client-store.ts` | Records add/edit/delete/import/export mutations with timestamps |
| 11 | Holding view (aggregation) | `app/api/_lib/client-store.ts` | `getClients('holding')` merges dealls+kantorku stores |
| 12 | Workspace badge in holding | `components/features/DataMasterTable.tsx` | `deriveWorkspaceSlug()` to show DE/KK badge per row in holding view |

## PARTIAL ⚠️ (4 items)

| # | Item | What's Done | What's Missing |
|---|------|-------------|----------------|
| 13 | Backend-proxied client list | `data-master/clients/route.ts` proxies GET with pagination + search | No stage/risk/bot/payment filters forwarded; no sort_by/sort_dir params |
| 14 | Import/export via backend | Legacy routes use in-memory mock store | Backend import/export endpoints not integrated; still uses `client-store.ts` |
| 15 | Client CRUD via backend | Legacy `/api/clients` uses mock store | No proxy to backend PUT/DELETE endpoints; `data-master/clients` only has GET |
| 16 | Mutation log | In-memory array in BFF | Spec requires backend `action_logs` table with trigger_id, template_id, phase |

## NOT DONE — Backend (Go) Required 🔴 (25 items)

### Critical (blocks other features)

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 17 | `master_data` table | 02-database-schema | Full schema with core fields + custom_fields JSONB, workspace_id FK |
| 18 | `custom_field_definitions` table | 02-database-schema | Per-workspace custom field config: key, label, type, options, sort_order |
| 19 | `action_logs` table | 02-database-schema | Workflow action history: trigger_id, template_id, status, channel, phase |
| 20 | GET `/master-data/clients` | 04-api-endpoints | Paginated list with stage/search/risk/bot/payment/expiry filters + sort |
| 21 | GET `/master-data/clients/{id}` | 04-api-endpoints | Single record by UUID |
| 22 | POST `/master-data/clients` | 04-api-endpoints | Create with company_id uniqueness check per workspace |
| 23 | PUT `/master-data/clients/{id}` | 04-api-endpoints | Partial update, custom_fields JSONB merge via `||` operator |
| 24 | DELETE `/master-data/clients/{id}` | 04-api-endpoints | Delete record |

### High Priority

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 25 | GET `/master-data/stats` | 04-api-endpoints | Summary: total, by_stage, total_revenue, high_risk, overdue, expiring_30d |
| 26 | GET `/master-data/attention` | 04-api-endpoints | Records needing attention: high risk, overdue payment, expiring contracts |
| 27 | POST `/master-data/clients/import` | 04-api-endpoints | Bulk import from Excel/CSV with add_new/update_existing modes |
| 28 | GET `/master-data/clients/export` | 04-api-endpoints | Excel export: core + custom fields (sorted by sort_order) |
| 29 | GET `/master-data/clients/template` | 04-api-endpoints | Import template with workspace-specific custom field columns |
| 30 | Holding view query | 04-api-endpoints | `WHERE workspace_id IN (member_ids)` with workspace_name per record |

### Medium Priority

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 31 | GET `/master-data/field-definitions` | 04-api-endpoints | List custom field definitions for workspace |
| 32 | POST `/master-data/field-definitions` | 04-api-endpoints | Create custom field (key immutable after creation) |
| 33 | PUT `/master-data/field-definitions/{id}` | 04-api-endpoints | Update label, required, options — NOT field_key |
| 34 | DELETE `/master-data/field-definitions/{id}` | 04-api-endpoints | Delete definition (orphan JSONB keys ignored) |
| 35 | PUT `/master-data/field-definitions/reorder` | 04-api-endpoints | Bulk update sort_order |
| 36 | POST `/master-data/clients/{id}/transition` | 04-api-endpoints | Atomic stage change + field updates + action log entry |
| 37 | POST `/master-data/query` | 04-api-endpoints | Flexible condition evaluation for workflow engine |
| 38 | GET `/master-data/mutations` | 04-api-endpoints | Mutation history with changed_fields, previous/new values |

### Low Priority

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 39 | GIN index on custom_fields | 02-database-schema | `CREATE INDEX ... USING GIN(custom_fields)` for JSONB query performance |
| 40 | Trigram search indexes | 04-api-endpoints (global search) | `pg_trgm` extension + GIN index on company_name |
| 41 | `days_to_expiry` cron | 02-database-schema | Computed/updated daily by cron job |

### Checker-Maker Approval

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 47 | Checker-maker for DELETE `/master-data/clients/{id}` | 04-api-endpoints | Require approval (type: `delete_client`) before executing client deletion |
| 48 | Checker-maker for POST `/master-data/clients/import` | 04-api-endpoints | Require approval (type: `bulk_import`) before executing bulk import |

## NOT DONE — Optional Frontend Improvements 🟡 (5 items)

| # | Item | Priority | Description |
|---|------|----------|-------------|
| 42 | Migrate legacy `/api/clients` to backend proxy | High | Replace mock client-store with backend-proxied CRUD (POST, PUT, DELETE) |
| 43 | Forward all filter params to backend | High | Stage, risk_flag, bot_active, payment_status, expiry_within, sort_by, sort_dir |
| 44 | Custom field column rendering | Medium | Render dynamic columns in DataMasterTable based on field-definitions API |
| 45 | Attention tab via backend | Medium | Replace frontend filtering with `/master-data/attention` endpoint |
| 46 | Mutation feed via backend | Low | Replace in-memory mutation log with `/master-data/mutations` endpoint |

---

## Recommended Implementation Order (Backend)

```
Week 1: #17 master_data table + #18 custom_field_definitions + #20 GET clients (list) + #21 GET client (single)
Week 2: #22 POST + #23 PUT + #24 DELETE clients + #25 stats + #26 attention
Week 3: #27-29 import/export/template + #30 holding view
Week 4: #31-35 field definitions CRUD + reorder
Week 5: #36 stage transition + #37 flexible query + #38 mutations
Later:  #19 action_logs + #39-41 indexes/cron
```

## Dependency Chain

```
master_data ──→ GET/POST/PUT/DELETE clients ──→ stats + attention
  │                     │
  │                     └──→ import/export/template
  │                     │
  │                     └──→ holding view (requires workspaces table)
  │
  └──→ custom_field_definitions ──→ field CRUD + reorder
  │                                      │
  │                                      └──→ template with custom columns
  │
  └──→ action_logs ──→ stage transition ──→ flexible query
                                               │
                                               └──→ mutation log
```
