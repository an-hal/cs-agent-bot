# Auth — Backend Implementation Guide

## IMPORTANT: Auth Architecture

Dashboard **TIDAK** mengelola database users/sessions sendiri.
Login di-proxy ke **existing auth service** milik Sejutacita:

```
Auth Service: https://ms-auth-proxy.up.railway.app
```

Dashboard hanya menambahkan **whitelist gate** — tidak semua user yang berhasil login boleh masuk dashboard. Hanya email yang terdaftar di whitelist.

## Auth Flow

```
User → Login Page
  │
  ├─ Email/Password:
  │   POST /api/auth/login (Next.js BFF)
  │     → Rate limit check
  │     → Whitelist check (GET ms-auth-proxy/api/v1/whitelist)
  │     → Proxy to ms-auth-proxy/api/v1/auth/login
  │     → Set httpOnly cookie with access_token
  │
  └─ Google OAuth:
      POST /api/auth/google (Next.js BFF)
        → Verify Google token via googleapis
        → Whitelist check
        → Generate HMAC session token
        → Set httpOnly cookie
```

## External Endpoints (Already Exist — DO NOT recreate)

### 1. Login
```bash
curl --location 'https://ms-auth-proxy.up.railway.app/api/v1/auth/login' \
--header 'Content-Type: application/json' \
--data-raw '{
  "email": "user@dealls.com",
  "password": "password123"
}'

# Response 200:
{
  "user": { "_id": "...", "email": "...", "roles": [...] },
  "access_token": "eyJ...",
  "expire": "2026-05-12T00:00:00Z"
}
```

### 2. Whitelist (requires fresh Bearer token)
```bash
curl --location 'https://ms-auth-proxy.up.railway.app/api/v1/whitelist' \
--header 'Authorization: Bearer {access_token}'

# Response 200:
[
  { "id": "...", "email": "arief.faltah@dealls.com", "is_active": true },
  { "id": "...", "email": "user@kantorku.id", "is_active": true }
]
```

**PENTING**: Token di header Authorization harus yang baru (fresh). Token expired → 401.

## What Dashboard Backend Needs to Implement

Dashboard backend **HANYA** perlu:

### 1. Whitelist Management (CRUD)
- Database table `whitelist` untuk kelola email yang boleh akses
- Admin bisa add/remove email via dashboard
- Check dilakukan saat login → jika email tidak ada di whitelist → tolak

### 2. Session Cookie Handling
- Proxy login ke ms-auth-proxy
- Set httpOnly cookie dari response token
- Validate cookie di middleware

### 3. Google OAuth Session
- Verify Google token → whitelist check → HMAC session → cookie

## What Dashboard Backend Does NOT Need

- ❌ `users` table — managed by ms-auth-proxy
- ❌ `sessions` table — cookie-based, no server-side session store needed
- ❌ `refresh_tokens` table — ms-auth-proxy handles token lifecycle
- ❌ `login_attempts` table — optional, bisa ditambah nanti untuk audit
- ❌ Password management (forgot/reset/change) — handled by ms-auth-proxy

## Session Management

```
Cookie: auth_session (httpOnly, secure, sameSite=lax)

Email/Password Login:
  Token = access_token from ms-auth-proxy
  Expiry = backend-provided `expire` timestamp

Google OAuth Login:
  Token = "google:{googleId}:{timestamp}.{hmac_signature}"
  Signature = HMAC-SHA256(SESSION_SECRET, "google:{googleId}:{timestamp}")  // full 64-char hex
  Expiry = 30 days
```

**SECURITY NOTE**: previously the signature was truncated to `.slice(0, 16)` (64-bit) — brute-force feasible. Current implementation uses full 256-bit HMAC. Backend validator MUST compare against the full 64-char hex; do not accept shorter sigs.

### Session Expiry Handling
1. API returns 401
2. Frontend `handleSessionExpired()` → clear localStorage → redirect `/auth/login?expired=1`
3. Login page shows toast "Session expired. Please login again."

## Middleware Protection (proxy.ts)

```
Request → /dashboard/*
  ├─ No auth_session cookie → Redirect to /auth/login
  ├─ Slug in URL → 301 redirect to UUID
  └─ Has cookie → Continue

Request → /auth/login
  ├─ Has cookie → Redirect to /dashboard
  └─ No cookie → Show login page
```

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `BACKEND_API_URL` | Yes | `https://ms-auth-proxy.up.railway.app` — existing auth service |
| `DASHBOARD_API_URL` | Yes | Dashboard backend for workspace/data APIs |
| `NEXT_PUBLIC_GOOGLE_CLIENT_ID` | Yes | Google OAuth Client ID |
| `SESSION_SECRET` | Yes | HMAC secret for Google session signing |
| `NODE_ENV` | Auto | `production` → cookie secure=true |
