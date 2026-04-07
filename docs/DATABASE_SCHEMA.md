# CS Agent Bot - Database Schema Documentation

> **Database**: PostgreSQL
> **Last Updated**: 2026-04-01
> **Total Tables**: 10 (operational) + 1 (system)

---

## Schema Diagram (Mermaid)

```mermaid
erDiagram
    clients ||--o{ invoices : "has many"
    clients ||--|| client_flags : "has one"
    clients ||--o{ escalations : "triggers"
    clients ||--o{ action_log : "logs"
    clients ||--o{ cron_log : "processed daily"
    clients ||--|| conversation_states : "has one"
    escalation_rules ||--o{ escalations : "defines rule"
    templates ||--o{ action_log : "used by"

    clients {
        VARCHAR(20) company_id PK
        VARCHAR(200) company_name
        VARCHAR(100) pic_name
        VARCHAR(20) pic_wa UK
        VARCHAR(100) pic_email
        VARCHAR(100) pic_role
        VARCHAR(20) hc_size
        VARCHAR(100) owner_name
        VARCHAR(20) owner_wa
        VARCHAR(30) owner_telegram_id
        VARCHAR(30) backup_owner_telegram_id
        BOOLEAN ae_assigned
        VARCHAR(30) ae_telegram_id
        VARCHAR(10) segment
        VARCHAR(50) plan_type
        VARCHAR(20) payment_terms
        DATE contract_start
        DATE contract_end
        SMALLINT contract_months
        DATE activation_date
        DECIMAL first_time_discount_pct
        DECIMAL next_discount_pct_manual
        DECIMAL final_price
        VARCHAR(500) quotation_link
        DATE quotation_link_expires
        DATE renewal_date
        VARCHAR(50) bd_prospect_id
        TEXT notes
        VARCHAR(20) payment_status
        DATE last_payment_date
        SMALLINT nps_score
        SMALLINT usage_score
        SMALLINT usage_score_avg_30d
        DATE last_interaction_date
        BOOLEAN risk_flag
        BOOLEAN bot_active
        BOOLEAN blacklisted
        BOOLEAN wa_undeliverable
        VARCHAR(20) response_status
        VARCHAR(20) sequence_cs
        BOOLEAN renewed
        BOOLEAN rejected
        VARCHAR(200) churn_reason
        BOOLEAN cross_sell_rejected
        BOOLEAN cross_sell_interested
        DATE cross_sell_resume_date
        SMALLINT days_since_cs_last_sent
        BOOLEAN feature_update_sent
        BOOLEAN checkin_replied
        TIMESTAMP created_at
    }

    invoices {
        VARCHAR(30) invoice_id PK
        VARCHAR(20) company_id FK
        DATE issue_date
        DATE due_date
        DECIMAL amount
        VARCHAR(20) payment_status
        TIMESTAMP paid_at
        DECIMAL amount_paid
        SMALLINT reminder_count
        VARCHAR(30) collection_stage
        BOOLEAN pre14_sent
        BOOLEAN pre7_sent
        BOOLEAN pre3_sent
        BOOLEAN post1_sent
        BOOLEAN post4_sent
        BOOLEAN post8_sent
        BOOLEAN post15_sent
        TIMESTAMP created_at
    }

    client_flags {
        VARCHAR(20) company_id PK_FK
        BOOLEAN ren60_sent
        BOOLEAN ren45_sent
        BOOLEAN ren30_sent
        BOOLEAN ren15_sent
        BOOLEAN ren0_sent
        BOOLEAN checkin_a1_form_sent
        BOOLEAN checkin_a1_call_sent
        BOOLEAN checkin_a2_form_sent
        BOOLEAN checkin_a2_call_sent
        BOOLEAN checkin_b1_form_sent
        BOOLEAN checkin_b1_call_sent
        BOOLEAN checkin_b2_form_sent
        BOOLEAN checkin_b2_call_sent
        BOOLEAN checkin_replied
        BOOLEAN nps1_sent
        BOOLEAN nps2_sent
        BOOLEAN nps3_sent
        BOOLEAN nps_replied
        BOOLEAN referral_sent_this_cycle
        BOOLEAN quotation_acknowledged
        BOOLEAN low_usage_msg_sent
        BOOLEAN low_nps_msg_sent
        BOOLEAN cs_h7
        BOOLEAN cs_h14
        BOOLEAN cs_h21
        BOOLEAN cs_h30
        BOOLEAN cs_h45
        BOOLEAN cs_h60
        BOOLEAN cs_h75
        BOOLEAN cs_h90
        BOOLEAN cs_lt1
        BOOLEAN cs_lt2
        BOOLEAN cs_lt3
    }

    conversation_states {
        VARCHAR(20) company_id PK_FK
        VARCHAR(200) company_name
        VARCHAR(30) active_flow
        VARCHAR(30) current_stage
        VARCHAR(50) last_message_type
        TIMESTAMP last_message_date
        VARCHAR(20) response_status
        VARCHAR(30) response_classification
        SMALLINT attempt_count
        TIMESTAMP cooldown_until
        BOOLEAN bot_active
        VARCHAR(200) reason_bot_paused
        VARCHAR(50) next_scheduled_action
        TIMESTAMP next_scheduled_date
        BOOLEAN human_owner_notified
        TIMESTAMP created_at
        TIMESTAMP updated_at
    }

    escalations {
        SERIAL id PK
        VARCHAR(10) esc_id
        VARCHAR(20) company_id FK
        TEXT trigger_condition
        VARCHAR(20) priority
        TEXT notified_party
        TEXT telegram_message_sent
        VARCHAR(20) status
        TIMESTAMP triggered_at
        TIMESTAMP resolved_at
        VARCHAR(100) resolved_by
        TEXT notes
    }

    escalation_rules {
        VARCHAR(10) esc_id PK
        VARCHAR(200) name
        TEXT trigger_condition
        VARCHAR(20) priority
        TEXT telegram_msg
        BOOLEAN active
        TIMESTAMP created_at
        TIMESTAMP updated_at
    }

    action_log {
        BIGSERIAL id PK
        VARCHAR(20) company_id FK
        TIMESTAMP triggered_at
        VARCHAR(50) trigger_type
        VARCHAR(50) template_id
        VARCHAR(100) message_id
        VARCHAR(10) channel
        VARCHAR(20) sent_to_wa
        BOOLEAN message_sent
        VARCHAR(20) status
        VARCHAR(30) intent
        TEXT response_received
        VARCHAR(30) response_classification
        VARCHAR(50) next_action_triggered
        VARCHAR(100) by_human
        TEXT log_notes
        VARCHAR(200) company_name
        TEXT notes
    }

    cron_log {
        SERIAL id PK
        DATE run_date
        VARCHAR(20) company_id FK
        VARCHAR(20) status
        TIMESTAMP processed_at
        TEXT error_message
    }

    system_config {
        VARCHAR(100) key PK
        TEXT value
        TEXT description
        TIMESTAMP updated_at
        VARCHAR(100) updated_by
    }

    templates {
        VARCHAR(50) template_id PK
        VARCHAR(100) template_name
        TEXT template_content
        VARCHAR(20) template_category
        VARCHAR(10) language
        BOOLEAN active
        TIMESTAMP created_at
        TIMESTAMP updated_at
    }
```

