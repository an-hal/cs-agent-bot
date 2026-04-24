# API Endpoints — Workspace

## Base URL
```
{BACKEND_API_URL}/api/v1
```

All endpoints require `Authorization: Bearer {token}` header unless noted otherwise.

---

## 1. List Workspaces

### GET `/api/workspaces`

List semua workspaces yang user punya akses. Endpoint ini dipanggil saat app startup oleh `CompanyProvider`.

```
Headers:
  Authorization: Bearer {token}

Response 200:
{
  "status": "success",
  "message": "Workspaces retrieved",
  "data": [
    {
      "id": "ws-dealls-001",
      "slug": "dealls",
      "name": "Dealls",
      "logo": "DE",
      "color": "#534AB7",
      "plan": "Enterprise",
      "is_holding": false,
      "member_ids": null,
      "created_at": "2025-01-01T00:00:00Z"
    },
    {
      "id": "ws-kantorku-001",
      "slug": "kantorku",
      "name": "KantorKu",
      "logo": "KK",
      "color": "#1D9E75",
      "plan": "Pro",
      "is_holding": false,
      "member_ids": null,
      "created_at": "2025-01-01T00:00:00Z"
    },
    {
      "id": "ws-holding-001",
      "slug": "holding",
      "name": "Sejutacita",
      "logo": "SC",
      "color": "#0EA5E9",
      "plan": "Holding",
      "is_holding": true,
      "member_ids": ["ws-dealls-001", "ws-kantorku-001"],
      "created_at": "2025-01-01T00:00:00Z"
    }
  ]
}

Response 401:
{
  "error": "Unauthorized"
}
```

**Frontend proxy flow:**
```
1. Frontend GET /api/workspaces (Next.js API route)
2. Extract auth_session cookie
3. Forward to backend GET {DASHBOARD_API_URL}/api/workspaces with token
4. Return { data: [...] } to client
5. If 401 → trigger handleSessionExpired() on dashboard pages
```

**Backend query:**
```sql
SELECT w.*
FROM workspaces w
JOIN workspace_members wm ON wm.workspace_id = w.id
WHERE wm.user_id = $1 AND wm.is_active = TRUE AND w.is_active = TRUE
ORDER BY w.is_holding ASC, w.name ASC;
```

---

## 2. Get Single Workspace

### GET `/workspaces/{id}`

Get workspace details by UUID.

```
Headers:
  Authorization: Bearer {token}

Response 200:
{
  "data": {
    "id": "ws-dealls-001",
    "slug": "dealls",
    "name": "Dealls",
    "logo": "DE",
    "color": "#534AB7",
    "plan": "Enterprise",
    "is_holding": false,
    "member_ids": null,
    "settings": {
      "timezone": "Asia/Jakarta",
      "currency": "IDR",
      "date_format": "DD/MM/YYYY",
      "working_hours": { "start": "09:00", "end": "17:00" },
      "features_enabled": ["pipeline", "invoices", "automation"]
    },
    "members": [
      {
        "user_id": "uuid-user-001",
        "email": "arief@dealls.com",
        "name": "Arief",
        "role": "owner",
        "joined_at": "2025-01-01T00:00:00Z"
      },
      {
        "user_id": "uuid-user-002",
        "email": "budi@dealls.com",
        "name": "Budi",
        "role": "member",
        "joined_at": "2025-03-15T00:00:00Z"
      }
    ],
    "stats": {
      "total_clients": 85,
      "total_members": 5,
      "created_at": "2025-01-01T00:00:00Z"
    }
  }
}

Response 403:
{
  "error": "Anda tidak memiliki akses ke workspace ini"
}

Response 404:
{
  "error": "Workspace tidak ditemukan"
}
```

---

## 3. Create Workspace

### POST `/workspaces`

Buat workspace baru. User yang membuat otomatis jadi owner.

