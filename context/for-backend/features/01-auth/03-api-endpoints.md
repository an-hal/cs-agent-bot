# API Endpoints — Auth

## Base URL
```
{BACKEND_API_URL}/api/v1
```

---

## 1. Login (Email/Password)

### POST `/auth/login`

Autentikasi user dengan email dan password. Return access token + user info.

```
Headers:
  Content-Type: application/json

Request body:
{
  "email": "arief@dealls.com",
  "password": "secret123"
}

Validasi:
  - email: required, format email valid
  - password: required, min 1 karakter

Response 200 (sukses):
{
  "requestId": "req-abc123",
  "status": "success",
  "message": "Login berhasil",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "expires": "2026-05-12T10:00:00Z",
    "expire": 1747044000000,
    "platform": "dashboard",
    "admin": true,
    "is_full_access_token": true,
    "user": {
      "_id": "uuid-user-001",
      "email": "arief@dealls.com",
      "phoneNumber": "628123456789",
      "companyId": "dealls",
      "createdAt": "2025-01-01T00:00:00Z"
    }
  }
}

Response 401 (invalid credentials):
{
  "error": "Email atau password salah"
}

Response 429 (rate limited):
{
  "error": "Terlalu banyak percobaan. Coba lagi dalam 1 menit."
}
Headers:
  X-RateLimit-Limit: 10
  X-RateLimit-Remaining: 0
  X-RateLimit-Reset: 1744425600
```

**Frontend flow:**
1. Rate limit check (10 req/min per IP)
2. Zod schema validation
3. Whitelist check (GET `/api/v1/whitelist`)
4. Forward ke backend `POST /api/v1/auth/login`
5. Set `auth_session` httpOnly cookie dari `access_token`
6. Cookie expiry = backend `expire` timestamp

---

## 2. Google OAuth

### POST `/auth/google`

Verifikasi Google ID token, cek whitelist, create session.

```
Headers:
  Content-Type: application/json

Request body:
{
  "credential": "eyJhbGciOiJSUzI1NiIs..."
}
(credential = Google ID token dari GSI callback)

Response 200 (sukses):
{
  "status": "success",
  "message": "Google login berhasil",
  "data": {
    "user": {
      "_id": "google-sub-12345",
      "email": "arief@dealls.com",
      "name": "Arief",
      "picture": "https://lh3.googleusercontent.com/..."
    },
    "admin": true,
    "provider": "google"
  }
}

Response 400 (missing credential):
{
  "error": "Google credential tidak ditemukan"
}

Response 401 (invalid/expired token):
{
  "error": "Token Google tidak valid atau sudah expired"
}

Response 403 (not whitelisted):
{
  "error": "not_whitelisted",
  "email": "unknown@gmail.com"
}

Response 500 (Google Client ID not configured):
{
  "error": "Google Client ID belum dikonfigurasi di server"
}
```

**Google token verification (server-side):**
```
GET https://oauth2.googleapis.com/tokeninfo?id_token={credential}

Checks:
  1. HTTP 200 dari Google → token valid
  2. payload.aud === GOOGLE_CLIENT_ID → token untuk app kita
  3. payload.email_verified === true → email sudah diverifikasi Google

Payload fields yang dipakai:
  - sub          → Google user ID (stored as _id)
  - email        → user email
  - name         → display name
  - picture      → avatar URL
  - aud          → audience (must match our client ID)
  - email_verified → boolean
```

**Session token untuk Google:**
```
Token format: "google:{googleId}:{timestamp}.{hmac_signature}"
Signature:    HMAC-SHA256(SESSION_SECRET, "google:{googleId}:{timestamp}")
              — full 64-char hex (NOT truncated to 16)
Cookie lifetime: 30 hari
```

Backend validator (if re-implementing this in Go): reconstruct the HMAC with `crypto/hmac` + `sha256`, compare via `hmac.Equal` (constant-time). Reject tokens whose signature portion is anything other than 64 hex chars.

---

## 3. Logout

### POST `/auth/logout`

Hapus session cookie. Tidak perlu auth token — idempotent.

```
No request body needed.

Response 200:
{
  "message": "Logged out"
}

Side effects:
  - Cookie auth_session di-set maxAge=0 (expire immediately)
  - Frontend clears localStorage: auth_user, auth_token, active_workspace
```

---

## 4. Whitelist

### GET `/whitelist`

List semua email yang boleh akses dashboard. **Tidak memerlukan auth token.**