---

## Table Descriptions

### 1. `clients` - Master Data Klien HRIS

**Representasi**: Tabel utama yang menyimpan seluruh informasi klien SaaS HRIS (perusahaan yang menggunakan platform Kantorku).

**Tugas**:
- Menyimpan data identitas perusahaan dan PIC (Person in Charge)
- Menyimpan informasi kontrak (mulai, selesai, durasi, renewal)
- Menyimpan data komersial (harga, diskon, pembayaran, segmentasi)
- Menyimpan data ownership (owner, AE yang ditugaskan)
- Menyimpan status bot dan flag otomasi per klien
- Menyimpan health metrics (NPS, usage score, risk flag)
- Menyimpan state sequence CS (renewal, cross-sell, check-in)

**Kolom Kelompok**:

| Kelompok | Kolom | Keterangan |
|----------|-------|------------|
| **Identitas** | `company_id`, `company_name`, `hc_size` | Data dasar perusahaan |
| **PIC** | `pic_name`, `pic_wa`, `pic_email`, `pic_role` | Kontak utama di sisi klien |
| **Ownership** | `owner_name`, `owner_wa`, `owner_telegram_id`, `backup_owner_telegram_id` | Account owner dari internal |
| **AE** | `ae_assigned`, `ae_telegram_id` | Account Executive yang ditugaskan |
| **Segmentasi** | `segment`, `plan_type`, `payment_terms` | Segmentasi bisnis klien |
| **Kontrak** | `contract_start`, `contract_end`, `contract_months`, `activation_date`, `renewal_date` | Siklus kontrak |
| **Finansial** | `final_price`, `first_time_discount_pct`, `next_discount_pct_manual`, `quotation_link` | Data harga dan penawaran |
| **Health** | `nps_score`, `usage_score`, `usage_score_avg_30d`, `risk_flag`, `last_interaction_date` | Indikator kesehatan klien |
| **Status Bot** | `bot_active`, `blacklisted`, `wa_undeliverable`, `response_status`, `sequence_cs` | Kontrol otomasi bot |
| **Renewal** | `renewed`, `rejected`, `churn_reason` | Status renewal kontrak |
| **Cross-sell** | `cross_sell_rejected`, `cross_sell_interested`, `cross_sell_resume_date`, `days_since_cs_last_sent` | State sequence cross-sell |
| **Invoice Flags** | `pre14_sent`..`post8_sent`, `checkin_replied` | Flag pengiriman reminder |

