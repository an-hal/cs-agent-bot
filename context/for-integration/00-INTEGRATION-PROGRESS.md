# Integration Progress — cs-agent-bot ↔ project-bumi-dashboard

Status integrasi endpoint backend cs-agent-bot ke FE `project-bumi-dashboard`.
Update tiap selesai integrasi satu endpoint atau flow.

> **Last updated:** 2026-04-26
> **Maintainer:** arief.faltah@dealls.com

## Ringkasan progres

| Metric | Count |
|--------|-------|
| **Total endpoint** | **247** |
| ✅ Done | 5 |
| 🚧 WIP | 0 |
| 🟡 Ready (backend siap, FE belum) | 242 |
| ⏳ Planned | 0 |
| ⚠️ Blocked | 0 |
| **Coverage** | **2.0%** |

## Cara baca

| Status | Arti |
|--------|------|
| ✅ done | Sudah terintegrasi end-to-end + diverifikasi via FE |
| 🚧 wip  | Backend ready, FE on-going |
| 🟡 ready | Backend ready, FE belum mulai |
| ⏳ planned | Belum ada di backend, sudah direncanakan |
| ⚠️ blocked | Ada blocker (catat di kolom "Catatan") |

## Bootstrap flow (login → dashboard)

Lihat doc lengkap: `context/for-frontend/08-login-bootstrap-flow.md`.

| # | Step | Endpoint | Status | Catatan |
|---|------|----------|--------|---------|
| 1 | Login | `POST /auth/login` / `POST /auth/google` | 🟡 ready | Local pakai `Bearer DEV.<email>` (`JWT_DEV_BYPASS_ENABLED=true`) |
| 2 | List workspace user punya akses | `GET /workspaces/mine` | ✅ done | Strict listing — cek `workspace_members` + `team_members`+assignment, no holding-bypass |
| 3 | Masuk dashboard + workspace switcher | (FE-side) | 🟡 ready | Persist `workspace_id` aktif, kirim `X-Workspace-ID` di tiap request |
| 4 | Permission matrix → render menu | `GET /team/permissions/me` | ✅ done | Re-fetch tiap switch workspace |

## Endpoint terintegrasi

### ✅ `GET /workspaces/mine`

**Tujuan:** Step 2 bootstrap — list workspace yang user benar-benar punya
akses (bukan permissive list).

**Request:**
```bash
curl --location 'http://localhost:8081/api/workspaces/mine' \
  --header 'Authorization: Bearer DEV.arief.faltah@dealls.com'
```

**Response (success, HTTP 200):**
```json
{
  "status": "success",
  "message": "Workspaces",
  "data": [
    {
      "id": "75f91966-1a19-4ef4-bfa2-9d553091c92f",
      "slug": "holding",
      "name": "Sejutacita",
      "logo": "SJT",
      "color": "#0EA5E9",
      "plan": "Holding",
      "is_holding": true,
      "settings": {},
      "is_active": true,
      "created_at": "2026-04-07T01:14:38.031137Z",
      "updated_at": "2026-04-14T09:29:18.824793Z"
    }
  ]
}
```

**Semantik:**
- Hanya return workspace yang user punya membership-nya, melalui salah satu dari:
  1. `workspace_members.user_email = caller AND is_active=true`, atau
  2. `team_members.email = caller AND status='active'` + ada `member_workspace_assignments` ke workspace itu.
- **Tidak ada holding-bypass** (beda dengan `GET /workspaces` legacy).
- Response sudah TIDAK include `member_ids` (sengaja disembunyikan dari API).

**Auth:** JWT only (tidak butuh `X-Workspace-ID`).

**FE integration:**
- Pakai response untuk render workspace switcher di topbar.
- Kalau `data.length === 0` → tampilkan empty state "Belum punya akses workspace, hubungi admin".
- Set cookie `allowed_workspaces` dengan list `id` yang dipisahkan koma — middleware FE (`proxy.ts`) pakai cookie ini untuk gate dashboard route.

