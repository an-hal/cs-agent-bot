# Database Schema — PostgreSQL

## Tabel Utama

### 1. `invoices` — Record invoice per klien

```sql
CREATE TABLE invoices (
  -- ══════════════════════════════════════════════════════════════
  -- IDENTITY
  -- ══════════════════════════════════════════════════════════════
  
  id                VARCHAR(50) PRIMARY KEY,        -- e.g. 'INV-DE-2026-001'
  workspace_id      UUID NOT NULL REFERENCES workspaces(id),
  company_id        VARCHAR(50) NOT NULL,            -- FK logical to master_data.company_id
  
  -- ══════════════════════════════════════════════════════════════
  -- INVOICE DETAILS
  -- ══════════════════════════════════════════════════════════════
  
  amount            BIGINT NOT NULL DEFAULT 0,       -- Total in IDR (sum of line items)
  issue_date        DATE,                            -- Tanggal terbit invoice
  due_date          DATE,                            -- Tanggal jatuh tempo
  payment_terms     INT DEFAULT 30,                  -- Net days (30, 60, 90)
  notes             TEXT DEFAULT '',                  -- Catatan internal
  
  -- ══════════════════════════════════════════════════════════════
  -- PAYMENT STATUS
  -- ══════════════════════════════════════════════════════════════
  
  payment_status    VARCHAR(20) NOT NULL DEFAULT 'Belum bayar',
                    -- Allowed: Lunas, Menunggu, Belum bayar, Terlambat
  payment_date      DATE,                            -- Tanggal pembayaran diterima
  payment_method    VARCHAR(100),                    -- 'Transfer BCA', 'VA BNI', 'QRIS', etc.
  amount_paid       BIGINT DEFAULT 0,                -- Actual amount paid (could differ from amount)
  
  -- ══════════════════════════════════════════════════════════════
  -- COLLECTION
  -- ══════════════════════════════════════════════════════════════
  
  days_overdue      INT NOT NULL DEFAULT 0,          -- Computed by cron: max(0, NOW() - due_date)
  collection_stage  VARCHAR(30) NOT NULL DEFAULT 'Stage 0 — Pre-due',
                    -- Allowed: 'Stage 0 — Pre-due', 'Stage 1 — Soft',
                    --          'Stage 2 — Firm', 'Stage 3 — Urgency',
                    --          'Stage 4 — Escalate', 'Closed'
  reminder_count    INT NOT NULL DEFAULT 0,          -- Berapa kali reminder dikirim
  last_reminder_date DATE,                           -- Tanggal terakhir reminder dikirim
  
  -- ══════════════════════════════════════════════════════════════
  -- PAPER.ID INTEGRATION
  -- ══════════════════════════════════════════════════════════════
  
  paper_id_url      TEXT,                            -- Paper.id invoice URL for client payment
  paper_id_ref      VARCHAR(100),                    -- Paper.id internal reference ID
  
  -- ══════════════════════════════════════════════════════════════
  -- META
  -- ══════════════════════════════════════════════════════════════
  
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by        VARCHAR(255),                    -- User email who created
  
  UNIQUE(workspace_id, id)
);

-- ══ Indexes ══
CREATE INDEX idx_inv_workspace         ON invoices(workspace_id);
CREATE INDEX idx_inv_workspace_company ON invoices(workspace_id, company_id);
CREATE INDEX idx_inv_workspace_status  ON invoices(workspace_id, payment_status);
CREATE INDEX idx_inv_workspace_due     ON invoices(workspace_id, due_date);
CREATE INDEX idx_inv_workspace_stage   ON invoices(workspace_id, collection_stage);
CREATE INDEX idx_inv_days_overdue      ON invoices(workspace_id, days_overdue) WHERE days_overdue > 0;
CREATE INDEX idx_inv_paper_id_ref      ON invoices(paper_id_ref) WHERE paper_id_ref IS NOT NULL;

-- Auto-update updated_at
CREATE TRIGGER trg_inv_updated_at
  BEFORE UPDATE ON invoices
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at();
  -- Reuses function from master_data schema
```

### 2. `invoice_line_items` — Detail baris invoice

