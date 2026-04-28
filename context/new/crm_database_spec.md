# CRM Database Spec — Project Bumi (Modular SaaS Architecture)

## Context
Derived from Project Bumi (kantorku.id HRIS Automation) Excel spec.
Architecture split into 3 layers: CRM Core, Bot Automation, Product Custom Fields.
Designed to be **reusable across SaaS products** (HRIS, Job Portal, Retail, etc.).

---

## Architecture Overview

```
┌─────────────────────┐     ┌──────────────────────┐     ┌──────────────────────┐
│   clients           │────▶│   bot_state          │     │   custom_fields      │
│   (CRM Core)        │     │   (Bot Automation)   │     │   (Per Product)      │
│   16 columns        │     │   18 columns         │     │   ~13 columns        │
│   Portable/Export   │     │   Internal Only      │     │   Tenant-specific    │
└─────────────────────┘     └──────────────────────┘     └──────────────────────┘
         │                           │
         ▼                           ▼
┌─────────────────────┐     ┌──────────────────────┐
│   invoices          │     │   conversation_state │
│   26 columns        │     │   15 columns         │
└─────────────────────┘     └──────────────────────┘
         │
         ▼
┌─────────────────────┐     ┌──────────────────────┐
│   action_log        │     │   escalation_rules   │
│   11 columns        │     │   10 columns         │
└─────────────────────┘     └──────────────────────┘
```

---

## Table 1: `clients` — CRM Core (Portable)
> One row per client. Primary key. All other tables reference `company_id`.
> This table is safe to import/export to any CRM.

| Column | Type | Notes |
|--------|------|-------|
| `company_id` | VARCHAR(10) PK | Auto-generated. Format: CXXXXX |
| `stage` | ENUM | `SDR / BD / AE / CHURNED / CONVERTED` |
| `company_name` | VARCHAR(255) | Full legal or common name |
| `industry` | VARCHAR(100) | e.g. Retail, F&B, Manufaktur, Jasa |
| `pic_name` | VARCHAR(100) | Primary contact at client |
| `pic_nickname` | VARCHAR(50) | For informal WA greeting |
| `pic_role` | VARCHAR(100) | e.g. HR Manager, CEO |
| `pic_wa` | VARCHAR(20) | Format: 628xxxxxxxxxx |
| `pic_email` | VARCHAR(100) | Work email, optional |
| `value_tier` | ENUM | `HIGH / MID / LOW` (ACV-based) |
| `nps_score` | TINYINT | 1–10. ≥8 = referral eligible |
| `last_interaction_date` | DATE | Auto-updated by bot on every send |
| `owner_name` | VARCHAR(100) | AE assigned to this client |
| `owner_wa` | VARCHAR(20) | AE WA for escalation |
| `notes` | TEXT | Free text, AE fills |
| `created_at` | TIMESTAMP | Auto on insert |

---

## Table 2: `invoices` — Billing & Payment
> One row per invoice. `days_overdue` drives payment flow. 
> `collection_stage` tracks automation aggressiveness.

| Column | Type | Notes |
|--------|------|-------|
| `invoice_id` | VARCHAR(20) PK | Format: INV-YYYY-XXX |
| `company_id` | VARCHAR(10) FK | → clients.company_id |
| `company_name` | VARCHAR(255) | Lookup / denormalized |
| `service` | VARCHAR(100) | Service description |
| `amount` | BIGINT | In IDR (Rupiah) |
| `link_invoice` | TEXT | URL to invoice doc |
| `status_invoice` | ENUM | `DRAFT / SENT / APPROVED` |
| `issue_date` | DATE | |
| `issue_at` | TIMESTAMP | Exact datetime issued |
| `qc_approved_by` | VARCHAR(100) | |
| `qc_approved_at` | TIMESTAMP | |
| `due_date` | DATE | |
| `payment_status` | ENUM | `PENDING / PAID / OVERDUE / PARTIAL` |
| `days_overdue` | INT | Computed: TODAY() - due_date if unpaid |
| `reminder_count` | TINYINT | Caps bot automation |
| `last_reminder_date` | DATE | |
| `collection_stage` | ENUM | `GENTLE / FIRM / ESCALATED` |
| `notes` | TEXT | |
| `payment_method` | VARCHAR(50) | Transfer, QRIS, etc. |
| `payment_term` | VARCHAR(20) | Net 7, Net 30, etc. |
| `new_subs` | BOOLEAN | TRUE if new subscription invoice |
| `paperid_invoice_id` | VARCHAR(50) | External system reference |
| `termin_breakdown` | TEXT | JSON or text for installment schedule |
| `payment_proof_status` | ENUM | `PENDING / UPLOADED / VERIFIED` |
| `payment_proof_by` | VARCHAR(100) | |
| `payment_proof_at` | TIMESTAMP | |

---

## Table 3: `bot_state` — Bot Automation (Internal Only)
> Never export this table. Bot reads/writes on every action.
> Flags reset on cycle completion via `resetCycleFlags()`.

