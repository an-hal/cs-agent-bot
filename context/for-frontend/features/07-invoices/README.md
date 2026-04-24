# feat/07 — Invoices

Full billing: invoices + line items + payment logs + Paper.id integration +
auto-resend reminders + PDF download.

## Status

**✅ 100%** — CRUD, Paper.id webhook → `payment_status=Lunas` sync, PDF
generator, aging cron, escalate cron, sequence numbers per workspace.

## Core CRUD

```
GET    /invoices?status=Pending&company_id=ACME&limit=50
POST   /invoices                                    # 202 + ApprovalRequest (create_invoice)
GET    /invoices/stats                              # includes unique_companies + lunas_pct
GET    /invoices/by-stage
GET    /invoices/{invoice_id}
PUT    /invoices/{invoice_id}                       # patch (payment_status stripped from body — only Paper.id webhook can write it)
DELETE /invoices/{invoice_id}
POST   /invoices/{invoice_id}/mark-paid             # 202 + ApprovalRequest (mark_invoice_paid)
POST   /invoices/{invoice_id}/send-reminder
GET    /invoices/{invoice_id}/payment-logs
GET    /invoices/{invoice_id}/pdf                   # HTML (client-side renderable to PDF)
GET    /invoices/{invoice_id}/activity              # unified timeline (payment_logs + mutations)
POST   /invoices/{invoice_id}/update-stage          # AE-only manual stage override
PUT    /invoices/{invoice_id}/confirm-partial       # AE-only multi-termin partial payment
```

### Invoice response adds `company_name` + `termin_breakdown`

Top-level `company_name` resolved from master_data — no separate join needed.
`termin_breakdown` is empty for single-payment invoices, populated for multi-termin.

### Stats response (Wave C1 additions)

```json
{
  "data": {
    "total": 128, "total_amount": 456000000,
    "unique_companies": 42,        // NEW — distinct count
    "lunas_pct": 73.5,             // NEW — percentage paid
    "by_status": {...}, "amount_by_status": {...}, "by_collection_stage": {...}
  }
}
```

### Activity timeline

```
GET /invoices/{invoice_id}/activity?limit=50
```
Returns newest-first mix of `payment_log` + mutation entries. Use for the
invoice detail page timeline.

### Update stage (AE-only)

```
POST /invoices/{invoice_id}/update-stage
{"new_stage": "Stage 2 — Firm", "reason": "Client requested extension"}
```
Writes a `payment_log` entry for audit.

### Confirm partial (multi-termin)

**Creating a multi-termin invoice:**
```
POST /invoices
{
  "company_id": "ACME-001",
  "due_date": "2026-06-30",
  "payment_method_route": "transfer_bank",
  "line_items": [{"description":"Annual","qty":1,"unit_price":120000000,"subtotal":120000000}],
  "termin_breakdown": [
    {"termin_number":1,"amount":40000000,"due_date":"2026-04-30","status":"pending"},
    {"termin_number":2,"amount":40000000,"due_date":"2026-05-31","status":"pending"},
    {"termin_number":3,"amount":40000000,"due_date":"2026-06-30","status":"pending"}
  ]
}
```
BE validates `sum(termin.amount) == line_item total` → 422 if mismatch.

**Confirming each termin:**
```
PUT /invoices/{invoice_id}/confirm-partial
{"termin_number":1,"amount_paid":40000000,"payment_method":"transfer_bank",
 "payment_ref":"TRX-001","paid_at":"2026-04-30T10:00:00Z","notes":"..."}
```
- Termin N moves to `status=paid`
- Invoice `amount_paid` updates to running total
- **When all termins paid** → `payment_status=Lunas` auto-flips
- Writes `payment_log` entry `event_type=partial_paid`

### Create body

```json
{
  "company_id": "ACME-001",
  "due_date": "2026-05-31",
  "line_items": [
    {"description": "HRIS subscription — April", "qty": 1, "unit_price": 12000000}
  ],
  "notes": "optional"
}
```

BE auto-generates `invoice_id` per workspace (`invoice_sequences` table).
Response: `202 Accepted` with an ApprovalRequest — another user must apply.

