# Database Schema — PostgreSQL

## Tabel Utama

### 1. `message_templates` — Template WA & Telegram

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

### 2. `email_templates` — Template Email (HTML via TipTap)

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

### 3. `template_variables` — Definisi variabel yang tersedia per workspace

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

### 4. `template_edit_logs` — Audit trail perubahan template

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

## Relasi Antar Tabel

```
workspaces
  │
  ├── 1:N → message_templates (WA/Telegram per workspace)
  ├── 1:N → email_templates (Email per workspace)
  ├── 1:N → template_variables (variable definitions per workspace)
  └── 1:N → template_edit_logs (audit trail)

message_templates
  │
  └── template.id ←── workflow node.templateId (dari workflow engine config)
                  ←── action_logs.template_id (execution history)

email_templates
  │
  └── template.id ←── action_logs.template_id (execution history)
```

## Notes

### Template ID sebagai Primary Key (bukan UUID)
Template ID adalah human-readable string (e.g. `TPL-OB-WELCOME`) karena:
1. Workflow node config (`workflow-node-data.ts`) merujuk template by ID
2. Mudah di-debug di logs
3. Sudah unique by convention per workspace

### Variables sebagai TEXT Array
Variabel disimpan sebagai PostgreSQL `TEXT[]` (bukan JSONB) karena:
1. Hanya list string, tidak perlu nested structure
2. Lebih efisien untuk `@>` (contains) queries
3. Frontend kirim dan terima sebagai string array

### body_html Sanitization
Backend **harus** sanitize `body_html` saat write (prevent XSS):
- Whitelist tags: `h1-h6, p, strong, em, ul, ol, li, a, table, tr, td, th, blockquote, br, code, img`
- Strip `<script>`, `onclick`, `onerror`, dan event handlers lain
- Frontend sudah pakai `sanitizeHtml()` tapi backend harus double-check
