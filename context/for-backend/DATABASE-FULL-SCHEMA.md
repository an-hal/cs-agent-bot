# Full Database Schema — All Tables

> Auto-generated from feature specs. Run migrations in dependency order.
> NOTE: Auth uses external ms-auth-proxy — no users/sessions tables needed.
> Last regenerated: 2026-04-12

---

## Shared: 02-user-preferences

Source: `features/00-shared/02-user-preferences.md`

```sql
CREATE TABLE user_preferences (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_email    VARCHAR(255) NOT NULL,       -- from auth token (no users table)
  workspace_id  UUID NOT NULL REFERENCES workspaces(id),
  
  -- Theme
  theme_id      VARCHAR(20),                 -- amethyst, emerald, ocean, etc.
  dark_mode     BOOLEAN DEFAULT TRUE,
  
  -- UI preferences (JSONB for flexibility)
  preferences   JSONB NOT NULL DEFAULT '{}',
  -- Example contents:
  -- {
  --   "master_data_columns": ["Company_ID","Company_Name","Stage","PIC_Name","Bot_Active"],
  --   "sidebar_collapsed": false,
  --   "activity_feed_interval": 60,
  --   "notification_sound": true,
  --   "default_pipeline": "406e6b25-..."
  -- }
  
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  UNIQUE(user_email, workspace_id)
);

CREATE INDEX idx_uprefs_user ON user_preferences(user_email, workspace_id);
```

---

## Shared: 05-checker-maker

Source: `features/00-shared/05-checker-maker.md`

```sql
CREATE TYPE approval_request_type AS ENUM (
  'mark_invoice_paid',
  'create_invoice',
  'bulk_import_master_data',
  'delete_client_record',
  'change_role_permission',
  'invite_remove_member',
  'stage_transition',
  'toggle_automation_rule',
  'integration_api_key_change',
  'collection_schema_change'
);

CREATE TYPE approval_status AS ENUM (
  'pending',
  'approved',
  'rejected',
  'expired'
);
```
```sql
CREATE TABLE approval_requests (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id     UUID NOT NULL REFERENCES workspaces(id),

  -- What
  request_type     approval_request_type NOT NULL,
  payload          JSONB NOT NULL,           -- the actual data to apply on approval
  description      TEXT NOT NULL,            -- human-readable summary, e.g. "Mark INV-2026-042 as Paid"

  -- Maker (initiator)
  maker_email      VARCHAR(255) NOT NULL,
  maker_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  -- Checker (approver/rejector)
  checker_email    VARCHAR(255),
  checker_at       TIMESTAMPTZ,
  rejection_reason TEXT,

  -- Status
  status           approval_status NOT NULL DEFAULT 'pending',
  expires_at       TIMESTAMPTZ NOT NULL,     -- maker_at + 72 hours
  applied_at       TIMESTAMPTZ,              -- when the change was actually executed

  -- Meta
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Query patterns: list pending by workspace, find by id, count pending
CREATE INDEX idx_ar_workspace_status ON approval_requests(workspace_id, status);
CREATE INDEX idx_ar_workspace_created ON approval_requests(workspace_id, created_at DESC);
CREATE INDEX idx_ar_expires ON approval_requests(expires_at) WHERE status = 'pending';
CREATE INDEX idx_ar_maker ON approval_requests(maker_email, workspace_id);
CREATE INDEX idx_ar_checker ON approval_requests(checker_email, workspace_id);
```
```sql
UPDATE approval_requests
SET status = 'expired', updated_at = NOW()
WHERE status = 'pending' AND expires_at < NOW();
```

---

## Feature: 01-auth

Source: `features/01-auth/02-database-schema.md`

```sql
CREATE TABLE whitelist (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email       VARCHAR(255) NOT NULL,
  is_active   BOOLEAN NOT NULL DEFAULT TRUE,
  added_by    VARCHAR(255),              -- email of admin who added
  notes       TEXT,                       -- e.g. "SDR team", "AE manager"
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  UNIQUE(email)
);

CREATE INDEX idx_whitelist_email ON whitelist(email);
CREATE INDEX idx_whitelist_active ON whitelist(is_active);
```
```sql
INSERT INTO whitelist (email, is_active, added_by, notes) VALUES
('arief.faltah@dealls.com', true, 'system', 'Super Admin'),
('dhimas.priyadi@sejutacita.id', true, 'system', 'Super Admin'),
('budi@kantorku.id', true, 'arief.faltah@dealls.com', 'KantorKu team');
```
```sql
CREATE TABLE login_attempts (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email           VARCHAR(255) NOT NULL,
  ip_address      INET,
  user_agent      TEXT,
  success         BOOLEAN NOT NULL,
  failure_reason  VARCHAR(50),           -- 'invalid_password', 'not_whitelisted', 'account_locked'
  provider        VARCHAR(20) NOT NULL,  -- 'email', 'google'
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_la_email ON login_attempts(email);
CREATE INDEX idx_la_created ON login_attempts(created_at DESC);
```

---

## Feature: 02-workspace

Source: `features/02-workspace/02-database-schema.md`

