# Mock Mode Guide

FE-facing guide to the mock-external-API layer. Lets FE develop, test, and
demo against realistic external-integration responses without real API keys.

## When mock mode is active

Per-integration, BE mocks when EITHER condition is true:
1. `MOCK_EXTERNAL_APIS=true` in env (default dev), OR
2. The integration's credential env var is empty

| Integration | Env check |
|---|---|
| Claude | `ANTHROPIC_API_KEY` empty → mock |
| Fireflies | `FIREFLIES_API_KEY` empty → mock |
| HaloAI outbound | `WA_API_TOKEN` empty → mock |
| SMTP | `SMTP_HOST` empty → mock |

Mock activation is transparent — the same real endpoints (e.g.
`POST /webhook/fireflies/{ws}`) still work; downstream processing uses the
mock client.

## The shared outbox

Every mock send records to a process-local in-memory ring buffer (200 records
default). FE can inspect to verify what *would* have been sent.

### Inspect outbox

```
GET /mock/outbox?limit=50
GET /mock/outbox?provider=claude|fireflies|haloai|smtp&limit=20
```

Response shape:
```json
{
  "status": "success",
  "data": {
    "records": [
      {
        "id": "mock-claude-1",
        "provider": "claude",
        "operation": "extract",
        "payload": {"transcript_chars": 1200, "hints": {...}},
        "response": {
          "model": "mock-claude-sonnet-4-6",
          "bants_score": 82.0,
          "bants_classification": "hot",
          "buying_intent": "high",
          "prompt_tokens": 634,
          "completion_tokens": 180
        },
        "status": "success",
        "timestamp": "2026-04-24T03:14:22Z"
      }
    ],
    "stats": {"claude": 3, "fireflies": 1, "haloai": 7, "smtp": 2}
  }
}
```

### Get a specific record
```
GET /mock/outbox/{id}
```

### Clear outbox (tests)
```
DELETE /mock/outbox                    # clear all
DELETE /mock/outbox?provider=claude    # clear one provider
```

## Trigger mocks directly (FE QA)

Useful for QA test suites — invoke mock clients without going through the
real feature flow.

### Claude BD extraction
```
POST /mock/claude/extract
{
  "transcript_text": "CFO: Saya putuskan anggaran. Budget 150jt siap. Urgent Q2 ...",
  "hints": {"meeting_title": "Discovery — Acme"}
}
```
Returns realistic BANTS (1-5 per dimension), overall score (0-100),
classification (hot/warm/cold), buying_intent (high/medium/low), 6 extracted
fields, coaching notes.

The keyword classifier looks for:
- **Budget**: `budget`, `anggaran`, `rp`, `juta`, `milyar` → +; `belum`, `nanti`, `ga ada` → –
- **Authority**: `ceo`, `cfo`, `direksi`, `saya yang putuskan`, `owner` → +; `tanya bos`, `perlu approval` → –
- **Need**: `butuh`, `harus`, `penting`, `urgent` → +; `nanti saja`, `tidak perlu` → –
- **Timing**: `minggu ini`, `q1`, `q2`, `sekarang`, `segera` → +; `tahun depan`, `bulan depan` → –
- **Sentiment**: `bagus`, `menarik`, `tertarik`, `oke` → +; `mahal`, `ribet`, `ragu` → –

### Fireflies canned transcript
```
POST /mock/fireflies/fetch
{"fireflies_id": "ff-mock-001"}
```
Returns one of 4 deterministic Indonesian discovery scenarios (chosen by
ID hash):
1. Discovery — Acme Corp (250 HC, CFO, 150jt budget, urgent)
2. Follow-up — Beta Industries (80 HC, skeptical, no decision)
3. Demo Request — Gamma Enterprise (1000+ HC, CEO, 2M budget)
4. Intro call — Delta Startup (50 HC, cold, tahun depan)

Use the same `fireflies_id` to always get the same scenario (deterministic).

### HaloAI WA send
```
POST /mock/haloai/send
{
  "workspace_id": "{{workspace_id}}",
  "to": "628123456789",
  "template_id": "REN90",
  "body": "Pak, renewal H-90...",
  "variables": {"pic_name": "Pak Budi"}
}
```
Returns `{message_id, status: "delivered", sent_at}`.

### SMTP email
```
POST /mock/smtp/send
{
  "to": ["client@example.com"],
  "subject": "Welcome",
  "body_html": "<p>Welcome aboard!</p>",
  "from_addr": "noreply@kantorku.id"
}
```
Returns `{status: "queued", message_id}`. Body is truncated at 2000 chars
in the outbox view.

## End-to-end mock flow (Fireflies → Claude)

1. FE uploads or webhook delivers Fireflies payload:
   ```
   POST /webhook/fireflies/{workspace_id}
   {"fireflies_id": "ff-xyz", "meeting_title": "...", "transcript_text": "..."}
   ```
2. BE stores transcript (`fireflies_transcripts`), returns `202` with
   `transcript_id` + `extraction_status: pending`.
3. Background goroutine: `FirefliesBridge` calls Claude mock.
4. FE polls `GET /fireflies/transcripts/{id}` — status transitions
   `pending → running → succeeded` with the BANTS results populated on
   the associated `claude_extractions` row.
5. Verify what Claude "saw" via `GET /mock/outbox?provider=claude`.

## Real vs mock: how FE distinguishes

BE doesn't expose a "which mode am I in?" flag per response. FE can infer:

- `/mock/*` endpoints always talk to the mock layer (separate from real flow).
- Real webhooks (`/webhook/fireflies/{ws}`) return the same shape whether mock
  is auto-active or not — extraction just succeeds silently.
- Claude mock model string is `mock-claude-sonnet-4-6` (vs real `claude-sonnet-4-6-20260401` etc.).
- `message_id` from mock HaloAI starts with `mock-wamid-`.
- `message_id` from mock SMTP starts with `mock-smtp-`.

FE can treat these as development signals — e.g. show a banner "Mock mode"
if outbox has recent entries.

## Postman

All 7 mock endpoints are in the **"Mock External APIs"** folder in
`cs-agent-bot.postman_collection.json` with realistic example payloads
(Indonesian discovery dialogue, CFO budget talk, WA renewal template).
