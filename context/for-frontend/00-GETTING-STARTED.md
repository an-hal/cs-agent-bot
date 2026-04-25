# Getting Started

## Run BE locally

```bash
cd /Users/macbook/Project/dealls/cs-agent-bot
cp .env.example .env    # edit as needed — mock mode defaults on
make migrate-up         # apply ±160 migrations
make dev                # swag gen + build + run on port 8080
```

Default port: `:8080`. Route prefix: `/v1/cs-agent-bot` (configurable via
`APP_ROUTE_PREFIX`). For Postman the `{{base_url}}` variable is set to
`https://localhost:8003` — change to `http://localhost:8080` if using the
default `make dev`.

## Mock mode (default for dev)

Every external integration falls back to a realistic mock when its credential
env var is empty OR when `MOCK_EXTERNAL_APIS=true` is set. Ship-ready defaults
for FE development:

```bash
MOCK_EXTERNAL_APIS=true         # default in dev (set false in prod)
# Leave these empty → all 4 integrations auto-mock:
ANTHROPIC_API_KEY=
FIREFLIES_API_KEY=
WA_API_TOKEN=
SMTP_HOST=
```

Mock endpoints return realistic data (see [04-mock-mode.md](04-mock-mode.md)).
All mock sends also record to an in-memory outbox viewable at
`GET /mock/outbox?provider=claude|fireflies|haloai|smtp`.

## Auth scheme

Three auth layers based on route:

| Layer | Header | When |
|---|---|---|
| JWT (Bearer) | `Authorization: Bearer {jwt}` | All dashboard routes |
| Workspace scope | `X-Workspace-ID: {uuid}` | Dashboard routes that touch workspace data |
| HMAC | `X-Signature: {hex}` | Webhook callers (Paper.id, Fireflies, handoff, payment) |
| OIDC | `Authorization: Bearer {oidc_jwt}` | GCP Cloud Scheduler cron routes |

Get a JWT via `POST /auth/login` or `POST /auth/google`. Example response:

```json
{
  "status": "success",
  "data": {
    "token": "eyJhbGciOi...",
    "user": {"email": "ae@kantorku.id", "workspace_ids": ["ws-1"]}
  }
}
```

Use the token as `Authorization: Bearer ...` for all subsequent requests.

## Headers checklist

For a typical dashboard call (e.g. `GET /master-data/clients`):

```
Authorization: Bearer {{jwt}}
X-Workspace-ID: {{workspace_id}}
Content-Type: application/json    # for PUT/POST
Accept: application/json
```

Missing `X-Workspace-ID` on a workspace-scoped route returns `400 BAD_REQUEST`:
```json
{"status": "failed", "error_code": "BAD_REQUEST",
 "message": "X-Workspace-ID header required"}
```

## Postman collection

Location: `docs/postman/cs-agent-bot.postman_collection.json` (29 folders, 229 requests).

1. Import `cs-agent-bot.postman_collection.json`
2. Import `cs-agent-bot.postman_environment.json`
3. Select the environment in Postman dropdown
4. Override `{{jwt}}` after login (use `POST /auth/login` response)
5. Override `{{workspace_id}}` after workspace switch

## First calls (smoke test)

```
# 1. Health
GET {{base_url}}/readiness  → 200 OK

# 2. Login (public)
POST {{base_url}}/v1/cs-agent-bot/auth/login
     {"email": "...", "password": "..."}

# 3. List workspaces (JWT only, no X-Workspace-ID yet)
GET  {{base_url}}/v1/cs-agent-bot/workspaces
     Authorization: Bearer {{jwt}}

# 3-alt. Dev shortcut (only when ENV=development AND JWT_DEV_BYPASS_ENABLED=true)
GET  {{base_url}}/v1/cs-agent-bot/workspaces
     Authorization: Bearer DEV.your.email@dealls.com

# 4. Get dashboard stats for a workspace
GET  {{base_url}}/v1/cs-agent-bot/dashboard/stats
     Authorization: Bearer {{jwt}}
     X-Workspace-ID: {{workspace_id}}

# 5. Trigger mock Claude extraction
POST {{base_url}}/v1/cs-agent-bot/mock/claude/extract
     Authorization: Bearer {{jwt}}
     {"transcript_text": "CFO: budget 150jt, urgent Q2...", "hints": {}}

# 6. View what mocks have "sent"
GET  {{base_url}}/v1/cs-agent-bot/mock/outbox?limit=50
```

## Env variable reference (highlights)

| Var | Default | Purpose |
|---|---|---|
| `APP_PORT` | `8080` | HTTP port |
| `APP_ROUTE_PREFIX` | `/v1/cs-agent-bot` | All routes prefixed |
| `APP_URL` | — | Required for OIDC verification in non-dev |
| `DB_*` | — | PostgreSQL |
| `REDIS_*` | — | Redis (optional — analytics cache degrades gracefully) |
| `JWT_VALIDATE_URL` | `https://api.sejutacita.id/v1/login/self` | External JWT validator endpoint |
| `JWT_DEV_BYPASS_ENABLED` | `false` | Local-only: accept `Bearer DEV.<email>` tokens. Active **only** when `ENV=development` or `local`. See [01-auth-and-errors.md § Dev-mode bypass](01-auth-and-errors.md#dev-mode-bypass-local-development-only). |
| `MOCK_EXTERNAL_APIS` | `true` | Auto-mock all 4 external integrations |
| `ANTHROPIC_API_KEY` | — | Claude; empty → mock |
| `FIREFLIES_API_KEY` | — | Fireflies; empty → mock |
| `WA_API_TOKEN` | — | HaloAI outbound; empty → mock |
| `SMTP_HOST` | — | SMTP; empty → mock |
| `CONFIG_ENCRYPTION_KEY` | — | AES-256 for workspace_integrations secrets; empty → plaintext |
| `TELEGRAM_BOT_TOKEN` | — | Real Telegram (no mock) |
| `CRON_ENABLED` | `true` | Toggle scheduled triggers |

Full list in `.env.example` + [03-integration-state.md](03-integration-state.md).