```sql
CREATE TABLE workspaces (
  -- ══════════════════════════════════════════════════════════════
  -- IDENTITY
  -- UUID sebagai primary key, slug untuk URL-friendly identifier.
  -- URL strategy: /dashboard/{uuid}/... (slug di-redirect ke UUID)
  -- ══════════════════════════════════════════════════════════════
  
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug              VARCHAR(50) NOT NULL UNIQUE,   -- URL-safe: 'dealls', 'kantorku'
  name              VARCHAR(100) NOT NULL,          -- Display: 'Dealls', 'KantorKu'
  
  -- Branding
  logo              VARCHAR(10) NOT NULL DEFAULT '',  -- Initials fallback: 'DE', 'KK'
  color             VARCHAR(7) NOT NULL DEFAULT '#534AB7',  -- Brand accent hex
  
  -- Plan & billing
  plan              VARCHAR(50) NOT NULL DEFAULT 'Basic',
                    -- Allowed: Basic, Pro, Enterprise, Holding
  
  -- ══════════════════════════════════════════════════════════════
  -- HOLDING CONCEPT
  -- is_holding = true → workspace ini mengagregasi data dari members
  -- member_ids = UUIDs of member workspaces
  -- Holding workspace TIDAK punya data sendiri di master_data
  -- ══════════════════════════════════════════════════════════════
  is_holding        BOOLEAN NOT NULL DEFAULT FALSE,
  member_ids        UUID[] DEFAULT NULL,           -- only for holding workspaces
  
  -- Settings (flexible per workspace)
  settings          JSONB NOT NULL DEFAULT '{}',
  -- Contoh isi:
  -- {
  --   "timezone": "Asia/Jakarta",
  --   "currency": "IDR",
  --   "date_format": "DD/MM/YYYY",
  --   "working_hours": { "start": "09:00", "end": "17:00" },
  --   "features_enabled": ["pipeline", "invoices", "automation"],
  --   "default_stage": "LEAD",
  --   "custom_domain": null
  -- }
  
  -- Meta
  is_active         BOOLEAN NOT NULL DEFAULT TRUE,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ══ Indexes ══
CREATE INDEX idx_ws_slug        ON workspaces(slug);
CREATE INDEX idx_ws_holding     ON workspaces(is_holding);
CREATE INDEX idx_ws_active      ON workspaces(is_active);
CREATE INDEX idx_ws_settings    ON workspaces USING GIN(settings);

-- Auto-update updated_at
CREATE TRIGGER trg_ws_updated_at
  BEFORE UPDATE ON workspaces
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at();
```
```sql
CREATE TABLE workspace_members (
  -- ══════════════════════════════════════════════════════════════
  -- Menentukan user mana yang boleh akses workspace mana.
  -- Role menentukan permission level di dalam workspace.
  -- ══════════════════════════════════════════════════════════════
  
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id      UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  
  -- Role dalam workspace
  role              VARCHAR(20) NOT NULL DEFAULT 'member',
                    -- Allowed: owner, admin, member, viewer
  
  -- Permissions (optional granular override)
  permissions       JSONB NOT NULL DEFAULT '{}',
  -- Contoh:
  -- {
  --   "master_data": { "read": true, "write": true, "delete": false },
  --   "pipeline": { "read": true, "write": true },
  --   "settings": { "read": true, "write": false },
  --   "invoices": { "read": true, "write": false }
  -- }
  
  -- Status
  is_active         BOOLEAN NOT NULL DEFAULT TRUE,
  invited_at        TIMESTAMPTZ,
  joined_at         TIMESTAMPTZ DEFAULT NOW(),
  invited_by        UUID REFERENCES users(id),
  
  -- Meta
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  -- Constraints
  UNIQUE(workspace_id, user_id)
);

-- ══ Indexes ══
CREATE INDEX idx_wm_workspace    ON workspace_members(workspace_id);
CREATE INDEX idx_wm_user         ON workspace_members(user_id);
CREATE INDEX idx_wm_user_active  ON workspace_members(user_id, is_active);
CREATE INDEX idx_wm_role         ON workspace_members(workspace_id, role);

CREATE TRIGGER trg_wm_updated_at
  BEFORE UPDATE ON workspace_members
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at();
```
```sql
CREATE TABLE workspace_settings (
  -- ══════════════════════════════════════════════════════════════
  -- Key-value settings per workspace.
  -- Alternatif dari JSONB settings di tabel workspaces.
  -- Digunakan untuk settings yang sering di-query individual.
  -- ══════════════════════════════════════════════════════════════
  
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id      UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  
  setting_key       VARCHAR(100) NOT NULL,        -- e.g. 'theme_preset', 'timezone'
  setting_value     TEXT NOT NULL,                 -- JSON-encoded value
  
  -- Meta
  updated_by        UUID REFERENCES users(id),
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  UNIQUE(workspace_id, setting_key)
);

-- ══ Indexes ══
CREATE INDEX idx_wss_workspace    ON workspace_settings(workspace_id);
CREATE INDEX idx_wss_key          ON workspace_settings(workspace_id, setting_key);

CREATE TRIGGER trg_wss_updated_at
  BEFORE UPDATE ON workspace_settings
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at();
```
```sql
CREATE TABLE workspace_invitations (
  -- ══════════════════════════════════════════════════════════════
  -- Invitation untuk join workspace (belum accepted).
  -- Setelah accepted, record pindah ke workspace_members.
  -- ══════════════════════════════════════════════════════════════
  
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id      UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  
  email             VARCHAR(255) NOT NULL,        -- invitee email
  role              VARCHAR(20) NOT NULL DEFAULT 'member',
  
  -- Token
  invite_token      VARCHAR(100) NOT NULL UNIQUE, -- URL-safe random token
  
  -- Status
  status            VARCHAR(20) NOT NULL DEFAULT 'pending',
                    -- Allowed: pending, accepted, expired, revoked
  
  -- Audit
  invited_by        UUID NOT NULL REFERENCES users(id),
  accepted_at       TIMESTAMPTZ,
  expires_at        TIMESTAMPTZ NOT NULL,         -- default: 7 days from creation
  
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  UNIQUE(workspace_id, email, status)  -- one pending invite per email per workspace
);

-- ══ Indexes ══
CREATE INDEX idx_wi_token        ON workspace_invitations(invite_token);
CREATE INDEX idx_wi_workspace    ON workspace_invitations(workspace_id);
CREATE INDEX idx_wi_email        ON workspace_invitations(email);
CREATE INDEX idx_wi_status       ON workspace_invitations(status, expires_at);
```
```sql
-- Regular workspace: data langsung dari workspace
SELECT * FROM master_data WHERE workspace_id = $1;

-- Holding workspace: aggregated dari semua member workspaces
SELECT md.*, w.name AS workspace_name
FROM master_data md
JOIN workspaces w ON w.id = md.workspace_id
WHERE md.workspace_id = ANY(
  SELECT unnest(member_ids) FROM workspaces WHERE id = $1 AND is_holding = TRUE
)
ORDER BY md.updated_at DESC;
```
```sql
-- Default workspaces
INSERT INTO workspaces (id, slug, name, logo, color, plan, is_holding, member_ids) VALUES
  ('ws-dealls-001',   'dealls',   'Dealls',     'DE', '#534AB7', 'Enterprise', FALSE, NULL),
  ('ws-kantorku-001', 'kantorku', 'KantorKu',   'KK', '#1D9E75', 'Pro',        FALSE, NULL),
  ('ws-holding-001',  'holding',  'Sejutacita',  'SC', '#0EA5E9', 'Holding',    TRUE,  ARRAY['ws-dealls-001'::UUID, 'ws-kantorku-001'::UUID]);

-- Default workspace members (assuming admin user exists)
INSERT INTO workspace_members (workspace_id, user_id, role) VALUES
  ('ws-dealls-001',   '{admin_user_id}', 'owner'),
  ('ws-kantorku-001', '{admin_user_id}', 'owner'),
  ('ws-holding-001',  '{admin_user_id}', 'owner');
```

---

## Feature: 03-master-data

