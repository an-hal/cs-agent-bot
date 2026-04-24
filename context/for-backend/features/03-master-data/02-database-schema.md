# Database Schema — PostgreSQL

> **Note on BANTS fields:** BANTS scoring columns (`bants_*`, `buying_intent`,
> `intent_*`) are specified here as dedicated columns — not inside
> `custom_fields` JSONB — because they are core to the BD flow (drive indexed
> cron queries + template branching) and universal across all workspaces, not
> workspace-custom. See `features/06-workflow-engine` `resolveTemplate()` and
> `features/06-workflow-engine/05-cron-engine.md` for the BD evaluator that
> reads these columns.

## Tabel Utama

### 1. `master_data` — Record perusahaan/client

```sql
CREATE TABLE master_data (
  -- ══════════════════════════════════════════════════════════════
  -- CORE FIELDS (fixed columns — dibutuhkan sistem)
  -- Jangan ubah nama kolom ini. Workflow logic bergantung padanya.
  -- ══════════════════════════════════════════════════════════════
  
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id      UUID NOT NULL REFERENCES workspaces(id),
  company_id        VARCHAR(50) NOT NULL,       -- unique per workspace
  company_name      VARCHAR(255) NOT NULL,
  stage             VARCHAR(20) NOT NULL DEFAULT 'LEAD',
                    -- Allowed: LEAD, PROSPECT, CLIENT, DORMANT
  
  -- Contact (PIC = Person In Charge)
  pic_name          VARCHAR(255),
  pic_nickname      VARCHAR(100),
  pic_role          VARCHAR(100),
  pic_wa            VARCHAR(20),                -- WhatsApp number
  pic_email         VARCHAR(255),
  
  -- Owner (internal team assignment)
  owner_name        VARCHAR(255),
  owner_wa          VARCHAR(20),
  owner_telegram_id VARCHAR(100),
  
  -- Automation control
  bot_active        BOOLEAN NOT NULL DEFAULT TRUE,
  blacklisted       BOOLEAN NOT NULL DEFAULT FALSE,
  sequence_status   VARCHAR(20) DEFAULT 'ACTIVE',
                    -- Allowed: ACTIVE, PAUSED, NURTURE, NURTURE_POOL, SNOOZED, DORMANT
  snooze_until      DATE,
  snooze_reason     TEXT,
  
  -- Risk
  risk_flag         VARCHAR(10) DEFAULT 'None',
                    -- Allowed: High, Mid, Low, None
  
  -- Contract & Payment
  contract_start    DATE,
  contract_end      DATE,
  contract_months   INT,
  days_to_expiry    INT,                        -- computed or updated by cron
  payment_status    VARCHAR(20) DEFAULT 'Pending',
                    -- Allowed: Paid, Pending, Overdue, Menunggu
  payment_terms     VARCHAR(50),
  final_price       BIGINT DEFAULT 0,           -- in IDR (cents not needed)
  last_payment_date DATE,
  renewed           BOOLEAN DEFAULT FALSE,
  
  -- Interaction tracking
  last_interaction_date TIMESTAMPTZ,
  
  -- Notes
  notes             TEXT,
  
  -- ══════════════════════════════════════════════════════════════
  -- BANTS SCORING (derived by Fireflies + Claude AI 3-stage pipeline)
  -- Drives BD template branching (high/low intent). See:
  -- - features/06-workflow-engine for resolveTemplate() logic
  -- - features/06-workflow-engine/05-cron-engine.md for BD evaluator
  -- ══════════════════════════════════════════════════════════════
  bants_budget         SMALLINT,                  -- 0-3
  bants_authority      SMALLINT,                  -- 0-3
  bants_need           SMALLINT,                  -- 0-3
  bants_timing         SMALLINT,                  -- 0-3
  bants_sentiment      SMALLINT,                  -- 0-3
  bants_score          DECIMAL(3,2),              -- weighted 0.00-3.00
  bants_percentage     SMALLINT,                  -- 0-100 (bants_score/3*100)
  bants_classification VARCHAR(10),               -- 'HOT' | 'WARM' | 'COLD'
  buying_intent        VARCHAR(4),                -- 'high' | 'low'
  
  -- Override audit trail (BD can override buying_intent with mandatory reason)
  intent_overridden    BOOLEAN NOT NULL DEFAULT FALSE,
  intent_original      VARCHAR(4),                -- original buying_intent before override
  intent_reason        TEXT,                      -- mandatory reason for override
  intent_by            VARCHAR(255),              -- email of overrider
  intent_at            TIMESTAMPTZ,               -- override timestamp
  
  -- ══════════════════════════════════════════════════════════════
  -- SDR/BD SCORING & PIPELINE STATUS (used by Forecast + Filter UI)
  -- FE populates these via enrichment (lib/mock-data-enrich.ts today);
  -- backend should compute & persist them per the rules below.
  -- ══════════════════════════════════════════════════════════════
  pipeline_status   VARCHAR(30),
                    -- fine-grained funnel state beyond `stage`. Allowed:
                    --   active_in_sequence, bd_meeting_scheduled, on_hold_from_client,
                    --   no_show, unresponsive, first_payment_pending, first_payment_paid,
                    --   first_payment_overdue, nurture, closed_won, closed_lost, lost
  sdr_score         SMALLINT,                   -- 1–5 engagement signal (SDR stage)
  bd_score          SMALLINT,                   -- 1–5 meeting-outcome signal (BD stage)
  verdict_sdr       VARCHAR(20),
                    -- Allowed: QUALIFIED, WARM, COLD, REJECTED, NULL
  location          VARCHAR(100),               -- city/region used by Filter UI
  first_blast_date  DATE,                       -- when SDR first reached out
  bd_meeting_date   DATE,                       -- when BD meeting was scheduled/held
  
  -- ══════════════════════════════════════════════════════════════
  -- CUSTOM FIELDS (dynamic, per workspace — stored as JSONB)
  -- Client define lewat custom_field_definitions table.
  -- 
  -- Contoh isi:
  -- {
  --   "hc_size": 150,
  --   "nps_score": 8,
  --   "plan_type": "Enterprise",
  --   "industry": "Technology",
  --   "onboarding_sent": true,
  --   "ob_checkin_sent": false,
  --   "usage_score": 72
  -- }
  -- ══════════════════════════════════════════════════════════════
  custom_fields     JSONB NOT NULL DEFAULT '{}',
  
  -- Meta
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  -- Constraints
  UNIQUE(workspace_id, company_id)
);

-- ══ Indexes ══
CREATE INDEX idx_dm_workspace        ON master_data(workspace_id);
CREATE INDEX idx_dm_workspace_stage  ON master_data(workspace_id, stage);
CREATE INDEX idx_dm_workspace_bot    ON master_data(workspace_id, bot_active);
CREATE INDEX idx_dm_company_name     ON master_data(workspace_id, company_name);
CREATE INDEX idx_dm_contract_end     ON master_data(workspace_id, contract_end);
CREATE INDEX idx_dm_payment_status   ON master_data(workspace_id, payment_status);
CREATE INDEX idx_dm_pipeline_status  ON master_data(workspace_id, pipeline_status);
CREATE INDEX idx_dm_sdr_score        ON master_data(workspace_id, sdr_score);
CREATE INDEX idx_dm_owner_name       ON master_data(workspace_id, owner_name);      -- Filter by Owner
CREATE INDEX idx_dm_location         ON master_data(workspace_id, location);        -- Filter by Location
CREATE INDEX idx_dm_buying_intent    ON master_data(workspace_id, buying_intent) WHERE buying_intent IS NOT NULL;
CREATE INDEX idx_dm_bants_score      ON master_data(workspace_id, bants_score);
CREATE INDEX idx_dm_custom_fields    ON master_data USING GIN(custom_fields);
-- GIN = Generalized Inverted Index (PostgreSQL built-in)
-- Membuat query JSONB cepat, bukan Gin Framework Go.

-- Auto-update updated_at
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_dm_updated_at
  BEFORE UPDATE ON master_data
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at();
```

