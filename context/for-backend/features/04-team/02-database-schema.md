# Database Schema — Team Management & RBAC

## 1. `roles` — Definisi role

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

---

## 2. `role_permissions` — Permission matrix per role per workspace per module

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

### Kenapa `view_list` bertipe VARCHAR, bukan BOOLEAN?
Karena ada 3 possible values: `'false'`, `'true'`, `'all'`.
- `'all'` dipakai Super Admin dan Finance (holding scope) untuk lihat data lintas workspace.
- Di application layer, mapping: `false` → block, `true` → filter by workspace, `'all'` → no workspace filter.

---

## 3. `team_members` — Daftar anggota tim

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

---

## 4. `member_workspace_assignments` — Member <-> Workspace mapping

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

### Kenapa tabel terpisah?
- 1 member bisa di-assign ke multiple workspaces
- Role permissions sudah per-workspace (role_permissions table)
- Assignment bisa ditambah/cabut tanpa ubah role

---

## 5. `role_workspace_scope` — Role <-> Workspace scope (opsional)

Menentukan di workspace mana suatu role berlaku. Ini berbeda dari member assignment —
ini mendefinisikan "role ini tersedia di workspace mana".

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

Contoh:
- Super Admin → scope: [dealls, kantorku, holding]
- Admin Dealls → scope: [dealls]
- AE Officer → scope: [dealls, kantorku]

---

## Entity Relationship

```
roles ──┬── role_permissions (per workspace, per module)
        ├── role_workspace_scope (in which workspaces this role exists)
        └── team_members (many members per role)
                └── member_workspace_assignments (which workspaces a member is in)
```

---

## Permission Resolution Query

Untuk cek apakah user X boleh melakukan action Y di module Z di workspace W:

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

Result: satu row dengan semua permission flags. Kalau no row → no access.