---

### ✅ `GET /team/permissions/me`

**Tujuan:** Step 4 bootstrap — full permission matrix untuk user di workspace
aktif. FE pakai untuk render menu kondisional.

**Request:**
```bash
curl --location 'http://localhost:8081/api/team/permissions/me' \
  --header 'Authorization: Bearer DEV.arief.faltah@dealls.com' \
  --header 'X-Workspace-ID: 75f91966-1a19-4ef4-bfa2-9d553091c92f'
```

**Response (success, HTTP 200):**
```json
{
  "status": "success",
  "message": "Permissions",
  "data": {
    "role": {
      "id": "ee9cf0ac-af73-470c-bbfa-fdccc2b0da0c",
      "name": "Super Admin",
      "description": "Akses penuh ke semua modul dan semua workspace.",
      "color": "#EF4444",
      "bg_color": "#FEF2F2",
      "is_system": true,
      "created_at": "2026-04-14T09:29:27.418105Z",
      "updated_at": "2026-04-14T09:29:27.418105Z"
    },
    "workspace_id": "75f91966-1a19-4ef4-bfa2-9d553091c92f",
    "permissions": {
      "ae":          { "view_list": "all", "view_detail": true, "create": true, "edit": true, "delete": true, "export": true, "import": true },
      "analytics":   { "view_list": "all", "view_detail": true, "create": true, "edit": true, "delete": true, "export": true, "import": true },
      "bd":          { "view_list": "all", "view_detail": true, "create": true, "edit": true, "delete": true, "export": true, "import": true },
      "cs":          { "view_list": "all", "view_detail": true, "create": true, "edit": true, "delete": true, "export": true, "import": true },
      "dashboard":   { "view_list": "all", "view_detail": true, "create": true, "edit": true, "delete": true, "export": true, "import": true },
      "data_master": { "view_list": "all", "view_detail": true, "create": true, "edit": true, "delete": true, "export": true, "import": true },
      "reports":     { "view_list": "all", "view_detail": true, "create": true, "edit": true, "delete": true, "export": true, "import": true },
      "sdr":         { "view_list": "all", "view_detail": true, "create": true, "edit": true, "delete": true, "export": true, "import": true },
      "team":        { "view_list": "all", "view_detail": true, "create": true, "edit": true, "delete": true, "export": true, "import": true }
    }
  }
}
```

**Permission flag shape per module:**

| Field | Tipe | Arti |
|-------|------|------|
| `view_list` | `string` (`"all"` / `"own"` / `"none"`) atau `bool` | Scope view list — `"all"` = semua data, `"own"` = milik sendiri saja |
| `view_detail` | `bool` | Akses ke detail page |
| `create` | `bool` | Tombol create / form input |
| `edit` | `bool` | Edit mode + tombol save |
| `delete` | `bool` | Tombol delete + confirm dialog |
| `export` | `bool` | Tombol export CSV/Excel |
| `import` | `bool` | Tombol import file |

**Auth:** JWT + `X-Workspace-ID` (wajib).

**Module yang di-return:**
`ae`, `analytics`, `bd`, `cs`, `dashboard`, `data_master`, `reports`, `sdr`, `team`

**FE integration:**
- Pakai untuk render menu kondisional: `permissions[module].view_list` truthy → tampilkan menu.
- Pakai untuk action button: `permissions[module].create` → tampilkan tombol "Add new".
- Re-fetch tiap kali user switch workspace.
- Handle error 403 `caller is not a team member` → user belum di-seed ke `team_members` (lihat `scripts/seed_team_member.go`).

---

## Endpoint pendukung (related)