| Column | Type | Notes |
|--------|------|-------|
| `company_id` | VARCHAR(10) PK/FK | → clients.company_id |
| `bot_active` | BOOLEAN | Master switch. FALSE = nothing fires |
| `sequence_status` | ENUM | `ACTIVE / SNOOZED / PAUSED / LONGTERM / DORMANT` |
| `sequence_cs` | ENUM | `ACTIVE / LONGTERM / SNOOZED / REJECTED / CONVERTED` |
| `snooze_until` | DATE | AE sets when client says "tanya lagi bulan X" |
| `snooze_reason` | VARCHAR(255) | |
| `blacklisted` | BOOLEAN | TRUE = permanent bot stop |
| `usage_score` | TINYINT | 0–100. Bot-computed. Risk if <40 |
| `risk_flag` | BOOLEAN | Auto: TRUE if usage_score < 40 |
| `cross_sell_rejected` | BOOLEAN | TRUE = stop all ATS cross-sell |
| `cross_sell_interested` | BOOLEAN | TRUE = AE handles manually |
| `cross_sell_resume_date` | DATE | AE sets for delayed follow-up |
| `owner_telegram_id` | VARCHAR(50) | REQUIRED for bot alerts |
| `feature_update_sent` | BOOLEAN | TRUE = skip CS this week. Resets Monday |
| `checkin_replied` | BOOLEAN | TRUE = client replied check-in. Resets per cycle |
| `nps_replied` | BOOLEAN | TRUE = client answered NPS. Resets per cycle |
| `days_since_cs_last_sent` | INT | Computed |
| `quotation_link` | TEXT | URL. Required before H-30 renewal |
| **CS Blast Flags** | | |
| `cs_h7_sent` | BOOLEAN | D+7 flag |
| `cs_h14_sent` | BOOLEAN | D+14 flag |
| `cs_h21_sent` | BOOLEAN | D+21 flag |
| `cs_h30_sent` | BOOLEAN | D+30 flag |
| `cs_h45_sent` | BOOLEAN | D+45 flag |
| `cs_h60_sent` | BOOLEAN | D+60 flag |
| `cs_h75_sent` | BOOLEAN | D+75 flag |
| `cs_h90_sent` | BOOLEAN | D+90 flag |
| `cs_lt1_sent` | BOOLEAN | Long-term cycle 1 |
| `cs_lt2_sent` | BOOLEAN | Long-term cycle 2 |
| `cs_lt3_sent` | BOOLEAN | Long-term cycle 3 (resets after) |

---

## Table 4: `conversation_state` — Anti-Spam & State Machine
> One row per client. Bot checks this BEFORE sending anything.

| Column | Type | Notes |
|--------|------|-------|
| `company_id` | VARCHAR(10) PK/FK | → clients.company_id |
| `company_name` | VARCHAR(255) | Denormalized |
| `active_flow` | VARCHAR(50) | Which flow is currently running |
| `current_stage` | VARCHAR(50) | Stage within active flow |
| `last_message_type` | VARCHAR(50) | Template ID or type sent last |
| `last_message_date` | TIMESTAMP | |
| `response_status` | ENUM | `WAITING / REPLIED / IGNORED / ESCALATED` |
| `response_classification` | VARCHAR(50) | POSITIVE / NEGATIVE / NEUTRAL / OOO |
| `attempt_count` | TINYINT | Increments per send |
| `cooldown_until` | TIMESTAMP | Bot waits until this time |
| `bot_active` | BOOLEAN | Redundant check (sync with bot_state) |
| `reason_bot_paused` | TEXT | |
| `next_scheduled_action` | VARCHAR(100) | Template or action name |
| `next_scheduled_date` | TIMESTAMP | |
| `human_owner_notified` | BOOLEAN | TRUE if AE already alerted |

---

## Table 5: `action_log` — Full Audit Trail
> Append-only. Never delete rows. Every bot action logged here.

| Column | Type | Notes |
|--------|------|-------|
| `id` | BIGINT AI PK | Auto-increment |
| `timestamp` | TIMESTAMP | |
| `company_id` | VARCHAR(10) FK | → clients.company_id |
| `company_name` | VARCHAR(255) | Denormalized for easy filtering |
| `trigger_type` | VARCHAR(100) | e.g. CS_H7, RENEWAL_H30, OVERDUE_D3 |
| `template_id` | VARCHAR(50) | Message template used |
| `channel` | ENUM | `WA / EMAIL / TELEGRAM` |
| `message_sent` | BOOLEAN | Y/N |
| `response_received` | BOOLEAN | |
| `response_classification` | VARCHAR(50) | |
| `next_action_triggered` | VARCHAR(100) | |
| `log_notes` | TEXT | |

> **Index recommendation:** Compound index on `(company_id, timestamp)` for client-level filtering.

---

## Table 6: `escalation_rules` — Reference Table (Static)
> Rarely changes. Defines when and how bot escalates to human.