```sql
CREATE TABLE invoice_line_items (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  invoice_id        VARCHAR(50) NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
  workspace_id      UUID NOT NULL REFERENCES workspaces(id),
  
  description       VARCHAR(500) NOT NULL,           -- 'Job Posting Premium — 12 bulan'
  qty               INT NOT NULL DEFAULT 1,
  unit_price        BIGINT NOT NULL DEFAULT 0,       -- Harga per unit in IDR
  subtotal          BIGINT NOT NULL DEFAULT 0,       -- qty × unit_price (computed on write)
  
  sort_order        INT DEFAULT 0,                   -- Urutan tampilan
  
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ili_invoice ON invoice_line_items(invoice_id);
CREATE INDEX idx_ili_workspace ON invoice_line_items(workspace_id);
```

### 3. `payment_logs` — Log pembayaran dan perubahan status

```sql
CREATE TABLE payment_logs (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id      UUID NOT NULL REFERENCES workspaces(id),
  invoice_id        VARCHAR(50) NOT NULL REFERENCES invoices(id),
  
  -- ══ Event type ══
  event_type        VARCHAR(30) NOT NULL,
                    -- Allowed: payment_received, payment_failed, status_change,
                    --          reminder_sent, stage_change, paper_id_webhook,
                    --          manual_mark_paid, created, updated
  
  -- ══ Payment details (for payment_received / paper_id_webhook) ══
  amount_paid       BIGINT,                          -- Amount in IDR
  payment_method    VARCHAR(100),                    -- 'Transfer BCA', 'VA BNI', etc.
  payment_channel   VARCHAR(50),                     -- Paper.id channel: 'bca', 'mandiri', 'qris'
  payment_ref       VARCHAR(200),                    -- External reference/transaction ID
  
  -- ══ Status change details ══
  old_status        VARCHAR(20),                     -- Previous payment_status
  new_status        VARCHAR(20),                     -- New payment_status
  old_stage         VARCHAR(30),                     -- Previous collection_stage
  new_stage         VARCHAR(30),                     -- New collection_stage
  
  -- ══ Context ══
  actor             VARCHAR(255),                    -- User email or 'system' or 'paper_id_webhook'
  notes             TEXT,                            -- Additional context
  raw_payload       JSONB,                           -- Raw webhook payload (for paper_id_webhook events)
  
  -- ══ Meta ══
  timestamp         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ══ Indexes ══
CREATE INDEX idx_pl_workspace      ON payment_logs(workspace_id);
CREATE INDEX idx_pl_invoice        ON payment_logs(invoice_id);
CREATE INDEX idx_pl_event_type     ON payment_logs(workspace_id, event_type);
CREATE INDEX idx_pl_timestamp      ON payment_logs(workspace_id, timestamp DESC);
```

### 4. `invoice_sequences` — Auto-increment ID per workspace per year

```sql
CREATE TABLE invoice_sequences (
  workspace_id      UUID NOT NULL REFERENCES workspaces(id),
  year              INT NOT NULL,
  last_seq          INT NOT NULL DEFAULT 0,
  
  PRIMARY KEY (workspace_id, year)
);
```

Penggunaan:
```sql
-- Atomic next ID generation
INSERT INTO invoice_sequences (workspace_id, year, last_seq)
VALUES ($1, $2, 1)
ON CONFLICT (workspace_id, year)
DO UPDATE SET last_seq = invoice_sequences.last_seq + 1
RETURNING last_seq;
-- Result: 42 → format as 'INV-DE-2026-042'
```

## Schema Extensions

### Payment Method Routing — Paper.id vs Bank Transfer [Gap #11 + #56]

Invoices route through one of two payment rails. Bot reads `payment_method` and selects the correct WA template (`TPL-AE-PAYMENT-PAPERID` vs `TPL-AE-PAYMENT-BANK`) at send-time.

