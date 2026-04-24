# Messaging — Backend Implementation Guide

## Context
Dashboard ini adalah multi-workspace CRM (Dealls, KantorKu, Sejutacita holding).
Setiap workspace punya template pesan sendiri untuk 3 channel: **WhatsApp**, **Email**, dan **Telegram**.
Template dipakai oleh workflow engine (cron-based automation) dan juga bisa dikirim manual oleh tim.

## Arsitektur Template System

```
                 ┌──────────────────────────┐
                 │   Template Management     │
                 │   (CRUD via Dashboard)    │
                 └────────────┬─────────────┘
                              │
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
     ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
     │ WA Templates │ │Email Templates│ │TG Templates  │
     │  (plaintext  │ │ (HTML/TipTap)│ │(plaintext +  │
     │  + emoji)    │ │              │ │ Markdown)    │
     └──────┬───────┘ └──────┬───────┘ └──────┬───────┘
            │                │                │
            └────────────────┼────────────────┘
                             ▼
                 ┌──────────────────────┐
                 │   Workflow Engine     │
                 │   (Cron Jobs)        │
                 │                      │
                 │   templateId links   │
                 │   node → template    │
                 └──────────┬───────────┘
                            │
                   RENDER with variables
                   from Master Data
                            │
                            ▼
                 ┌──────────────────────┐
                 │   Delivery Layer     │
                 │   WA: HaloAI API    │
                 │   Email: SMTP/SES   │
                 │   TG: Bot API       │
                 └──────────────────────┘
```

## Dua Jenis Template

### 1. Message Templates (WA + Telegram)
- **Channel**: `whatsapp` atau `telegram`
- **Format**: Plaintext dengan emoji, bold via `*text*` (WA) atau `**text**` (TG Markdown)
- **Dipakai oleh**: Workflow automation (AE lifecycle, SDR outreach, BD closing)
- **Role**: `sdr`, `bd`, `ae` — setiap role punya template set sendiri
- **Phases**: P0 (Onboarding) sampai P6 (Overdue), plus HANDOFF dan DORMANT

### 2. Email Templates
- **Format**: HTML (edited via TipTap rich-text editor di frontend)
- **Fields tambahan**: `subject` (email subject line), `body_html` (HTML content)
- **Status**: `active`, `draft`, `archived`
- **Dipakai oleh**: SDR cold outreach (via Apollo), BD proposals, AE lifecycle emails

## Koneksi Template ke Workflow Nodes

Setiap node di workflow (`lib/workflow-node-data.ts`) punya field `templateId` yang merujuk ke template.

```
Workflow Node (AE_NODES, SDR_NODES, BD_NODES)
  │
  ├── triggerId: 'Onboarding_Welcome'
  ├── templateId: 'TPL-OB-WELCOME'        ◄── links to message_templates.id
  ├── condition: 'days_since_activation...'
  ├── sentFlag: 'onboarding_sent'
  └── dataRead / dataWrite: fields from Master Data
```

### Template ID Conventions

| Prefix | Channel | Role | Contoh |
|--------|---------|------|--------|
| `TPL-OB-*` | WA | AE | `TPL-OB-WELCOME`, `TPL-OB-CHECKIN` |
| `TPL-NPS-*` | WA | AE | `TPL-NPS-1`, `TPL-NPS-FU` |
| `TPL-CS-*` | WA | AE | `TPL-CS-AWARENESS`, `TPL-CS-PRICING` |
| `TPL-CHECKIN-*` | WA | AE | `TPL-CHECKIN-FORM`, `TPL-CHECKIN-CALL` |
| `TPL-REN*` | WA | AE | `TPL-REN90`, `TPL-REN60`, `TPL-REN0` |
| `TPL-PAY-*` | WA | AE | `TPL-PAY-PRE14`, `TPL-PAY-POST1` |
| `TPL-REFERRAL` | WA | AE | Referral pitch |
| `TPL-SDR-WA-*` | WA | SDR | `TPL-SDR-WA-H0`, `TPL-SDR-WA-H1` |
| `TPL-SDR-EMAIL-*` | Email | SDR | `TPL-SDR-EMAIL-H0`, `TPL-SDR-EMAIL-H4` |
| `TPL-SDR-BROADCAST` | WA | SDR | Feature broadcast |
| `TPL-SDR-NURTURE-*` | WA | SDR | `TPL-SDR-NURTURE-D60`, `TPL-SDR-NURTURE-D90` |
| `TPL-SDR-SEASONAL-*` | WA | SDR | `TPL-SDR-SEASONAL-NEWYEAR` |
| `TPL-SDR-SNOOZE-*` | WA | SDR | `TPL-SDR-SNOOZE-RESUME` |
| `TPL-KK-*` | WA | AE | KantorKu-specific: `TPL-KK-OB-WELCOME` |
| `ETPL-DE-*` | Email | varies | Dealls email: `ETPL-DE-SDR-001` |
| `ETPL-KK-*` | Email | varies | KantorKu email: `ETPL-KK-SDR-001` |