### 2. `custom_field_definitions` — Definisi kolom custom per workspace

```sql
CREATE TABLE custom_field_definitions (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    UUID NOT NULL REFERENCES workspaces(id),
  
  field_key       VARCHAR(50) NOT NULL,         -- snake_case, e.g. 'hc_size'
  field_label     VARCHAR(100) NOT NULL,         -- display name, e.g. 'Jumlah Karyawan'
  field_type      VARCHAR(20) NOT NULL,          -- text, number, date, boolean, select, multiselect, url, email
                                                 -- MUST match FE FieldType in lib/custom-fields-store.ts
  
  is_required     BOOLEAN DEFAULT FALSE,
  default_value   TEXT,                          -- default saat create record baru
  placeholder     VARCHAR(200),                 -- placeholder text di form
  description     TEXT,                         -- tooltip/help text
  
  -- Untuk field_type = 'select'
  options         JSONB,                        -- ["Basic", "Mid", "Enterprise"]
  
  -- Validasi
  min_value       NUMERIC,                      -- untuk number: min
  max_value       NUMERIC,                      -- untuk number: max
  regex_pattern   VARCHAR(255),                 -- untuk text: validation regex
  
  -- Display
  sort_order      INT DEFAULT 0,                -- urutan di form/table
  visible_in_table BOOLEAN DEFAULT TRUE,        -- tampil di table list?
  column_width    INT DEFAULT 120,              -- default width di table (px)
  
  -- Meta
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  UNIQUE(workspace_id, field_key)
);

CREATE INDEX idx_cfd_workspace ON custom_field_definitions(workspace_id);
CREATE INDEX idx_cfd_sort      ON custom_field_definitions(workspace_id, sort_order);
```