| Endpoint | Status | Catatan |
|----------|--------|---------|
| `POST /auth/login` | 🟡 ready | Standard email+password login |
| `POST /auth/google` | 🟡 ready | Google ID token verify |
| `POST /auth/logout` | 🟡 ready | Invalidate session |
| `GET /whitelist/check` | 🟡 ready | Cek email allowed (no auth) |
| `GET /workspaces` | ✅ done | Permissive list (legacy) — masih ada untuk backward compat |
| `GET /workspaces/{id}` | 🟡 ready | Detail workspace + members |
| `POST /workspaces/{id}/switch` | 🟡 ready | Switch active workspace endpoint |
| `GET /team/members` | 🟡 ready | List team member di workspace aktif |
| `GET /team/roles` | 🟡 ready | List role definitions |
| `GET /sessions/revoked` | 🟡 ready | List session yang sudah di-revoke |

## Catatan teknis untuk FE

### Header yang wajib dikirim per request

| Header | Format | Wajib di mana |
|--------|--------|---------------|
| `Authorization` | `Bearer <jwt>` atau `Bearer DEV.<email>` (local) | Semua endpoint kecuali public (`/auth/login`, `/auth/google`, `/whitelist/check`) |
| `X-Workspace-ID` | UUID workspace aktif | Semua endpoint workspace-scoped (kebanyakan dashboard/team endpoint) |
| `Content-Type` | `application/json` | POST/PUT/PATCH dengan body |

### Error handling

| HTTP | errorCode | Trigger | FE handle |
|------|-----------|---------|-----------|
| 401 | `UNAUTHORIZED` | JWT invalid/expired | Force re-login |
| 403 | `FORBIDDEN` | Tidak punya permission | Cek pesan; bisa "not team member" → empty state, atau "permission denied" → hide UI element |
| 400 | `BAD_REQUEST` | Header/body salah (mis. `X-Workspace-ID` kosong) | Tampilkan validation error |
| 404 | `NOT_FOUND` | Resource ID tidak ada | "Data tidak ditemukan" |
| 500 | `INTERNAL` | Server error | Generic "Terjadi kesalahan" + retry button |

### Local dev — JWT bypass

Backend `.env`:
```
JWT_DEV_BYPASS_ENABLED=true
```

FE pakai header:
```
Authorization: Bearer DEV.<email>
```

Backend langsung treat sebagai user dengan email itu (tanpa hit auth-proxy).
Jangan dipakai di staging/prod.

### Seeding user untuk akses `/team/permissions/me`

Kalau dapat 403 `caller is not a team member`, run:
```bash
go run scripts/seed_team_member.go \
  --email=<email> \
  --role="Super Admin" \
  --workspace=<workspace_uuid> \
  --name="Display Name"
```

Role yang tersedia: `Super Admin`, `Admin`, `Manager`, `AE Officer`,
`SDR Officer`, `CS Officer`, `Finance`, `Viewer`.

## Roadmap berikutnya

| Prioritas | Endpoint/Feature | Deskripsi |
|-----------|------------------|-----------|
| P0 | `POST /auth/login` integrasi penuh | Currently FE hardcode `allowed_workspaces` di `proxy.ts:88` — harus dynamic dari `/workspaces/mine` |
| P1 | Master data list view | `GET /master-data/clients` — primary table di dashboard |
| P1 | Activity feed | `GET /activity-log/feed` — homepage feed |
| P2 | Notifications | `GET /notifications` + `PUT /notifications/{id}/read` |
| P2 | Background jobs | `GET /jobs` — track import/export progress |

## Full endpoint inventory

Disusun per domain (top-level path segment). Default status: 🟡 ready
(backend done, FE belum). Ubah jadi ✅ saat selesai integrasi.

### `/auth` (3) — Authentication
| Status | Method | Path |
|---|---|---|
| 🟡 | POST | `/auth/google` |
| 🟡 | POST | `/auth/login` |
| 🟡 | POST | `/auth/logout` |