```
Response 200:
{
  "status": "success",
  "message": "Whitelist retrieved",
  "data": [
    {
      "id": 1,
      "email": "arief@dealls.com",
      "is_active": true,
      "created_at": "2025-01-01T00:00:00Z",
      "updated_at": "2025-01-01T00:00:00Z"
    },
    {
      "id": 2,
      "email": "budi@kantorku.id",
      "is_active": true,
      "created_at": "2025-02-01T00:00:00Z",
      "updated_at": "2025-02-01T00:00:00Z"
    }
  ]
}
```

**Whitelist check logic (dilakukan oleh frontend BFF):**
```
1. GET /api/v1/whitelist → ambil semua entries
2. Filter: entry.email.toLowerCase() === inputEmail.toLowerCase() && entry.is_active === true
3. Jika tidak ada match → return 403 { error: "not_whitelisted", email: inputEmail }
4. Jika ada match → continue login flow
```

### POST `/whitelist` (Admin only)

Tambah email ke whitelist.

```
Headers:
  Authorization: Bearer {admin_token}
  Content-Type: application/json

Request body:
{
  "email": "new-user@dealls.com",
  "notes": "New BD team member"
}

Response 201:
{
  "data": {
    "id": 3,
    "email": "new-user@dealls.com",
    "is_active": true,
    "added_by": "uuid-admin",
    "notes": "New BD team member",
    "created_at": "2026-04-12T10:00:00Z"
  }
}

Response 409 (email sudah ada):
{
  "error": "Email sudah ada di whitelist"
}
```

### DELETE `/whitelist/{id}` (Admin only)

Soft-delete (set `is_active = false`). User yang sudah login tetap bisa akses sampai session expired.

```
Headers:
  Authorization: Bearer {admin_token}

Response 200:
{
  "message": "Email dihapus dari whitelist",
  "id": 3
}
```

---

## 5. Token Refresh

### POST `/auth/refresh`

Perpanjang access token menggunakan refresh token.

```
Headers:
  Content-Type: application/json

Request body:
{
  "refresh_token": "rt-abc123..."
}

Response 200:
{
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "rt-new-xyz789...",
    "expires": "2026-04-12T10:15:00Z",
    "expire": 1744451700000
  }
}

Response 401 (expired/revoked/invalid):
{
  "error": "Refresh token tidak valid atau sudah expired"
}
```

**Refresh token rotation:**
```
1. Client kirim refresh_token
2. Backend verify: token_hash exists, not revoked, not expired
3. Mark old refresh_token as rotated (rotated_at = NOW())
4. Issue new access_token + new refresh_token
5. New refresh_token has same family_id

SECURITY: Jika old (already-rotated) refresh_token dipakai lagi:
  → Revoke ALL tokens in that family_id
  → Force user re-login (possible token theft)
```

---

## 6. Session Validation

### GET `/auth/me`

Validate current session dan return user info. Dipakai untuk session check saat page load.

```
Headers:
  Authorization: Bearer {access_token}

Response 200:
{
  "data": {
    "user": {
      "_id": "uuid-user-001",
      "email": "arief@dealls.com",
      "name": "Arief",
      "is_admin": true,
      "last_login_at": "2026-04-12T08:00:00Z"
    }
  }
}

Response 401:
{
  "error": "Token tidak valid atau sudah expired"
}
```

---

## 7. Password Management (Coming Soon)

### POST `/auth/forgot-password`

```
Request: { "email": "arief@dealls.com" }
Response 200: { "message": "Email reset password telah dikirim" }
```

### POST `/auth/reset-password`

```
Request: { "token": "reset-token-123", "new_password": "newPass456" }
Response 200: { "message": "Password berhasil diubah" }
```

### POST `/auth/change-password`

```
Headers: Authorization: Bearer {token}
Request: { "current_password": "old123", "new_password": "new456" }
Response 200: { "message": "Password berhasil diubah" }
```

---

## Error Code Reference

| HTTP Status | Error Code | Penjelasan |
|---|---|---|
| 400 | `validation_error` | Input tidak valid (email format, password kosong) |
| 401 | `invalid_credentials` | Email/password salah |
| 401 | `token_expired` | Access token sudah expired |
| 401 | `token_invalid` | Token tidak valid (malformed, tampered) |
| 401 | `google_invalid` | Google ID token tidak valid |
| 403 | `not_whitelisted` | Email tidak ada di whitelist |
| 403 | `account_disabled` | Akun di-deactivate oleh admin |
| 409 | `duplicate_email` | Email sudah terdaftar |
| 429 | `rate_limited` | Terlalu banyak request (10/min per IP) |
| 500 | `config_error` | Server configuration missing (Google Client ID, etc.) |
