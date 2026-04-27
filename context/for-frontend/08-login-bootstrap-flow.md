# Login → Workspace → Dashboard → Menu Bootstrap Flow

End-to-end FE bootstrap setelah user login: ambil workspace yang user
benar-benar punya akses, masuk dashboard dengan workspace switcher aktif,
lalu render menu kondisional berdasar permission matrix per workspace.

> Lihat juga: `01-auth-and-errors.md` untuk detail JWT/auth shape.

## Ringkasan flow (4 step)

```
┌──────────────┐   ┌────────────────────┐   ┌─────────────────────┐   ┌──────────────────────┐
│ 1. Login     │ → │ 2. List workspaces │ → │ 3. Enter dashboard  │ → │ 4. Get permission    │
│   POST       │   │    user has        │   │    + activate       │   │    matrix → render   │
│   /auth/...  │   │    access to       │   │    workspace        │   │    menu              │
│              │   │    GET /workspaces │   │    switcher         │   │    GET /team/        │
│              │   │    ⚠ saat ini      │   │    (set X-Workspace │   │    permissions/me    │
│              │   │      permissive    │   │    -ID untuk req    │   │                      │
│              │   │      — lihat gap   │   │    selanjutnya)     │   │                      │
│              │   │      di Step 2     │   │                     │   │                      │
└──────────────┘   └────────────────────┘   └─────────────────────┘   └──────────────────────┘
       │                    │                         │                          │
       ▼                    ▼                         ▼                          ▼
   simpan JWT          render switcher          set active           render menu
                       (jumlah workspace          workspace_id        kondisional dari
                       yang accessible)           ke state            permissions matrix
```

## Step 1 — Login

### Production / staging
```http
POST /auth/login
Content-Type: application/json

{ "email": "user@dealls.com", "password": "..." }
```

atau Google OAuth:

```http
POST /auth/google
Content-Type: application/json

{ "id_token": "<google-id-token>" }
```

Response berisi JWT (simpan di `localStorage` / `sessionStorage` /
secure cookie sesuai security policy).

### Local dev — JWT bypass
Kalau `JWT_DEV_BYPASS_ENABLED=true` di backend `.env`, bisa skip login. Tinggal
pakai header:

```
Authorization: Bearer DEV.<email>
```

Contoh: `Authorization: Bearer DEV.arief.faltah@dealls.com`. Backend langsung
treat sebagai user dengan email itu, tanpa hit auth-proxy.

## Step 2 — List workspace user punya akses

```http
GET /workspaces
Authorization: Bearer <jwt>
```

Response (success):
```json
{
  "status": "success",
  "data": [
    {
      "id": "75f91966-...",
      "slug": "holding",
      "name": "Sejutacita",
      "logo": "SJT",
      "color": "#0EA5E9",
      "plan": "Holding",
      "is_holding": true,
      "settings": {},
      "is_active": true,
      "created_at": "...",
      "updated_at": "..."
    }
  ]
}
```

### ⚠️ Catatan penting tentang visibility

Endpoint `/workspaces` **permissive by design**:

- Workspace di-return kalau user adalah member di `workspace_members`, **ATAU**
- Workspace ber-flag `is_holding=true` (visible ke semua user yang JWT-valid).

Artinya: user bisa **lihat** holding workspace di list walau bukan member —
ini sengaja, supaya holding workspace muncul di switcher untuk onboarding.

**Implikasi FE**: jangan asumsikan "ada di list = user punya akses penuh".
Selalu cross-check dengan permission matrix di Step 3 sebelum render menu.

### Pakai data ini untuk

- Tampilkan jumlah workspace yang user punya akses.
- Render workspace switcher (logo + name + color).
- Tentukan default workspace aktif (mis. workspace pertama atau dari user preference).

## Step 3 — Masuk dashboard + workspace switcher aktif

Setelah user pilih workspace dari list (atau auto-select workspace pertama):

- Persist `workspace_id` aktif di state global (Zustand / Redux / Context).
- Kirim sebagai header `X-Workspace-ID` di **semua request workspace-scoped**
  berikutnya.
