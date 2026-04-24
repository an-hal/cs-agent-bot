# Database Schema — Activity Log

## Catatan
Tabel `action_logs` sudah didefinisikan di `master-data/02-database-schema.md`.
Di sini kita **extend** skema tersebut dan menambah tabel baru.

---

## 1. `action_logs` — Extend (sudah ada di master-data spec)

Kolom tambahan yang perlu ditambahkan ke tabel existing:

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

Referensi skema lengkap dari master-data:
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

---

## 2. `data_mutation_logs` — Log perubahan Master Data oleh pengguna

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

---

## 3. `team_activity_logs` — Log aktivitas manajemen tim

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

---

## 4. View: `unified_activity_logs` — Gabungan untuk query halaman Activity Log

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

> **Catatan:** View ini bisa dipakai untuk query unified feed.
> Untuk performa produksi, pertimbangkan materialized view atau query langsung
> dengan UNION ALL di application layer (lebih fleksibel untuk pagination).

---

## Auto-recording Pattern

Setiap kali ada perubahan data, backend harus otomatis mencatat log:

```
PUT /master-data/clients/{id}
  → update record
  → INSERT INTO data_mutation_logs (action='edit_client', changed_fields=[...], ...)

POST /master-data/clients
  → insert record
  → INSERT INTO data_mutation_logs (action='add_client', ...)

DELETE /master-data/clients/{id}
  → delete record
  → INSERT INTO data_mutation_logs (action='delete_client', ...)

POST /team/members/invite
  → invite member
  → INSERT INTO team_activity_logs (action='invite_member', ...)

PUT /team/roles/{id}/permissions
  → update permissions
  → INSERT INTO team_activity_logs (action='update_policy', ...)
```