### `/whitelist` (4) — Email allowlist
| Status | Method | Path |
|---|---|---|
| 🟡 | GET    | `/whitelist` |
| 🟡 | POST   | `/whitelist` |
| 🟡 | GET    | `/whitelist/check` |
| 🟡 | DELETE | `/whitelist/{id}` |

### `/sessions` (2) — Session management
| Status | Method | Path |
|---|---|---|
| 🟡 | POST | `/sessions/revoke` |
| 🟡 | GET  | `/sessions/revoked` |

### `/workspaces` (12) — Workspace CRUD + members
| Status | Method | Path |
|---|---|---|
| 🟡 | GET    | `/workspaces` (legacy permissive) |
| ✅ | GET    | `/workspaces/mine` |
| 🟡 | POST   | `/workspaces` |
| 🟡 | GET    | `/workspaces/{id}` |
| 🟡 | PUT    | `/workspaces/{id}` |
| 🟡 | DELETE | `/workspaces/{id}` |
| 🟡 | POST   | `/workspaces/{id}/switch` |
| 🟡 | GET    | `/workspaces/{id}/members` |
| 🟡 | POST   | `/workspaces/{id}/members/invite` |
| 🟡 | PUT    | `/workspaces/{id}/members/{member_id}` |
| 🟡 | DELETE | `/workspaces/{id}/members/{member_id}` |
| 🟡 | POST   | `/workspaces/invitations/{token}/accept` |

### `/workspace` (3) — Workspace theme + holding
| Status | Method | Path |
|---|---|---|
| 🟡 | GET | `/workspace/theme` |
| 🟡 | PUT | `/workspace/theme` |
| 🟡 | GET | `/workspace/holding/expand` |

### `/team` (18) — Team management + permissions
| Status | Method | Path |
|---|---|---|
| ✅ | GET    | `/team/permissions/me` |
| 🟡 | GET    | `/team/members` |
| 🟡 | POST   | `/team/members/invite` |
| 🟡 | GET    | `/team/members/{id}` |
| 🟡 | PUT    | `/team/members/{id}` |
| 🟡 | DELETE | `/team/members/{id}` |
| 🟡 | PUT    | `/team/members/{id}/role` |
| 🟡 | PUT    | `/team/members/{id}/status` |
| 🟡 | PUT    | `/team/members/{id}/workspaces` |
| 🟡 | POST   | `/team/invitations/{token}/accept` |
| 🟡 | GET    | `/team/roles` |
| 🟡 | POST   | `/team/roles` |
| 🟡 | GET    | `/team/roles/{id}` |
| 🟡 | PUT    | `/team/roles/{id}` |
| 🟡 | DELETE | `/team/roles/{id}` |
| 🟡 | PUT    | `/team/roles/{id}/permissions` |
| 🟡 | GET    | `/team/activity` |
| 🟡 | POST   | `/team/activity` |

### `/master-data` (21) — Master data CRUD + custom fields
| Status | Method | Path |
|---|---|---|
| 🟡 | GET    | `/master-data/clients` |
| 🟡 | POST   | `/master-data/clients` |
| 🟡 | GET    | `/master-data/clients/{id}` |
| 🟡 | PUT    | `/master-data/clients/{id}` |
| 🟡 | DELETE | `/master-data/clients/{id}` |
| 🟡 | POST   | `/master-data/clients/{id}/transition` |
| 🟡 | POST   | `/master-data/clients/{id}/reactivate` |
| 🟡 | GET    | `/master-data/clients/{id}/reactivation-history` |
| 🟡 | GET    | `/master-data/clients/export` |
| ✅ | POST   | `/master-data/clients/import` |
| ✅ | POST   | `/master-data/clients/import/preview` |
| 🟡 | GET    | `/master-data/clients/template` |
| 🟡 | POST   | `/master-data/query` |
| 🟡 | GET    | `/master-data/stats` |
| 🟡 | GET    | `/master-data/attention` |
| 🟡 | GET    | `/master-data/mutations` |
| ✅ | GET    | `/master-data/field-definitions` |
| 🟡 | POST   | `/master-data/field-definitions` |
| 🟡 | PUT    | `/master-data/field-definitions/reorder` |
| 🟡 | PUT    | `/master-data/field-definitions/{id}` |
| 🟡 | DELETE | `/master-data/field-definitions/{id}` |

