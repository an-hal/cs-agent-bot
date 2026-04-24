# Auth & Error Codes

## Auth layers by route

| Route pattern | Layer |
|---|---|
| `/auth/login`, `/auth/google`, `/auth/logout`, `/whitelist/check` | Public |
| `/whitelist/*` (except check) | JWT |
| `/cron/*` | OIDC (GCP Cloud Scheduler) — except `/cron/pdp/retention` which is JWT-scoped |
| `/webhook/wa` | HaloAI HMAC signature |
| `/webhook/paperid/{workspace_id}` | Paper.id HMAC (workspace-scoped secret) |
| `/webhook/fireflies/{workspace_id}` | Fireflies HMAC |
| `/webhook/checkin-form` | Public (form submission) |
| `/handoff/new-client`, `/payment/verify` | HMAC |
| `/workspaces*`, `/integrations*`, `/preferences*`, `/approvals*`, `/manual-actions*`, `/audit-logs/*`, `/fireflies/*`, `/reactivation/*`, `/coaching/*`, `/rejection-analysis/*`, `/pdp/*`, `/sessions/*`, `/dashboard/*`, `/analytics/*`, `/reports/*`, `/data-master/*`, `/master-data/*`, `/invoices/*`, `/templates/*`, `/workflows/*`, `/automation-rules/*`, `/collections/*`, `/team/*`, `/activity-logs/*`, `/activity-log/*`, `/jobs/*`, `/mock/*`, `/notifications/*`, `/revenue-targets*`, `/workspace/*` | **JWT + X-Workspace-ID** (most routes) |
| `/readiness` | Public |

## JWT obtain flow

```
POST /auth/login
{"email": "user@example.com", "password": "xxx"}

→ 200
{"status": "success",
 "data": {"token": "eyJ...", "user": {...}}}
```

Or Google OAuth:
```
POST /auth/google
{"id_token": "google-id-token-from-fe"}
```

Token is a standard JWT. Expiry per the external `JWT_VALIDATE_URL` service
— typically 24h. On 401, re-login.

## Workspace switch

JWT does not bind to a workspace. FE chooses one per request via
`X-Workspace-ID` header. To programmatically "switch" (persist the choice):

```
POST /workspaces/{id}/switch
```

…but FE can just send the preferred workspace_id header without this call.

## Response envelope (standard)

Every JSON response follows this shape:

```json
{
  "status": "success",              // or "failed"
  "entity": "clients",              // resource kind (optional)
  "state": "getAll",                // operation (optional)
  "message": "Human-readable",
  "data": { ... } | [ ... ],        // payload
  "meta": { "total": 100, "limit": 50, "offset": 0 },  // pagination
  "error_code": "NOT_FOUND",        // only on failure
  "errors": { "field": ["msg"] }    // field-level validation
}
```

Success responses: `status=success`, `data` present, `error_code` absent.
Failure responses: `status=failed`, `data` absent, `error_code` + `message` present.

## Error codes

| HTTP | `error_code` | Meaning | FE handling |
|---|---|---|---|
| 400 | `BAD_REQUEST` | Invalid input (malformed JSON, missing required header, bad format) | Show message as-is |
| 401 | `UNAUTHORIZED` | JWT missing/expired/invalid | Redirect to login |
| 403 | `FORBIDDEN` | JWT valid but caller lacks permission | Show "Access denied"; check role assignments |
| 404 | `NOT_FOUND` | Resource not found | Show not-found UI |
| 409 | `CONFLICT` | Dedup collision / rate-limit / state conflict | Show message (e.g. "reactivation already fired for this client in last 30 days") |
| 422 | `VALIDATION_ERROR` | Field-level validation failed | Render `errors` map per-field below inputs |
| 429 | `TOO_MANY_REQUESTS` | Rate-limited | Retry with backoff |
| 500 | `INTERNAL_ERROR` | Server error | Generic "Something went wrong"; log `request_id` |

## Request ID

Every request gets an `X-Request-ID` header in the response. Use this when
filing bugs — BE logs are indexed on it.

## Common validation errors

### Missing workspace header
```json
{"status":"failed","error_code":"BAD_REQUEST",
 "message":"X-Workspace-ID header required"}
```

### Redacted secret sent back
```json
// When PUT /integrations/{provider} with config containing "***REDACTED***"
{"status":"failed","error_code":"VALIDATION_ERROR",
 "message":"config contains redacted placeholder; send the real secret or omit the field"}
```
**FE rule:** After reading an integration config (with secrets redacted),
don't send the redacted placeholder back on PUT. Either send the real new
secret OR omit the key so the existing secret is preserved.

### Approval self-checker
```json
// When the same email tries to approve their own request
{"status":"failed","error_code":"BAD_REQUEST",
 "message":"cannot approve your own request"}
```

### Rate limit on reactivation
```json
// When manual reactivation attempted <30d after last fire for the same trigger code
{"status":"failed","error_code":"CONFLICT",
 "message":"reactivation for this client+code already fired within the last 30 days"}
```
(Manual trigger code `manual` bypasses the rate limit — see features/03).

### Template send guard
```json
// When resolving a template with unresolved [variables]
{"status":"failed","error_code":"INTERNAL_ERROR",
 "message":"unresolved variable in template TPL-REN-90"}
```

## CORS

FE origin must be allow-listed in BE config. Default dev: `http://localhost:3000`,
`http://localhost:5173`. For staging/prod, coordinate with ops for CORS config.

## Rate limits

No explicit rate-limit middleware right now — BE relies on upstream (CDN/gateway).
The cron dispatcher does enforce a 300ms sleep between outbound WA sends per
HaloAI rate-limit; this is invisible to FE.
