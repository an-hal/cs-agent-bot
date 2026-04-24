# feat/01 — Auth

Login + Google OAuth + whitelist + session revocation. Dashboard access
gated by JWT; whitelist restricts new signups.

## Status

**✅ 100%** — login, OAuth, whitelist CRUD, session revocation all live.
Audit cross-workspace access tracked via `audit_logs_workspace_access` (see
[../00-shared/05-audit.md](../00-shared/05-audit.md)).

## Endpoints

### Login
```
POST /auth/login
{"email": "user@example.com", "password": "xxx"}
→ {"data": {"token": "jwt...", "user": {"email": "...", "workspace_ids": [...]}}}
```

### Google OAuth
```
POST /auth/google
{"id_token": "google-id-token-from-fe"}
→ same shape as /auth/login
```

### Logout
```
POST /auth/logout
```
Stateless on BE. FE clears token locally.

### Whitelist

Whitelist gates new signups — only emails on the whitelist can register.

```
GET    /whitelist/check?email=x@y.z     # Public — returns whether allowed
GET    /whitelist                        # JWT — list all entries
POST   /whitelist                        # JWT — add entry
DELETE /whitelist/{id}                   # JWT — remove
```

POST body:
```json
{"email": "new@example.com"}
```

### Session revocation

See [../00-shared/12-sessions.md](../00-shared/12-sessions.md).

## Response shape

```json
{
  "status": "success",
  "data": {
    "token": "eyJ...",
    "user": {
      "email": "ae@kantorku.id",
      "name": "Budi Ae",
      "workspace_ids": ["ws-1", "ws-2"]
    }
  }
}
```

## FE UX

- Login form: email + password OR Google button
- Post-login: pick workspace if user has multiple (use `workspace_ids`), set
  `X-Workspace-ID` for subsequent requests
- Token storage: use httpOnly cookie in production; localStorage only for dev
- Expiry handling: on 401, clear token + redirect to login
- Session revocation: on 401 with `"session revoked"` message, show toast
  + redirect
