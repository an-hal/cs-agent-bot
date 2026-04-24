# Invoices & Billing — Backend Implementation Guide

## Context
Dashboard ini adalah multi-workspace CRM (Dealls, KantorKu, Sejutacita holding).
Setiap workspace mengelola invoice untuk kliennya. Invoice terhubung ke Master Data via `company_id` dan ke Paper.id sebagai payment gateway.

## Invoice Lifecycle

```
                     ┌─────────────┐
                     │  CREATED    │
                     │  (Draft)    │
                     └──────┬──────┘
                            │  Issue invoice
                            ▼
                     ┌─────────────┐
               ┌─────│  BELUM BAYAR│─────┐
               │     └──────┬──────┘     │
               │            │            │
               │  Send to   │  Paper.id  │  Direct
               │  client    │  link gen  │  payment
               │            ▼            │
               │     ┌─────────────┐     │
               │     │  MENUNGGU   │     │
               │     │  (Pending)  │     │
               │     └──────┬──────┘     │
               │            │            │
               │     ┌──────┴──────┐     │
               │     │             │     │
               │     ▼             ▼     │
        ┌──────┴──────┐   ┌─────────────┐│
        │  TERLAMBAT   │   │   LUNAS     │◄┘
        │  (Overdue)   │   │   (Paid)    │
        └──────┬───────┘   └──────┬──────┘
               │                  │
               │  Payment         │
               │  received        │
               │─────────────────►│
               │                  ▼
               │           ┌─────────────┐
               └──────────►│   CLOSED    │
                           │ (completed) │
                           └─────────────┘
```

### Payment Status Values
| Status | DB Value | Deskripsi |
|--------|----------|-----------|
| Lunas | `Lunas` | Pembayaran diterima dan diverifikasi |
| Menunggu | `Menunggu` | Invoice terkirim, menunggu pembayaran |
| Belum bayar | `Belum bayar` | Invoice dibuat tapi belum dikirim ke klien |
| Terlambat | `Terlambat` | Sudah melewati jatuh tempo |

### Status Transitions
- `Belum bayar` --> `Menunggu` (invoice dikirim ke klien / Paper.id link di-generate)
- `Menunggu` --> `Lunas` (pembayaran diterima via Paper.id webhook atau manual mark)
- `Menunggu` --> `Terlambat` (cron job: `due_date < NOW()` dan belum bayar)
- `Terlambat` --> `Lunas` (pembayaran diterima meskipun terlambat)
- `Belum bayar` --> `Lunas` (direct payment tanpa melalui fase menunggu)

## Collection Stages

Collection stage menentukan intensitas follow-up untuk invoice yang overdue.
Stage diupdate otomatis oleh cron atau manual oleh AE.

```
Stage 0 — Pre-due     │ Invoice belum jatuh tempo. Monitoring saja.
                       │
Stage 1 — Soft         │ D+1 to D+3 overdue. Reminder sopan.
                       │ "Ada yang bisa kami bantu?"
                       │
Stage 2 — Firm         │ D+4 to D+7 overdue. Reminder tegas.
                       │ "Mohon segera proses pembayaran."
                       │
Stage 3 — Urgency      │ D+8 to D+14 overdue. Warning akses dibatasi.
                       │ Eskalasi ke manajemen klien.
                       │
Stage 4 — Escalate     │ D+15+ overdue. Bot stop, AE manual.
                       │ Diteruskan ke legal jika perlu.
                       │
Closed                 │ Invoice sudah lunas (bayar tepat waktu atau setelah overdue).
```

### Auto-escalation Rules (Cron Job)

```
IF days_overdue BETWEEN 1 AND 3  → Stage 1 — Soft
IF days_overdue BETWEEN 4 AND 7  → Stage 2 — Firm
IF days_overdue BETWEEN 8 AND 14 → Stage 3 — Urgency
IF days_overdue >= 15            → Stage 4 — Escalate
IF payment_status = 'Lunas'      → Closed
```

## Relasi ke Master Data

Invoice terhubung ke Master Data via `company_id`:

```
master_data (company)
  │
  ├── company_id: 'C00001'
  ├── company_name: 'PT Maju Digital'
  ├── payment_status: 'Paid'        ◄── synced from latest invoice
  ├── final_price: 25_000_000       ◄── synced from contract
  └── last_payment_date: '2026-02-10' ◄── synced from latest paid invoice
  
invoices
  │
  ├── id: 'INV-DE-2026-001'
  ├── company_id: 'C00001'          ◄── FK to master_data
  ├── amount: 25_000_000
  ├── payment_status: 'Lunas'
  └── payment_date: '2026-02-10'
```

Saat invoice di-mark paid, backend harus **JUGA update Master Data**:
1. `master_data.payment_status = 'Paid'`
2. `master_data.last_payment_date = invoice.payment_date`
3. Jika renewal invoice: `master_data.renewed = TRUE`

## Paper.id Integration

Paper.id digunakan sebagai payment gateway dan invoice management platform.

### Flow

```
1. User creates invoice in Dashboard
   │
2. Backend calls Paper.id API → create invoice
   │
3. Paper.id returns invoice URL (link_invoice)
   │
4. URL disimpan di invoices.paper_id_url
   │
5. Client bayar via Paper.id (transfer, VA, QRIS, dll)
   │
6. Paper.id sends webhook → backend endpoint
   │
7. Backend updates invoice status → Lunas
   │
8. Backend updates Master Data payment_status
```

### Paper.id Webhook Payload (expected)

```json
{
  "event": "invoice.paid",
  "data": {
    "invoice_id": "paper-id-invoice-id",
    "external_id": "INV-DE-2026-001",
    "amount_paid": 25000000,
    "payment_method": "bank_transfer",
    "payment_channel": "bca",
    "paid_at": "2026-02-10T14:30:00+07:00",
    "status": "paid"
  }
}
```

Backend harus:
1. Verify webhook signature (HMAC dari Paper.id)
2. Match `external_id` ke `invoices.id`
3. Update invoice: `payment_status = 'Lunas'`, `payment_date`, `payment_method`
4. Update Master Data: `payment_status`, `last_payment_date`
5. Log payment ke `payment_logs`

## Invoice ID Convention

Format: `INV-{WS}-{YEAR}-{SEQ}`

| Workspace | Prefix | Contoh |
|-----------|--------|--------|
| Dealls | `INV-DE` | `INV-DE-2026-001` |
| KantorKu | `INV-KK` | `INV-KK-2026-001` |

Sequential per workspace per year. Backend harus generate ID atomically (avoid duplicates).

## Invoice Line Items

Invoice bisa punya multiple line items:

```
Invoice INV-DE-2026-001
  │
  ├── Line 1: Job Posting Premium — 12 bulan × Rp 1.500.000 = Rp 18.000.000
  ├── Line 2: ATS Module — 1 × Rp 5.000.000 = Rp 5.000.000
  └── Line 3: Setup Fee — 1 × Rp 2.000.000 = Rp 2.000.000
                                              ──────────────
                                    Total:     Rp 25.000.000
```

## Stat Cards (Frontend)

Frontend menampilkan 4 stat cards:
1. **Total Invoice** — jumlah invoice + jumlah perusahaan unik
2. **Total Nilai** — total amount semua invoice + persentase lunas
3. **Overdue** — jumlah invoice overdue + total nilai tertunggak
4. **Lunas** — jumlah invoice lunas + total nilai terkumpul

Plus right panel:
- Jumlah per collection stage (Stage 0-4)
- Activity feed (recent payments, reminders, escalations)

## Reminder System

AE bisa kirim reminder ke klien via:
1. **Manual trigger** dari dashboard (tombol "Kirim Reminder")
2. **Automated** via workflow engine (P5-P6 payment templates)

Setiap reminder di-track: `reminder_count` di-increment, `last_reminder_date` di-update.

Template reminder diambil dari `message_templates` (e.g. `TPL-PAY-PRE14`, `TPL-PAY-POST1`).