### `/data-master` (26) — Legacy dashboard prefix (clients, escalations, invoices, system-config, trigger-rules)
| Status | Method | Path |
|---|---|---|
| 🟡 | GET    | `/data-master/clients` |
| 🟡 | POST   | `/data-master/clients` |
| 🟡 | GET    | `/data-master/clients/{company_id}` |
| 🟡 | PUT    | `/data-master/clients/{company_id}` |
| 🟡 | DELETE | `/data-master/clients/{company_id}` |
| 🟡 | GET    | `/data-master/clients/{company_id}/escalations` |
| 🟡 | POST   | `/data-master/clients/export` |
| 🟡 | POST   | `/data-master/clients/import` |
| 🟡 | GET    | `/data-master/escalations` |
| 🟡 | GET    | `/data-master/escalations/{id}` |
| 🟡 | PUT    | `/data-master/escalations/{id}/resolve` |
| 🟡 | GET    | `/data-master/invoices` |
| 🟡 | GET    | `/data-master/invoices/{invoice_id}` |
| 🟡 | PUT    | `/data-master/invoices/{invoice_id}` |
| 🟡 | GET    | `/data-master/message-templates` |
| 🟡 | GET    | `/data-master/message-templates/{template_id}` |
| 🟡 | PUT    | `/data-master/message-templates/{template_id}` |
| 🟡 | GET    | `/data-master/system-config` |
| 🟡 | PUT    | `/data-master/system-config/{key}` |
| 🟡 | GET    | `/data-master/template-variables` |
| 🟡 | GET    | `/data-master/trigger-rules` |
| 🟡 | POST   | `/data-master/trigger-rules` |
| 🟡 | GET    | `/data-master/trigger-rules/{rule_id}` |
| 🟡 | PUT    | `/data-master/trigger-rules/{rule_id}` |
| 🟡 | DELETE | `/data-master/trigger-rules/{rule_id}` |
| 🟡 | POST   | `/data-master/trigger-rules/cache/invalidate` |

### `/invoices` (14) — Invoice CRUD + actions
| Status | Method | Path |
|---|---|---|
| 🟡 | GET    | `/invoices` |
| 🟡 | POST   | `/invoices` |
| 🟡 | GET    | `/invoices/stats` |
| 🟡 | GET    | `/invoices/by-stage` |
| 🟡 | GET    | `/invoices/{invoice_id}` |
| 🟡 | PUT    | `/invoices/{invoice_id}` |
| 🟡 | DELETE | `/invoices/{invoice_id}` |
| 🟡 | POST   | `/invoices/{invoice_id}/mark-paid` |
| 🟡 | POST   | `/invoices/{invoice_id}/send-reminder` |
| 🟡 | GET    | `/invoices/{invoice_id}/payment-logs` |
| 🟡 | GET    | `/invoices/{invoice_id}/pdf` |
| 🟡 | GET    | `/invoices/{invoice_id}/activity` |
| 🟡 | POST   | `/invoices/{invoice_id}/update-stage` |
| 🟡 | PUT    | `/invoices/{invoice_id}/confirm-partial` |

### `/templates` (14) — Message + email templates
| Status | Method | Path |
|---|---|---|
| 🟡 | GET    | `/templates/messages` |
| 🟡 | POST   | `/templates/messages` |
| 🟡 | GET    | `/templates/messages/{id}` |
| 🟡 | PUT    | `/templates/messages/{id}` |
| 🟡 | DELETE | `/templates/messages/{id}` |
| 🟡 | GET    | `/templates/emails` |
| 🟡 | POST   | `/templates/emails` |
| 🟡 | GET    | `/templates/emails/{id}` |
| 🟡 | PUT    | `/templates/emails/{id}` |
| 🟡 | DELETE | `/templates/emails/{id}` |
| 🟡 | POST   | `/templates/preview` |
| 🟡 | GET    | `/templates/edit-logs` |
| 🟡 | GET    | `/templates/edit-logs/{template_id}` |
| 🟡 | GET    | `/templates/variables` |

