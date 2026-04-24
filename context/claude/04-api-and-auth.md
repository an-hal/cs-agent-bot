# API & Authentication

## HTTP Server

- Framework: **none** — custom router built on `net/http` (`internal/delivery/http/router`).
- Port: `APP_PORT` (default `8080`).
- Base path: `APP_ROUTE_PREFIX` (e.g. `/v1/cs-agent-bot`).
- All routes registered in `internal/delivery/http/route.go`.

## Middleware Stack

Applied globally (outer → inner):

1. `TracingMiddleware` — OpenTelemetry spans
2. `RecoveryMiddleware` — panic → 500 JSON response
3. `RequestIDMiddleware` — unique request ID header
4. `LoggingMiddleware` — structured JSON log per request
5. `ErrorHandlingMiddleware` — exception → JSON error response

Per-route auth middleware (one of):

- `OIDCAuthMiddleware` — validates GCP Cloud Scheduler OIDC JWT (uses `SCHEDULER_SA_EMAIL` + `APP_URL` audience).
- `JWTAuthMiddleware` — validates dashboard user JWT via external `JWT_VALIDATE_URL`.
- `HaloAISignatureMiddleware` — HMAC signature on `/webhook/wa` using `WA_WEBHOOK_SECRET`.
- `HMACAuthMiddleware` — HMAC on handoff + Paper.id webhooks.

Dashboard routes additionally run `WorkspaceIDMiddleware` — extracts `workspace_id` header and scopes repository calls.

## Auth by Route

| Route | Auth |
|---|---|
| `GET /cron/run` | OIDC JWT (GCP Cloud Scheduler SA) |
| `POST /webhook/wa` | HaloAI HMAC signature |
| `POST /handoff/new-client` | HMAC |
| `POST /payment/verify` | HMAC (Paper.id) |
| `/data-master/*`, `/jobs`, `/activity-logs`, `/invoices`, dashboard | JWT + `workspace_id` header |
| `/auth/login`, `/auth/google` | Public |
| `/readiness`, `/liveness` | Public |

## Standardized Response Format

```json
{
  "status": "success|failed",
  "entity": "clients|invoices|...",
  "state": "getAll|create|update|...",
  "message": "Human-readable message",
  "data": { ... },
  "meta": { "page": 1, "total": 100 },
  "error_code": "NOT_FOUND|INVALID_INPUT|...",
  "errors": { "field": ["validation error"] }
}
```

Helpers live in `internal/delivery/http/response/`.

## Representative Endpoints

```
GET    /readiness                                    # Public health probe
GET    /cron/run                                     # OIDC: Trigger batch run

POST   /webhook/wa                                   # HaloAI sig: Inbound WA reply
POST   /webhook/paper-id                             # HMAC: Paper.id payment webhook

POST   /handoff/new-client                           # HMAC: Onboard new client
POST   /payment/verify                               # HMAC: Payment verification

POST   /auth/login                                   # Public: Login
POST   /auth/google                                  # Public: Google OAuth

GET    /data-master/clients                          # JWT: List clients
GET    /data-master/clients/{company_id}             # JWT: Get client detail
PUT    /data-master/clients/{company_id}             # JWT: Update client
POST   /data-master/clients                          # JWT: Create client
POST   /data-master/trigger-rules/cache/invalidate   # JWT: Invalidate rule cache

GET    /invoices                                     # JWT: List invoices
POST   /invoices/{invoice_id}/mark-paid              # JWT: AE marks invoice paid

GET    /jobs/{job_id}/download                       # JWT: Download background-job export

GET    /activity-logs                                # JWT: Unified audit feed
GET    /analytics/*                                  # JWT: Analytics dashboards + forecasts

POST   /workspaces                                   # JWT: Create workspace
GET    /workspaces/members                           # JWT: List workspace members
```

## Swagger

Generated via `make swag` (runs `swag init`). Output: `docs/swagger.json`, `docs/swagger.yaml`, `docs/docs.go`.