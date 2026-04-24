# Collections — Implementation Progress

> **Overall: 38% complete** (13/35 items done) · last updated 2026-04-21
> - Frontend/BFF: 75% done (11/14)
> - Backend (Go): 0% done (0/21)
> - Feature-flagged via `NEXT_PUBLIC_FEATURES_COLLECTIONS` (default ON in dev)

---

## NOT STARTED — Backend (Go) ❌ (18 items)

| # | Item | Spec | Priority |
|---|------|------|----------|
| 1 | Migration — `collections` table | `02-database-schema.md` | P0 |
| 2 | Migration — `collection_fields` table | `02-database-schema.md` | P0 |
| 3 | Migration — `collection_records` table | `02-database-schema.md` | P0 |
| 4 | Go model + repository (collections) | — | P0 |
| 5 | Go model + repository (fields) | — | P0 |
| 6 | Go model + repository (records) | — | P0 |
| 7 | `GET /collections` | `03-api-endpoints.md` | P0 |
| 8 | `POST /collections` (with approval) | `03-api-endpoints.md` | P0 |
| 9 | `GET /collections/{id}` | `03-api-endpoints.md` | P0 |
| 10 | `PATCH /collections/{id}` | `03-api-endpoints.md` | P1 |
| 11 | `DELETE /collections/{id}` (with approval) | `03-api-endpoints.md` | P1 |
| 12 | Field CRUD endpoints | `03-api-endpoints.md` | P0 |
| 13 | Record CRUD endpoints | `03-api-endpoints.md` | P0 |
| 14 | Filter DSL evaluation against JSONB | reuse `00-shared/01-filter-dsl.md` | P1 |
| 15 | Record schema validation engine | `03-api-endpoints.md` | P0 |
| 16 | Bulk record operations | `03-api-endpoints.md` | P2 |
| 17 | Import/export (xlsx/csv) | `03-api-endpoints.md` | P2 |
| 18 | Audit logging integration | `08-activity-log` | P1 |
| 19 | Sort on arbitrary JSONB field key (per-type semantics) | `03-api-endpoints.md` | P1 |
| 20 | `in` filter on arbitrary JSONB field key | `03-api-endpoints.md` | P1 |
| 21 | `GET /collections/{id}/records/distinct` endpoint | `03-api-endpoints.md` | P1 |

## Frontend/BFF — 11 of 14 done ✅

| # | Item | Target File | Status |
|---|------|-------------|--------|
| 1 | Sidebar entry "Collections" | `components/common/Sidebar.tsx` | ✅ (feature-flagged) |
| 2 | List page — `/dashboard/[workspace]/collections/page.tsx` | built | ✅ |
| 3 | Detail page — `/dashboard/[workspace]/collections/[slug]/page.tsx` | built | ✅ |
| 4 | `CollectionBuilder` component (schema editor) | `components/features/CollectionBuilder.tsx` | ✅ |
| 5 | `CollectionTable` component (record grid) | `components/features/CollectionTable.tsx` | ✅ |
| 6 | Field type renderers (text, number, enum, date, link_client, ...) | inside `CollectionTable` + drawer | ✅ |
| 7 | Record drawer (create/edit) | `components/features/CollectionRecordDrawer.tsx` | ✅ |
| 8 | BFF proxy — `app/api/collections/route.ts` | — | ❌ (not needed yet; service is client-side) |
| 9 | BFF proxy — `app/api/collections/[id]/records/route.ts` | — | ❌ (not needed yet) |
| 10 | Collection service — `lib/api/collection.service.ts` | built (localStorage + in-memory) | ✅ |
| 11 | Types — `types/collection.ts` | built | ✅ |
| 12 | Zod schemas — `app/api/_lib/schemas/collection.ts` | — | ❌ (BFF not built yet) |
| 13 | Approval integration (schema changes → `ApprovalSystem`) | `ApprovalType: 'collection_schema_change'` | ✅ |
| 14 | Import/export UI (xlsx/csv) | — | ❌ (P2, post-backend) |
| 15 | Fixtures — built-in collection templates | `lib/fixtures/collection-templates.ts` | ✅ |
| 16 | Fixtures — seed sample collections | `lib/fixtures/collection-seeds.ts` | ✅ |
| 17 | Tests — `collection.service.test.ts` | `__tests__/collection.service.test.ts` | ✅ |

## Definition of Done

- [ ] User bisa bikin collection baru dari UI tanpa restart/deploy
- [ ] Schema changes trigger checker-maker approval flow
- [ ] Record CRUD validated against schema (strict mode)
- [ ] Filter DSL bisa query JSONB data
- [ ] Audit log mencatat semua schema + record changes
- [ ] Per-workspace scoping enforced (tidak leak antar-workspace)
- [ ] Link-to-client field bisa reference Data Master (one-way)
- [ ] Hard limits enforced (50 collections/ws, 30 fields, 10k records)

## Non-Goals (untuk MVP)

- ❌ Cross-collection relations (link ke collection lain)
- ❌ Formula / computed fields
- ❌ Views / saved filters
- ❌ Real-time collaboration
- ❌ Version history per record
- ❌ Workflow trigger dari Collection data

Kalau salah satu non-goal di atas benar-benar dibutuhkan → re-evaluate apakah data itu harus jadi **dedicated feature**, bukan Collection.
