# Invoices & Billing — Implementation Progress

## 2026-04-23 — Multi-invoice + legal escalation sync

### FE (shipped)
- **Multi-invoice tab** in client drawer with termin breakdown (DP / Termin 1 / Termin 2 / etc.)
- **Legal escalation chip** — flags invoices >30 days overdue with legal hand-off needed
- **Partial payment indicator** — shows paid_amount / total_amount on invoice rows
- **Early Bird countdown** — surfaces remaining days for early-payment discount window
- **Paper.id badge** — distinguishes Paper.id-routed invoices vs manual transfer_bank invoices
- **First Payment chase ownership badge** — shows who owns the first-payment chase (BD vs AE vs Finance)

### Backend spec (documented, implementation pending)
- `payment_method` routing added in `01-overview.md` — split path: `paper_id` (auto-link via Paper.id API) vs `transfer_bank` (manual confirm flow)
- `legal_escalation_flag` column + 30+ day cron added in `02-database-schema.md` — cron sets flag when overdue exceeds threshold
- Partial payment state machine added in `01-overview.md` — Belum bayar → Sebagian → Lunas with audit trail
- New endpoints in `03-api-endpoints.md`:
  - `POST /invoices/{id}/confirm-partial` (manual transfer_bank flow — confirm received amount)
  - `GET /invoices/{id}/payment-history` (lists all partial payments with timestamps)
  - `GET /invoices/legal-escalations` (list of invoices flagged for legal hand-off)