### Workspace Scoping
- Template bersifat **workspace-scoped** — setiap workspace punya versi template sendiri
- Dealls templates fokus ATS/Recruitment
- KantorKu templates fokus HRIS/Payroll
- Holding view bisa lihat semua template dari semua member workspaces

## Variable System

Template menggunakan placeholder variabel dalam format `[Variable_Name]`.
Saat rendering, variabel di-replace dengan data dari Master Data atau context lain.

### Common Variables (semua channel)

| Variable | Source | Contoh |
|----------|--------|--------|
| `[Company_Name]` | master_data.company_name | PT Maju Digital |
| `[PIC_Name]` | master_data.pic_name | John Doe |
| `[PIC_Nickname]` | master_data.pic_nickname | John |
| `[contact_name_prefix_manual]` | master_data (custom) | Pak/Bu |
| `[contact_name_primary]` | master_data.pic_name | Budi |
| `[SDR_Name]` / `[SDR_Owner]` | master_data.owner_name | Rina |
| `[AM_Name]` | master_data.owner_name (AE) | Arief |
| `[HC_Size]` | master_data.custom_fields.hc_size | 150 |
| `[Industry]` | master_data.custom_fields.industry | Technology |

### Link Variables

| Variable | Source |
|----------|--------|
| `[link_wiki]` | Workspace config |
| `[link_nps_survey]` | Generated per company |
| `[link_checkin_form]` | Generated per company |
| `[link_calendar]` | AE/SDR calendar booking URL |
| `[link_invoice]` | Invoice paper_id_url |
| `[link_quotation]` | Generated per deal |
| `[link_pricing]` | Static workspace URL |
| `[link_deck]` | Static workspace URL |
| `[referral_form_link]` | Generated per company |

### Computed Variables

| Variable | Computation |
|----------|-------------|
| `[due_date]` | master_data.contract_end or invoice.due_date |
| `[amount]` | invoice.amount formatted as IDR |
| `[contract_end]` | master_data.contract_end |
| `[months_active]` | NOW() - contract_start in months |
| `[Usage_Score]` | master_data.custom_fields.usage_score |
| `[Expiry_Date]` | master_data.contract_end formatted |
| `[Invoice_ID]` | invoice.id |

## Categories

Templates dikelompokkan berdasarkan kategori:

| Category | Deskripsi | Phases |
|----------|-----------|--------|
| `onboarding` | Welcome, check-in, usage check | P0 |
| `assessment` | NPS survey, cross-sell awareness | P1 |
| `warmup` | Mid-contract check-in, social proof | P2 |
| `promo` | NPS-2, referral, pricing | P3 |
| `renewal` | Renewal negotiation sequence | P4 |
| `payment` | Payment reminders pre-due | P5 |
| `first_payment` | First payment after closing (BD) | BD pipeline |
| `overdue` | Post-due collection messages | P6 |
| `outreach` | SDR cold outreach (WA + Email) | SDR P1-P2 |
| `qualification` | SDR qualification messages | SDR P2 |
| `nurture` | Long-term nurture cycle | SDR P4 |
| `blast` | Feature broadcasts, proposals | Manual trigger |
| `escalation` | Telegram alerts to internal team | System |

## Inbound Reply Classification (HaloAI → BD Pipeline) [Gap #32]

HaloAI agent classifies every inbound WA reply for BD prospects into 11 categories before deciding whether to continue the bot sequence or escalate.

### Classification Categories