### 2b. BD→AE Handoff Fields (Gap #57 — propagated at deal close)

When a `master_data` record reaches `closing_status='CLOSING'`, the BD discovery context MUST propagate to the AE-owned record so AE has full context at the start of the relationship. These fields live on `master_data` (same row) — no separate handoff table.

```sql
ALTER TABLE master_data
  -- Decision-maker contact captured by BD (also see BD field block §2c)
  ADD COLUMN IF NOT EXISTS dm_name           VARCHAR(255),
  ADD COLUMN IF NOT EXISTS dm_wa             VARCHAR(20),
  -- BD-side discovery snapshot, frozen at handoff
  ADD COLUMN IF NOT EXISTS pain_point_bd     TEXT,
  ADD COLUMN IF NOT EXISTS bd_score          INTEGER CHECK (bd_score BETWEEN 1 AND 5),
  ADD COLUMN IF NOT EXISTS bd_owner_email    VARCHAR(255),
  ADD COLUMN IF NOT EXISTS bd_assigned_at    TIMESTAMPTZ,
  -- AE assignment SLA marker
  ADD COLUMN IF NOT EXISTS ae_assigned_at    TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_dm_bd_owner   ON master_data(workspace_id, bd_owner_email);
CREATE INDEX IF NOT EXISTS idx_dm_ae_unassigned
  ON master_data(workspace_id)
  WHERE stage = 'CLIENT' AND ae_assigned_at IS NULL;  -- powers SLA cron
```

**SLA — AE assignment within 4h of `closing_status='CLOSING'`:**

```sql
-- Cron `triggerAEAssignmentSLA` runs every 15min
SELECT id, workspace_id, bd_owner_email
  FROM master_data
 WHERE workspace_id = $1
   AND closing_status = 'CLOSING'
   AND ae_assigned_at IS NULL
   AND updated_at < NOW() - INTERVAL '4 hours';
-- → emit escalation (see 01-auth/04-security.md §2d, AE row)
```