**Indexes**: `pic_wa`, `owner_telegram_id`, `bot_active`, `segment`, `contract_end`, `renewal_date`, `blacklisted`

---

### 2. `invoices` - Data Tagihan & Pembayaran

**Representasi**: Tabel yang menyimpan seluruh invoice/tagihan yang dikeluarkan untuk setiap klien.

**Tugas**:
- Menyimpan detail invoice (nomor, tanggal, jumlah, status)
- Menyediakan tracking collection stage (pipeline dunning)
- Menyimpan flag pengiriman reminder per milestone (H-14, H-7, H-3, H+1, H+4, H+8, H+15)
- Menyimpan data pembayaran (tanggal bayar, jumlah dibayar)

**Kolom Kelompok**:

| Kelompok | Kolom | Keterangan |
|----------|-------|------------|
| **Identitas** | `invoice_id`, `company_id` (FK) | ID invoice unik dan relasi ke klien |
| **Tanggal** | `issue_date`, `due_date` | Tanggal terbit dan jatuh tempo |
| **Finansial** | `amount`, `amount_paid`, `paid_at` | Nominal dan realisasi pembayaran |
| **Status** | `payment_status`, `collection_stage` | Status bayar dan tahap collection |
| **Reminder Flags** | `pre14_sent`..`post8_sent`, `reminder_count` | Tracking milestone reminder |

**Collection Stages**: `Stage 0 — Pre-due` → `Stage 1 — Pre-due` → `Stage 2 — Overdue` → `Stage 3 — Critical`

**Indexes**: `company_id`, `payment_status`, `due_date`, `collection_stage`, `issue_date`

---

### 3. `client_flags` - Flag Otomasi Bot per Klien

**Representasi**: Tabel pendamping `clients` yang menyimpan flag boolean per klien untuk mengontrol alur otomasi bot (message sent tracking).

**Tugas**:
- Menyimpan flag pengiriman reminder renewal (H-60, H-45, H-30, H-15, H-0)
- Menyimpan flag sequence check-in (Branch A & B - form dan call)
- Menyimpan flag survey NPS (3 tahap)
- Menyimpan flag cross-sell sequence High Touch (8 tahap: H7..H90)
- Menyimpan flag cross-sell sequence Low Touch (3 tahap: LT1..LT3)
- Menyimpan flag referral, quotation acknowledgment, dan health monitoring

**Relasi**: 1:1 dengan `clients` (shared primary key `company_id`)

**Flag Categories**:

| Kategori | Kolom | Jumlah Flag |
|----------|-------|-------------|
| **Renewal** | `ren60_sent`..`ren0_sent` | 5 |
| **Check-in A** | `checkin_a1_form_sent`, `checkin_a1_call_sent`, `checkin_a2_*` | 4 |
| **Check-in B** | `checkin_b1_form_sent`, `checkin_b1_call_sent`, `checkin_b2_*` | 4 |
| **NPS** | `nps1_sent`..`nps3_sent`, `nps_replied` | 4 |
| **Cross-sell HT** | `cs_h7`..`cs_h90` | 8 |
| **Cross-sell LT** | `cs_lt1`..`cs_lt3` | 3 |
| **Lainnya** | `referral_sent_this_cycle`, `quotation_acknowledged`, `low_usage_msg_sent`, `low_nps_msg_sent` | 4 |

---

### 4. `conversation_states` - State Mesin Percakapan

**Representasi**: Tabel yang menyimpan state aktif dari percakapan bot dengan setiap klien. Ini adalah "state machine" untuk mengontrol alur percakapan 2 arah via WhatsApp.

**Tugas**:
- Menyimpan flow aktif (renewal, check-in, NPS, cross-sell, payment)
- Menyimpan stage saat ini dalam flow
- Menyimpan klasifikasi response klien (interested, rejected, objection, dll)
- Mengatur cooldown antar pesan untuk menghindari spam
- Mengontrol bot active/paused per klien
- Menyimpan next scheduled action untuk scheduling

**State Machine Flow**:

```
[Active Flow] → [Current Stage] → [Response] → [Classification] → [Next Action]
     │                                      │
     ├── renewal                            ├── INTERESTED → lanjut proses
     ├── checkin                            ├── REJECTED → eskalasi
     ├── nps                                ├── OBJECTION → handle objection
     ├── cross_sell                         ├── PAYMENT_CLAIM → verifikasi
     ├── payment                            └── NO_REPLY → retry + cooldown
     └── churn_prevention
```

**Kolom Kunci**:

| Kolom | Fungsi |
|-------|--------|
| `active_flow` | Flow percakapan aktif (renewal/checkin/nps/cross_sell/payment) |
| `current_stage` | Posisi dalam flow (misal: `awaiting_response`, `follow_up`) |
| `response_classification` | Klasifikasi jawaban klien (INTERESTED/REJECTED/OBJECTION/NEUTRAL) |
| `attempt_count` | Berapa kali bot sudah follow up |
| `cooldown_until` | Kapan bot boleh kirim pesan lagi |
| `bot_active` | Apakah bot aktif untuk klien ini |
| `next_scheduled_action` | Aksi terjadwal berikutnya |

---

### 5. `escalations` - Log Eskalasi ke Tim Internal

**Representasi**: Tabel yang menyimpan setiap kejadian eskalasi dari bot ke tim AE/BD/Management via Telegram.

**Tugas**:
- Menyimpan log setiap eskalasi yang dipicu oleh aturan (rules)
- Menyimpan prioritas eskalasi (P0 Emergency → P1 Critical → P2 High)
- Menyimpan pesan Telegram yang dikirim ke tim internal
- Tracking status eskalasi (Open → Resolved)
- Menyimpan data siapa yang resolve dan kapan

**Relasi**: Many-to-one dengan `clients`, Many-to-one dengan `escalation_rules`

**Priority Levels**:

| Level | Label | Contoh Trigger |
|-------|-------|----------------|
| P0 | Emergency | Klien marah/kecewa |
| P1 | Critical | Invoice overdue 15+, payment claim, high-value churn risk |
| P2 | High | NPS rendah, objection, zero-day no reply |

---

### 6. `escalation_rules` - Aturan Pemicu Eskalasi

**Representasi**: Tabel konfigurasi yang mendefinisikan kapan dan bagaimana eskalasi harus dipicu.

**Tugas**:
- Menyimpan rules engine untuk eskalasi otomatis
- Setiap rule memiliki ID, nama, kondisi trigger, prioritas, dan template pesan
- Rules bisa di-activate/deactivate tanpa code change
- Template pesan mendukung variable substitution

**Seed Rules**:

