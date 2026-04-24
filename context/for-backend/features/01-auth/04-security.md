# Security — Auth Backend

## 1. Rate Limiting

### Current Implementation (Frontend BFF)
```
Algorithm:   Sliding window log (per IP)
Window:      60 seconds
Max:         10 requests per window
Key:         x-forwarded-for → x-real-ip → "unknown"
Storage:     In-memory Map (per serverless instance)
Cleanup:     Every 5 minutes, prune expired entries
```

### Recommended Backend Implementation
```
Algorithm:   Sliding window (Redis-backed)
Storage:     Redis SORTED SET per key

Rate limits per endpoint:
  POST /auth/login     → 5 req/min per IP, 3 req/min per email
  POST /auth/google    → 10 req/min per IP
  POST /auth/refresh   → 20 req/min per user
  GET  /whitelist      → 30 req/min per IP (public endpoint)

Redis key pattern:
  rl:{endpoint}:{ip}   → ZADD timestamp, ZRANGEBYSCORE to count window
  rl:login:{email}     → separate counter per email to prevent credential stuffing
```

### Response Headers (IETF Draft Standard)
```
X-RateLimit-Limit: 10          — max requests per window
X-RateLimit-Remaining: 7       — requests left in current window
X-RateLimit-Reset: 1744425600  — Unix epoch (seconds) when window resets
```

### Brute Force Protection
```
After 5 failed login attempts for same email within 15 minutes:
  → Lock account for 15 minutes
  → Return 429 with message: "Akun dikunci sementara. Coba lagi dalam 15 menit."
  → Log to login_attempts table with failure_reason = 'account_locked'

After 20 failed attempts from same IP within 1 hour:
  → Block IP for 1 hour
  → Return 429 with extended lockout message
```

---

## 2. Cookie Security

### auth_session Cookie Configuration
```
Name:       auth_session
Value:      {access_token} atau {signed_session_token}
httpOnly:   true          — JavaScript tidak bisa akses → anti XSS token theft
secure:     true (prod)   — hanya kirim via HTTPS
sameSite:   lax           — kirim untuk top-level navigation + same-site requests
                          — TIDAK kirim untuk cross-origin POST (anti CSRF)
path:       /             — available di semua routes
expires:    backend expire time (email login) atau 30 hari (Google login)
```

### Kenapa `sameSite: lax` (bukan `strict`)?
```
strict → cookie TIDAK dikirim saat user klik link dari external site ke dashboard
         → user harus login ulang setiap kali buka link dari email/Slack
lax    → cookie dikirim untuk top-level GET navigation (link klik)
         → TIDAK dikirim untuk cross-origin form POST (tetap aman dari CSRF)
         → Best balance antara UX dan security
```

### Cookie Rotation on Login
```
Sebelum set cookie baru, delete cookie lama dulu:
  response.cookies.delete('auth_session')
  response.cookies.set('auth_session', newToken, {...})

Ini prevent edge case di mana browser menyimpan multiple cookies
dengan path berbeda.
```

---

## 2b. Workspace Scope Enforcement

Every authenticated API request carries an `X-Workspace-ID` header (UUID). **Backend MUST treat this header as untrusted** and re-validate it against the user's session claims on every request.

### Trust model

```
Browser → FE middleware (proxy.ts) → FE handler (requireWorkspaceScope)
         │                            │
         │ strips client-sent         │ reads trusted header set by
         │ X-Workspace-ID             │ middleware, cross-checks
         │ re-derives from referer    │ allowed_workspaces cookie
         │ gated by allowed_workspaces│
         │ cookie
         ▼                            ▼
       TRUSTED HEADER               HANDLER ACCESS GRANTED
                │
                ▼
   Backend API (DASHBOARD_API_URL)
                │
                │ ⚠ DO NOT TRUST the forwarded header.
                │   Re-validate against the session user's
                │   workspace membership.
                ▼
         Backend enforcement
```

### Backend MUST

1. **Ignore query params for workspace scope.** Never accept `?company=` or `?workspace_id=` from the client — those are display-only hints, not authorization inputs. FE handlers already refuse to read them.
2. **Re-validate `X-Workspace-ID` against session.** Look up the session's user, fetch their workspace memberships (or read `allowed_workspaces` from the same JWT/session store), and reject 403 if the header's workspace isn't in the allowed list.
3. **Reject missing header** with 400 for any workspace-scoped endpoint. Example shapes:
   - `GET /master-data/clients` → required
   - `POST /workflows/{id}/save` → required
   - `GET /analytics/*` → required