### `/workflows` (16) — Workflow + canvas + steps
| Status | Method | Path |
|---|---|---|
| 🟡 | GET    | `/workflows` |
| 🟡 | POST   | `/workflows` |
| 🟡 | GET    | `/workflows/{id}` |
| 🟡 | PUT    | `/workflows/{id}` |
| 🟡 | DELETE | `/workflows/{id}` |
| 🟡 | GET    | `/workflows/by-slug/{slug}` |
| 🟡 | PUT    | `/workflows/{id}/canvas` |
| 🟡 | GET    | `/workflows/{id}/config` |
| 🟡 | GET    | `/workflows/{id}/data` |
| 🟡 | GET    | `/workflows/{id}/steps` |
| 🟡 | PUT    | `/workflows/{id}/steps` |
| 🟡 | GET    | `/workflows/{id}/steps/{stepKey}` |
| 🟡 | PUT    | `/workflows/{id}/steps/{stepKey}` |
| 🟡 | PUT    | `/workflows/{id}/tabs` |
| 🟡 | PUT    | `/workflows/{id}/stats` |
| 🟡 | PUT    | `/workflows/{id}/columns` |

### `/automation-rules` (6) — Automation rules
| Status | Method | Path |
|---|---|---|
| 🟡 | GET    | `/automation-rules` |
| 🟡 | POST   | `/automation-rules` |
| 🟡 | GET    | `/automation-rules/{id}` |
| 🟡 | PUT    | `/automation-rules/{id}` |
| 🟡 | DELETE | `/automation-rules/{id}` |
| 🟡 | GET    | `/automation-rules/change-logs` |

### `/collections` (15) — Collection (custom tables) + records
| Status | Method | Path |
|---|---|---|
| 🟡 | GET    | `/collections` |
| 🟡 | POST   | `/collections` |
| 🟡 | GET    | `/collections/{id}` |
| 🟡 | PATCH  | `/collections/{id}` |
| 🟡 | DELETE | `/collections/{id}` |
| 🟡 | POST   | `/collections/{id}/fields` |
| 🟡 | PATCH  | `/collections/{id}/fields/{field_id}` |
| 🟡 | DELETE | `/collections/{id}/fields/{field_id}` |
| 🟡 | POST   | `/collections/approvals/{approval_id}/approve` |
| 🟡 | GET    | `/collections/{id}/records` |
| 🟡 | GET    | `/collections/{id}/records/distinct` |
| 🟡 | POST   | `/collections/{id}/records` |
| 🟡 | PATCH  | `/collections/{id}/records/{record_id}` |
| 🟡 | DELETE | `/collections/{id}/records/{record_id}` |
| 🟡 | POST   | `/collections/{id}/records/bulk` |

### `/analytics` (6) + `/dashboard` (1) + `/revenue-targets` (2) + `/reports` (6) — Analytics & reports
| Status | Method | Path |
|---|---|---|
| 🟡 | GET  | `/dashboard/stats` |
| 🟡 | GET  | `/analytics/kpi` |
| 🟡 | GET  | `/analytics/kpi/bundle` |
| 🟡 | GET  | `/analytics/distributions` |
| 🟡 | GET  | `/analytics/engagement` |
| 🟡 | GET  | `/analytics/revenue-trend` |
| 🟡 | GET  | `/analytics/forecast-accuracy` |
| 🟡 | GET  | `/revenue-targets` |
| 🟡 | PUT  | `/revenue-targets` |
| 🟡 | GET  | `/reports/executive-summary` |
| 🟡 | GET  | `/reports/revenue-contracts` |
| 🟡 | GET  | `/reports/client-health` |
| 🟡 | GET  | `/reports/engagement-retention` |
| 🟡 | GET  | `/reports/workspace-comparison` |
| 🟡 | POST | `/reports/export` |

