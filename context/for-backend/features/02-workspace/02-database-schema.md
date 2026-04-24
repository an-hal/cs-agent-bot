# Database Schema — Workspace (PostgreSQL)

## Tabel Utama

### 1. `workspaces` — Workspace / bisnis unit

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

### 2. `workspace_members` — Relasi user ↔ workspace (many-to-many)

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

### 3. `workspace_settings` — Konfigurasi workspace yang lebih structured

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

### 4. `workspace_invitations` — Pending invitations

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

## ER Diagram

```
┌─────────────────┐     ┌──────────────────────┐     ┌────────────────┐
│     users        │     │  workspace_members    │     │   workspaces    │
│                  │     │                      │     │                │
│ id (PK)    ◄────┼─────┤ user_id (FK)          │     │ id (PK)   ◄───┤
│ email           │     │ workspace_id (FK) ────┼────>│ slug (UNIQUE) │
│ name            │     │ role                  │     │ name          │
│ ...             │     │ permissions (JSONB)   │     │ logo          │
│                 │     │ is_active             │     │ color         │
└────────┬────────┘     └──────────────────────┘     │ plan          │
         │                                            │ is_holding    │
         │ 1:N                                        │ member_ids[]  │
         │                                            │ settings JSON │
         ▼                                            └───────┬───────┘
┌─────────────────┐                                           │ 1
│ workspace_       │                                           │
│   invitations    │                                    ┌──────┼──────┐
│                  │                                    │ 1:N  │      │ 1:N
│ id (PK)          │                              ┌─────▼────┐ │ ┌───▼──────────┐
│ workspace_id(FK) │                              │workspace_│ │ │ master_data   │
│ email            │                              │settings  │ │ │              │
│ invite_token     │                              │          │ │ │ workspace_id │
│ status           │                              │ key      │ │ │ (FK)         │
│ invited_by (FK)  │                              │ value    │ │ │ ...          │
│ expires_at       │                              └──────────┘ │ └──────────────┘
└──────────────────┘                                           │
                                                          ┌────▼────────────┐
                                                          │ custom_field_    │
                                                          │   definitions    │
                                                          │                 │
                                                          │ workspace_id FK │
                                                          │ field_key       │
                                                          │ field_type      │
                                                          └─────────────────┘
```

## Role Permissions Matrix

| Permission | owner | admin | member | viewer |
|---|---|---|---|---|
| View master data | Y | Y | Y | Y |
| Edit master data | Y | Y | Y | N |
| Delete master data | Y | Y | N | N |
| Import/export data | Y | Y | Y | N |
| View pipeline | Y | Y | Y | Y |
| Edit pipeline | Y | Y | Y | N |
| Manage custom fields | Y | Y | N | N |
| View settings | Y | Y | Y | Y |
| Edit settings | Y | Y | N | N |
| Invite members | Y | Y | N | N |
| Remove members | Y | N | N | N |
| Delete workspace | Y | N | N | N |
| Manage billing | Y | N | N | N |

## Holding Workspace Query Pattern

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

## Seed Data

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
