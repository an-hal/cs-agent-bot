# Fireflies Integration

## Overview

Fireflies fires a webhook to our backend after a meeting ends and the transcript is ready (typically 5–15 minutes after call end). The webhook receiver verifies signature, fetches the full transcript via GraphQL (the webhook payload only contains meta), hands off to the Claude extraction pipeline (`06-claude-extraction-pipeline.md`), creates / updates the `prospects` row, notifies the BD owner via Telegram, and schedules D0 send within 2 hours.

```
Fireflies → POST /webhook/fireflies
  → verify HMAC
  → fetch transcript (GraphQL)
  → INSERT fireflies_transcripts
  → POST /extraction/run (async)
    → on extraction_status='accepted' → INSERT prospects + prospect_flags
    → notify BD via Telegram
    → set prospects.status='READY_FOR_D0'
  → cron picks up READY_FOR_D0 within 30 min, schedules D0 in next bot send window
```

## Webhook Spec

### POST `/webhook/fireflies`

Public endpoint. NO auth_session cookie. Authenticated via HMAC signature header.

```
Headers:
  Content-Type: application/json
  X-Fireflies-Signature: sha256={hex_hmac}    — HMAC-SHA256 of raw body using FIREFLIES_WEBHOOK_SECRET
  X-Fireflies-Event-Id: evt_xxx                — for dedup

Body:
{
  "eventType": "transcript.completed",
  "meetingId": "01HX...",
  "title": "Discovery Call - PT Maju Jaya",
  "transcript": "...",                  // may be truncated, ALWAYS re-fetch via GraphQL
  "participants": [
    { "name": "Alice BD", "email": "alice@dealls.com" },
    { "name": "Budi Santoso", "email": "budi@majujaya.id" }
  ],
  "date": "2026-04-22T08:30:00Z",
  "duration_seconds": 1842,
  "host_email": "alice@dealls.com",
  "meeting_url": "https://app.fireflies.ai/view/01HX..."
}

Response 200: { "received": true, "event_id": "evt_xxx" }
Response 401: { "error": "Invalid signature" }
Response 409: { "error": "Event already processed", "event_id": "evt_xxx" }
```

### HMAC Verification

```go
func verifyFirefliesSignature(rawBody []byte, header string, secret string) error {
    if !strings.HasPrefix(header, "sha256=") {
        return errors.New("missing sha256 prefix")
    }
    expected := strings.TrimPrefix(header, "sha256=")
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(rawBody)
    actual := hex.EncodeToString(mac.Sum(nil))
    if !hmac.Equal([]byte(expected), []byte(actual)) {
        return errors.New("signature mismatch")
    }
    return nil
}
```

`FIREFLIES_WEBHOOK_SECRET` is a global env var (Fireflies signs with one secret, regardless of workspace). Workspace routing happens by matching `host_email` to `users.email` and looking up the user's primary workspace.

## Transcript Fetch (GraphQL)

The webhook body's `transcript` field may be truncated by Fireflies. Backend MUST re-fetch the full transcript via GraphQL.

```
Endpoint: https://api.fireflies.ai/graphql
Auth:     Authorization: Bearer {FIREFLIES_API_KEY}
```

```graphql
query GetTranscript($transcriptId: String!) {
  transcript(id: $transcriptId) {
    id
    title
    date
    duration
    participants
    sentences {
      text
      speaker_name
      start_time
    }
    summary {
      overview
      action_items
      keywords
    }
  }
}
```

Backend assembles `sentences[]` into a single text body (Speaker-prefixed) before passing to the Claude pipeline. Cache the assembled text in `fireflies_transcripts` keyed by `meeting_id` so that re-extracts don't re-hit Fireflies API.

## Pipeline

```
1. POST /webhook/fireflies arrives
2. Verify HMAC + dedup by X-Fireflies-Event-Id
3. INSERT fireflies_webhooks (status='received')
4. Lookup workspace_id by host_email → users.workspace_id
   - If no match → mark webhook status='unmatched_host', alert BD Lead, return 200
5. Fetch transcript via GraphQL → INSERT fireflies_transcripts
6. POST /extraction/run (internal call, async via job queue)
7. On extraction completion (worker callback):
   a. If extraction_status='accepted':
      - INSERT prospects (or UPDATE if exists by company+phone match)
      - INSERT prospect_flags from BANTS sub-scores
      - Notify BD owner via Telegram with link to Prospect Detail Drawer
      - Set prospects.status='READY_FOR_D0', prospects.d0_scheduled_after=NOW()+2h
   b. If extraction_status IN ('review_pending', 'low_confidence', 'incomplete'):
      - Notify BD owner via Telegram with "Review required" + drawer link
      - Set prospects.status='AWAITING_BD_REVIEW'
      - DO NOT schedule D0
8. UPDATE fireflies_webhooks SET status='processed', processed_at=NOW()
```

## Error Handling

