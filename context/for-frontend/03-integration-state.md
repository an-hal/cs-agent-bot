# Integration State

Snapshot 2026-04-24. For each external integration: what's wired, what's mock,
and how to swap to real. Every integration has a working path right now —
nothing is blocked.

## Decision matrix

| Integration | Real client | Mock client | Auto-behavior |
|---|---|---|---|
| Claude (Anthropic) | Interface ready, SDK call TODO | ✅ Keyword BANTS classifier | Mock when `ANTHROPIC_API_KEY=""` OR `MOCK_EXTERNAL_APIS=true` |
| Fireflies | Interface ready, GraphQL fetch TODO | ✅ 4 canned Indonesian transcripts | Mock when `FIREFLIES_API_KEY=""` OR mock flag |
| HaloAI (WhatsApp) | Existing `haloai.Client` for webhook receive; outbound send wire TODO in cron dispatcher | ✅ Records to outbox + fake wamid | Mock when `WA_API_TOKEN=""` OR mock flag |
| SMTP | ✅ Real TLS client implemented — needs host+creds | ✅ Records to outbox | Mock when `SMTP_HOST=""` OR mock flag |
| Paper.id | ✅ Full real client with HMAC webhook | N/A | Always active (workspace-scoped `paper_id_webhook_secret`) |
| Telegram | ✅ Real `telegram.Notifier` | N/A | Always active when `TELEGRAM_BOT_TOKEN` set |

## Claude (BD extraction pipeline)

**Current state:** `internal/usecase/claude_client/` has 2 impls:
- `NewClient()` — noop placeholder (returns empty result)
- `NewMockClient()` — realistic mock with:
  - Keyword-based BANTS 1-5 per dimension (budget/authority/need/timing/sentiment)
  - Overall score 0-100 + classification (hot/warm/cold)
  - Buying intent (high/medium/low)
  - 6 extracted fields (company_size, pain_point, DM role, competitor, next_step, sentiment_note)
  - Auto-generated coaching notes when sub-scores are weak
  - 400ms simulated latency

**How the bridge works:** `claude_extraction.FirefliesBridge` implements
`fireflies.Extractor` — when a transcript ingests, the bridge:
1. Creates a pending `claude_extractions` row
2. Marks prior attempts for same source as `superseded`
3. Calls the configured `Client.Extract()` (mock or real)
4. Writes result back + updates `fireflies_transcripts.extraction_status`

**To swap to real:** replace `NewClient()` impl in `client.go` with an
`anthropic-sdk-go` call. Set `ANTHROPIC_API_KEY`, flip `MOCK_EXTERNAL_APIS=false`.

## Fireflies

**Current state:** `internal/usecase/fireflies_client/` has:
- `NewClient()` — returns error "not configured"
- `NewMockClient()` — 4 deterministic canned scenarios (chosen by ID hash):
  1. Discovery — Acme Corp (250 HC, CFO, 150jt budget, urgent)
  2. Follow-up — Beta Industries (80 HC, skeptical, no decision)
  3. Demo Request — Gamma Enterprise (1000+ HC, CEO, 2M budget)
  4. Intro call — Delta Startup (50 HC, cold, tahun depan)

**Webhook receive works real today:** `POST /webhook/fireflies/{workspace_id}`
with HMAC signature stores the transcript regardless of fetch mode.

**To swap to real fetch:** implement `FetchTranscript` in `client.go` with
Fireflies GraphQL. Set `FIREFLIES_API_KEY`, flip flag off.

## HaloAI (WhatsApp Business)

**Current state:**
- Inbound (`/webhook/wa`): ✅ Real HMAC signature verification + existing
  `haloai.Client` for response messages.
- Outbound (from cron dispatcher): `usecase/haloai_mock/` adapter records
  every send to the outbox + returns realistic wamid.

**Cron dispatcher now actually "sends":** `channelDispatcher.Dispatch` with
`WithManualEnqueuer(mockEnq)` + `WASender(mockWASender)` routes:
- Manual-flow trigger IDs → `manual_action_queue` (human composes)
- Other WA channel rules → mock send (or real if wired)

**To swap to real send:** write a new adapter in `cmd/server/main.go` that
implements `cron.WASender` with the existing `haloai.Client.SendMessage()`,
and use it instead of `mockWASenderAdapter`.

## SMTP

**Current state:** `internal/usecase/smtp_client/` has BOTH impls working:
- `NewClient()` — real `net/smtp` with optional STARTTLS, MIME-wrapped body
- `NewMockClient()` — logs + records to outbox

Real client is production-ready (TLS, auth, Cc/Bcc, From override).

**To activate real:** set `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`,
`SMTP_PASSWORD`, `SMTP_FROM_ADDR`, `SMTP_USE_TLS=true`, `MOCK_EXTERNAL_APIS=false`.
No code change needed.

## Paper.id (invoice payment)

**Already real + production-ready.** `invoice.PaperIDService` at
`internal/usecase/invoice/paperid.go` does:
- HMAC-SHA256 webhook signature verification (workspace-scoped secret)
- Payment status sync — exception to Rule 3 (the only path that writes
  `payment_status = Lunas` on an invoice)
- Payment log append with raw payload preserved

Webhook endpoint: `POST /webhook/paperid/{workspace_id}`. Invoice creation
returns a `paper_id_url` + `paper_id_ref` when workspace has the secret.

## Telegram

**Already real.** `usecase/telegram/notify.go` with:
- Per-workspace or global bot token
- Escalation fan-out (AE lead ID, workspace group, etc.)
- Template-resolved messages (variables substituted from client/escalation)

Triggered by `escalation.TriggerEscalation()` which atomically:
1. Sets `BotActive=FALSE` on the client
2. Inserts `action_log` entry
3. Sends Telegram
4. Sets `escalations.status = Open`

Deduplication: repeat fires within same `(esc_id, company_id)` only send
reminder (no new row).

## Mock outbox — FE QA surface

Every mock impl records every call to a shared in-memory ring buffer
(`internal/usecase/mockoutbox/`) so FE/QA can:

```
GET    /mock/outbox?provider=claude|fireflies|haloai|smtp
GET    /mock/outbox/{id}
DELETE /mock/outbox?provider=
POST   /mock/claude/extract      # trigger mock directly
POST   /mock/fireflies/fetch
POST   /mock/haloai/send
POST   /mock/smtp/send
```

Outbox is process-local (no persistence) and bounded (200 records default).
Records across **all 4 providers** share the buffer with `provider` tag.

## Credential requirements summary

| For production deploy | Need to obtain |
|---|---|
| Claude real extraction | `ANTHROPIC_API_KEY` (Anthropic console) |
| Fireflies real fetch | `FIREFLIES_API_KEY` (Fireflies settings) |
| HaloAI real WA send | `WA_API_TOKEN` (already standard for cs-agent-bot) |
| SMTP real delivery | SMTP host/credentials (SendGrid, AWS SES, etc.) |
| Paper.id | Already configured per-workspace (`paper_id_webhook_secret`, `paper_id_secret`) |
| Telegram | Already configured (`TELEGRAM_BOT_TOKEN`, `TELEGRAM_AE_LEAD_ID`) |
| Secret vault | `CONFIG_ENCRYPTION_KEY` (32-byte base64/hex) — optional but recommended for prod |

Without any of these, the system still works — mocks auto-activate and FE
can develop against realistic data.