- Render workspace switcher di topbar — saat user ganti workspace,
  re-fetch Step 4 dengan `X-Workspace-ID` baru (permission matrix
  spesifik per workspace).

## Step 4 — Get permission matrix (untuk render menu)

```http
GET /team/permissions/me
Authorization: Bearer <jwt>
X-Workspace-ID: <workspace_id-dari-step-2>
```

Response (success):
```json
{
  "status": "success",
  "data": {
    "role": {
      "id": "role-uuid",
      "name": "Admin",
      "is_system": true
    },
    "workspace_id": "75f91966-...",
    "permissions": {
      "team":          { "view_list": true,  "view_detail": true,  "create": true,  "edit": true,  "delete": true,  "export": false, "import": false },
      "master_data":   { "view_list": true,  "view_detail": true,  "create": true,  "edit": true,  "delete": false, "export": true,  "import": true },
      "invoice":       { "view_list": true,  "view_detail": true,  "create": true,  "edit": true,  "delete": false, "export": true,  "import": false },
      "workflow":      { "view_list": true,  "view_detail": true,  "create": false, "edit": false, "delete": false, "export": false, "import": false },
      "analytics":     { "view_list": true,  "view_detail": true,  "create": false, "edit": false, "delete": false, "export": true,  "import": false }
    }
  }
}
```

### Cara render menu

Mapping module → menu item:

| Permission key   | Menu item                                  |
|------------------|--------------------------------------------|
| `team`           | Team Management (members, roles)           |
| `master_data`    | Master Data (clients, custom fields)       |
| `invoice`        | Invoices                                   |
| `workflow`       | Workflow & Pipeline                        |
| `analytics`      | Analytics & Reports                        |
| `collection`     | Collections                                |
| `messaging`      | Templates (messages, emails)               |
| `automation_rule`| Automation Rules                           |

Pseudocode FE:

```ts
// Tampilkan menu kalau minimal punya view_list permission
function shouldShowMenu(perms, module) {
  return perms?.[module]?.view_list === true;
}

// Tombol "Create" hanya muncul kalau punya create permission
function canCreate(perms, module) {
  return perms?.[module]?.create === true;
}

// Untuk page detail / edit
function canEdit(perms, module) {
  return perms?.[module]?.edit === true;
}
```

Render:
```tsx
{shouldShowMenu(perms, 'team') && (
  <NavLink to="/team">Team Management</NavLink>
)}
{shouldShowMenu(perms, 'invoice') && (
  <NavLink to="/invoices">Invoices</NavLink>
)}
// ... dst
```

### ⚠️ Trap: 403 "caller is not a team member"

Kalau dapat error:
```json
{
  "status": "error",
  "message": "caller is not a team member",
  "errorCode": "FORBIDDEN"
}
```

Penyebabnya: user **belum di-insert ke tabel `team_members`**. Catat:

- Endpoint `/workspaces` (Step 2) cek tabel `workspace_members`.
- Endpoint `/team/permissions/me` cek tabel **`team_members`** (tabel berbeda).
- Keduanya **tidak otomatis sinkron** — user bisa member di salah satu tapi
  bukan di lainnya.

**Solusi (per skenario):**

| Skenario                | Cara fix                                                   |
|-------------------------|------------------------------------------------------------|
| User baru pertama kali  | Owner/admin invite via `POST /team/members/invite`         |
| Dev/QA pakai DEV bypass | Insert manual ke `team_members` + `member_workspace_assignments` |
| User gabung workspace baru | `POST /team/invitations/{token}/accept`                 |

**FE handling:**

```ts
try {
  const perms = await fetchPermissions(workspaceId);
  setMenu(buildMenuFromPerms(perms));
} catch (err) {
  if (err.errorCode === 'FORBIDDEN' && err.message.includes('team member')) {
    // Tampilkan empty state: "Anda belum diundang ke workspace ini"
    // atau redirect ke onboarding flow
    showOnboardingPrompt();
  } else {
    handleGenericError(err);
  }
}
```