4. **Return 403, not 404**, if the user doesn't have access — so probing can't be used to enumerate workspace IDs.

### Why re-validate (defense-in-depth)

The FE middleware could be bypassed if: it's misconfigured, the matcher excludes a route, or the request comes from a non-browser client (curl, Postman, server-to-server). In all those cases the backend becomes the last line of defense. A crafted `Referer:` header can otherwise set an arbitrary workspace — the allow-list check MUST happen on the server that owns the data.

### Cookie contract: `allowed_workspaces`

The FE workspace gating depends on a cookie that **the backend `/auth/login` handler MUST set on successful login**:

```
Cookie name:   allowed_workspaces
Value:         comma-separated workspace UUIDs
               e.g., "ws-dealls-001,ws-kantorku-001"
Attributes:    httpOnly, sameSite=lax, secure (in production)
Lifetime:      match auth_session (30 days)
```

**Used by:**
- `proxy.ts` middleware — gates which workspace UUID the FE will derive from the request `Referer` header before forwarding it as `X-Workspace-ID`. UUID not in the cookie → header dropped → handler 400s.
- `requireWorkspaceScope()` in `app/api/_lib/auth-guard.ts` — re-validates against the cookie even though middleware already did.

**FE env toggle: `STRICT_WORKSPACE_ALLOWLIST`**

| Value | Behavior |
|---|---|
| `STRICT_WORKSPACE_ALLOWLIST=true` | Cookie REQUIRED. Missing cookie → 403 on every workspace-scoped `/api/*` call. |
| unset / `false` (default) | Permissive fallback — missing cookie treated as "any valid workspace allowed". For backward compat during rollout. |

**Critical for backend:** if `/auth/login` does NOT set `allowed_workspaces`, every workspace-scoped API call breaks the moment FE flips `STRICT_WORKSPACE_ALLOWLIST=true`. Set the cookie unconditionally on login success, even if the user has access to only one workspace.

---

## 2c. Workspace Data Isolation (Gap #47)

3-tier workspace topology. Backend MUST enforce row-level isolation in addition to the header-validation step in §2b.

### Tier model

```
┌─ Holding (parent)            workspace.tier='holding'
│  │                           workspace.is_holding=true
│  │                           workspace.member_ids=[ws-dealls-001, ws-kantorku-001]
│  │
│  ├─ Dealls (child)           workspace.tier='operating', parent_id=<holding>
│  └─ KantorKu (child)         workspace.tier='operating', parent_id=<holding>
```

| Role accessing | Own workspace | Sibling workspace | Parent (Holding) | Child (operating) |
|---|---|---|---|---|
| operating user | READ + WRITE | DENY (403) | DENY (403) | n/a |
| holding user | n/a | n/a | READ + WRITE on holding-scoped tables | **READ-ONLY** on child tables |

> Holding users see aggregated child data via `GET /master-data/clients?holding=true` (see `03-master-data/04-api-endpoints.md` §8). Mutations against any `workspace_id` in `holding.member_ids` MUST 403.

### Query-layer enforcement

Every `SELECT`/`UPDATE`/`DELETE` against a workspace-scoped table MUST include `WHERE workspace_id = $session_workspace_id` (or `WHERE workspace_id = ANY($holding_member_ids)` for holding READ).

```sql
-- Operating user reading own workspace
SELECT * FROM master_data
 WHERE workspace_id = $1                  -- $1 = session.workspace_id, NEVER from query/body
   AND id = $2;

-- Holding user reading aggregated children (READ-ONLY path)
SELECT * FROM master_data
 WHERE workspace_id = ANY($1::uuid[])     -- $1 = holding.member_ids
   AND ($2::uuid IS NULL OR id = $2);

-- Any UPDATE/DELETE: reject if session.tier='holding' AND target.workspace_id IN member_ids
```

The `workspace_id` clause is **non-optional**. Code review MUST reject any query against `master_data`, `action_logs`, `custom_field_definitions`, `clients`, `import_quarantine`, etc. that omits it.

### Backend MUST