| esc_id | Nama | Prioritas |
|--------|------|-----------|
| ESC-001 | Invoice Overdue 15+ Days | P1 Critical |
| ESC-002 | Client Objection | P2 High |
| ESC-003 | Low NPS Score | P2 High |
| ESC-004 | Renewal Zero Day No Reply | P2 High |
| ESC-005 | High Value Churn Risk | P1 Critical |
| ESC-006 | Angry Client | P0 Emergency |
| ESC-007 | Payment Claim | P1 Critical |

---

### 7. `action_log` - Audit Trail Seluruh Aksi Bot

**Representasi**: Tabel append-only yang menyimpan log lengkap setiap aksi yang dilakukan bot (immutable audit trail).

**Tugas**:
- Mencatat setiap pesan yang dikirim bot (trigger type, template, channel)
- Mencatat setiap response yang diterima dari klien
- Mencatat klasifikasi response dan next action yang dipicu
- Mencatat interaksi manual oleh human agent
- Menyediakan data untuk analytics dan reporting

> **Keamanan**: Revoked UPDATE/DELETE untuk role `cs_agent_app` agar audit trail tidak bisa diubah.

**Kolom Kelompok**:

| Kelompok | Kolom | Keterangan |
|----------|-------|------------|
| **Trigger** | `trigger_type`, `template_id`, `triggered_at` | Apa yang memicu aksi |
| **Pengiriman** | `channel`, `sent_to_wa`, `message_id`, `message_sent` | Detail pengiriman pesan |
| **Response** | `response_received`, `response_classification`, `intent` | Response dari klien |
| **Follow-up** | `next_action_triggered`, `status` | Aksi lanjutan |
| **Human** | `by_human`, `log_notes` | Interaksi manual |

**Trigger Types**: `RENEWAL`, `PAYMENT`, `CHECKIN`, `NPS`, `CROSS_SELL`, `REFERRAL`, `HEALTH`, `ESCALATION`

---

### 8. `cron_log` - Log Eksekusi Cron Harian

**Representasi**: Tabel yang menyimpan status pemrosesan cron harian per klien.

**Tugas**:
- Memastikan setiap klien hanya diproses sekali per hari (unique constraint: `run_date + company_id`)
- Menyimpan status pemrosesan (`pending`, `success`, `failed`)
- Menyimpan error message jika gagal
- Memungkinkan retry untuk klien yang gagal diproses

**Status Flow**: `pending` → `success` | `failed`

---

### 9. `system_config` - Konfigurasi Sistem

**Representasi**: Tabel key-value yang menyimpan konfigurasi sistem secara dinamis tanpa perlu restart.

**Tugas**:
- Mengaktifkan/menonaktifkan cron job (`cron_enabled`)
- Mengatur jam eksekusi cron (`cron_hour`)
- Menyimpan endpoint API (WhatsApp, Telegram)
- Menyimpan parameter operasional (batch delay, retry, timeout)
- Menyimpan konfigurasi default segment

**Default Config Values**:

| Key | Default | Keterangan |
|-----|---------|------------|
| `cron_enabled` | `true` | Aktifkan/nonaktifkan cron harian |
| `cron_hour` | `8` | Jam eksekusi cron (08:00) |
| `batch_delay_ms` | `300` | Delay antar proses klien |
| `max_retries` | `3` | Maksimum retry |
| `wa_timeout_seconds` | `30` | Timeout API WhatsApp |
| `escalation_enabled` | `true` | Aktifkan eskalasi |
| `default_segment` | `SMB` | Segment default klien baru |

---

### 10. `templates` - Template Pesan WhatsApp

**Representasi**: Tabel yang menyimpan seluruh template pesan yang digunakan bot untuk berkomunikasi dengan klien via WhatsApp.

**Tugas**:
- Menyimpan konten pesan per kategori dan trigger ID
- Mendukung variable substitution dengan format `[Variable_Name]`
- Mendukung multi-bahasa (default: Bahasa Indonesia)
- Templates bisa di-activate/deactivate tanpa code change

**Kategori Template**:

| Kategori | Jumlah | Trigger IDs |
|----------|--------|-------------|
| **RENEWAL** | 5 | REN60, REN45, REN30, REN15, REN0 |
| **PAYMENT** | 7 | TPL-PAY-PRE14, PRE7, PRE3, POST1, POST4, POST8, POST15 |
| **CHECKIN** | 2 | TPL-CHECKIN-FORM, TPL-CHECKIN-CALL |
| **NPS** | 3 | NPS1, NPS2, NPS3 |
| **REFERRAL** | 1 | REFERRAL |
| **CROSS_SELL** | 11 | CS_H7..CS_H90, CS_LT1..CS_LT3 |
| **HEALTH** | 2 | LOW_USAGE, LOW_NPS |

**Variable Template**: `[PIC_Name]`, `[Company_Name]`, `[Company_ID]`, `[Due_Date]`, `[link_quotation]`, `[Owner_Name]`, `[Reason]`, `[Benefit_Referral]`, `[link_survey]`, `[link_checkin_form]`

---

## Entity Relationship Summary

```
                         ┌─────────────┐
                         │ system_config│  (standalone key-value)
                         └─────────────┘

                         ┌─────────────┐
                         │  templates   │  (lookup table)
                         └──────┬───────┘
                                │ template_id referenced by action_log
                                │
    ┌───────────────────────────┼────────────────────────────┐
    │                           │                            │
┌───┴───────┐         ┌────────┴────────┐         ┌────────┴────────┐
│  clients   │────1:1──▶│ client_flags    │         │ escalation_rules│
│ (master)   │         └─────────────────┘         │  (lookup)       │
└───┬───┬────┘                                      └────────┬────────┘
    │   │                                                     │
    │   │                                                     │ esc_id
    │   │                                                     │
    │   │                                             ┌───────┴────────┐
    │   ├────1:N──────────────────────────────────────▶│  escalations   │
    │   │                                               └────────────────┘
    │   │
    │   ├────1:N────▶┌──────────────┐
    │   │            │  invoices    │
    │   │            └──────────────┘
    │   │
    │   ├────1:1────▶┌──────────────────────┐
    │   │            │ conversation_states  │
    │   │            └──────────────────────┘
    │   │
    │   ├────1:N────▶┌──────────────┐
    │   │            │  action_log  │  (append-only audit trail)
    │   │            └──────────────┘
    │   │
    │   └────1:N────▶┌──────────────┐
    │                │   cron_log   │
    │                └──────────────┘
    │
    └───────────────────────────────────────
     company_id = shared foreign key
```

---

## Data Flow Overview

```
┌─────────┐     ┌─────────────┐     ┌──────────────────┐     ┌──────────────┐
│  Cron    │────▶│   clients   │────▶│ conversation_    │────▶│   action_    │
│ (daily)  │     │   (read)    │     │ states (update)  │     │ log (append) │
└─────────┘     └──────┬───────┘     └──────────────────┘     └──────────────┘
                       │
          ┌────────────┼────────────────┐
          ▼            ▼                ▼
   ┌────────────┐ ┌──────────┐  ┌──────────────┐
   │ client_    │ │ invoices │  │ templates    │
   │ flags (rw) │ │ (rw)     │  │ (read-only)  │
   └────────────┘ └────┬─────┘  └──────────────┘
                       │
                       ▼ (if escalation needed)
                ┌──────────────┐     ┌──────────────────┐
                │ escalations  │◀────│ escalation_rules │
                │ (insert)     │     │ (read-only)      │
                └──────────────┘     └──────────────────┘
```

---

## Migration History

| Version | Description |
|---------|-------------|
| 20250330000001 | Create `clients` table |
| 20250330000002 | Create `invoices` table |
| 20250330000003 | Create `client_flags` table |
| 20250330000004 | Create `escalations` table |
| 20250330000005 | Create `action_log` table |
| 20250330000006 | Create `cron_log` table |
| 20250330000007 | Create `system_config` table + seed defaults |
| 20250330000008 | Create `templates` table + seed initial templates |
| 20250330000009 | Seed full template library (all categories) |
| 20250401000001 | Create `conversation_states` table |
| 20250401000002 | Add missing columns to `clients` (invoice flags, cross-sell) |
| 20250401000003 | Create `escalation_rules` table |
| 20250401000004 | Seed escalation rules (ESC-001 through ESC-007) |
| 20250401000005 | Add columns to `action_log` (message_id, classification, etc.) |