```
Headers:
  Authorization: Bearer {token}
  Content-Type: application/json

Request body:
{
  "name": "New Venture",
  "slug": "new-venture",
  "logo": "NV",
  "color": "#534AB7",
  "plan": "Basic",
  "settings": {
    "timezone": "Asia/Jakarta",
    "currency": "IDR"
  }
}

Validasi:
  - name: required, 1-100 chars
  - slug: required, 1-50 chars, lowercase, alphanumeric + hyphens only
  - logo: optional, max 10 chars (defaults to first 2 chars of name uppercase)
  - color: optional, valid hex color (defaults to #534AB7)
  - plan: optional (defaults to Basic)

Response 201:
{
  "data": {
    "id": "uuid-new-workspace",
    "slug": "new-venture",
    "name": "New Venture",
    "logo": "NV",
    "color": "#534AB7",
    "plan": "Basic",
    "is_holding": false,
    "member_ids": null,
    "created_at": "2026-04-12T10:00:00Z"
  }
}

Side effects:
  - workspace_members entry created: { user_id: requester, role: 'owner' }

Response 409:
{
  "error": "Slug 'new-venture' sudah digunakan"
}
```

---

## 4. Update Workspace

### PUT `/workspaces/{id}`

Partial update workspace. Requires owner or admin role.

```
Headers:
  Authorization: Bearer {token}
  X-Workspace-ID: {uuid}
  Content-Type: application/json

Request body (partial — hanya field yang berubah):
{
  "name": "Dealls Jobs",
  "color": "#059669",
  "settings": {
    "timezone": "Asia/Singapore"
  }
}

Catatan:
  - slug TIDAK boleh diubah setelah creation (break existing URLs/bookmarks)
  - settings di-MERGE (bukan replace), sama seperti custom_fields di master_data

Response 200:
{
  "data": {
    "id": "ws-dealls-001",
    "slug": "dealls",
    "name": "Dealls Jobs",
    "color": "#059669",
    ...
  }
}

Response 403:
{
  "error": "Hanya owner atau admin yang bisa mengubah workspace"
}
```

---

## 5. Delete Workspace

### DELETE `/workspaces/{id}`

Soft-delete workspace. Requires owner role. **Destructive operation** — semua data di workspace ini akan di-archive.

```
Headers:
  Authorization: Bearer {token}
  X-Workspace-ID: {uuid}

Response 200:
{
  "message": "Workspace berhasil di-nonaktifkan",
  "id": "ws-dealls-001"
}

Side effects:
  - workspace.is_active = false
  - Semua workspace_members.is_active = false
  - Data master_data TIDAK dihapus (soft delete / archive)

Response 403:
{
  "error": "Hanya owner yang bisa menghapus workspace"
}
```

---

## 6. Workspace Members

### GET `/workspaces/{id}/members`

List members of a workspace.

```
Headers:
  Authorization: Bearer {token}
  X-Workspace-ID: {uuid}

Response 200:
{
  "data": [
    {
      "id": "uuid-member-001",
      "user_id": "uuid-user-001",
      "email": "arief@dealls.com",
      "name": "Arief",
      "role": "owner",
      "permissions": {},
      "is_active": true,
      "joined_at": "2025-01-01T00:00:00Z"
    }
  ]
}
```

### POST `/workspaces/{id}/members/invite`

Invite user ke workspace. Requires owner or admin role.

```
Headers:
  Authorization: Bearer {token}
  X-Workspace-ID: {uuid}
  Content-Type: application/json

Request body:
{
  "email": "new-member@dealls.com",
  "role": "member"
}

Response 201:
{
  "data": {
    "id": "uuid-invitation",
    "email": "new-member@dealls.com",
    "role": "member",
    "invite_token": "inv-random-token-123",
    "expires_at": "2026-04-19T10:00:00Z",
    "status": "pending"
  },
  "message": "Invitation terkirim ke new-member@dealls.com"
}

Response 409:
{
  "error": "User sudah menjadi member workspace ini"
}
```

### PUT `/workspaces/{id}/members/{member_id}`

Update member role. Requires owner role.

```
Headers:
  Authorization: Bearer {token}
  X-Workspace-ID: {uuid}
  Content-Type: application/json

Request body:
{
  "role": "admin"
}

Response 200:
{
  "data": { "id": "uuid-member-002", "role": "admin", ... }
}
```

### DELETE `/workspaces/{id}/members/{member_id}`

Remove member from workspace. Requires owner role. Cannot remove self (owner).