1. **Include `workspace_id` in every JWT/session claim** (already present in §4 access-token claims as `workspace_ids[]`); on each request resolve the **active** workspace from the validated `X-Workspace-ID` header (§2b) and bind it to a request-scoped variable used by every query.
2. **Validate on every API call** — no endpoint may execute a workspace-scoped query without the active workspace bound.
3. **Block cross-workspace ATTEMPTS at the query layer** — if a handler is asked for `master_data.id=X` and that row's `workspace_id` doesn't match the bound active workspace, return 403 (NOT 404 — see §2b rule 4) and emit an audit row (below).

### Backend MUST NOT

- Trust `workspace_id` from query params, request body, path segments, or any header other than `X-Workspace-ID` after it has passed §2b re-validation.
- Allow holding-tier sessions to call any mutating endpoint (`POST`/`PUT`/`PATCH`/`DELETE`) on child-workspace data. Reject with 403 `holding_role_read_only`.

### Audit log enrichment: `audit_logs.workspace_access`

Every workspace-scope decision (allow OR deny) is logged. Schema addition:

```sql
ALTER TABLE audit_logs
  ADD COLUMN workspace_access JSONB;
-- Shape:
-- {
--   "actor_id":               "uuid",       -- session.user_id
--   "actor_workspace_id":     "uuid",       -- bound active workspace
--   "actor_tier":             "operating" | "holding",
--   "attempted_workspace_id": "uuid",       -- resolved from row.workspace_id or X-Workspace-ID
--   "allowed":                true | false,
--   "denied_reason":          "cross_workspace" | "holding_role_read_only"
--                           | "not_in_member_ids" | "header_session_mismatch"
--                           | null,
--   "endpoint":               "GET /master-data/clients/{id}",
--   "request_id":             "uuid"
-- }

CREATE INDEX idx_audit_workspace_access_denied
  ON audit_logs ((workspace_access->>'allowed'))
  WHERE (workspace_access->>'allowed') = 'false';
```

Cross-ref `08-activity-log` for the audit pipeline and retention.

---

## 2d. Role-Specific Escalation Severity (Gap #50)

When a workflow stalls past its acknowledgement SLA, the escalation severity, notification channel, and re-escalation cadence are role-specific. Backend cron `triggerEscalationReminder` consults the matrix below.

### Severity matrix

| Role | Severity | Notification channel | Ack SLA | On miss |
|---|---|---|---|---|
| **SDR** | LOW–MEDIUM | Telegram **batch** (digest, hourly) + dashboard badge | 24h | escalate to BD Lead |
| **BD** | MEDIUM–HIGH | Telegram **individual** DM + email + dashboard | 4h | escalate to BD Lead |
| **AE** | HIGH–CRITICAL | Telegram individual DM + email + dashboard | 1h | escalate to AE Lead |
| **Finance** | CRITICAL | Telegram individual DM + email + dashboard + SMS-eligible | 30min | escalate to Finance Lead + CFO cc |
| **Lead** (any) | escalated | Telegram individual + email + dashboard + on-call page | 15min | escalate to CEO/COO cc |

### Config source

The matrix is **not hard-coded**. Cron reads it from `system_config` so Ops can tune SLAs without redeploy:

```sql
INSERT INTO system_config (key, value) VALUES (
  'ESCALATION_SEVERITY_TIERS',
  '{
    "SDR":     { "severity": "LOW-MEDIUM",    "channels": ["telegram_batch", "dashboard"],                          "ack_sla_minutes": 1440, "escalate_to": "BD_LEAD" },
    "BD":      { "severity": "MEDIUM-HIGH",   "channels": ["telegram_individual", "email", "dashboard"],            "ack_sla_minutes": 240,  "escalate_to": "BD_LEAD" },
    "AE":      { "severity": "HIGH-CRITICAL", "channels": ["telegram_individual", "email", "dashboard"],            "ack_sla_minutes": 60,   "escalate_to": "AE_LEAD" },
    "FINANCE": { "severity": "CRITICAL",      "channels": ["telegram_individual", "email", "dashboard", "sms"],     "ack_sla_minutes": 30,   "escalate_to": "FINANCE_LEAD" },
    "LEAD":    { "severity": "ESCALATED",     "channels": ["telegram_individual", "email", "dashboard", "oncall"],  "ack_sla_minutes": 15,   "escalate_to": "EXEC" }
  }'::jsonb
);
```

### Cron behaviour: `triggerEscalationReminder`