### Open dependencies (backend)
- Implement Paper.id integration (#33-36 below) — webhook receiver still pending
- Implement legal flag cron — runs daily, scans `due_date < NOW() - 30 days AND payment_status != Lunas`, sets `legal_escalation_flag = true`
- Implement partial payment confirmation flow — ties into checker-maker for amount confirmation
- First Payment chase ownership routing — needs join to workflow assignment (BD vs AE owner)

### Cross-refs
- FE: `app/dashboard/[workspace]/invoices/page.tsx`, `components/features/InvoiceDrawer.tsx`
- Legal escalation also surfaces in 06-workflow-engine (BD escalations matrix)
- Multi-invoice surfaces in 03-master-data (Multi-invoice tab in client drawer)
- Backend gap doc: `claude/for-backend/07-invoices/gap-payment-method-routing.md`, `gap-partial-payment.md`, `gap-legal-escalation.md`

---

> **Overall: 38% complete** (18/47 items done or partial)
> - Frontend: 80% done (14 done + 4 partial)
> - Backend (Go): 5% done (1 partial — proxy route exists but backend incomplete)
> - Optional improvements: 0% done (6 items)

---

## DONE — Frontend ✅ (14 items)

| # | Item | File | Notes |
|---|------|------|-------|
| 1 | Invoice list page | `app/dashboard/[workspace]/invoices/page.tsx` | Full table with search, pagination, status tabs (All/Pending/Overdue/Paid) |
| 2 | Invoice data model (TypeScript) | Same file | `Invoice` interface: id, company_id, company_name, workspace, amount, dates, status, collection_stage, paper_id_url |
| 3 | Server-side pagination + search | Same file | Fetches from `/api/data-master/invoices` with offset/limit/search params |
| 4 | Status tab filtering | Same file | Client-side filter by payment_status (all/pending/overdue/paid) |
| 5 | Invoice detail drawer | Same file | Shows all fields: amount, dates, status, collection stage, Paper.id link, notes |
| 6 | Stat cards (4 cards) | Same file | Total Invoice, Total Nilai, Overdue, Lunas — computed from fetched data |
| 7 | Collection stage sidebar | Same file | Shows count per stage (Stage 0-4 + Closed) with color-coded badges |
| 8 | Activity feed | Same file | Mock activity feed (recent payments, reminders, escalations) |
| 9 | Create invoice wizard (4-step) | Same file | Step 0: company, Step 1: details, Step 2: line items, Step 3: review |
| 10 | Invoice line items UI | Same file | Add/remove line items with description, qty, unit_price, auto-computed subtotal |
| 11 | Mark as Paid action | Same file | Button in drawer + dropdown menu to mark invoice as paid |
| 12 | Send Reminder action | Same file | Button to increment reminder_count and update last_reminder_date |
| 13 | Holding view support | Same file | Fetches from multiple workspace UUIDs when isHolding=true |
| 14 | Workspace badge display | Same file | Dealls (DE) / KantorKu (KK) badges per invoice row |

## PARTIAL ⚠️ (4 items)

| # | Item | What's Done | What's Missing |
|---|------|-------------|----------------|
| 15 | BFF proxy for invoice list | `app/api/data-master/invoices/route.ts` + `lib/api/invoice.service.ts` | Proxy calls backend API with token + workspace header. Backend endpoint exists but returns limited data (no company_name, no line_items) |
| 16 | Create invoice flow | 4-step wizard UI complete with form validation | Simulates API call with `setTimeout` — no real POST to backend. ID generation is client-side |
| 17 | Mark as Paid | UI action updates local state | Client-side only (`setInvoices`), no POST `/invoices/{id}/mark-paid` API call |
| 18 | Send Reminder | UI action increments counter in local state | Client-side only, no POST `/invoices/{id}/send-reminder` API call |

## NOT DONE — Backend (Go) Required 🔴 (23 items)

### Critical (blocks real data)

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 19 | `invoices` table | 02-database-schema | id (VARCHAR PK), workspace_id, company_id, amount, dates, payment_status, collection_stage, paper_id fields |
| 20 | `invoice_line_items` table | 02-database-schema | invoice_id FK, description, qty, unit_price, subtotal |
| 21 | `payment_logs` table | 02-database-schema | invoice_id FK, event_type, amount_paid, payment_method, status changes, raw_payload |
| 22 | `invoice_sequences` table | 02-database-schema | Atomic ID generation per workspace per year |
| 23 | GET `/invoices` | 03-api-endpoints | List with pagination, filter by payment_status/collection_stage, search, sort |
| 24 | GET `/invoices/{invoice_id}` | 03-api-endpoints | Single invoice with line_items + payment_logs |

### High Priority

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 25 | POST `/invoices` | 03-api-endpoints | Create invoice + line items + generate ID via sequences + call Paper.id API |
| 26 | PUT `/invoices/{invoice_id}` | 03-api-endpoints | Update notes/due_date (block amount change after Paper.id link) |
| 27 | DELETE `/invoices/{invoice_id}` | 03-api-endpoints | Only if payment_status = 'Belum bayar' |
| 28 | POST `/invoices/{id}/mark-paid` | 03-api-endpoints | Manual mark paid + sync to master_data + create payment_log |
| 29 | POST `/invoices/{id}/send-reminder` | 03-api-endpoints | Increment reminder_count, render template, send via channel, log |
| 30 | POST `/invoices/{id}/update-stage` | 03-api-endpoints | Manual collection stage override |
| 31 | GET `/invoices/stats` | 03-api-endpoints | Stat cards: total, by_status, amount_by_status, by_collection_stage |
| 32 | Sync invoice status to master_data | 01-overview | On mark-paid: update master_data.payment_status, last_payment_date, renewed |

### Medium Priority — Paper.id Integration

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 33 | Paper.id API integration | 01-overview | Call Paper.id to create invoice, get paper_id_url and paper_id_ref |
| 34 | POST `/webhooks/paper-id` | 03-api-endpoints | Receive payment webhook, verify HMAC, update invoice + master_data |
| 35 | Webhook HMAC verification | 03-api-endpoints | Verify X-Paper-Signature using PAPER_ID_WEBHOOK_SECRET |
| 36 | Idempotent webhook handling | 03-api-endpoints | Skip already-paid invoices, return 200 for unknown invoice_ids |

### Medium Priority — Cron Jobs

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 37 | Overdue status cron (daily) | 02-database-schema | Mark invoices as Terlambat when due_date < NOW(), update days_overdue |
| 38 | Collection stage auto-escalation cron | 02-database-schema | Stage 1 (D+1-3), Stage 2 (D+4-7), Stage 3 (D+8-14), Stage 4 (D+15+) |
| 39 | Master data sync cron | 02-database-schema | Update master_data.payment_status based on latest invoice status |

### Low Priority

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 40 | GET `/invoices/activity` | 03-api-endpoints | Activity feed from payment_logs (recent payments, reminders, escalations) |
| 41 | GET `/invoices/export` | 03-api-endpoints | Export invoices as Excel with all columns |
| 42 | Holding view aggregation | 03-api-endpoints | Query across member workspaces, enrich with workspace_name |
| 43 | Seed data: sample invoices | 01-overview | Migrate mock invoice data for demo/testing |

### Checker-Maker Approval

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 50 | Checker-maker for POST `/invoices` | 03-api-endpoints | Require approval (type: `create_invoice`) before creating new invoices |
| 51 | Checker-maker for POST `/invoices/{id}/mark-paid` | 03-api-endpoints | Require approval (type: `mark_paid`) before marking invoices as paid |

## NOT DONE — Optional Frontend Improvements 🟡 (6 items)

| # | Item | Priority | Description |
|---|------|----------|-------------|
| 44 | Connect create invoice to POST API | High | Replace setTimeout simulation with real POST `/invoices` call |
| 45 | Connect mark-paid to POST API | High | Call POST `/invoices/{id}/mark-paid` instead of client-side state update |
| 46 | Connect send-reminder to POST API | High | Call POST `/invoices/{id}/send-reminder` with template selection |
| 47 | Connect stats to GET API | Medium | Fetch from `/invoices/stats` instead of computing client-side from loaded data |
| 48 | Connect activity feed to GET API | Medium | Fetch from `/invoices/activity` instead of mock data |
| 49 | Invoice export button | Low | Call GET `/invoices/export` to download Excel |

---

## Recommended Implementation Order (Backend)

```
Week 1: #19 invoices + #20 invoice_line_items + #22 invoice_sequences
         #23 GET /invoices + #24 GET /invoices/{id}
Week 2: #21 payment_logs + #25 POST /invoices (create + ID generation)
         #26 PUT + #27 DELETE
Week 3: #28 mark-paid + #30 update-stage + #31 GET /invoices/stats
         #32 sync to master_data
Week 4: #29 send-reminder (needs template rendering from messaging feature)
         #37 overdue cron + #38 collection stage cron + #39 master data sync cron
Week 5: #33-36 Paper.id integration + webhook
Later:  #40 activity feed + #41 export + #42 holding view + #43 seed data
```

## Dependency Chain

```
invoices ──→ invoice_line_items
  │          invoice_sequences ──→ atomic ID generation
  │
  ├──→ GET /invoices + GET /invoices/{id}
  │
  ├──→ POST /invoices ──→ Paper.id API integration ──→ paper_id_url/ref
  │
  ├──→ payment_logs ──→ POST /mark-paid ──→ sync to master_data
  │                  ──→ POST /send-reminder ──→ message_templates (rendering)
  │                  ──→ POST /webhooks/paper-id ──→ HMAC verification
  │
  ├──→ overdue cron ──→ collection stage cron ──→ master_data sync cron
  │
  └──→ GET /invoices/stats + /activity (from payment_logs)
```