| Failure | Backend MUST |
|---|---|
| HMAC mismatch | Return 401, do not persist |
| Event already processed (dedup) | Return 409, no-op |
| `host_email` not in `users` table | Persist webhook with `status='unmatched_host'`, Telegram-alert BD Lead, return 200 |
| Fireflies GraphQL 4xx/5xx | Retry 3x exponential (2s, 8s, 32s); on final failure → mark `status='fetch_failed'`, alert BD Lead |
| Extraction returns `incomplete` (transcript <500 words) | Delay D0 + alert BD; user can manually re-run via `POST /extraction/re-run/{prospect_id}` |
| Telegram delivery fail (network, rate limit) | Retry exponential 5x (1m, 5m, 15m, 1h, 4h); on final failure → blacklist that chat_id and alert via email |
| WA delivery fail (D0 send) | Retry exponential 5x; on final failure → blacklist phone, mark `prospects.wa_send_blocked=true`, alert BD owner |

## Re-Extract Endpoint

### POST `/extraction/re-run/{prospect_id}`

Used by the FE "Re-extract" button on the Prospect Detail Drawer. Idempotent within 5 min (prevents double-clicks); after that, creates a fresh extraction run.

```
Request: {} (empty body)

Response 202:
{
  "run_id": "uuid-zzz",
  "status": "running",
  "message": "Re-extraction queued. Check back in ~30 seconds."
}

Response 409:
{
  "error": "Re-extraction already in progress",
  "existing_run_id": "uuid-yyy"
}
```

Backend looks up the original `transcript_url` from `fireflies_transcripts` and calls `POST /extraction/run` internally with `force=true`. If no cached transcript, returns 404 with hint "Original Fireflies meeting not found — manual extraction not supported."

## DB Schema

```sql
CREATE TYPE fireflies_webhook_status AS ENUM (
  'received',
  'processed',
  'unmatched_host',
  'fetch_failed',
  'duplicate'
);

CREATE TABLE fireflies_webhooks (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  event_id        VARCHAR(100) NOT NULL UNIQUE,    -- X-Fireflies-Event-Id
  meeting_id      VARCHAR(100) NOT NULL,
  workspace_id    UUID REFERENCES workspaces(id),  -- nullable when unmatched_host
  host_email      VARCHAR(255) NOT NULL,
  received_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  processed_at    TIMESTAMPTZ,
  status          fireflies_webhook_status NOT NULL DEFAULT 'received',
  raw_payload     JSONB NOT NULL,
  error_text      TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_fw_workspace ON fireflies_webhooks(workspace_id, received_at DESC);
CREATE INDEX idx_fw_meeting ON fireflies_webhooks(meeting_id);
CREATE INDEX idx_fw_status ON fireflies_webhooks(status) WHERE status != 'processed';

CREATE TABLE fireflies_transcripts (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    UUID NOT NULL REFERENCES workspaces(id),
  meeting_id      VARCHAR(100) NOT NULL UNIQUE,
  title           VARCHAR(500),
  meeting_url     TEXT NOT NULL,
  meeting_date    TIMESTAMPTZ NOT NULL,
  duration_seconds INTEGER,
  participants    JSONB NOT NULL,                  -- array of {name, email}
  transcript_text TEXT NOT NULL,                   -- assembled from sentences[]
  word_count      INTEGER NOT NULL,
  fetched_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  raw_graphql     JSONB                            -- full GraphQL response for debug
);

CREATE INDEX idx_ft_workspace ON fireflies_transcripts(workspace_id, meeting_date DESC);
```

## D0 Scheduler Cron

Runs every 30 minutes. Picks up prospects ready for D0 outreach.

```sql
SELECT id, workspace_id, bd_owner_email, d0_scheduled_after
FROM prospects
WHERE status = 'READY_FOR_D0'
  AND d0_scheduled_after <= NOW()
  AND wa_send_blocked = FALSE
ORDER BY d0_scheduled_after ASC
LIMIT 100;
```

For each row, the scheduler asks `isWorkingDay(NOW(), workspace_id)` (see `10-working-day-timezone.md`). If true and within send window → enqueue WA send. If false → defer to next sendable time. After successful enqueue → `UPDATE prospects SET status='D0_QUEUED'`.

## Backend MUST

- Verify HMAC signature on EVERY webhook request before reading body
- Dedup by `X-Fireflies-Event-Id` (treat duplicates as 409, no side-effects)
- Re-fetch transcript via GraphQL (do not trust webhook body's `transcript` field)
- Cache transcript in `fireflies_transcripts` to avoid Fireflies API rate limits on re-runs
- Notify BD owner via Telegram within 60s of extraction completion (success or review-required)
- Block D0 send if `extraction_status != 'accepted'`

## Backend MAY

- Skip GraphQL fetch if webhook body's `transcript` length > 5000 chars and word count > 500 (best-effort latency optimisation)
- Auto-merge into existing `prospects` row when fuzzy match on `(company_name, dm_email_or_phone)` exceeds 0.85 confidence
- Ship a `POST /webhook/fireflies/replay/{event_id}` admin endpoint for re-processing failed webhooks (Lead role only)
