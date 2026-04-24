# API Endpoints — Invoices & Billing

## Base URL
```
{BACKEND_API_URL}/api/v1
```

All endpoints require `Authorization: Bearer {token}` header.
Workspace-scoped endpoints require `X-Workspace-ID: {uuid}` header.

---

## 1. Invoice CRUD

### GET `/invoices`
List invoices dengan pagination + filter + search.

```
Query params:
  ?offset=0&limit=20
  &workspace_id=uuid               (required for non-holding)
  &search=keyword                  (searches invoice_id, company_name, company_id)
  &payment_status=Terlambat        (optional, comma-separated)
  &collection_stage=Stage 1        (optional)
  &sort_by=due_date                (optional, default: created_at)
  &sort_dir=desc                   (optional: asc/desc)

Response 200:
{
  "data": [
    {
      "invoice_id": "INV-DE-2026-001",
      "company_id": "C00001",
      "company_name": "PT Maju Digital",
      "workspace_id": "uuid",
      "amount": 25000000,
      "issue_date": "2026-01-15",
      "due_date": "2026-02-15",
      "payment_status": "Lunas",
      "paid_at": "2026-02-10",
      "amount_paid": 25000000,
      "payment_method": "Transfer BCA",
      "days_overdue": 0,
      "reminder_count": 0,
      "last_reminder_date": null,
      "collection_stage": "Closed",
      "notes": "Pembayaran tepat waktu",
      "link_invoice": "https://pay.paper.id/invoice/inv-de-001",
      "created_at": "2026-01-15T10:00:00Z"
    }
  ],
  "meta": {
    "offset": 0,
    "limit": 20,
    "total": 45
  }
}
```

> **Catatan**: Frontend saat ini fetch via `/api/data-master/invoices` sebagai proxy.
> Backend real endpoint tetap `/invoices`.
> Response field `invoice_id` (bukan `id`) sesuai kontrak API yang sudah dipakai frontend.

### GET `/invoices/{invoice_id}`
Get single invoice with line items.

```
Response 200:
{
  "data": {
    "invoice_id": "INV-DE-2026-001",
    "company_id": "C00001",
    "company_name": "PT Maju Digital",
    ... all fields ...,
    "line_items": [
      {
        "id": "uuid",
        "description": "Job Posting Premium — 12 bulan",
        "qty": 12,
        "unit_price": 1500000,
        "subtotal": 18000000
      },
      {
        "id": "uuid",
        "description": "ATS Module",
        "qty": 1,
        "unit_price": 5000000,
        "subtotal": 5000000
      }
    ],
    "payment_logs": [
      {
        "id": "uuid",
        "event_type": "payment_received",
        "amount_paid": 25000000,
        "payment_method": "Transfer BCA",
        "actor": "paper_id_webhook",
        "timestamp": "2026-02-10T14:30:00Z"
      }
    ]
  }
}
```