```sql
ALTER TABLE invoices
  ADD COLUMN payment_method_route VARCHAR(20) NOT NULL DEFAULT 'paper_id'
    CHECK (payment_method_route IN ('paper_id', 'transfer_bank'));

-- Paper.id rail
ALTER TABLE invoices ADD COLUMN paperid_invoice_id VARCHAR(50) NULL;
ALTER TABLE invoices ADD COLUMN paperid_link       VARCHAR(500) NULL;

-- Bank transfer rail
ALTER TABLE invoices ADD COLUMN bank_account       VARCHAR(50) NULL;
ALTER TABLE invoices ADD COLUMN verification_url   VARCHAR(500) NULL;  -- manual proof upload link

-- Validation: enforce per-route required fields at app layer
-- paper_id   → paperid_invoice_id + paperid_link required
-- transfer_bank → bank_account + verification_url required
```

> Note: existing `invoices.payment_method` column (legacy, free-form 'Transfer BCA' / 'VA BNI' / etc.)
> remains for the **actual** rail used at payment time.
> New `payment_method_route` is the **intended** rail set at invoice creation,
> driving template selection. Paper.id auth: see Paper.id docs (OAuth2 / API key).

### Legal Escalation Flag — 30+ Day Overdue [Gap #67]

```sql
ALTER TABLE invoices ADD COLUMN legal_escalation     BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE invoices ADD COLUMN legal_escalated_at   TIMESTAMPTZ NULL;
ALTER TABLE invoices ADD COLUMN legal_cc_recipient   VARCHAR(255) NULL;
  -- e.g. 'ka.dhika@dealls.com' when amount > 50,000,000
ALTER TABLE invoices ADD COLUMN cc_founder           BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX idx_inv_legal_escalation
  ON invoices(workspace_id, legal_escalation)
  WHERE legal_escalation = TRUE;
```

**Cron trigger (daily, jam 00:15 WIB):**

```sql
-- Flag overdue 30+ for legal escalation; fire ESC-009
UPDATE invoices
SET legal_escalation   = TRUE,
    legal_escalated_at = NOW(),
    legal_cc_recipient = CASE WHEN amount > 50000000 THEN 'ka.dhika@dealls.com' ELSE NULL END,
    cc_founder         = (amount > 50000000),
    updated_at         = NOW()
WHERE payment_status = 'Terlambat'
  AND CURRENT_DATE - due_date > INTERVAL '30 days'
  AND legal_escalation = FALSE;
```

After flip → fire `ESC-009` to AE Lead, cc Ka Dhika if `cc_founder = TRUE`. See `06-workflow-engine` for escalation table.

### Partial Payment State Machine [Gap #60]

Supports termin-based payments (e.g. 50% upfront + 50% on delivery). When any termin is paid but not all, sequence pauses until AE confirms via `PUT /invoices/{id}/confirm-partial`.

```sql
ALTER TABLE invoices ADD COLUMN termin_breakdown JSONB NULL;
-- Shape: [{ "termin_no": 1, "amount": 12500000, "due_date": "2026-04-15", "paid_at": "2026-04-14" },
--         { "termin_no": 2, "amount": 12500000, "due_date": "2026-05-15", "paid_at": null }]

CREATE INDEX idx_inv_termin_partial
  ON invoices(workspace_id)
  WHERE termin_breakdown IS NOT NULL;
```

**Computed status `partial_payment_status`** (derived at read-time, NOT a column):

```sql
-- Pseudo-SQL — backend computes in app layer or via VIEW
CASE
  WHEN termin_breakdown IS NULL                                        THEN NULL
  WHEN NOT EXISTS (elem WHERE elem->>'paid_at' IS NULL)                THEN 'fully_paid'
  WHEN     EXISTS (elem WHERE elem->>'paid_at' IS NOT NULL)
       AND EXISTS (elem WHERE elem->>'paid_at' IS NULL)                THEN 'partial'
  ELSE                                                                      'pending'
END
```

**State machine:**
- `partial` → bot **pauses** payment-reminder sequence (no TPL-PAY-* sent until AE confirms)
- AE calls `PUT /invoices/{id}/confirm-partial` → bot resumes at next FP- (first-payment) template
- `fully_paid` → equivalent to `payment_status = 'Lunas'`, set `collection_stage = 'Closed'`

---

## Relasi Antar Tabel

```
workspaces
  │
  ├── 1:N → invoices
  ├── 1:N → payment_logs
  └── 1:N → invoice_sequences

invoices
  │
  ├── 1:N → invoice_line_items (detail baris)
  ├── 1:N → payment_logs (history)
  └── N:1 → master_data (via company_id — logical FK, not enforced)

master_data
  │
  └── payment_status, last_payment_date ← synced from invoices
```