| Category | Trigger | Escalation |
|----------|---------|------------|
| `closing_signal` | Buyer says "deal", "ok", "siap teken" | ESC-BD-001 (P1) |
| `demo_request` | Asks for product demo / meeting | ESC-BD-002 (P1) |
| `demo_scheduled` | Confirmed demo slot | ESC-BD-002 (P1) |
| `pricing` | Asks pricing / quote | continue bot — TPL-BD-PRICING |
| `competitor` | Mentions competitor by name | continue bot, set `competitor_mentioned=TRUE` |
| `wants_demo_excel_manual` | Wants Excel/manual data shown | continue bot |
| `budget_objection` | Says budget too high / no budget | continue bot — TPL-BD-OBJECTION |
| `delay_snooze` | Asks to follow-up later | snooze sequence to `snooze_until` |
| `reject` | Explicit "tidak tertarik" / "no" | trigger Rejection Analysis (Gap #31) |
| `angry` | Hostile tone / unsubscribe demand | ESC-BD-006 (P0), set `Bot_Active=FALSE`, `blacklisted=TRUE` |
| `technical_wants_human` | Asks technical question bot can't answer | ESC-BD-005 (P1) |

### Escalation Rules

- `replied_count >= 3` → ESC-BD-003 (P2) regardless of category
- All escalations are written to `bd_escalations` table — see `06-workflow-engine/02-database-schema.md` (Agent F2)

### DB columns added to `clients`

```sql
ALTER TABLE clients ADD COLUMN last_reply_classification VARCHAR(40);
ALTER TABLE clients ADD COLUMN replied_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE clients ADD COLUMN competitor_mentioned BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE clients ADD COLUMN snooze_until TIMESTAMPTZ NULL;
ALTER TABLE clients ADD COLUMN bot_active BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE clients ADD COLUMN blacklisted BOOLEAN NOT NULL DEFAULT FALSE;
```

Backend reads classification from HaloAI inbound webhook payload and updates the columns atomically. See `03-api-endpoints.md` → HaloAI Inbound Webhook.

---

## Rejection Analysis Pipeline (HaloAI → Claude → DB) [Gap #31]

Triggered when the inbound classifier returns `reject` OR when conversation is `DORMANT` (no reply >14 days). Forwards full conversation history to Claude for structured rejection categorization.

### Pipeline Flow

```
HaloAI (detects reject / dormant)
   │
   ▼
POST /webhook/haloai/rejection         ◄── HMAC signature verified with HALOAI_WEBHOOK_SECRET_REJECTION
   │
   ▼
Backend forwards conversation_json → Claude (system prompt: HALOAI_REJECTION_ANALYSIS_PROMPT)
   │
   ▼
Claude returns structured JSON (10-category enum + sentiment + reengagement signal)
   │
   ▼
Persist to clients.rejection_* columns + insert rejection_analysis_log row
   │
   ▼
Decision tree per category → schedule downstream actions (winback, ESC, blacklist, etc.)
```

### 10-Category Enum

```
price_objection | feature_gap | timing_not_right | competitor_locked
| wrong_contact | no_budget | no_response | already_have_solution
| needs_internal_approval | other
```

### Decision Tree (per category)

| `rejection_category` | Recommended action | Schedule |
|----------------------|--------------------|----------|
| `price_objection` | `discount_winback_in_30d` | re-engage at D+30 with TPL-BD-WINBACK-DISCOUNT |
| `feature_gap` | `feature_request_logged` | log to product backlog, no auto re-engage |
| `timing_not_right` | `nurture_in_60d` | TPL-SDR-NURTURE-D60 |
| `competitor_locked` | `winback_in_90d` | schedule WINBACK at `contract_end - 90d` |
| `wrong_contact` | `re_route_to_dm` | trigger DM discovery, fire ESC-BD-005 |
| `no_budget` | `nurture_in_90d` | TPL-SDR-NURTURE-D90 |
| `no_response` | `dormant_archive` | set `Bot_Active=FALSE`, no follow-up |
| `already_have_solution` | `winback_in_180d` | check renewal window |
| `needs_internal_approval` | `dm_followup_in_14d` | escalate to DM 1-pager |
| `other` | `manual_review` | ESC-BD-005 to BD Lead |

### DB Schema (additions to `clients` + new log table)

```sql
ALTER TABLE clients ADD COLUMN rejection_category VARCHAR(40) NULL;
ALTER TABLE clients ADD COLUMN rejection_detail TEXT NULL;
ALTER TABLE clients ADD COLUMN rejection_confidence NUMERIC(3,2) NULL;  -- 0.00–1.00
ALTER TABLE clients ADD COLUMN prospect_sentiment VARCHAR(20) NULL;     -- positive | neutral | negative | hostile
ALTER TABLE clients ADD COLUMN reengagement_signal VARCHAR(40) NULL;
ALTER TABLE clients ADD COLUMN reengagement_timeframe VARCHAR(20) NULL; -- 30d | 60d | 90d | 180d | never
ALTER TABLE clients ADD COLUMN recommended_action VARCHAR(60) NULL;
ALTER TABLE clients ADD COLUMN rejection_analyzed_at TIMESTAMPTZ NULL;

CREATE TABLE rejection_analysis_log (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id          UUID NOT NULL REFERENCES workspaces(id),
  client_id             UUID NOT NULL REFERENCES clients(id),
  conversation_id       VARCHAR(100) NOT NULL,
  trigger_source        VARCHAR(20) NOT NULL,  -- 'reject_classified' | 'dormant_14d'
  conversation_json     JSONB NOT NULL,
  claude_request_id     VARCHAR(100),
  claude_response_json  JSONB NOT NULL,
  rejection_category    VARCHAR(40),
  rejection_confidence  NUMERIC(3,2),
  recommended_action    VARCHAR(60),
  scheduled_followup_at TIMESTAMPTZ NULL,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ral_workspace ON rejection_analysis_log(workspace_id);
CREATE INDEX idx_ral_client    ON rejection_analysis_log(client_id);
CREATE INDEX idx_ral_category  ON rejection_analysis_log(workspace_id, rejection_category);
```

Cross-ref: `03-master-data` schema for `clients.rejection_*` field definitions.

---

## D7 Telegram Alert — High BD Score [Gap #65]

Fires when D7 HIGH-INTENT WA template is scheduled AND `bd_score` is at/above the workspace threshold. Notifies the BD owner via Telegram so they can do a personal touch / DM elevation.

### Trigger Condition

```sql
-- evaluated by D7 cron (workflow engine)
SELECT * FROM clients
WHERE days_since_first_contact = 7
  AND bd_score >= (SELECT value::int FROM system_config WHERE key = 'BD_SCORE_ALERT_THRESHOLD')  -- default 4
  AND bd_d10_dm_alert_sent = FALSE
  AND bot_active = TRUE;
```

### Telegram Payload

```
"⚡ {company_name} — bd_score {N}/5, ready for personal touch / DM elevation.
Last reply: {last_reply_excerpt}"
```

Sent to `clients.bd_owner_telegram_chat_id`.

### Idempotency

```sql
ALTER TABLE clients ADD COLUMN bd_d10_dm_alert_sent BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE clients ADD COLUMN bd_d10_dm_alert_sent_at TIMESTAMPTZ NULL;
```

Backend MUST set `bd_d10_dm_alert_sent = TRUE` immediately after firing. Threshold tunable via `system_config['BD_SCORE_ALERT_THRESHOLD']`.

---

## D10 DM Escalation — Telegram Alert [Gap #63]

D10 cron evaluates whether the prospect's Decision Maker (DM) is present in BANT call notes. If yes and follow-up still needed, fire Telegram with DM 1-pager preview to the BD owner.

### Trigger Condition

```sql
SELECT * FROM clients
WHERE days_since_first_contact = 10
  AND dm_followup_needed = TRUE
  AND dm_present_in_call = TRUE
  AND dm_followup_count < 3;
```

### Action

1. Send WA template `TPL-BD-D10-DM-PRESENT-CHECK` (DM 1-pager) to prospect
2. Fire Telegram alert to BD owner with 1-pager preview
3. `UPDATE clients SET dm_followup_alert_at = NOW(), dm_followup_count = dm_followup_count + 1`
4. If `dm_followup_count = 3` after this run → fire ESC-BD-004 (stalled deal alert)

### DB columns added to `clients`

```sql
ALTER TABLE clients ADD COLUMN dm_followup_needed BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE clients ADD COLUMN dm_present_in_call BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE clients ADD COLUMN dm_followup_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE clients ADD COLUMN dm_followup_alert_at TIMESTAMPTZ NULL;
```

---

## Edit Logging

Setiap perubahan template di-log untuk audit trail:
- **Field yang berubah** (message, subject, body_html, status, dll.)
- **Old value** dan **new value**
- **Editor** (email pengguna)
- **Timestamp**

Frontend menampilkan edit history di drawer detail template.