### `/notifications` (5) — User notifications
| Status | Method | Path |
|---|---|---|
| 🟡 | GET | `/notifications` |
| 🟡 | POST | `/notifications` |
| 🟡 | GET | `/notifications/count` |
| 🟡 | PUT | `/notifications/{id}/read` |
| 🟡 | PUT | `/notifications/read-all` |

### `/preferences` (4) — User preferences
| Status | Method | Path |
|---|---|---|
| 🟡 | GET    | `/preferences` |
| 🟡 | GET    | `/preferences/{namespace}` |
| 🟡 | PUT    | `/preferences/{namespace}` |
| 🟡 | DELETE | `/preferences/{namespace}` |

### `/integrations` (4) — Workspace integrations
| Status | Method | Path |
|---|---|---|
| 🟡 | GET    | `/integrations` |
| 🟡 | GET    | `/integrations/{provider}` |
| 🟡 | PUT    | `/integrations/{provider}` |
| 🟡 | DELETE | `/integrations/{provider}` |

### `/approvals` (1) — Approval dispatcher
| Status | Method | Path |
|---|---|---|
| 🟡 | POST | `/approvals/{id}/apply` |

### `/manual-actions` (4) — GUARD manual action queue
| Status | Method | Path |
|---|---|---|
| 🟡 | GET   | `/manual-actions` |
| 🟡 | GET   | `/manual-actions/{id}` |
| 🟡 | PATCH | `/manual-actions/{id}/mark-sent` |
| 🟡 | PATCH | `/manual-actions/{id}/skip` |

### `/audit-logs` (2) — Workspace access audit
| Status | Method | Path |
|---|---|---|
| 🟡 | GET  | `/audit-logs/workspace-access` |
| 🟡 | POST | `/audit-logs/workspace-access` |

### `/fireflies` (2) — Transcript dashboard views
| Status | Method | Path |
|---|---|---|
| 🟡 | GET | `/fireflies/transcripts` |
| 🟡 | GET | `/fireflies/transcripts/{id}` |

### `/reactivation` (4) — Reactivation triggers
| Status | Method | Path |
|---|---|---|
| 🟡 | GET    | `/reactivation/triggers` |
| 🟡 | POST   | `/reactivation/triggers` |
| 🟡 | GET    | `/reactivation/triggers/{id}` |
| 🟡 | DELETE | `/reactivation/triggers/{id}` |

### `/coaching` (6) — Coaching sessions (BD peer review)
| Status | Method | Path |
|---|---|---|
| 🟡 | GET    | `/coaching/sessions` |
| 🟡 | POST   | `/coaching/sessions` |
| 🟡 | GET    | `/coaching/sessions/{id}` |
| 🟡 | PATCH  | `/coaching/sessions/{id}` |
| 🟡 | DELETE | `/coaching/sessions/{id}` |
| 🟡 | POST   | `/coaching/sessions/{id}/submit` |

### `/rejection-analysis` (4) — Rejection analysis
| Status | Method | Path |
|---|---|---|
| 🟡 | GET  | `/rejection-analysis` |
| 🟡 | POST | `/rejection-analysis` |
| 🟡 | POST | `/rejection-analysis/analyze` |
| 🟡 | GET  | `/rejection-analysis/stats` |