### Mark as paid (AE confirms transfer)

```
POST /invoices/{invoice_id}/mark-paid
{"note": "confirmed transfer Mandiri 24/04"}
```

Returns `202` with an ApprovalRequest (type `mark_invoice_paid`). After the
checker approves via `/approvals/{id}/apply`, `payment_status` flips to
`Lunas` and a payment_log entry is written.

### Send reminder (manual)

```
POST /invoices/{invoice_id}/send-reminder
{"channel": "wa", "template_id": "PRE7_REMINDER"}
```

BE dispatches via configured integration. Mocked in dev — check
`/mock/outbox?provider=haloai` or `provider=smtp`.

### PDF download

```
GET /invoices/{invoice_id}/pdf
```

Returns HTML `Content-Type: text/html` with embedded CSS — suitable for
client-side PDF generation via headless Chrome / print-to-pdf. Includes:
- Invoice metadata + company block
- Line items table
- Subtotal, Tax (PPN 11%), Total
- Paper.id payment link if configured
- Status badge (Lunas/Pending/Overdue)

FE can:
- Embed in an `<iframe>` for preview
- `window.print()` for PDF
- Send to server-side Chromium for PDF generation

## Paper.id integration (real)

Real HMAC-signed webhook + status sync. See
[../00-shared/02-workspace-integrations.md](../00-shared/02-workspace-integrations.md)
for per-workspace config.

### Webhook (Paper.id → BE)
```
POST /webhook/paperid/{workspace_id}
Headers: X-Signature: {hmac_hex}
Body: {
  "external_id": "INV-2026-001",     // = invoice_id
  "status": "paid",
  "amount_paid": 12000000,
  "payment_method": "bank_transfer",
  "paid_at": "2026-04-24T00:00:00Z"
}
```

BE verifies HMAC using the workspace's `paper_id_webhook_secret`, then:
1. Flips `invoice.payment_status = Lunas`
2. Writes a `payment_log` entry (`event_type=paperid_webhook`)
3. Records timestamps

**This is the ONLY path that writes `payment_status=Lunas` directly** —
exception to the "bot never writes payment fields" rule.

## Aging cron

```
# Nightly via Cloud Scheduler OIDC:
GET /cron/invoices/overdue         # marks past-due as Terlambat
GET /cron/invoices/escalate        # promotes collection_stage (stage1→4)
```

**Auto-resend cadence** (exposed via cron hook `AutoResendReminders`) —
fires a reminder for any invoice matching the cadence offset from due_date:

| Offset | When |
|---|---|
| +14 days | Pre-due friendly heads-up |
| +7 days | Pre-due reminder |
| +3 days | Pre-due urgent |
| 0 | Due today |
| -3 days | Soft follow-up |
| -8 days | Empathetic firm |
| -15 days | Final pre-suspend |

Idempotent — skips `Lunas` invoices. Emits `SendReminder` which logs a
`payment_log`.

## Data model

See [../../05-data-models.md](../../05-data-models.md#invoice).

## FE UX

**List page:**
- Stats bar: total Pending, total Terlambat, total Lunas this month
- Filter chips: `status`, `company_id`, `stage`
- Export button

**Detail page:**
- Invoice metadata + line items + edit/delete
- Payment logs timeline (every event: issued, reminder, payment)
- "Mark paid" button (confirms approval flow)
- "Download PDF" (opens `/pdf` route, print dialog)
- Paper.id link if configured

**Reminder flow:**
- Manual: click "Send reminder" → template picker → dispatched
- Auto: no UI — cron handles; surface recent auto-sends in payment logs

**Approval queue:**
- Pending invoice creates + mark-paid requests show up for checkers
- Checker uses `/approvals/{id}/apply`

## Collection stages

When overdue, cron auto-escalates:

| Days overdue | Stage | Typical action |
|---|---|---|
| 1–6 | `stage1` | Soft reminder |
| 7–13 | `stage2` | Firm reminder + AE escalation |
| 14–20 | `stage3` | Manager alert |
| 21–29 | `stage4` | Pre-suspend warning |
| 30+ | `stage4` | Suspend/legal |

FE shows the stage as a pill color (green → yellow → red).
