# Auth — Database Schema

## IMPORTANT: Minimal Schema

Auth service (`ms-auth-proxy.up.railway.app`) sudah handle user management.
Dashboard backend **hanya** perlu 1 table: `whitelist`.

Tables yang **TIDAK perlu** dibuat:
- ~~`users`~~ — managed by ms-auth-proxy
- ~~`sessions`~~ — cookie-based, no server store
- ~~`refresh_tokens`~~ — ms-auth-proxy handles token lifecycle
- ~~`login_attempts`~~ — optional audit trail (lihat bawah)

---

## Table: `whitelist`

Menentukan siapa yang boleh akses dashboard setelah berhasil login di ms-auth-proxy.

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

### Seed Data
```sql
INSERT INTO whitelist (email, is_active, added_by, notes) VALUES
('arief.faltah@dealls.com', true, 'system', 'Super Admin'),
('dhimas.priyadi@sejutacita.id', true, 'system', 'Super Admin'),
('budi@kantorku.id', true, 'arief.faltah@dealls.com', 'KantorKu team');
```

---

## Optional: `login_attempts` (for audit trail)

Bisa ditambah nanti jika perlu tracking login activity. Tidak blocking fitur lain.

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

## How Whitelist Check Works

```
Login Request (email/password or Google)
  │
  ▼
ms-auth-proxy/api/v1/auth/login → 200 OK (credentials valid)
  │
  ▼
Dashboard checks: SELECT 1 FROM whitelist WHERE email = $1 AND is_active = true
  │
  ├─ Found     → Set httpOnly cookie, return user info, allow access
  └─ Not found → Return 403 { error: "not_whitelisted" }
```

**Current implementation note**: BFF (Next.js) checks whitelist by calling `GET ms-auth-proxy/api/v1/whitelist` with Bearer token and filtering client-side. When dashboard backend has its own whitelist table, switch to direct DB query — faster and no token dependency.
