# Claude Extraction Pipeline

BD intelligence: extract structured fields + BANTS scoring from unstructured
sources (Fireflies transcripts, manual notes, emails).

## Lifecycle

Extractions run via the `FirefliesBridge` adapter (transcript → Claude →
store). No direct HTTP API to trigger real extractions — they're started
automatically by ingesting a transcript. FE can trigger via the mock endpoint
for testing.

## Data shape

Each attempt gets its own `claude_extractions` row; retries mark prior rows
as `superseded` so the timeline is auditable.

See [../../05-data-models.md#claudeextraction](../../05-data-models.md) for
full shape.

## Endpoints

### View linked extractions for a client
```
# Not currently exposed as a separate endpoint; client record includes
# the most recent extraction inline via custom_fields (ae_bants_* after handoff).
# To retrieve full history:
GET /fireflies/transcripts?status=succeeded    # transcripts for this workspace
# Then FE can cross-reference by master_data_id
```

If a dedicated `/claude-extractions/*` endpoint is needed, ping BE team —
interface is already wired internally.

### Trigger mock extraction (FE QA)
```
POST /mock/claude/extract
{"transcript_text": "...", "hints": {"meeting_title": "Discovery — Acme"}}
```
See [../../04-mock-mode.md](../../04-mock-mode.md).

## BANTS classification rules

| Classification | Score range |
|---|---|
| `hot` | ≥ 75 |
| `warm` | 50–74 |
| `cold` | < 50 |

Each sub-score (budget/authority/need/timing/sentiment) is 1-5. Overall =
avg × 20.

Buying intent derivation: avg of sentiment + timing ≥ 4 → `high`; ≥ 3 →
`medium`; else `low`.

## Handoff integration

When a prospect transitions to `client` stage, BE auto-copies BD discovery
fields (including BANTS) into AE-owned `custom_fields.ae_*` keys. FE can
display AE's view of the handoff context without knowing extraction details.

See [features/03-master-data/](../03-master-data/) for the 30-field map.

## Coaching notes

BE auto-generates coaching bullets for low sub-scores. Example output when
BD has weak timing + sentiment:

```
"Timing soft — anchor to fiscal-year / annual planning, not 'soon'. Sentiment
cool — consider shorter next touch + relationship-building."
```

FE can render these as actionable coaching cards for the BD's manager.

## Real vs mock

See [03-integration-state.md](../../03-integration-state.md#claude). In mock
mode (default dev), extraction always succeeds with plausible data derived
from keyword matching. FE shouldn't distinguish — same response shape.