Source: `features/03-master-data/02-database-schema.md`

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
  -- FE populates these via enrichment today; backend should compute & persist.
  -- See features/03-master-data/02-database-schema.md for full notes.
  -- ══════════════════════════════════════════════════════════════
  pipeline_status   VARCHAR(30),
                    -- Allowed: active_in_sequence, bd_meeting_scheduled,
                    -- on_hold_from_client, no_show, unresponsive,
                    -- first_payment_pending, first_payment_paid,
                    -- first_payment_overdue, nurture, closed_won, closed_lost, lost
  sdr_score         SMALLINT,                   -- 1–5
  bd_score          SMALLINT,                   -- 1–5
  verdict_sdr       VARCHAR(20),                -- QUALIFIED | WARM | COLD | REJECTED | NULL
  location          VARCHAR(100),               -- city/region for Filter UI
  first_blast_date  DATE,
  bd_meeting_date   DATE,
  
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
CREATE INDEX idx_dm_owner_name       ON master_data(workspace_id, owner_name);
CREATE INDEX idx_dm_location         ON master_data(workspace_id, location);
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
```sql
CREATE TABLE custom_field_definitions (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    UUID NOT NULL REFERENCES workspaces(id),
  
  field_key       VARCHAR(50) NOT NULL,         -- snake_case, e.g. 'hc_size'
  field_label     VARCHAR(100) NOT NULL,         -- display name, e.g. 'Jumlah Karyawan'
  field_type      VARCHAR(20) NOT NULL,          -- text, number, date, boolean, select, url, email
  
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

---

## Feature: 04-team

Source: `features/04-team/02-database-schema.md`

```sql
CREATE TABLE roles (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  
  name            VARCHAR(100) NOT NULL,        -- e.g. 'Super Admin', 'AE Officer'
  description     TEXT,                         -- deskripsi role
  
  -- Display
  color           VARCHAR(7),                   -- hex color, e.g. '#534AB7'
  bg_color        VARCHAR(7),                   -- bg hex, e.g. '#EEF2FF'
  
  -- Scope
  is_system       BOOLEAN NOT NULL DEFAULT FALSE,
                  -- true = role bawaan (Super Admin, Viewer, dll)
                  -- system roles tidak bisa dihapus, hanya bisa diubah permissions-nya
  
  -- Meta
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  UNIQUE(name)
);

CREATE TRIGGER trg_roles_updated_at
  BEFORE UPDATE ON roles
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at();
```
```sql
CREATE TABLE role_permissions (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  role_id         UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  
  module_id       VARCHAR(50) NOT NULL,
                  -- 'dashboard', 'analytics', 'reports',
                  -- 'ae', 'sdr', 'bd', 'cs',
                  -- 'data_master', 'team'
  
  -- Permission flags
  view_list       VARCHAR(5) NOT NULL DEFAULT 'false',
                  -- 'false' = no access
                  -- 'true'  = access within workspace
                  -- 'all'   = access across all workspaces
  view_detail     BOOLEAN NOT NULL DEFAULT FALSE,
  can_create      BOOLEAN NOT NULL DEFAULT FALSE,
  can_edit        BOOLEAN NOT NULL DEFAULT FALSE,
  can_delete      BOOLEAN NOT NULL DEFAULT FALSE,
  can_export      BOOLEAN NOT NULL DEFAULT FALSE,
  can_import      BOOLEAN NOT NULL DEFAULT FALSE,
  
  -- Meta
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  UNIQUE(role_id, workspace_id, module_id)
);

-- ══ Indexes ══
CREATE INDEX idx_rp_role       ON role_permissions(role_id);
CREATE INDEX idx_rp_workspace  ON role_permissions(workspace_id);
CREATE INDEX idx_rp_role_ws    ON role_permissions(role_id, workspace_id);