## Penyimpanan state (rekomendasi)

Setelah Step 3 sukses, simpan di state global (Zustand / Redux / Context):

```ts
type SessionState = {
  jwt: string;
  user: { email: string; name: string; };
  workspaces: Workspace[];           // hasil Step 2
  activeWorkspaceID: string;
  permissions: PermissionMatrix;     // hasil Step 3
  role: Role;                        // hasil Step 3
};
```

Re-fetch Step 4 setiap kali user **switch workspace** — permission matrix
spesifik per workspace.

## Alur lengkap (TypeScript example)

```ts
async function bootstrap(jwt: string) {
  // Step 2 — list workspace user punya akses
  const workspacesRes = await fetch('/api/workspaces', {
    headers: { Authorization: `Bearer ${jwt}` },
  });
  const { data: workspaces } = await workspacesRes.json();

  if (workspaces.length === 0) {
    return { state: 'no-workspace' };
  }

  // Step 3 — masuk dashboard + tentukan workspace aktif
  const activeWs = restoreFromStorage() ?? workspaces[0];
  setActiveWorkspaceID(activeWs.id); // persist ke localStorage / state global

  // Step 4 — fetch permission matrix untuk render menu
  const permsRes = await fetch('/api/team/permissions/me', {
    headers: {
      Authorization: `Bearer ${jwt}`,
      'X-Workspace-ID': activeWs.id,
    },
  });

  if (permsRes.status === 403) {
    return { state: 'not-team-member', workspace: activeWs };
  }

  const { data: perms } = await permsRes.json();

  return {
    state: 'ready',
    workspaces,
    activeWorkspace: activeWs,
    role: perms.role,
    permissions: perms.permissions,
  };
}

// Saat user pilih workspace lain dari switcher
async function switchWorkspace(jwt: string, newWorkspaceID: string) {
  setActiveWorkspaceID(newWorkspaceID);
  const permsRes = await fetch('/api/team/permissions/me', {
    headers: {
      Authorization: `Bearer ${jwt}`,
      'X-Workspace-ID': newWorkspaceID,
    },
  });
  const { data: perms } = await permsRes.json();
  return { role: perms.role, permissions: perms.permissions };
}
```

## Endpoint tambahan terkait akses

| Endpoint                       | Kegunaan                                          |
|--------------------------------|---------------------------------------------------|
| `GET /whitelist/check?email=…` | Cek apakah email diizinkan login (no auth)        |
| `GET /workspaces/{id}/members` | List anggota workspace + role-nya                 |
| `GET /team/members`            | List team member (butuh `team:view_list`)         |
| `GET /team/roles`              | List semua role di workspace                      |
| `GET /team/roles/{id}`         | Detail role + permission per module               |
| `POST /sessions/revoke`        | Logout / revoke session                           |
| `GET /sessions/revoked`        | List session yang sudah di-revoke                 |

## Header yang wajib dikirim

| Header             | Kapan                                              |
|--------------------|----------------------------------------------------|
| `Authorization`    | Selalu, kecuali endpoint public (`/whitelist/check`, `/auth/login`, `/auth/google`) |
| `X-Workspace-ID`   | Semua endpoint workspace-scoped (kebanyakan dashboard endpoint) |
| `Content-Type`     | `application/json` untuk POST/PUT/PATCH dengan body |

## Error codes umum yang FE harus handle

| HTTP | errorCode      | Arti                                         |
|------|----------------|----------------------------------------------|
| 401  | `UNAUTHORIZED` | JWT invalid / expired → force re-login       |
| 403  | `FORBIDDEN`    | Auth valid, tapi tidak punya akses (cek pesan) |
| 400  | `BAD_REQUEST`  | Header / param salah (mis. `X-Workspace-ID` kosong) |
| 404  | `NOT_FOUND`    | Resource ID tidak ada                        |
| 500  | `INTERNAL`     | Server error → tampilkan generic + retry     |