```
Schedule: every 5 minutes (per workspace)
Steps:
  1. Load ESCALATION_SEVERITY_TIERS from system_config (cache 60s).
  2. SELECT pending escalations WHERE acknowledged = FALSE
       AND created_at + (tiers->role->>'ack_sla_minutes')::int * interval '1 minute' < NOW().
  3. For each row:
     a. Route notification per tiers[role].channels (resolve channel-specific
        recipient: telegram_id from users, email, sms_number).
     b. Bump escalation.attempt_count, set last_notified_at = NOW().
     c. If attempt_count >= 3 → bump role to tiers[role].escalate_to, reset
        ack_sla clock.
  4. Emit audit_log entry per notification dispatched.
```

### Backend MUST

- **Route notifications per channel** — `telegram_batch` aggregates into a 1/hour digest message; `telegram_individual` sends immediately. Channel dispatcher implementations live in `05-messaging`.
- **Escalate to Lead after T+ack_sla unacknowledged** — bump role per matrix, do NOT delete the original escalation row (keep audit trail).
- **Enforce idempotency** — same `(escalation_id, attempt_count)` MUST NOT dispatch twice; use a sent_notifications dedup table keyed `(escalation_id, attempt, channel)`.

### Cross-references

- `06-workflow-engine/05-cron-engine.md` — Agent F2 documents Gap #17 (AE Escalation SLA cron) which uses this matrix.
- `05-messaging` — channel dispatcher contracts (Telegram batch vs individual, SMS gateway).
- `08-activity-log` — audit pipeline for escalation events.

---

## 3. CSRF Protection

### Current Mitigations
```
1. sameSite: lax          → browser tidak kirim cookie untuk cross-origin POST
2. JSON-only endpoints    → Content-Type: application/json required
                          → HTML form POST tidak bisa set JSON content type
                          → CORS preflight akan block cross-origin JSON POST
3. No cookie-based GET    → state-changing operations hanya via POST/PUT/DELETE
   mutations
```

### Recommended: Double Submit Cookie (optional, extra layer)
```
1. Server generate CSRF token, set di cookie:
     csrf_token (NOT httpOnly, sameSite=strict)
2. Client baca token dari cookie, kirim di header:
     X-CSRF-Token: {token}
3. Server verify: header value === cookie value
4. Attacker tidak bisa baca cookie cross-origin → tidak bisa set header
```

---

## 4. Token Rotation & Lifecycle

### Access Token
```
Type:       JWT (HS256)
Lifetime:   15 minutes (recommended) — currently backend-defined
Stored:     httpOnly cookie (auth_session)
Validation: Verify signature + check expiry on every API request

Claims:
  sub:          user ID
  email:        user email
  is_admin:     boolean
  workspace_ids: [uuid, uuid, ...]  — workspaces user has access to
  iat:          issued at
  exp:          expiration
```

### Refresh Token
```
Type:       Opaque random string (not JWT)
Lifetime:   30 days
Stored:     Database (hashed with SHA-256)
Rotation:   One-time use — issue new refresh token on each refresh

Flow:
  access_token expired
    → POST /auth/refresh { refresh_token }
    → verify hash, check not revoked, check not expired
    → issue new access_token + new refresh_token
    → mark old refresh_token as rotated
    → set new access_token in cookie

Theft detection:
  If rotated refresh_token is used again:
    → family_id = compromised
    → REVOKE ALL tokens in family_id
    → force re-login
    → log security event
```

### Session Token Format (general contract)

ALL session tokens issued by the FE today (and to be issued by backend later) follow ONE of two shapes. The FE function `verifySessionToken(token)` in `app/api/_lib/auth-guard.ts` accepts both.

**Shape A — Signed (preferred):**
```
Format:     "<scheme>.<subject>.<issuedAt>.<sigHex>"
            └─ scheme:    arbitrary string label (e.g., "google", "email")
            └─ subject:   user identifier (e.g., google_sub, email)
            └─ issuedAt:  epoch ms, integer
            └─ sigHex:    HMAC-SHA-256 of "<scheme>.<subject>.<issuedAt>" keyed
                          with SESSION_SECRET, hex-encoded (full 64 chars / 256-bit)
Signature:  HMAC-SHA256(SESSION_SECRET, "<scheme>.<subject>.<issuedAt>")
Lifetime:   30 days
Validation: Re-derive HMAC over the first 3 parts; constant-time compare with sigHex
            (hmac.Equal in Go, crypto.timingSafeEqual in Node).
            Reject if sig length != 64, bytes don't match, or issuedAt outside
            the freshness window.
```

Example (Google login): `"google.110782345678901234.1735689600000.<64-hex-sig>"`

