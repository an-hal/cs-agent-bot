# Workspace Integrations

Per-workspace third-party credentials for HaloAI (WhatsApp), Telegram,
Paper.id, SMTP. Stored encrypted at rest (AES-256-GCM) when
`CONFIG_ENCRYPTION_KEY` is set.

## Supported providers

| Provider slug | Purpose |
|---|---|
| `haloai` | WhatsApp send/receive via HaloAI |
| `telegram` | Telegram bot for escalation alerts |
| `paper_id` | Paper.id invoice payment webhooks |
| `smtp` | Outbound email delivery |

## Endpoints

### List all integrations for this workspace
```
GET /integrations
```

### Get one integration (secrets redacted)
```
GET /integrations/{provider}
```

Response:
```json
{
  "data": {
    "provider": "haloai",
    "display_name": "HaloAI Production",
    "config": {
      "api_url": "https://halo.example",
      "business_id": "biz-123",
      "wa_api_token": "***REDACTED***"
    },
    "is_active": true,
    "updated_at": "..."
  }
}
```

**Any key containing `token|secret|password|api_key|key` is redacted on
read.** FE gets a marker string `"***REDACTED***"` — don't send it back.

### Upsert integration
```
PUT /integrations/{provider}
{
  "display_name": "HaloAI Production",
  "is_active": true,
  "config": {
    "api_url": "https://halo.example",
    "business_id": "biz-123",
    "channel_id": "ch-456",
    "wa_api_token": "REAL_TOKEN_HERE"
  }
}
```

**Rules:**
1. `provider` in URL must match an enum value above.
2. Any key in `config` matching the secret-key pattern is encrypted before
   write (if vault configured).
3. If you send `"***REDACTED***"` for a secret value, BE returns 422
   `VALIDATION_ERROR`. Either send the real new secret OR omit the key.
4. Omitting a config key keeps the existing stored value (no DB touch).

### Delete integration
```
DELETE /integrations/{provider}
```

### Rotate credentials with approval gate
```
POST /integrations/{provider}/rotate    # (not a real route; use the approval flow)
```
Actually done via:
```
POST /approvals                          # create integration_key_change approval
# …then the other user approves:
POST /approvals/{id}/apply               # dispatcher applies the config
```
See [03-approvals.md](03-approvals.md). The approval `request_type` is
`integration_key_change`; payload carries the new config.

## FE UX notes

**Initial state** — before any integration is configured, BE uses global env
var defaults (HaloAI token from `WA_API_TOKEN`, Telegram from `TELEGRAM_BOT_TOKEN`,
SMTP from env). FE can detect this by listing integrations — an empty list
means "falling back to env".

**Edit form** — populate inputs from `GET /integrations/{provider}`, but
**show secret inputs as empty placeholders**, not the redacted marker.
Label them "(leave empty to keep existing)". On submit, only include secret
keys the user actually typed a new value for.

**Mock awareness** — when the integration has no config AND the relevant
env var is also empty (or `MOCK_EXTERNAL_APIS=true`), BE auto-mocks. FE
can show a subtle "mock mode" pill using the mock outbox as signal.
See [../../04-mock-mode.md](../../04-mock-mode.md).

## Example config shapes per provider

**haloai:**
```json
{
  "api_url": "https://halo.example",
  "business_id": "biz-xxx",
  "channel_id": "ch-yyy",
  "wa_api_token": "xxxxxx",
  "webhook_secret": "xxxxxx"
}
```

**telegram:**
```json
{
  "bot_token": "xxx:yyy",
  "default_chat_id": "-100xxx"
}
```

**paper_id:**
```json
{
  "api_key": "xxxxxx",
  "webhook_secret": "xxxxxx",
  "callback_url": "https://.../webhook/paperid/{ws}"
}
```

**smtp:**
```json
{
  "host": "smtp.sendgrid.net",
  "port": 587,
  "username": "apikey",
  "password": "xxxxxx",
  "from_email": "noreply@kantorku.id",
  "use_tls": true
}
```