### `/pdp` (9) — PDP compliance (erasure + retention)
| Status | Method | Path |
|---|---|---|
| 🟡 | GET    | `/pdp/erasure-requests` |
| 🟡 | POST   | `/pdp/erasure-requests` |
| 🟡 | GET    | `/pdp/erasure-requests/{id}` |
| 🟡 | POST   | `/pdp/erasure-requests/{id}/approve` |
| 🟡 | POST   | `/pdp/erasure-requests/{id}/reject` |
| 🟡 | POST   | `/pdp/erasure-requests/{id}/execute` |
| 🟡 | GET    | `/pdp/retention-policies` |
| 🟡 | POST   | `/pdp/retention-policies` |
| 🟡 | DELETE | `/pdp/retention-policies/{id}` |

### `/jobs` (3) — Background jobs (import/export/etc)
| Status | Method | Path |
|---|---|---|
| 🟡 | GET | `/jobs` |
| 🟡 | GET | `/jobs/{job_id}` |
| 🟡 | GET | `/jobs/{job_id}/download` |

### `/activity-log` + `/activity-logs` + `/action-log` (10) — Activity & action logs
| Status | Method | Path |
|---|---|---|
| 🟡 | GET  | `/activity-log/feed` |
| 🟡 | GET  | `/activity-log/today` |
| 🟡 | GET  | `/activity-logs` |
| 🟡 | POST | `/activity-logs` |
| 🟡 | GET  | `/activity-logs/recent` |
| 🟡 | GET  | `/activity-logs/stats` |
| 🟡 | GET  | `/activity-logs/companies/{company_id}/summary` |
| 🟡 | GET  | `/action-log/recent` |
| 🟡 | GET  | `/action-log/today` |
| 🟡 | GET  | `/action-log/summary` |

### `/mock` (7) — Mock outbox + per-provider trigger (FE QA)
| Status | Method | Path |
|---|---|---|
| 🟡 | GET    | `/mock/outbox` |
| 🟡 | GET    | `/mock/outbox/{id}` |
| 🟡 | DELETE | `/mock/outbox` |
| 🟡 | POST   | `/mock/claude/extract` |
| 🟡 | POST   | `/mock/fireflies/fetch` |
| 🟡 | POST   | `/mock/haloai/send` |
| 🟡 | POST   | `/mock/smtp/send` |

### `/webhook` + `/handoff` + `/payment` (6) — External webhooks (HMAC/signature based)
| Status | Method | Path |
|---|---|---|
| 🟡 | POST | `/webhook/wa` |
| 🟡 | POST | `/webhook/checkin-form` |
| 🟡 | POST | `/webhook/paperid/{workspace_id}` |
| 🟡 | POST | `/webhook/fireflies/{workspace_id}` |
| 🟡 | POST | `/handoff/new-client` |
| 🟡 | POST | `/payment/verify` |

### `/cron` (6) — Scheduled job triggers (OIDC-protected)
| Status | Method | Path |
|---|---|---|
| 🟡 | GET | `/cron/run` |
| 🟡 | GET | `/cron/invoices/overdue` |
| 🟡 | GET | `/cron/invoices/escalate` |
| 🟡 | GET | `/cron/analytics/rebuild-snapshots` |
| 🟡 | GET | `/cron/sessions/cleanup` |
| 🟡 | GET | `/cron/pdp/retention` |

### `/readiness` (1) — Health check
| Status | Method | Path |
|---|---|---|
| 🟡 | GET | `/readiness` |

## Changelog

### 2026-04-26
- ✅ `GET /workspaces/mine` integrated — strict membership listing.
- ✅ `GET /team/permissions/me` integrated — permission matrix.
- ✅ `GET /master-data/field-definitions` integrated — list custom field definitions per workspace.
- ✅ `POST /master-data/clients/import/preview` integrated — dry-run xlsx parser dengan custom_fields support.
- ✅ `POST /master-data/clients/import` integrated — submit bulk import (creates approval request).
- ✅ Seed script dibuat untuk bootstrap `team_members` di local dev.
- ✅ `member_ids` field disembunyikan dari semua workspace API response.
- 📋 Full endpoint inventory (247 endpoint) di-list di doc ini.