CREATE TRIGGER trg_rp_updated_at
  BEFORE UPDATE ON role_permissions
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at();
```
```sql
CREATE TABLE team_members (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  
  -- Identity (linked to auth user)
  user_id         UUID REFERENCES users(id),    -- nullable saat pending (belum accept invite)
  name            VARCHAR(255) NOT NULL,
  email           VARCHAR(255) NOT NULL UNIQUE,
  initials        VARCHAR(5),                   -- auto-generated dari name
  
  -- Role assignment
  role_id         UUID NOT NULL REFERENCES roles(id),
  
  -- Status
  status          VARCHAR(10) NOT NULL DEFAULT 'pending',
                  -- Allowed: active, pending, inactive
  
  -- Organization
  department      VARCHAR(100),                 -- e.g. 'Account Executive', 'SDR', 'Finance'
  
  -- Display
  avatar_color    VARCHAR(7),                   -- hex color for avatar circle
  
  -- Invite
  invite_token    VARCHAR(255),                 -- token untuk accept undangan
  invite_expires  TIMESTAMPTZ,                  -- expiry undangan
  invited_by      UUID REFERENCES team_members(id),
  
  -- Tracking
  joined_at       TIMESTAMPTZ,                  -- saat status pertama kali jadi 'active'
  last_active_at  TIMESTAMPTZ,                  -- last API call / login
  
  -- Meta
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ══ Indexes ══
CREATE INDEX idx_tm_email      ON team_members(email);
CREATE INDEX idx_tm_role       ON team_members(role_id);
CREATE INDEX idx_tm_status     ON team_members(status);

CREATE TRIGGER trg_tm_updated_at
  BEFORE UPDATE ON team_members
  FOR EACH ROW
  EXECUTE FUNCTION update_updated_at();
```
```sql
CREATE TABLE member_workspace_assignments (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  member_id       UUID NOT NULL REFERENCES team_members(id) ON DELETE CASCADE,
  workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  
  assigned_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  assigned_by     UUID REFERENCES team_members(id),
  
  UNIQUE(member_id, workspace_id)
);

-- ══ Indexes ══
CREATE INDEX idx_mwa_member    ON member_workspace_assignments(member_id);
CREATE INDEX idx_mwa_workspace ON member_workspace_assignments(workspace_id);
```
```sql
CREATE TABLE role_workspace_scope (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  role_id         UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  
  UNIQUE(role_id, workspace_id)
);

CREATE INDEX idx_rws_role      ON role_workspace_scope(role_id);
CREATE INDEX idx_rws_workspace ON role_workspace_scope(workspace_id);
```
```sql
SELECT rp.view_list, rp.view_detail, rp.can_create, rp.can_edit,
       rp.can_delete, rp.can_export, rp.can_import
FROM team_members tm
JOIN role_permissions rp ON rp.role_id = tm.role_id
WHERE tm.id = $1                      -- member ID
  AND rp.workspace_id = $2            -- current workspace
  AND rp.module_id = $3               -- module being accessed
  AND tm.status = 'active';
```

---

## Feature: 05-messaging

Source: `features/05-messaging/02-database-schema.md`

```sql
CREATE TABLE message_templates (
  -- ══════════════════════════════════════════════════════════════
  -- IDENTITY
  -- ══════════════════════════════════════════════════════════════
  
  id                VARCHAR(50) PRIMARY KEY,       -- e.g. 'TPL-OB-WELCOME', 'TPL-SDR-WA-H0'
  workspace_id      UUID NOT NULL REFERENCES workspaces(id),
  trigger_id        VARCHAR(100) NOT NULL,          -- e.g. 'Onboarding_Welcome', 'WA_H0'
  
  -- ══════════════════════════════════════════════════════════════
  -- CLASSIFICATION
  -- ══════════════════════════════════════════════════════════════
  
  phase             VARCHAR(10) NOT NULL,           -- P0, P1, P2, P3, P4, P5, P6, HANDOFF, DORMANT
  phase_label       VARCHAR(50) NOT NULL,           -- 'Onboarding', 'First Assessment', etc.
  channel           VARCHAR(20) NOT NULL DEFAULT 'whatsapp',
                    -- Allowed: whatsapp, telegram
  role              VARCHAR(10) NOT NULL DEFAULT 'ae',
                    -- Allowed: sdr, bd, ae
  category          VARCHAR(30) NOT NULL,
                    -- Allowed: onboarding, assessment, warmup, promo, renewal,
                    --          payment, overdue, blast, nurture, first_payment,
                    --          outreach, qualification, escalation

  -- ══════════════════════════════════════════════════════════════
  -- CONTENT
  -- ══════════════════════════════════════════════════════════════
  
  action            VARCHAR(255) NOT NULL,          -- 'Send welcome message', 'NPS Survey baseline'
  timing            VARCHAR(100) NOT NULL,          -- 'D+0 to D+5', 'H-90 to H-85'
  condition         TEXT NOT NULL,                   -- SQL-like condition for workflow engine
  message           TEXT NOT NULL,                   -- Template message body (plaintext + emoji)
  variables         TEXT[] NOT NULL DEFAULT '{}',    -- Array of variable names: {'Company_Name', 'PIC_Name'}
  stop_if           TEXT,                            -- Condition to stop sending
  sent_flag         VARCHAR(100) NOT NULL,           -- Flag to set after sending: 'onboarding_sent'
  priority          VARCHAR(10),                     -- P0, P1, ..., P6
  
  -- ══════════════════════════════════════════════════════════════
  -- META
  -- ══════════════════════════════════════════════════════════════
  
  updated_at        TIMESTAMPTZ,
  updated_by        VARCHAR(255),                   -- email of last editor
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  UNIQUE(workspace_id, trigger_id, channel)
);

-- ══ Indexes ══
CREATE INDEX idx_mt_workspace       ON message_templates(workspace_id);
CREATE INDEX idx_mt_workspace_role  ON message_templates(workspace_id, role);
CREATE INDEX idx_mt_workspace_phase ON message_templates(workspace_id, phase);
CREATE INDEX idx_mt_trigger         ON message_templates(workspace_id, trigger_id);
CREATE INDEX idx_mt_category        ON message_templates(workspace_id, category);
CREATE INDEX idx_mt_channel         ON message_templates(workspace_id, channel);
```
```sql
CREATE TABLE email_templates (
  -- ══════════════════════════════════════════════════════════════
  -- IDENTITY
  -- ══════════════════════════════════════════════════════════════
  
  id                VARCHAR(50) PRIMARY KEY,       -- e.g. 'ETPL-DE-SDR-001', 'ETPL-KK-AE-001'
  workspace_id      UUID NOT NULL REFERENCES workspaces(id),
  
  -- ══════════════════════════════════════════════════════════════
  -- CLASSIFICATION
  -- ══════════════════════════════════════════════════════════════
  
  name              VARCHAR(255) NOT NULL,          -- Display name: 'SDR Cold Outreach — Intro Dealls'
  role              VARCHAR(10) NOT NULL DEFAULT 'ae',
                    -- Allowed: sdr, bd, ae
  category          VARCHAR(30) NOT NULL,
                    -- Allowed: outreach, onboarding, nurture, renewal, payment,
                    --          overdue, blast, qualification, escalation
  status            VARCHAR(20) NOT NULL DEFAULT 'draft',
                    -- Allowed: active, draft, archived

  -- ══════════════════════════════════════════════════════════════
  -- CONTENT
  -- ══════════════════════════════════════════════════════════════
  
  subject           VARCHAR(500) NOT NULL,          -- Email subject (with variables)
  body_html         TEXT NOT NULL,                   -- HTML content from TipTap editor
  variables         TEXT[] NOT NULL DEFAULT '{}',    -- {'PIC_Name', 'Company_Name', 'link_deck'}
  
  -- ══════════════════════════════════════════════════════════════
  -- META
  -- ══════════════════════════════════════════════════════════════
  
  updated_at        TIMESTAMPTZ,
  updated_by        VARCHAR(255),                   -- email of last editor
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  UNIQUE(workspace_id, id)
);

-- ══ Indexes ══
CREATE INDEX idx_et_workspace       ON email_templates(workspace_id);
CREATE INDEX idx_et_workspace_role  ON email_templates(workspace_id, role);
CREATE INDEX idx_et_workspace_status ON email_templates(workspace_id, status);
CREATE INDEX idx_et_category        ON email_templates(workspace_id, category);
```
```sql
CREATE TABLE template_variables (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id      UUID NOT NULL REFERENCES workspaces(id),
  
  variable_key      VARCHAR(100) NOT NULL,          -- 'Company_Name', 'PIC_Name', etc.
  display_label     VARCHAR(200) NOT NULL,          -- 'Nama Perusahaan'
  source_type       VARCHAR(30) NOT NULL,
                    -- Allowed: master_data_core, master_data_custom, invoice,
                    --          computed, workspace_config, generated
  source_field      VARCHAR(200),                   -- 'company_name', 'custom_fields.hc_size', etc.
  description       TEXT,                           -- 'Nama perusahaan dari Master Data'
  example_value     VARCHAR(500),                   -- 'PT Maju Digital'
  
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  UNIQUE(workspace_id, variable_key)
);

CREATE INDEX idx_tv_workspace ON template_variables(workspace_id);
```
```sql
CREATE TABLE template_edit_logs (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id      UUID NOT NULL REFERENCES workspaces(id),
  
  template_id       VARCHAR(50) NOT NULL,           -- FK to message_templates.id OR email_templates.id
  template_type     VARCHAR(10) NOT NULL,            -- 'message' or 'email'
  
  field             VARCHAR(50) NOT NULL,            -- 'message', 'subject', 'body_html', 'status', 'created', 'deleted'
  old_value         TEXT,                            -- Previous value (null for creation)
  new_value         TEXT,                            -- New value (null for deletion)
  
  edited_by         VARCHAR(255) NOT NULL,           -- User email
  edited_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  -- Constraint: workspace_id harus match dengan template
  CONSTRAINT fk_tel_workspace FOREIGN KEY (workspace_id) REFERENCES workspaces(id)
);

-- ══ Indexes ══
CREATE INDEX idx_tel_workspace    ON template_edit_logs(workspace_id);
CREATE INDEX idx_tel_template     ON template_edit_logs(template_id);
CREATE INDEX idx_tel_edited_at    ON template_edit_logs(workspace_id, edited_at DESC);
CREATE INDEX idx_tel_template_ts  ON template_edit_logs(template_id, edited_at DESC);
```

---

## Feature: 06-workflow-engine

Source: `features/06-workflow-engine/02-database-schema.md`

```sql
CREATE TABLE workflows (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    UUID NOT NULL REFERENCES workspaces(id),

  -- Identity
  name            VARCHAR(255) NOT NULL,      -- e.g. 'SDR Lead Outreach'
  icon            VARCHAR(10),                -- emoji, e.g. emoji phone
  slug            VARCHAR(100),               -- URL-safe identifier (auto-generated from name or manual)
  description     TEXT,

  -- State
  status          VARCHAR(20) NOT NULL DEFAULT 'active',
                  -- Allowed: active, draft, disabled

  -- Stage filter — which master_data records this workflow operates on
  stage_filter    VARCHAR(50)[] NOT NULL DEFAULT '{}',
                  -- e.g. {'LEAD','DORMANT'} for SDR, {'CLIENT'} for AE
                  -- Cron uses this to route records to the correct workflow

  -- Meta
  created_by      VARCHAR(255),
  updated_by      VARCHAR(255),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE(workspace_id, slug)
);

CREATE INDEX idx_wf_workspace ON workflows(workspace_id);
CREATE INDEX idx_wf_status    ON workflows(workspace_id, status);
```
```sql
CREATE TABLE workflow_nodes (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workflow_id     UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,

  -- React Flow identity
  node_id         VARCHAR(100) NOT NULL,       -- e.g. 'ae-p01', 'sdr-wa-h3'
  node_type       VARCHAR(20) NOT NULL,         -- 'workflow' | 'zone'
                  -- 'zone' nodes are visual groupings (non-executable)

  -- Position (React Flow coordinates)
  position_x      FLOAT NOT NULL DEFAULT 0,
  position_y      FLOAT NOT NULL DEFAULT 0,

  -- Dimensions (zone nodes only)
  width           FLOAT,
  height          FLOAT,

  -- Node data (JSONB — all node-specific fields)
  -- For workflow nodes:
  --   category: 'trigger' | 'condition' | 'action' | 'delay'
  --   label: display name
  --   icon: emoji
  --   description: node description
  --   templateId: reference to message_templates
  --   triggerId: reference to automation_rules.trigger_id
  --   timing, condition, stopIf, sentFlag: inline overrides
  -- For zone nodes:
  --   label, color, bg
  data            JSONB NOT NULL DEFAULT '{}',

  -- Display flags
  draggable       BOOLEAN DEFAULT TRUE,
  selectable      BOOLEAN DEFAULT TRUE,
  connectable     BOOLEAN DEFAULT TRUE,
  z_index         INT DEFAULT 0,

  -- Meta
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE(workflow_id, node_id)
);

CREATE INDEX idx_wn_workflow ON workflow_nodes(workflow_id);
CREATE INDEX idx_wn_data_trigger ON workflow_nodes USING GIN(data);
```
```sql
CREATE TABLE workflow_edges (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workflow_id     UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,

  -- React Flow identity
  edge_id         VARCHAR(100) NOT NULL,       -- e.g. 'sdr-e01', 'ae-e03'

  -- Connection
  source_node_id  VARCHAR(100) NOT NULL,        -- references workflow_nodes.node_id
  target_node_id  VARCHAR(100) NOT NULL,
  source_handle   VARCHAR(50),                 -- 'bottom', 'top', 'left', 'right' (null = default)
  target_handle   VARCHAR(50),

  -- Display
  label           VARCHAR(255),                -- e.g. "READ Stage='LEAD'", "No reply -> NURTURE"
  animated        BOOLEAN DEFAULT FALSE,

  -- Style (JSONB for flexibility)
  -- { "stroke": "#2563eb", "strokeWidth": 2, "strokeDasharray": "6,3" }
  style           JSONB,

  -- Meta
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE(workflow_id, edge_id)
);

CREATE INDEX idx_we_workflow ON workflow_edges(workflow_id);
CREATE INDEX idx_we_source   ON workflow_edges(workflow_id, source_node_id);
CREATE INDEX idx_we_target   ON workflow_edges(workflow_id, target_node_id);
```
```sql
CREATE TABLE workflow_steps (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workflow_id       UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,

  -- Identity
  step_key          VARCHAR(50) NOT NULL,       -- e.g. 'p0', 'blast', 'escalation'
  label             VARCHAR(255) NOT NULL,
  phase             VARCHAR(20) NOT NULL,       -- e.g. 'P0', 'SDR', 'BD', 'ESC'
  icon              VARCHAR(10),
  description       TEXT,
  sort_order        INT DEFAULT 0,

  -- Automation config (inline — may override automation_rules)
  timing            TEXT,                       -- e.g. 'D+0 to D+5', 'H-120 to H-115'
  condition         TEXT,                       -- SQL-like condition expression
  stop_if           TEXT,                       -- stop condition expression
  sent_flag         VARCHAR(100),               -- custom_field key to mark as sent
  template_id       VARCHAR(100),               -- template reference (e.g. 'TPL-OB-WELCOME')

  -- Template library references (optional — for Step Config page)
  message_template_id VARCHAR(100),             -- WA/Telegram template from library
  email_template_id   VARCHAR(100),             -- email template from library

  -- Meta
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE(workflow_id, step_key)
);

CREATE INDEX idx_ws_workflow ON workflow_steps(workflow_id);
CREATE INDEX idx_ws_phase    ON workflow_steps(workflow_id, phase);
```
```sql
CREATE TABLE automation_rules (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    UUID NOT NULL REFERENCES workspaces(id),

  -- Identity
  rule_code       VARCHAR(100) NOT NULL,        -- human-readable e.g. 'RULE-KK-AE-OB-001'
  trigger_id      VARCHAR(100) NOT NULL,        -- e.g. 'Onboarding_Welcome', 'BD_D0'
  template_id     VARCHAR(100),                 -- message template ref

  -- Classification
  role            VARCHAR(10) NOT NULL,          -- 'sdr' | 'bd' | 'ae' | 'cs'
  phase           VARCHAR(20) NOT NULL,          -- 'P0', 'P1', ..., 'P6', 'SDR', 'BD', 'BD-FP', 'BD-NUR', 'ESC', 'CS'
  phase_label     VARCHAR(100),                 -- display label e.g. 'Onboarding', 'Renewal Negotiation'
  priority        VARCHAR(20),                  -- same as phase (used for ordering in UI)

  -- Execution config
  timing          TEXT NOT NULL,                -- e.g. 'D+0 to D+5', 'H-90 to H-85', 'Immediate'
  condition       TEXT NOT NULL,                -- SQL-like condition string
  stop_if         TEXT DEFAULT '-',             -- stop condition
  sent_flag       VARCHAR(200),                 -- flag(s) to set after execution
  channel         VARCHAR(20) DEFAULT 'whatsapp',
                  -- Allowed: whatsapp, email, telegram

  -- State
  status          VARCHAR(20) NOT NULL DEFAULT 'active',
                  -- Allowed: active, paused, disabled

  -- Meta
  updated_at      TIMESTAMPTZ,
  updated_by      VARCHAR(255),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE(workspace_id, rule_code)
);

CREATE INDEX idx_ar_workspace   ON automation_rules(workspace_id);
CREATE INDEX idx_ar_status      ON automation_rules(workspace_id, status);
CREATE INDEX idx_ar_role        ON automation_rules(workspace_id, role);
CREATE INDEX idx_ar_trigger     ON automation_rules(trigger_id);
CREATE INDEX idx_ar_phase       ON automation_rules(workspace_id, phase);
```
```sql
CREATE TABLE rule_change_logs (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  rule_id         UUID NOT NULL REFERENCES automation_rules(id) ON DELETE CASCADE,
  workspace_id    UUID NOT NULL REFERENCES workspaces(id),

  field           VARCHAR(50) NOT NULL,         -- e.g. 'timing', 'condition', 'status', 'created'
  old_value       TEXT,
  new_value       TEXT,

  edited_by       VARCHAR(255) NOT NULL,
  edited_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_rcl_rule       ON rule_change_logs(rule_id);
CREATE INDEX idx_rcl_workspace  ON rule_change_logs(workspace_id);
CREATE INDEX idx_rcl_edited_at  ON rule_change_logs(workspace_id, edited_at DESC);
```
```sql
CREATE TABLE pipeline_tabs (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workflow_id     UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,

  tab_key         VARCHAR(50) NOT NULL,         -- e.g. 'semua', 'contacted', 'qualified'
  label           VARCHAR(100) NOT NULL,
  icon            VARCHAR(10),                  -- emoji
  filter          VARCHAR(100) DEFAULT 'all',
                  -- Filter DSL:
                  --   'all' — no filter
                  --   'bot_active' — Bot_Active = TRUE
                  --   'risk' — Risk_Flag IN (High, Mid) OR !Bot_Active OR Payment = Terlambat
                  --   'stage:LEAD' — Stage = LEAD
                  --   'stage:DORMANT' — Stage = DORMANT
                  --   'value_tier:High,Mid' — Value_Tier IN (High, Mid)
                  --   'sequence:active' — sequence_status = ACTIVE
                  --   'payment:Menunggu' — Payment_Status = Menunggu
                  --   'expiry:30' — Days_to_Expiry <= 30
  sort_order      INT DEFAULT 0,

  UNIQUE(workflow_id, tab_key)
);

CREATE INDEX idx_pt_workflow ON pipeline_tabs(workflow_id);
```
```sql
CREATE TABLE pipeline_stats (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workflow_id     UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,

  stat_key        VARCHAR(50) NOT NULL,         -- e.g. 'total', 'revenue', 'risk'
  label           VARCHAR(100) NOT NULL,        -- display label e.g. 'Total Client'
  metric          VARCHAR(100) NOT NULL,
                  -- Metric DSL:
                  --   'count' — total records
                  --   'count:bot_active' — count where Bot_Active = TRUE
                  --   'count:risk' — count risk conditions
                  --   'count:stage:LEAD' — count by stage
                  --   'count:value_tier:High,Mid' — count by value tier
                  --   'count:payment:Menunggu' — count by payment status
                  --   'count:expiry:30' — count contracts expiring in 30d
                  --   'sum:Final_Price' — sum of Final_Price
                  --   'avg:Days_to_Expiry' — average Days_to_Expiry
  color           VARCHAR(50),                  -- tailwind text class e.g. 'text-brand-400'
  border          VARCHAR(50),                  -- tailwind border class e.g. 'border-brand-400/20'
  sort_order      INT DEFAULT 0,

  UNIQUE(workflow_id, stat_key)
);

CREATE INDEX idx_ps_workflow ON pipeline_stats(workflow_id);
```
```sql
CREATE TABLE pipeline_columns (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workflow_id     UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,

  column_key      VARCHAR(50) NOT NULL,         -- e.g. 'Company_Name', 'Stage', 'Bot_Active'
  field           VARCHAR(100) NOT NULL,         -- master_data field name or custom_fields key
  label           VARCHAR(100) NOT NULL,         -- display header
  width           INT DEFAULT 120,              -- column width in pixels
  visible         BOOLEAN DEFAULT TRUE,
  sort_order      INT DEFAULT 0,

  UNIQUE(workflow_id, column_key)
);

CREATE INDEX idx_pc_workflow ON pipeline_columns(workflow_id);
```

---

## Feature: 07-invoices

Source: `features/07-invoices/02-database-schema.md`

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
```sql
CREATE TABLE invoice_sequences (
  workspace_id      UUID NOT NULL REFERENCES workspaces(id),
  year              INT NOT NULL,
  last_seq          INT NOT NULL DEFAULT 0,
  
  PRIMARY KEY (workspace_id, year)
);
```
```sql
-- Atomic next ID generation
INSERT INTO invoice_sequences (workspace_id, year, last_seq)
VALUES ($1, $2, 1)
ON CONFLICT (workspace_id, year)
DO UPDATE SET last_seq = invoice_sequences.last_seq + 1
RETURNING last_seq;
-- Result: 42 → format as 'INV-DE-2026-042'
```
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

---

## Feature: 08-activity-log

Source: `features/08-activity-log/02-database-schema.md`

```sql
-- Tambahan kolom untuk unified activity log view
ALTER TABLE action_logs ADD COLUMN IF NOT EXISTS
  actor_type    VARCHAR(10) NOT NULL DEFAULT 'bot';
  -- 'bot' = aksi otomatis dari workflow engine
  -- 'human' = aksi manual (misal: manual send via dashboard)

ALTER TABLE action_logs ADD COLUMN IF NOT EXISTS
  actor_name    VARCHAR(255);
  -- Nama actor (untuk human: nama user, untuk bot: 'Bot Otomasi')

ALTER TABLE action_logs ADD COLUMN IF NOT EXISTS
  actor_email   VARCHAR(255);
  -- Email actor (null untuk bot)

ALTER TABLE action_logs ADD COLUMN IF NOT EXISTS
  reply_text    TEXT;
  -- Snippet balasan dari PIC (jika replied = true)
```
```sql
-- Recap: existing action_logs schema
CREATE TABLE action_logs (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id      UUID NOT NULL REFERENCES workspaces(id),
  master_data_id    UUID NOT NULL REFERENCES master_data(id),
  
  trigger_id        VARCHAR(100) NOT NULL,     -- e.g. 'Renewal_H30', 'Overdue_H14'
  template_id       VARCHAR(100),
  
  status            VARCHAR(20) NOT NULL,       -- delivered, failed, escalated, manual
  channel           VARCHAR(20),                -- whatsapp, email, telegram
  phase             VARCHAR(10),                -- P0..P6, ESC (derived from trigger_id)
  
  fields_read       JSONB,
  fields_written    JSONB,
  
  replied           BOOLEAN DEFAULT FALSE,
  reply_text        TEXT,                        -- NEW
  conversation_id   VARCHAR(100),
  
  actor_type        VARCHAR(10) NOT NULL DEFAULT 'bot',  -- NEW
  actor_name        VARCHAR(255),                         -- NEW
  actor_email       VARCHAR(255),                         -- NEW
  
  timestamp         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes (existing + new)
CREATE INDEX idx_al_workspace    ON action_logs(workspace_id);
CREATE INDEX idx_al_master_data  ON action_logs(master_data_id);
CREATE INDEX idx_al_trigger      ON action_logs(trigger_id);
CREATE INDEX idx_al_timestamp    ON action_logs(workspace_id, timestamp DESC);
CREATE INDEX idx_al_actor_type   ON action_logs(workspace_id, actor_type);
CREATE INDEX idx_al_status       ON action_logs(workspace_id, status);
```
```sql
CREATE TABLE data_mutation_logs (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id      UUID NOT NULL REFERENCES workspaces(id),
  
  -- What happened
  action            VARCHAR(20) NOT NULL,
                    -- Allowed: add_client, edit_client, delete_client,
                    --          import_bulk, export_bulk
  
  -- Who did it
  actor_email       VARCHAR(255) NOT NULL,      -- user email
  actor_name        VARCHAR(255),               -- display name
  
  -- Which record (nullable for bulk actions)
  master_data_id    UUID REFERENCES master_data(id) ON DELETE SET NULL,
  company_id        VARCHAR(50),                -- denormalized for display
  company_name      VARCHAR(255),               -- denormalized for display
  
  -- Change tracking
  changed_fields    TEXT[],                     -- e.g. ['Payment_Status', 'Last_Payment_Date']
  previous_values   JSONB,                      -- snapshot before change
  new_values        JSONB,                      -- snapshot after change
  
  -- Bulk action metadata
  count             INT,                        -- jumlah rows di bulk import/export
  note              TEXT,                        -- deskripsi (e.g. 'Export laporan bulanan')
  
  -- Meta
  timestamp         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ══ Indexes ══
CREATE INDEX idx_dml_workspace   ON data_mutation_logs(workspace_id);
CREATE INDEX idx_dml_timestamp   ON data_mutation_logs(workspace_id, timestamp DESC);
CREATE INDEX idx_dml_action      ON data_mutation_logs(workspace_id, action);
CREATE INDEX idx_dml_actor       ON data_mutation_logs(actor_email);
CREATE INDEX idx_dml_master_data ON data_mutation_logs(master_data_id);
```
```sql
CREATE TABLE team_activity_logs (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id      UUID NOT NULL REFERENCES workspaces(id),
  
  -- What happened
  action            VARCHAR(30) NOT NULL,
                    -- Allowed: invite_member, update_role, update_policy,
                    --          activate_member, deactivate_member,
                    --          remove_member, create_role, reset_password
  
  -- Who did it
  actor_email       VARCHAR(255) NOT NULL,
  actor_name        VARCHAR(255),
  
  -- Target (affected member or role)
  target_name       VARCHAR(255) NOT NULL,      -- member name or role name
  target_email      VARCHAR(255),               -- null if target is a role
  target_id         UUID,                       -- FK to team_members or roles (soft reference)
  
  -- Extra detail
  detail            TEXT,                        -- e.g. 'Manager -> AE Officer', 'Data Master: Delete -> Ditolak'
  
  -- Meta
  timestamp         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ══ Indexes ══
CREATE INDEX idx_tal_workspace  ON team_activity_logs(workspace_id);
CREATE INDEX idx_tal_timestamp  ON team_activity_logs(workspace_id, timestamp DESC);
CREATE INDEX idx_tal_action     ON team_activity_logs(workspace_id, action);
CREATE INDEX idx_tal_actor      ON team_activity_logs(actor_email);
```
```sql
CREATE OR REPLACE VIEW unified_activity_logs AS

-- Bot action logs
SELECT
  al.id,
  al.workspace_id,
  'bot' AS category,
  al.actor_type,
  COALESCE(al.actor_name, 'Bot Otomasi') AS actor,
  al.trigger_id AS action,
  md.company_name AS target,
  CONCAT('Trigger: ', al.trigger_id, ' · ', al.status) AS detail,
  al.status,
  al.timestamp
FROM action_logs al
LEFT JOIN master_data md ON al.master_data_id = md.id

UNION ALL

-- Data mutation logs
SELECT
  dml.id,
  dml.workspace_id,
  'data' AS category,
  'human' AS actor_type,
  dml.actor_name AS actor,
  dml.action,
  COALESCE(dml.company_name, dml.count::text || ' klien') AS target,
  CASE
    WHEN dml.action = 'edit_client' THEN 'Ubah: ' || array_to_string(dml.changed_fields, ', ')
    WHEN dml.action = 'import_bulk' THEN COALESCE(dml.note, 'Bulk import')
    WHEN dml.action = 'export_bulk' THEN COALESCE(dml.note, 'Bulk export')
    WHEN dml.action = 'add_client'  THEN 'Company ID: ' || dml.company_id
    WHEN dml.action = 'delete_client' THEN 'Company ID: ' || dml.company_id || ' · dihapus'
    ELSE NULL
  END AS detail,
  NULL AS status,
  dml.timestamp
FROM data_mutation_logs dml

UNION ALL

-- Team activity logs
SELECT
  tal.id,
  tal.workspace_id,
  'team' AS category,
  'human' AS actor_type,
  tal.actor_name AS actor,
  tal.action,
  tal.target_name AS target,
  tal.detail,
  NULL AS status,
  tal.timestamp
FROM team_activity_logs tal;
```

---

## Feature: 02-workspace (notifications)

```sql
CREATE TABLE notifications (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    UUID NOT NULL REFERENCES workspaces(id),
  
  -- Who
  recipient_id    UUID REFERENCES users(id),     -- NULL = broadcast to workspace
  
  -- What
  type            VARCHAR(20) NOT NULL,           -- alert, success, workflow, info
  icon            VARCHAR(10) NOT NULL,           -- emoji: 🚨, ✅, 🤝, 💬
  message         TEXT NOT NULL,
  
  -- Where to go when clicked
  href            TEXT,                           -- deep-link path with query params
  
  -- Source
  source_feature  VARCHAR(30),                    -- workflow-engine, invoices, team, etc.
  source_id       UUID,                           -- related record ID (invoice, escalation, etc.)
  
  -- State
  read            BOOLEAN NOT NULL DEFAULT FALSE,
  read_at         TIMESTAMPTZ,
  
  -- Delivery
  telegram_sent   BOOLEAN DEFAULT FALSE,
  email_sent      BOOLEAN DEFAULT FALSE,
  
  -- Meta
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notif_recipient   ON notifications(workspace_id, recipient_id, read, created_at DESC);
CREATE INDEX idx_notif_unread      ON notifications(workspace_id, recipient_id) WHERE read = FALSE;
CREATE INDEX idx_notif_created     ON notifications(created_at DESC);
```

---

## Shared: integrations (workspace_integrations)

Source: `features/00-shared/04-integrations.md`

```sql
CREATE TABLE workspace_integrations (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    UUID NOT NULL REFERENCES workspaces(id),
  
  -- HaloAI (WhatsApp)
  haloai_api_url      TEXT,                      -- e.g. https://api.haloai.id
  haloai_api_key      TEXT,                      -- encrypted
  haloai_wa_number    VARCHAR(20),               -- e.g. 628123456789
  haloai_webhook_secret TEXT,                    -- for verifying inbound webhooks
  
  -- Telegram
  telegram_bot_token  TEXT,                      -- encrypted
  telegram_default_chat_id VARCHAR(50),          -- default group/channel for alerts
  
  -- Paper.id (Invoices)
  paperid_api_url     TEXT,                      -- e.g. https://api.paper.id
  paperid_api_key     TEXT,                      -- encrypted
  paperid_webhook_secret TEXT,                   -- for verifying payment webhooks
  
  -- Email (per-workspace From address, SMTP server is global)
  email_from_name     VARCHAR(100),              -- e.g. "Dealls Team"
  email_from_address  VARCHAR(255),              -- e.g. noreply@dealls.com
  email_reply_to      VARCHAR(255),              -- e.g. support@dealls.com
  
  -- Status
  haloai_active       BOOLEAN DEFAULT FALSE,
  telegram_active     BOOLEAN DEFAULT FALSE,
  paperid_active      BOOLEAN DEFAULT FALSE,
  email_active        BOOLEAN DEFAULT FALSE,
  
  -- Meta
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  UNIQUE(workspace_id)
);

CREATE INDEX idx_wi_workspace ON workspace_integrations(workspace_id);
```

---

## Feature: 10-collections

Source: `features/10-collections/02-database-schema.md`

User-defined ad-hoc tables. Meta (`collections`), schema (`collection_fields`), data (`collection_records`). Schema changes route through checker-maker via `collection_schema_change` approval type.

```sql
-- Meta — 1 row per user-defined table
CREATE TABLE collections (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  slug            VARCHAR(64) NOT NULL,           -- url-safe, unique per workspace; ^[a-z0-9-]{3,64}$
  name            VARCHAR(128) NOT NULL,
  description     TEXT,
  icon            VARCHAR(8),                     -- emoji
  permissions     JSONB NOT NULL DEFAULT '{}',    -- { viewer: [], editor: [], admin: [] }
  created_by      UUID NOT NULL REFERENCES users(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at      TIMESTAMPTZ,

  UNIQUE (workspace_id, slug)
);

CREATE INDEX idx_collections_workspace ON collections(workspace_id) WHERE deleted_at IS NULL;

-- Schema — 1 row per field per collection
CREATE TABLE collection_fields (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  collection_id   UUID NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
  key             VARCHAR(64) NOT NULL,           -- snake_case; ^[a-z][a-z0-9_]{0,63}$
  label           VARCHAR(128) NOT NULL,
  type            VARCHAR(32) NOT NULL,
                  -- Allowed: text|textarea|number|boolean|date|datetime|
                  --          enum|multi_enum|url|email|link_client|file
  required        BOOLEAN NOT NULL DEFAULT false,
  options         JSONB NOT NULL DEFAULT '{}',    -- { choices: [...], min, max, maxLength, accept }
  default_value   JSONB,
  "order"         INTEGER NOT NULL DEFAULT 0,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE (collection_id, key)
);

CREATE INDEX idx_fields_collection ON collection_fields(collection_id, "order");

-- Data — 1 row per record
CREATE TABLE collection_records (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  collection_id   UUID NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
  data            JSONB NOT NULL DEFAULT '{}',    -- keyed by collection_fields.key
  created_by      UUID NOT NULL REFERENCES users(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_records_collection ON collection_records(collection_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_records_data_gin   ON collection_records USING GIN (data);
```

**Write-time validation** (enforced in backend service, not DB):
- `record.data` keys must all exist in `collection_fields` for that collection
- `type` coercion per field (number → numeric, date → ISO, boolean → bool, enum → must be in `options.choices`)
- Required fields must be non-null at insert; on `PATCH`, preserve existing
- `link_client` value must be a valid `master_data.id` in the same workspace

**Schema-change approval flow** (`collection_schema_change` in checker-maker):
- Adding a field → apply as-is after approval
- Removing a field → value removed from each `collection_records.data` JSONB
- Renaming key → migrate value key in each JSONB row
- Retyping → coerce existing values; reject approval if any value fails coercion (caller must fix data first)

Cosmetic-only edits (name/icon/description) apply directly via `PATCH /collections/{id}` — no approval required.