**Constraint — once promoted to CLIENT, AE must be assigned:**

```sql
ALTER TABLE master_data
  ADD CONSTRAINT chk_client_has_ae
  CHECK (stage <> 'CLIENT' OR ae_assigned_at IS NOT NULL);
-- Apply with NOT VALID first, backfill, then VALIDATE CONSTRAINT.
```

Cross-ref `06-workflow-engine` for the BD→AE handoff workflow node.

---

### 2c. BD Discovery Field Additions (subset of Gap #24)

30+ BD-stage fields populated **automatically** by the Claude extraction pipeline (`context/claude/00-shared/06-claude-extraction-pipeline.md` — Agent F1). Most fields are read by `resolveTemplate()` for BD branching and by AE during handoff (§2b). Additive only — does not replace existing BANTS columns.

```sql
ALTER TABLE master_data
  -- Pain & trigger
  ADD COLUMN IF NOT EXISTS pain_point_bd                    TEXT,
  ADD COLUMN IF NOT EXISTS pain_confirmation                BOOLEAN,
  ADD COLUMN IF NOT EXISTS bd_search_trigger                VARCHAR(100),
  ADD COLUMN IF NOT EXISTS bd_urgency_reason                TEXT,
  -- Operational profile
  ADD COLUMN IF NOT EXISTS bd_payroll_cycle                 VARCHAR(20),
  ADD COLUMN IF NOT EXISTS bd_attendance_needs              TEXT,
  ADD COLUMN IF NOT EXISTS bd_integration_needed            BOOLEAN,
  ADD COLUMN IF NOT EXISTS bd_integration_notes             TEXT,
  ADD COLUMN IF NOT EXISTS bd_approval_flow_multilevel      BOOLEAN,
  ADD COLUMN IF NOT EXISTS bd_approval_flow_notes           TEXT,
  -- Verdict & intent
  ADD COLUMN IF NOT EXISTS verdict_bd                       VARCHAR(20),
                          -- Allowed: QUALIFIED, WARM, COLD, REJECTED
  ADD COLUMN IF NOT EXISTS deal_stage                       VARCHAR(20),
  ADD COLUMN IF NOT EXISTS budget_allocated                 VARCHAR(10),
                          -- Allowed: yes, no, unknown
  ADD COLUMN IF NOT EXISTS competitor_demo_given            BOOLEAN,
  ADD COLUMN IF NOT EXISTS internal_admin_identified        BOOLEAN,
  -- Deployment preference
  ADD COLUMN IF NOT EXISTS cloud_or_onpremise               VARCHAR(15),
                          -- Allowed: cloud, on_premise, hybrid, unknown
  ADD COLUMN IF NOT EXISTS on_premise_interest              BOOLEAN,
  -- Segmentation
  ADD COLUMN IF NOT EXISTS prospect_size_tier               VARCHAR(15),
                          -- Allowed: SMB, MID, ENTERPRISE
  ADD COLUMN IF NOT EXISTS prospect_role_type               VARCHAR(15),
                          -- Allowed: HR, FINANCE, OPS, IT, CXO
  -- Blockers & timeline
  ADD COLUMN IF NOT EXISTS feature_gap_is_blocker           BOOLEAN,
  ADD COLUMN IF NOT EXISTS implementation_timeline_urgency  VARCHAR(15),
                          -- Allowed: immediate, 1_3_months, 3_6_months, 6_plus
  ADD COLUMN IF NOT EXISTS switching_urgency                VARCHAR(15),
                          -- Allowed: immediate, q1, q2, q3, q4, none
  ADD COLUMN IF NOT EXISTS competitor_type                  VARCHAR(50),
  ADD COLUMN IF NOT EXISTS expansion_plan                   TEXT,
  ADD COLUMN IF NOT EXISTS deal_complexity                  VARCHAR(10),
                          -- Allowed: low, medium, high
  -- Source / channel
  ADD COLUMN IF NOT EXISTS lead_source                      VARCHAR(20),
  ADD COLUMN IF NOT EXISTS meeting_type                     VARCHAR(20),
                          -- Allowed: discovery, demo, technical, closing
  -- Decision-maker contact (also referenced by §2b handoff)
  ADD COLUMN IF NOT EXISTS dm_name                          VARCHAR(255),
  ADD COLUMN IF NOT EXISTS dm_wa                            VARCHAR(20),
  ADD COLUMN IF NOT EXISTS dm_followup_needed               BOOLEAN,
  ADD COLUMN IF NOT EXISTS DM_present_in_call               BOOLEAN;

CREATE INDEX IF NOT EXISTS idx_dm_verdict_bd       ON master_data(workspace_id, verdict_bd);
CREATE INDEX IF NOT EXISTS idx_dm_prospect_size    ON master_data(workspace_id, prospect_size_tier);
CREATE INDEX IF NOT EXISTS idx_dm_deal_stage       ON master_data(workspace_id, deal_stage);
```