## Cron Jobs

### 1. Update Overdue Status (harian, jam 00:05 WIB)

```sql
-- Mark overdue invoices
UPDATE invoices
SET payment_status = 'Terlambat',
    days_overdue = CURRENT_DATE - due_date,
    updated_at = NOW()
WHERE payment_status IN ('Menunggu', 'Belum bayar')
  AND due_date < CURRENT_DATE
  AND payment_status != 'Terlambat';

-- Update days_overdue for already-overdue invoices
UPDATE invoices
SET days_overdue = CURRENT_DATE - due_date,
    updated_at = NOW()
WHERE payment_status = 'Terlambat'
  AND due_date < CURRENT_DATE;
```

### 2. Auto-escalate Collection Stage (harian, jam 00:10 WIB)

```sql
-- Stage 1 — Soft (D+1 to D+3)
UPDATE invoices
SET collection_stage = 'Stage 1 — Soft', updated_at = NOW()
WHERE payment_status = 'Terlambat'
  AND days_overdue BETWEEN 1 AND 3
  AND collection_stage = 'Stage 0 — Pre-due';

-- Stage 2 — Firm (D+4 to D+7)
UPDATE invoices
SET collection_stage = 'Stage 2 — Firm', updated_at = NOW()
WHERE payment_status = 'Terlambat'
  AND days_overdue BETWEEN 4 AND 7
  AND collection_stage IN ('Stage 0 — Pre-due', 'Stage 1 — Soft');

-- Stage 3 — Urgency (D+8 to D+14)
UPDATE invoices
SET collection_stage = 'Stage 3 — Urgency', updated_at = NOW()
WHERE payment_status = 'Terlambat'
  AND days_overdue BETWEEN 8 AND 14
  AND collection_stage IN ('Stage 0 — Pre-due', 'Stage 1 — Soft', 'Stage 2 — Firm');

-- Stage 4 — Escalate (D+15+)
UPDATE invoices
SET collection_stage = 'Stage 4 — Escalate', updated_at = NOW()
WHERE payment_status = 'Terlambat'
  AND days_overdue >= 15
  AND collection_stage != 'Stage 4 — Escalate';
```

### 3. Sync to Master Data (after any invoice status change)

```sql
-- Update master_data payment_status based on latest invoice
UPDATE master_data md
SET payment_status = CASE
      WHEN EXISTS (
        SELECT 1 FROM invoices i
        WHERE i.company_id = md.company_id
          AND i.workspace_id = md.workspace_id
          AND i.payment_status = 'Terlambat'
      ) THEN 'Overdue'
      WHEN EXISTS (
        SELECT 1 FROM invoices i
        WHERE i.company_id = md.company_id
          AND i.workspace_id = md.workspace_id
          AND i.payment_status IN ('Menunggu', 'Belum bayar')
      ) THEN 'Pending'
      ELSE 'Paid'
    END,
    last_payment_date = (
      SELECT MAX(i.payment_date) FROM invoices i
      WHERE i.company_id = md.company_id
        AND i.workspace_id = md.workspace_id
        AND i.payment_status = 'Lunas'
    ),
    updated_at = NOW()
WHERE md.workspace_id = $1
  AND md.company_id = $2;
```

## Notes

### Invoice ID sebagai Primary Key (bukan UUID)
Sama dengan template system: human-readable ID, mudah di-log dan di-debug.
Generated atomically via `invoice_sequences` table.

### Amount in IDR (BIGINT, bukan DECIMAL)
Indonesia tidak pakai sen/cents. Semua harga dalam Rupiah penuh.
`BIGINT` lebih cepat dari `DECIMAL` dan cukup untuk Rp 9,2 quadrillion.

### Logical FK ke Master Data
`invoices.company_id` tidak di-enforce sebagai real FK karena:
1. Master Data bisa di-delete (soft delete), invoice harus tetap ada
2. Invoice bisa dibuat sebelum company masuk Master Data
3. Company ID format bisa beda antar workspace

Backend validate saat create: warn jika company_id tidak ada di master_data.