```
Response 200:
{
  "message": "Member dihapus dari workspace",
  "member_id": "uuid-member-002"
}

Response 403:
{
  "error": "Owner tidak bisa menghapus dirinya sendiri dari workspace"
}
```

---

## 7. Workspace Settings

### GET `/workspaces/{id}/settings`

Get all settings for a workspace.

```
Headers:
  Authorization: Bearer {token}
  X-Workspace-ID: {uuid}

Response 200:
{
  "data": {
    "timezone": "Asia/Jakarta",
    "currency": "IDR",
    "date_format": "DD/MM/YYYY",
    "working_hours": { "start": "09:00", "end": "17:00" },
    "features_enabled": ["pipeline", "invoices", "automation"],
    "theme_preset": "amethyst"
  }
}
```

### PUT `/workspaces/{id}/settings`

Update workspace settings (merge). Requires owner or admin role.

```
Headers:
  Authorization: Bearer {token}
  X-Workspace-ID: {uuid}
  Content-Type: application/json

Request body (partial merge):
{
  "timezone": "Asia/Singapore",
  "theme_preset": "emerald"
}

Response 200:
{
  "data": {
    "timezone": "Asia/Singapore",
    "currency": "IDR",
    "date_format": "DD/MM/YYYY",
    "working_hours": { "start": "09:00", "end": "17:00" },
    "features_enabled": ["pipeline", "invoices", "automation"],
    "theme_preset": "emerald"
  }
}
```

---

## 8. Theme Preferences

### GET `/workspaces/{id}/theme`

Get theme preferences for workspace. Currently stored client-side (localStorage), but endpoint provided for future server-side persistence.

```
Headers:
  Authorization: Bearer {token}
  X-Workspace-ID: {uuid}

Response 200:
{
  "data": {
    "theme_preset": "amethyst",
    "color": "#534AB7",
    "dark_mode": false,
    "palette": {
      "50": "#F0EFF8",
      "100": "#D9D6EF",
      "200": "#B5AFDF",
      "400": "#7F77DD",
      "600": "#534AB7",
      "800": "#352F72",
      "900": "#201C44"
    }
  }
}
```

### PUT `/workspaces/{id}/theme`

Update theme preference. Accessible to all members (personal preference per workspace).

```
Headers:
  Authorization: Bearer {token}
  X-Workspace-ID: {uuid}
  Content-Type: application/json

Request body:
{
  "theme_preset": "emerald"
}

Available presets: amethyst, emerald, ocean, sunset, rose, lavender, mint, golden, sakura

Response 200:
{
  "data": {
    "theme_preset": "emerald",
    "color": "#059669"
  }
}
```

**Note:** Saat ini theme disimpan di localStorage per-user per-workspace. Backend endpoint ini disiapkan untuk migrasi ke server-side storage agar theme sync lintas device.

---

## 9. Switch Workspace

### POST `/workspaces/{id}/switch`

Switch active workspace. Optional endpoint — saat ini switching dilakukan client-side saja (localStorage + context state). Endpoint ini berguna untuk:
1. Server-side audit trail (siapa switch ke mana, kapan)
2. Prefetch workspace data saat switch

```
Headers:
  Authorization: Bearer {token}
  Content-Type: application/json

Request body:
{
  "workspace_id": "ws-kantorku-001"
}

Response 200:
{
  "data": {
    "workspace": {
      "id": "ws-kantorku-001",
      "slug": "kantorku",
      "name": "KantorKu",
      "color": "#1D9E75",
      ...
    },
    "user_role": "owner",
    "last_active_at": "2026-04-12T10:00:00Z"
  }
}

Response 403:
{
  "error": "Anda tidak memiliki akses ke workspace ini"
}
```

---

## 10. Integrations (per-workspace)

External integrations (HaloAI WA, Telegram bot, Paper.id) are scoped per workspace. SMTP is global and lives outside these endpoints. See `00-shared/04-integrations.md` for the canonical spec, credential masking rules, and full request/response shapes.

### GET `/integrations`

List integration configs for the active workspace. Secret fields (`api_key`, `bot_token`) are returned masked (`****ab3f`).