> Most fields are populated automatically by the 3-stage Fireflies+Claude pipeline; manual override allowed via PUT `/master-data/clients/{id}` with audit trail (same pattern as BANTS `intent_override*`). See `00-shared/06-claude-extraction-pipeline.md` (Agent F1).

---

### 2d. Master Data Import Dedup (Gap #52)

Backend MUST detect probable duplicates **before** inserting on `POST /master-data/import` and quarantine ambiguous rows for human review.

**Composite match key** (case-insensitive):

```sql
-- Pseudo: dedup_key(row) =
LOWER(TRIM(row.company_name))
  || '|' || COALESCE(SPLIT_PART(LOWER(row.pic_email), '@', 2), '')   -- domain only
  || '|' || LEFT(REGEXP_REPLACE(COALESCE(row.pic_wa, ''), '\D', '', 'g'), 4);
-- Match if any existing master_data row in the same workspace produces the same key.
```

Two rows are dedup matches when **all three** components equal. Example:
- `acme corp + acme.com + 6281` matches `Acme Corp + sales@acme.com + 628177712345`.

**Quarantine table:**

```sql
CREATE TABLE import_quarantine (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    UUID NOT NULL REFERENCES workspaces(id),
  batch_id        UUID NOT NULL,                          -- groups rows from one upload
  row_index       INT  NOT NULL,                          -- 1-based row in source file
  row_data        JSONB NOT NULL,                         -- full row (core + custom)
  dedup_match_id  UUID REFERENCES master_data(id),        -- existing record we matched
  reason          VARCHAR(50) NOT NULL,
                  -- Allowed: 'duplicate_match', 'ambiguous_dm', 'enum_invalid', 'pic_collision'
  status          VARCHAR(20) NOT NULL DEFAULT 'pending',
                  -- Allowed: pending, accepted, rejected, expired
  reviewed_by     VARCHAR(255),
  reviewed_at     TIMESTAMPTZ,
  reviewer_note   TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_iq_workspace_status ON import_quarantine(workspace_id, status);
CREATE INDEX idx_iq_batch            ON import_quarantine(workspace_id, batch_id);
CREATE INDEX idx_iq_pending_age      ON import_quarantine(created_at)
  WHERE status = 'pending';                               -- powers SLA query
```

**SLA:** human reviewer MUST clear pending queue within **48h**. Cron `triggerImportQuarantineSLA` runs hourly, escalates rows older than 48h to BD/AE Lead per `01-auth/04-security.md §2d` (severity = MEDIUM-HIGH).

**Pre-flight preview contract** — see `04-api-endpoints.md` for the `?preview=true` request/response shape.

---

### 2e. Churn Reactivation State Machine (Gap #54)

Churned clients (`stage='DORMANT'`) cycle through a bounded reactivation flow. State lives on `clients` (alias of `master_data` for AE-owned records).