**Shape B — Opaque (back-compat fallback):**
```
Format:     any string with NO "." separator
Validation: accepted as-is (scheme = "opaque", subject = "")
```

This path exists for the current external `ms-auth-proxy` token, which is opaque. Once backend takes over `/auth/login`, prefer Shape A — but FE will keep accepting opaque tokens unless explicitly removed.

**Backend implication:** when backend takes over `/auth/login`, it must EITHER (a) issue tokens in Shape A using the same `SESSION_SECRET`, OR (b) keep issuing opaque tokens. If backend invents a third shape, the FE `verifySessionToken()` parser must be updated in lockstep — coordinate the deploy.

> **Historical note**: an earlier version truncated the signature via `.slice(0, 16)` (64-bit, brute-forceable). That weakness has been fixed; backend validator must require the full-length signature.

---

## 5. Session Expiry & Cleanup

### Client-Side Handling
```
┌─ API returns 401
│
├─ handleSessionExpired() is called
│  │
│  ├─ Guard: if already redirecting → skip (prevent loops)
│  ├─ Clear localStorage: auth_user, auth_token, active_workspace
│  ├─ Fire-and-forget: POST /api/auth/logout (clear cookie server-side)
│  └─ Redirect to: /auth/login?expired=1&redirect={currentPath}
│
└─ Login page shows toast: "Sesi Anda telah berakhir"
```

### Server-Side Cleanup (Cron)
```sql
-- Run daily at 03:00 WIB

-- 1. Remove expired sessions
DELETE FROM sessions
WHERE expires_at < NOW()
   OR (revoked = TRUE AND revoked_at < NOW() - INTERVAL '7 days');

-- 2. Remove expired/revoked refresh tokens
DELETE FROM refresh_tokens
WHERE expires_at < NOW()
   OR (revoked = TRUE AND created_at < NOW() - INTERVAL '7 days');

-- 3. Clean old login attempts (keep 90 days for audit)
DELETE FROM login_attempts
WHERE created_at < NOW() - INTERVAL '90 days';
```

---

## 6. Whitelist Security

### Public Endpoint Risks & Mitigations
```
GET /api/v1/whitelist is PUBLIC (no auth required).
This is by design — frontend needs it before login.

Risks:
  1. Email enumeration — attacker can see all whitelisted emails
  2. DDoS — public endpoint could be hammered

Mitigations:
  1. Rate limit: 30 req/min per IP
  2. Consider returning only boolean check:
     GET /api/v1/whitelist/check?email=foo@bar.com → { "allowed": true }
     instead of full list (prevents enumeration)
  3. If full list is needed (admin), require auth token
  4. Cache whitelist in Redis (TTL 5 min) to avoid DB hits
```

### Recommended: Split into Two Endpoints
```
GET /api/v1/whitelist/check?email={email}   → PUBLIC, returns { allowed: bool }
GET /api/v1/whitelist                       → ADMIN ONLY, returns full list
POST /api/v1/whitelist                      → ADMIN ONLY, add entry
DELETE /api/v1/whitelist/{id}               → ADMIN ONLY, remove entry
```

---

## 7. Input Validation

### Login
```
email:    required, valid email format, max 255 chars, lowercase normalize
password: required, min 1 char (length policy on registration, not login)
```

### Google Credential
```
credential: required, string, must be valid JWT format
Verification: ALWAYS server-side via Google tokeninfo endpoint
NEVER trust client-side token without server verification
```

### Security Headers (Backend)
```
Strict-Transport-Security: max-age=31536000; includeSubDomains
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 0  (legacy, CSP replaces this)
Content-Security-Policy: default-src 'self'; script-src 'self' https://accounts.google.com
Referrer-Policy: strict-origin-when-cross-origin
```

---

## 8. Audit Trail

### What to Log
```
login_attempts table tracks:
  - Every login attempt (success + failure)
  - IP address + user agent
  - Provider (email/google)
  - Failure reason (for failed attempts)

Monitor for:
  - Multiple failed logins from same IP → possible brute force
  - Multiple failed logins for same email → possible credential stuffing
  - Login from new IP/device → optional: send notification email
  - Successful login after many failures → possible compromise
```

### Alerting Thresholds
```
WARN:  > 10 failed logins from same IP in 5 minutes
ALERT: > 50 failed logins from same IP in 15 minutes
ALERT: > 5 failed logins for same email in 5 minutes
INFO:  Login from new IP for user (first time seen)
```