### POST `/invoices`
Create new invoice. Routes through Paper.id rail or Bank-transfer rail based on `payment_method_route`. [Gap #11 + #56]

```json
// Request body:
{
  "company_id": "C00001",
  "company_name": "PT Maju Digital",
  "workspace_id": "uuid",
  "issue_date": "2026-04-12",
  "payment_terms": 30,
  "payment_method_route": "paper_id",     // 'paper_id' | 'transfer_bank'
  "bank_account": null,                    // required if route = 'transfer_bank'
  "verification_url": null,                // required if route = 'transfer_bank'
  "notes": "Perpanjangan kontrak 12 bulan",
  "termin_breakdown": null,                // optional — see Partial Payments [Gap #60]
  "line_items": [
    { "description": "Job Posting Premium", "qty": 12, "unit_price": 1500000 },
    { "description": "ATS Module", "qty": 1, "unit_price": 5000000 }
  ]
}
```

**Validation per route:**
- `payment_method_route = 'paper_id'` → backend calls Paper.id API, populates `paperid_invoice_id` + `paperid_link`. Reject if Paper.id API fails (return 502).
- `payment_method_route = 'transfer_bank'` → `bank_account` + `verification_url` required in request, no Paper.id call. Reject 400 if either missing.

```json
// Response 400 (route validation):
{
  "error": "Validation failed",
  "details": [
    { "field": "verification_url", "message": "Required when payment_method_route = 'transfer_bank'" }
  ]
}
```

Backend flow:
1. Generate invoice ID via `invoice_sequences` table
2. Calculate `amount` = sum of (qty * unit_price) for all line items
3. Calculate `due_date` = issue_date + payment_terms days
4. Insert `invoices` record (incl. `payment_method_route`, route-specific fields, `termin_breakdown`)
5. Insert `invoice_line_items` records
6. If `payment_method_route = 'paper_id'` → **Call Paper.id API** to create invoice (auth: OAuth2 / API key — see Paper.id docs) → get `paperid_invoice_id` + `paperid_link`
7. Create `payment_logs` entry with `event_type = 'created'`
8. Bot resolves template at send-time: `TPL-AE-PAYMENT-PAPERID` if route = 'paper_id', else `TPL-AE-PAYMENT-BANK`

```json
// Response 201:
{
  "data": {
    "invoice_id": "INV-DE-2026-042",
    "amount": 23000000,
    "due_date": "2026-05-12",
    "payment_status": "Belum bayar",
    "collection_stage": "Stage 0 — Pre-due",
    "link_invoice": "https://pay.paper.id/invoice/inv-de-042",
    "line_items": [ ... ]
  }
}
```

### PUT `/invoices/{invoice_id}`
Update invoice details (notes, due_date, etc.). Cannot change amount after Paper.id link created.

```json
// Request body:
{
  "notes": "Updated notes",
  "due_date": "2026-05-20"
}

// Response 200:
{
  "data": { ... updated invoice ... }
}

// Response 400 (if trying to change amount after Paper.id link):
{
  "error": "Cannot change amount after Paper.id invoice has been created"
}
```

### DELETE `/invoices/{invoice_id}`
Only allowed if `payment_status = 'Belum bayar'` (draft invoice).

```
Response 200:
{
  "message": "Deleted",
  "invoice_id": "INV-DE-2026-042"
}

Response 400:
{
  "error": "Cannot delete invoice with payment_status 'Menunggu'. Void the invoice instead."
}
```

---

## 2. Invoice Actions

### POST `/invoices/{invoice_id}/mark-paid`
Manual mark as paid (for payments received outside Paper.id).

```json
// Request body:
{
  "payment_date": "2026-04-12",
  "payment_method": "Transfer BCA",
  "amount_paid": 25000000,
  "notes": "Bukti transfer diterima via email"
}

// Response 200:
{
  "data": {
    "invoice_id": "INV-DE-2026-001",
    "payment_status": "Lunas",
    "payment_date": "2026-04-12",
    "collection_stage": "Closed",
    "days_overdue": 0
  }
}
```

Backend must:
1. Update invoice: `payment_status = 'Lunas'`, `collection_stage = 'Closed'`
2. Create `payment_logs` entry with `event_type = 'manual_mark_paid'`
3. **Sync to Master Data**: update `master_data.payment_status` and `last_payment_date`

### POST `/invoices/{invoice_id}/send-reminder`
Send payment reminder to client.

```json
// Request body (optional):
{
  "channel": "whatsapp",
  "template_id": "TPL-PAY-PRE14",
  "custom_message": null
}

// Response 200:
{
  "data": {
    "invoice_id": "INV-DE-2026-001",
    "reminder_count": 3,
    "last_reminder_date": "2026-04-12",
    "channel_sent": "whatsapp",
    "template_used": "TPL-PAY-PRE14"
  }
}
```

Backend must:
1. Increment `invoices.reminder_count`
2. Update `invoices.last_reminder_date`
3. Render template with invoice + master_data variables
4. Send via channel (WA API / Email)
5. Create `payment_logs` entry with `event_type = 'reminder_sent'`
6. Create `action_logs` entry (links to master_data)

### POST `/invoices/{invoice_id}/update-stage`
Manually update collection stage (override auto-escalation).

```json
// Request body:
{
  "collection_stage": "Stage 3 — Urgency",
  "notes": "Eskalasi manual — PIC tidak merespons"
}

// Response 200:
{
  "data": {
    "invoice_id": "INV-DE-2026-001",
    "collection_stage": "Stage 3 — Urgency"
  }
}
```

Backend must:
1. Update `invoices.collection_stage`
2. Create `payment_logs` entry with `event_type = 'stage_change'`

---

## 3. Stats & Summary

### GET `/invoices/stats`
Stat cards untuk halaman Invoice.

```
Response 200:
{
  "total": 45,
  "total_amount": 650000000,
  "unique_companies": 20,
  "by_status": {
    "Lunas": 20,
    "Menunggu": 10,
    "Terlambat": 8,
    "Belum bayar": 7
  },
  "amount_by_status": {
    "Lunas": 350000000,
    "Menunggu": 150000000,
    "Terlambat": 120000000,
    "Belum bayar": 30000000
  },
  "lunas_pct": 54,
  "by_collection_stage": {
    "Stage 0": 17,
    "Stage 1": 3,
    "Stage 2": 2,
    "Stage 3": 2,
    "Stage 4": 1,
    "Closed": 20
  }
}
```

### GET `/invoices/activity`
Recent activity feed.

```
Query: ?limit=10

Response 200:
{
  "data": [
    {
      "id": "uuid",
      "text": "INV-DE-2026-004 marked as Paid",
      "event_type": "manual_mark_paid",
      "invoice_id": "INV-DE-2026-004",
      "actor": "arief@dealls.com",
      "time": "2026-03-30T14:20:00Z"
    },
    {
      "id": "uuid",
      "text": "Reminder sent to PT Delta Abadi",
      "event_type": "reminder_sent",
      "invoice_id": "INV-DE-2026-008",
      "actor": "system",
      "time": "2026-03-05T10:00:00Z"
    }
  ]
}
```

Built from `payment_logs` table, sorted by timestamp DESC.

### PUT `/invoices/{invoice_id}/confirm-partial` [Gap #60]
AE-only role gated. Marks a specific termin paid and resumes the bot's payment-reminder sequence.

```json
// Request body:
{
  "termin_no": 1,
  "paid_at": "2026-04-14",
  "amount_paid": 12500000,
  "payment_method": "Transfer BCA",
  "notes": "Bukti transfer termin 1 diterima"
}

// Response 200:
{
  "data": {
    "invoice_id": "INV-DE-2026-001",
    "partial_payment_status": "partial",     // computed: partial | fully_paid | pending
    "termin_breakdown": [
      { "termin_no": 1, "amount": 12500000, "due_date": "2026-04-15", "paid_at": "2026-04-14" },
      { "termin_no": 2, "amount": 12500000, "due_date": "2026-05-15", "paid_at": null }
    ],
    "next_template_id": "TPL-FP-TERMIN-2",   // bot resumes at next FP- template
    "sequence_resumed": true
  }
}

// Response 403:
{ "error": "AE role required to confirm partial payment" }

// Response 409:
{ "error": "Termin 1 already marked paid" }
```

Backend must:
1. **Role gate**: reject 403 if requester is not AE / AE Lead
2. Update the matching `termin_breakdown[]` element's `paid_at`
3. Recompute `partial_payment_status` (partial → fully_paid if all paid)
4. If `fully_paid` → also set `payment_status = 'Lunas'`, `collection_stage = 'Closed'`
5. Insert `payment_logs` entry with `event_type = 'partial_payment_confirmed'`
6. **Resume bot sequence** at next `FP-` (first-payment / next-termin) template

### GET `/invoices/{invoice_id}/payment-history` [Gap #60]
Returns termin breakdown + all payment_proof events.

```
Response 200:
{
  "data": {
    "invoice_id": "INV-DE-2026-001",
    "amount_total": 25000000,
    "amount_paid_total": 12500000,
    "partial_payment_status": "partial",
    "termin_breakdown": [
      { "termin_no": 1, "amount": 12500000, "due_date": "2026-04-15", "paid_at": "2026-04-14" },
      { "termin_no": 2, "amount": 12500000, "due_date": "2026-05-15", "paid_at": null }
    ],
    "payment_events": [
      {
        "event_type": "partial_payment_confirmed",
        "termin_no": 1,
        "amount_paid": 12500000,
        "payment_method": "Transfer BCA",
        "actor": "ae.lead@dealls.com",
        "timestamp": "2026-04-14T14:30:00Z"
      },
      {
        "event_type": "created",
        "actor": "ae@dealls.com",
        "timestamp": "2026-04-12T10:00:00Z"
      }
    ]
  }
}
```

Built from `invoices.termin_breakdown` + `payment_logs` filtered by `invoice_id`, sorted by `timestamp DESC`.

### GET `/invoices/legal-escalations` [Gap #67]
Lead+ role gated. Returns invoices flagged for legal escalation (overdue 30+ days).

```
Query params:
  ?cc_founder=true                 (optional — filter to enterprise > 50M)
  &offset=0&limit=20

Response 200:
{
  "data": [
    {
      "invoice_id": "INV-DE-2026-008",
      "company_id": "C00045",
      "company_name": "PT Delta Abadi",
      "amount": 75000000,
      "due_date": "2026-03-15",
      "days_overdue": 38,
      "legal_escalation": true,
      "legal_escalated_at": "2026-04-15T00:15:00Z",
      "legal_cc_recipient": "ka.dhika@dealls.com",
      "cc_founder": true,
      "collection_stage": "Stage 4 — Escalate"
    }
  ],
  "meta": { "offset": 0, "limit": 20, "total": 3 }
}

// Response 403:
{ "error": "Lead role required to view legal escalations" }
```

Backend must:
1. **Role gate**: reject 403 if requester is not AE Lead / Founder / Ops Lead
2. Query `invoices WHERE legal_escalation = TRUE` for current workspace
3. Cron at `00:15 WIB` is responsible for flipping the flag — see `02-database-schema.md` → Legal Escalation Flag

---

## 4. Paper.id Webhook

### POST `/webhooks/paper-id`
Receive payment notification from Paper.id.

```
Headers:
  X-Paper-Signature: {HMAC signature}
  Content-Type: application/json

Request body (from Paper.id):
{
  "event": "invoice.paid",
  "data": {
    "invoice_id": "paper-internal-id",
    "external_id": "INV-DE-2026-001",
    "amount_paid": 25000000,
    "payment_method": "bank_transfer",
    "payment_channel": "bca",
    "paid_at": "2026-02-10T14:30:00+07:00",
    "status": "paid"
  }
}

Response 200:
{
  "received": true
}
```

Backend must:
1. **Verify HMAC signature** — reject if invalid (return 401)
2. Find invoice by `external_id` (= our `invoices.id`)
3. If not found → log warning, return 200 (idempotent)
4. If already `Lunas` → skip, return 200 (idempotent)
5. Update invoice:
   - `payment_status = 'Lunas'`
   - `payment_date = data.paid_at`
   - `payment_method = data.payment_method + ' ' + data.payment_channel`
   - `amount_paid = data.amount_paid`
   - `collection_stage = 'Closed'`
   - `days_overdue = 0`
6. Create `payment_logs` entry:
   - `event_type = 'paper_id_webhook'`
   - `raw_payload = full webhook body as JSONB`
7. **Sync to Master Data**:
   - `master_data.payment_status = 'Paid'`
   - `master_data.last_payment_date = payment_date`
8. Optionally send payment verification message (TPL-PAY-VERIF) via WA

### Webhook Security
- Store Paper.id webhook secret in environment variable: `PAPER_ID_WEBHOOK_SECRET`
- Verify: `HMAC-SHA256(raw_body, secret)` must match `X-Paper-Signature` header
- Return 200 even for duplicate events (idempotent)
- Log all incoming webhooks regardless of processing result

---

## 5. Bulk Operations

### GET `/invoices/export`
Export invoices sebagai Excel.

```
Query: ?payment_status=Terlambat   (optional filter)

Response headers:
  Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
  Content-Disposition: attachment; filename="invoices-export-2026-04-12.xlsx"

Columns:
  Invoice ID, Perusahaan, Company ID, Jumlah, Tanggal Issue,
  Jatuh Tempo, Status, Hari Terlambat, Collection Stage,
  Reminder Count, Last Reminder, Paper.id URL, Notes,
  Tanggal Bayar, Metode Bayar
```

---

## 6. Holding View

Jika workspace adalah holding (`is_holding = true`):
- `GET /invoices` query semua member workspaces
- Setiap invoice di-enrich dengan `workspace_name` field
- Stats di-aggregate dari semua member workspaces
- Same logic as master-data holding view

Frontend mengirim multiple requests per workspace UUID saat holding view aktif.
Backend bisa optimize dengan accepting `workspace_ids` array:

```
GET /invoices?workspace_ids=uuid1,uuid2&offset=0&limit=20
```

---

## Error Responses (Standard)

```json
// 400 Bad Request
{
  "error": "Validation failed",
  "details": [
    { "field": "company_id", "message": "Required" }
  ]
}

// 401 Unauthorized
{
  "error": "Token expired or invalid"
}

// 403 Forbidden
{
  "error": "No access to this workspace"
}

// 404 Not Found
{
  "error": "Invoice not found"
}

// 409 Conflict
{
  "error": "Invoice already marked as Lunas"
}
```

---

## Checker-Maker Approval Required

The following endpoints require approval before execution.
See `00-shared/05-checker-maker.md` for the full approval system spec.

### POST `/invoices` → Approval Required

Instead of executing directly, create an approval request:

```
POST /approvals
{
  "request_type": "create_invoice",
  "payload": {
    "company_id": "C00001",
    "company_name": "PT Maju Digital",
    "workspace_id": "uuid",
    "issue_date": "2026-04-12",
    "payment_terms": 30,
    "amount": 23000000,
    "line_items": [
      { "description": "Job Posting Premium", "qty": 12, "unit_price": 1500000 },
      { "description": "ATS Module", "qty": 1, "unit_price": 5000000 }
    ],
    "notes": "Perpanjangan kontrak 12 bulan"
  }
}
```

When approved, the system creates the invoice (generates ID, inserts records, calls Paper.id API).

**Approval payload format:**
| Field | Type | Description |
|-------|------|-------------|
| `company_id` | string | Client company ID |
| `company_name` | string | Display name for the approval reviewer |
| `workspace_id` | UUID | Target workspace |
| `issue_date` | date | Invoice issue date |
| `payment_terms` | number | Payment terms in days |
| `amount` | number | Pre-computed total amount for reviewer display |
| `line_items` | array | Full line items to create |
| `notes` | string | Invoice notes |

### POST `/invoices/{id}/mark-paid` → Approval Required

Instead of executing directly, create an approval request:

```
POST /approvals
{
  "request_type": "mark_paid",
  "payload": {
    "invoice_id": "INV-DE-2026-001",
    "company_name": "PT Maju Digital",
    "amount": 25000000,
    "payment_date": "2026-04-12",
    "payment_method": "Transfer BCA",
    "amount_paid": 25000000,
    "notes": "Bukti transfer diterima via email"
  }
}
```

When approved, the system marks the invoice as paid, creates payment_log, and syncs to master_data.

**Approval payload format:**
| Field | Type | Description |
|-------|------|-------------|
| `invoice_id` | string | Invoice ID to mark as paid |
| `company_name` | string | Display name for the approval reviewer |
| `amount` | number | Original invoice amount for context |
| `payment_date` | date | When payment was received |
| `payment_method` | string | How payment was received |
| `amount_paid` | number | Amount received |
| `notes` | string | Payment notes / proof reference |