```sql
ALTER TABLE master_data
  ADD COLUMN IF NOT EXISTS reactivation_attempts   INTEGER NOT NULL DEFAULT 0
                                                   CHECK (reactivation_attempts BETWEEN 0 AND 3),
  ADD COLUMN IF NOT EXISTS last_reactivation_at    TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS reactivation_status     VARCHAR(25),
                                                   -- Allowed: pending, recycled_to_sdr,
                                                   --          closed_permanently, escalated_to_am
  ADD COLUMN IF NOT EXISTS reactivation_outcome    TEXT,
  ADD COLUMN IF NOT EXISTS reactivation_owner      VARCHAR(255),
  ADD COLUMN IF NOT EXISTS recycle_handed_at       TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_dm_reactivation_status
  ON master_data(workspace_id, reactivation_status)
  WHERE reactivation_status IS NOT NULL;
```

**Decision logic — cron `triggerChurnReactivation` runs daily 09:00 WIB per workspace:**

```
For each master_data WHERE stage='DORMANT' AND reactivation_status IN (NULL, 'pending'):
  days_since_churn = DATE_PART('day', NOW() - last_interaction_date)

  CASE
    WHEN reactivation_attempts < 3 AND days_since_churn <= 90 THEN
      -- schedule next attempt: queue WA/email, increment attempts, set last_reactivation_at=NOW()
      reactivation_status := 'pending'

    WHEN reactivation_attempts = 3 AND final_price > 50000000 THEN
      reactivation_status := 'escalated_to_am'
      -- alert Account Manager (per 01-auth/04-security.md §2d, severity HIGH-CRITICAL)

    WHEN reactivation_attempts = 3 AND final_price <= 50000000 THEN
      reactivation_status := 'recycled_to_sdr'
      recycle_handed_at   := NOW()
      -- copy minimal record into SDR queue (new master_data row in PROSPECT stage,
      -- preserve original via foreign key `recycled_from_id`)

    WHEN days_since_churn > 90 AND reactivation_attempts = 3 THEN
      reactivation_status := 'closed_permanently'
  END
```

**Audit log entry per state transition** — every status change emits one row to `action_logs`:

```sql
INSERT INTO action_logs (workspace_id, master_data_id, trigger_id, status, fields_written, timestamp)
VALUES ($1, $2, 'CHURN_REACTIVATION', 'state_transition',
        jsonb_build_object(
          'reactivation_status_from', $3,
          'reactivation_status_to',   $4,
          'attempts',                 $5,
          'days_since_churn',         $6,
          'reason',                   $7
        ),
        NOW());
```

> The `> 50_000_000` IDR threshold for AM escalation is configurable via `system_config['CHURN_AM_THRESHOLD_IDR']`.

---

### 3. `action_logs` — Log setiap action dari workflow

```sql
CREATE TABLE action_logs (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id      UUID NOT NULL REFERENCES workspaces(id),
  master_data_id    UUID NOT NULL REFERENCES master_data(id),
  
  trigger_id        VARCHAR(100) NOT NULL,      -- e.g. 'Onboarding_Welcome', 'BD_D0'
  template_id       VARCHAR(100),               -- e.g. 'TPL-OB-WELCOME'
  
  status            VARCHAR(20) NOT NULL,        -- delivered, failed, escalated, manual
  channel           VARCHAR(20),                 -- whatsapp, email, telegram
  phase             VARCHAR(10),                 -- P0, P1, ..., P6, ESC
  
  -- What was read/written
  fields_read       JSONB,                       -- snapshot of fields read for this action
  fields_written    JSONB,                       -- what was written back to master_data
  
  -- Conversation
  replied           BOOLEAN DEFAULT FALSE,
  conversation_id   VARCHAR(100),
  
  timestamp         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_al_workspace    ON action_logs(workspace_id);
CREATE INDEX idx_al_master_data  ON action_logs(master_data_id);
CREATE INDEX idx_al_trigger      ON action_logs(trigger_id);
CREATE INDEX idx_al_timestamp    ON action_logs(workspace_id, timestamp DESC);
```
