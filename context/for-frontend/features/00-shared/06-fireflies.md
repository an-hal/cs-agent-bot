# Fireflies Integration

Fireflies.ai meeting transcript ingestion + extraction. Webhook receives the
transcript; a background job runs Claude extraction (real or mock) and
populates BANTS scoring + discovery fields onto the target client.

## Flow

```
Fireflies meeting ends
    ↓
Fireflies sends webhook → POST /webhook/fireflies/{workspace_id}
    ↓ (HMAC-signed, idempotent by fireflies_id)
BE stores transcript → fireflies_transcripts table
    ↓ (202 returned with {transcript_id, extraction_status: "pending"})
Background goroutine: FirefliesBridge → Claude extraction
    ↓
claude_extractions row updated with BANTS + fields
    ↓
fireflies_transcripts.extraction_status = "succeeded"
```

## Endpoints

### Webhook receiver (Fireflies → BE)
```
POST /webhook/fireflies/{workspace_id}
Headers: X-Signature: {hmac_hex}
Body: {
  "fireflies_id": "ff-xyz",
  "meeting_title": "Discovery — Acme",
  "meeting_date": "2026-04-24T10:00:00Z",
  "duration_seconds": 1800,
  "host_email": "bd@kantorku.id",
  "participants": ["bd@kantorku.id", "cfo@acme.co.id"],
  "transcript_text": "full transcript...",
  "raw": { /* passthrough */ }
}
```
- Idempotent — duplicate `fireflies_id` returns existing record.
- Returns 202 with `{transcript_id, extraction_status}`.

### List transcripts (dashboard)
```
GET /fireflies/transcripts?status=succeeded&limit=50
```

### Get one
```
GET /fireflies/transcripts/{id}
```
Returns full transcript + extraction status + linked `master_data_id`.

## Extraction status transitions

```
pending → running → succeeded | failed
              ↓
          (on retry: new claude_extractions row, old one marked superseded)
```

FE should poll after `POST /webhook/fireflies/*` if it needs to display
BANTS results. Typical real extraction: 2-5s; mock: 400ms.

## Mock mode

When `FIREFLIES_API_KEY` is empty OR `MOCK_EXTERNAL_APIS=true`, fetch
returns one of 4 canned Indonesian discovery scenarios (deterministic by
fireflies_id hash). See [../../04-mock-mode.md](../../04-mock-mode.md).

## FE UX

**Inbox view:** list transcripts newest-first with extraction_status
filter. Show status pill (pending/running/succeeded/failed).

**Detail view:** transcript on left, Claude extraction results on right
(fetch via `claude_extractions` linked by `source_id = fireflies_id`).

**Link to client:** when extraction `master_data_id` populates, auto-open
the client record with a "BD extraction ready" badge.