| Column | Type | Notes |
|--------|------|-------|
| `escalation_id` | VARCHAR(10) PK | Format: ESC-XXX |
| `trigger_condition` | TEXT | What triggers this rule |
| `when_it_fires` | TEXT | Timing / condition description |
| `priority` | ENUM | `CRITICAL / HIGH / MEDIUM / LOW` |
| `who_gets_notified` | VARCHAR(100) | AE / Finance / Owner / etc. |
| `telegram_message` | TEXT | Template message to send |
| `bot_action` | TEXT | What bot does (e.g. set Bot_Active=FALSE) |
| `human_action_required` | TEXT | What human must do after |
| `escalation_status` | ENUM | `ACTIVE / RESOLVED / SNOOZED` |
| `notes` | TEXT | |

---

## Product Custom Fields (Swap per SaaS Type)

These ~13 fields live in a `client_product_context` table or as `custom_fields` JSON.
They replace each other depending on your SaaS model:

| Field Concept | HRIS (per headcount) | Job Portal (per job slot) | Retail (per transaction) |
|---------------|---------------------|--------------------------|--------------------------|
| **Billing unit** | `hc_size` (headcount) | `active_job_slots` | `monthly_transaction_vol` |
| **Contract type** | `contract_months` (fixed 6/12/24) | `contract_months` (optional) | N/A (pay-as-you-go) |
| **Contract end** | `contract_end` (hard date) | `contract_end` (optional) | N/A |
| **Days to expiry** | `days_to_expiry` (computed) | optional | not relevant |
| **Previous platform** | `current_system` | `current_platform` | `current_platform` |
| **Previous contract end** | `current_system_contract_end` | same | same |
| **Discount (first)** | `first_time_discount_pct` | same | same |
| **Discount (renewal)** | `next_discount_pct` | same | same |
| **Final price** | `final_price` | same | same |
| **Plan tier** | `plan_type` (Basic/Mid/Enterprise) | same | same |
| **Payment terms** | `payment_terms` (Net 7 / Net 30) | same | `billing_cycle` |
| **Quotation URL** | `quotation_link` | same | same |
| **Value alignment** | HRIS employee modules | Job listing credits | SKU/GMV volume |

### Implementation Options

**Option A — JSON column** (flexible, single table):
```sql
ALTER TABLE clients ADD COLUMN custom_fields JSON;
-- Example value for HRIS:
-- {"hc_size": "50-100", "contract_months": 12, "plan_type": "Mid"}
```

**Option B — Separate table** (queryable, normalized):
```sql
CREATE TABLE client_product_context (
  company_id VARCHAR(10) FK,
  product_type ENUM('HRIS', 'JOB_PORTAL', 'RETAIL'),
  field_key VARCHAR(100),
  field_value TEXT,
  PRIMARY KEY (company_id, field_key)
);
```

**Option C — Typed table per product** (strictest, most performant):
```sql
CREATE TABLE client_hris_context (
  company_id VARCHAR(10) PK/FK,
  hc_size VARCHAR(20),
  contract_months TINYINT,
  contract_start DATE,
  contract_end DATE,
  days_to_expiry INT, -- computed
  plan_type ENUM('BASIC','MID','ENTERPRISE'),
  first_time_discount_pct DECIMAL(5,2),
  next_discount_pct DECIMAL(5,2),
  final_price BIGINT,
  payment_terms VARCHAR(20),
  current_system VARCHAR(100),
  current_system_contract_end DATE,
  quotation_link TEXT
);
```
> Recommended: **Option C** for single-product SaaS, **Option A** if multi-product tenant.

---

## Summary: Column Count per Table

| Table | Columns | Layer | Portable? |
|-------|---------|-------|-----------|
| `clients` | 16 | CRM Core | ✅ Yes — import/export freely |
| `invoices` | 26 | Finance | ✅ Yes |
| `bot_state` | 28 | Bot Automation | ❌ No — internal only |
| `conversation_state` | 15 | Bot Automation | ❌ No — volatile |
| `action_log` | 12 | Audit | ⚠️ Read-only export for compliance |
| `escalation_rules` | 10 | Reference | ✅ Yes — config table |
| `client_product_context` | ~13 | Product-specific | ✅ Yes — per product type |
| **TOTAL** | **~120** | | |

---

## Prompt for Claude Code

Paste this into Claude Code to generate migrations:

```
I have a CRM + bot automation database spec (attached as crm_database_spec.md).

Please generate:
1. SQL migration file (MySQL/PostgreSQL — specify which) creating all 7 tables with proper types, constraints, indexes, and foreign keys
2. A seed file with 2–3 sample rows per table
3. A README.md explaining the architecture layers (CRM Core vs Bot vs Product Custom Fields)

Key rules:
- clients.company_id is the master FK referenced by all other tables
- action_log is append-only, never UPDATE or DELETE
- bot_state blast flags (cs_h7_sent etc.) must reset via a stored procedure or function resetCycleFlags(company_id)
- conversation_state.bot_active must stay in sync with bot_state.bot_active via trigger or application logic
- Use ENUM types where specified
- Add index on action_log(company_id, timestamp)
```
