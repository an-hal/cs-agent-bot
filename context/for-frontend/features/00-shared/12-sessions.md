# Session Revocation

Admin-initiated JWT jti revocation list. Revoked JTIs are rejected by the
auth middleware until their natural expiry.

## Endpoints

### Revoke a session
```
POST /sessions/revoke
{
  "jti": "jwt-jti-to-revoke",
  "user_email": "user@example.com",
  "reason": "Suspected account compromise",
  "expires_in_hours": 24              // default 24 if omitted
}
```

The `expires_in_hours` should match the original JWT's remaining lifetime
— BE uses it to compute `expires_at` so the revocation row auto-cleans up
once the JWT would've expired anyway.

### List active revocations for a user (admin)
```
GET /sessions/revoked?user_email=user@example.com&limit=50
```
Returns non-expired revocations for the user + workspace.

### Cleanup expired (cron — OIDC)
```
GET /cron/sessions/cleanup
```
Deletes rows where `expires_at <= NOW()`. Idempotent.

## FE UX

**Security admin page:**
- Per-user action button "Revoke all sessions"
  - Requires the target's current JTI (from session store or audit log)
  - Or use bulk "Revoke all" by listing + revoking each JTI
- Reason textarea (required for audit)

**Post-revocation:**
- Revoked user sees 401 on next request
- FE should surface a toast "Your session was revoked by admin" + redirect to login
- Support link with `request_id` for appeals

## Integration with auth middleware

When JWT middleware sees a token whose `jti` claim matches a non-expired
`revoked_sessions` row, it returns:
```json
{"status": "failed", "error_code": "UNAUTHORIZED",
 "message": "session revoked"}
```

FE should treat 401 identically whether from expiry or revocation — just
redirect to login.

## JTI convention

The auth service must populate a unique `jti` claim in every JWT for
revocation to work. If legacy tokens lack `jti`, they can't be revoked
(but still expire normally).