```
Headers:
  Authorization: Bearer {token}
  X-Workspace-ID: {uuid}

Response 200:
{
  "data": {
    "haloai":   { "api_url": "https://api.haloai.id", "api_key": "****ab3f", "wa_number": "628123456789", "active": true },
    "telegram": { "bot_token": "****xx12", "default_chat_id": "-100123456789", "active": false },
    "paperid":  { "api_url": "https://api.paper.id", "api_key": "****9f20", "active": true }
  }
}
```

Frontend usage: `app/dashboard/[workspace]/settings/page.tsx` → Integrations tab (state kept in `localStorage` key `integrations_{workspace_id}` until this endpoint lands).

### POST `/integrations/test/{provider}`

Test live connection for one provider without persisting changes. `{provider}` ∈ `haloai` | `telegram` | `paperid` | `email`.

```
Headers:
  Authorization: Bearer {token}
  X-Workspace-ID: {uuid}
  Content-Type: application/json

Request body (optional — test unsaved draft values):
{
  "api_url": "https://api.haloai.id",
  "api_key": "new-key-to-test",
  "wa_number": "628123456789"
}

Response 200: { "success": true,  "message": "Connected successfully" }
Response 400: { "success": false, "message": "Invalid API key" }
```

> **Cross-reference:** full integration schema, PUT `/integrations` (which triggers `change_integration` approval on API-key changes), and webhook/SMTP semantics are defined in `context/for-backend/features/00-shared/04-integrations.md`.

---

## Holding-Specific Behavior

Untuk workspace dengan `is_holding = true`:

```
GET /api/workspaces → response includes holding workspace with member_ids

Frontend resolution:
  1. member_ids berisi UUIDs: ["ws-dealls-001", "ws-kantorku-001"]
  2. Frontend maps UUIDs ke slugs via data array:
     const member = data.find(d => d.id === mid)
     return member?.slug ?? mid
  3. Used for: navigation, data aggregation queries

Holding queries (any workspace-scoped endpoint):
  If active workspace is holding:
    → Backend checks is_holding = true
    → Expands to all member workspace_ids
    → Queries with WHERE workspace_id IN (member_ids)
    → Response includes workspace_name per record
```

---

## Error Code Reference

| HTTP Status | Error Code | Penjelasan |
|---|---|---|
| 400 | `validation_error` | Input tidak valid (slug format, name kosong) |
| 401 | `unauthorized` | Token tidak valid atau tidak ada |
| 403 | `forbidden` | User tidak punya akses ke workspace / role insufficient |
| 404 | `not_found` | Workspace tidak ditemukan |
| 409 | `duplicate_slug` | Slug sudah digunakan workspace lain |
| 409 | `duplicate_member` | User sudah menjadi member workspace |

---

## Checker-Maker Approval Required

The following endpoints require approval before execution.
See `00-shared/05-checker-maker.md` for the full approval system spec.

### PUT `/integrations` (when API keys change) → Approval Required

When updating workspace integrations that involve API key changes (e.g., Paper.id API key, WhatsApp API key, Telegram bot token), create an approval request:

```
POST /approvals
{
  "request_type": "change_integration",
  "payload": {
    "workspace_id": "uuid",
    "workspace_name": "Dealls",
    "integration_type": "paper_id",
    "changed_fields": ["api_key"],
    "has_active_invoices": true,
    "active_invoice_count": 12
  }
}
```

When approved, the system applies the integration changes.

**Approval payload format:**
| Field | Type | Description |
|-------|------|-------------|
| `workspace_id` | UUID | Workspace whose integration is changing |
| `workspace_name` | string | Display name for the approval reviewer |
| `integration_type` | string | Which integration is being changed (e.g., `paper_id`, `whatsapp`, `telegram`) |
| `changed_fields` | string[] | Which fields are changing (e.g., `["api_key"]`, `["webhook_url"]`) |
| `has_active_invoices` | boolean | Whether there are active invoices using this integration |
| `active_invoice_count` | number | Count of active invoices for risk assessment |

> **Note:** Only changes to sensitive fields (API keys, secrets, webhook URLs) require approval. Non-sensitive updates (display name, description) do not require approval.
