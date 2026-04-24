# Claude Extraction Pipeline

## Overview

Backend pipeline that converts a Fireflies meeting transcript into structured BD prospect fields (60+ keys), BANTS sub-scores, and a final HOT/WARM/COLD classification with `buying_intent`. Single Claude API call returns BOTH the extraction JSON and (in the second JSON object) the coaching scores consumed by `11-bd-coaching-pipeline.md`. Fireflies handoff is documented in `07-fireflies-integration.md`. Business rules and rubrics live in `context/claude/15-buying-intent-and-bants-scoring.md`.

## 3-Stage Architecture

| Stage | Purpose | Input | Output |
|---|---|---|---|
| **1. Extraction** | Pull 60+ structured fields from raw transcript | Fireflies transcript text + meta | Flat JSON (`pain_points`, `dm_present_in_call`, `competitor_mentioned`, `next_step_agreed`, …) + per-field confidence |
| **2. Engagement scoring** | Compute per-dimension BANTS sub-scores from Stage 1 + transcript signals | Stage 1 JSON + transcript | `bants_budget`, `bants_authority`, `bants_need`, `bants_timing`, `bants_sentiment` (each 0–3) + `quality_score` |
| **3. Final classification** | Apply persona adjustment + product-specific weights → HOT / WARM / COLD | Stage 2 sub-scores + persona ctx + product (HRIS / ATS) | `bants_score`, `bants_percentage`, `bants_classification`, `buying_intent` |

All three stages run server-side in the `extraction-worker` service. Stages 1 and 2 are independent Claude calls; Stage 3 is a deterministic Go calculation (no Claude call) using the weights below.

### Product weights

```
HRIS: bants_score = B*0.296 + A*0.074 + N*0.296 + T*0.185 + S*0.148
ATS:  bants_score = B*0.3125 + A*0.125 + N*0.3125 + T*0.125 + S*0.125
```

## Claude API Contract

```
Model:        claude-sonnet-4-20250514
API:          Anthropic Messages API (https://api.anthropic.com/v1/messages)
Max tokens:   8000 (Stage 1), 4000 (Stage 2)
Temperature:  0.1
System:       value of system_config[CLAUDE_EXTRACTION_PROMPT] for Stage 1
              value of system_config[CLAUDE_BANTS_SCORING_PROMPT] for Stage 2
Auth header:  x-api-key: {ANTHROPIC_API_KEY}  (env var, NOT per-workspace)
Timeout:      90s per stage; 1 retry with exponential backoff on 429/5xx
```

`system_config` keys are read per request (cached 60s) so prompt iteration does not require deploy.

### Stage 1 expected output (truncated — 60+ fields total)

```json
{
  "company_name": "PT Maju Jaya",
  "industry": "manufacturing",
  "employee_count_band": "100-500",
  "dm_present_in_call": true,
  "dm_name": "Budi Santoso",
  "dm_title": "HR Director",
  "pain_points": ["payroll manual Excel", "absensi multi-cabang"],
  "current_solution": "Talenta",
  "competitor_mentioned": ["Mekari", "Gadjian"],
  "budget_discussed": true,
  "budget_nominal_idr": 8000000,
  "implementation_timeline": "Q3 2026",
  "next_step_agreed": "demo Selasa depan",
  "objections_raised": ["pricing", "integration with existing payroll"],
  "extraction_confidence_per_field": {
    "company_name": 0.99,
    "dm_present_in_call": 0.92,
    "budget_nominal_idr": 0.71,
    "...": "..."
  },
  "extraction_confidence_overall": 0.84
}
```

### Stage 2 expected output

```json
{
  "bants_budget": 2,
  "bants_authority": 3,
  "bants_need": 3,
  "bants_timing": 2,
  "bants_sentiment": 2,
  "quality_score": 2.4,
  "evidence": {
    "bants_budget": "Kami sekarang bayar 8 juta/bulan di Talenta",
    "bants_authority": "saya yang putuskan budget HR-tech"
  }
}
```

Backend MUST validate the JSON against a JSONSchema before persisting. Schema-invalid responses count as Stage failure (see Failure Handling).

## Per-Field Confidence Threshold Rules

Aggregated `extraction_confidence_overall = mean(field_confidence × field_criticality_weight)`. Critical fields are: `dm_present_in_call`, `pain_points`, `next_step_agreed`, `budget_discussed`, `industry`.

| Confidence | Status | Backend MUST |
|---|---|---|
| `≥ 0.80` | **AUTO-ACCEPT** | Persist extraction, proceed to Stage 2/3, mark `extraction_status='accepted'`, schedule D0 send via cron |
| `0.60 – 0.79` | **MANUAL REVIEW** | Persist extraction, mark `extraction_status='review_pending'`, enqueue to `extraction_review_queue`, surface banner on Prospect Detail Drawer, **block D0 send** until BD confirms via `POST /extraction/feedback` |
| `< 0.60` | **ESCALATE** | Persist extraction, mark `extraction_status='low_confidence'`, send Telegram alert to BD owner + BD Lead, **block D0 send** indefinitely until manual override |

## Failure Handling

Backend MUST set `extraction_status='incomplete'` AND alert BD via Telegram AND block D0 when ANY of:

| Condition | Reason field |
|---|---|
| Transcript word count `< 500` | `transcript_too_short` |
| Critical field missing in Stage 1 (any of `dm_present_in_call`, `pain_points`, `next_step_agreed`) | `critical_field_missing:{field}` |
| Stage 2 returns null / schema-invalid JSON after retry | `stage2_invalid_response` |
| Claude API returns non-2xx after 1 retry | `claude_api_error:{status_code}` |
| Total per-prospect retry count `≥ 3` | `max_retries_exhausted` |

D0 unblock requires either (a) BD confirmation via `POST /extraction/feedback`, or (b) Admin override via `PUT /extractions/{run_id}/force-accept`.

## Feedback Loop (gap #43)

Every BD manual correction (via the drawer's BANTS edit affordance OR `POST /extraction/feedback`) writes to `extraction_corrections`:

```sql
CREATE TABLE extraction_corrections (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id      UUID NOT NULL REFERENCES workspaces(id),
  prospect_id       UUID NOT NULL,
  extraction_run_id UUID NOT NULL REFERENCES extractions(id),
  field_key         VARCHAR(100) NOT NULL,
  claude_value      JSONB,
  claude_confidence DECIMAL(3,2),
  corrected_value   JSONB NOT NULL,
  correction_type   VARCHAR(20) NOT NULL,  -- 'override' | 'confirm' | 'reject'
  bd_email          VARCHAR(255) NOT NULL,
  corrected_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ec_workspace_field ON extraction_corrections(workspace_id, field_key);
CREATE INDEX idx_ec_run ON extraction_corrections(extraction_run_id);
```

Queue is consumed monthly to fine-tune `CLAUDE_EXTRACTION_PROMPT` and identify systematically low-confidence fields. No automatic prompt updates — Lead reviews diff first.

## API Surface

All endpoints scoped to `/{workspace_id}` and require `auth_session` cookie.

### POST `/extraction/run`

Manual trigger (re-extract). Idempotent within 24h window — returns existing run_id if a run already exists for the same `(prospect_id, transcript_url)` pair.

```
Request:
{
  "prospect_id": "uuid-xxx",
  "transcript_url": "https://app.fireflies.ai/view/xxx",
  "transcript_text": "...",        // optional; if omitted, backend fetches via Fireflies API
  "force": false                    // if true, bypass 24h dedup
}

Response 202:
{
  "run_id": "uuid-yyy",
  "status": "running",
  "started_at": "2026-04-22T10:00:00Z"
}

Response 409:
{
  "error": "Extraction already running for this prospect",
  "existing_run_id": "uuid-zzz"
}
```

### POST `/extraction/feedback`

BD submits correction or confirmation for a Stage 1 field. Single endpoint covers manual override AND confirmation of low-confidence fields.

```
Request:
{
  "extraction_run_id": "uuid-yyy",
  "prospect_id": "uuid-xxx",
  "corrections": [
    {
      "field_key": "budget_nominal_idr",
      "corrected_value": 12000000,
      "correction_type": "override"
    },
    {
      "field_key": "dm_present_in_call",
      "corrected_value": true,
      "correction_type": "confirm"
    }
  ]
}

Response 200:
{
  "run_id": "uuid-yyy",
  "status": "accepted",
  "d0_unblocked": true,
  "scheduled_d0_at": "2026-04-22T13:30:00Z"
}
```

Backend MUST: write to `extraction_corrections`; recompute Stage 3 if any BANTS-input field changed; if `extraction_status` was `review_pending` AND all critical fields confirmed → set `extraction_status='accepted'` and unblock D0.

### GET `/extraction/queue`

Returns prospects awaiting BD review (`extraction_status IN ('review_pending', 'low_confidence', 'incomplete')`).

```
Query: ?status=review_pending&page=1&per_page=20
Response 200:
{
  "data": [
    {
      "run_id": "uuid-yyy",
      "prospect_id": "uuid-xxx",
      "company_name": "PT Maju Jaya",
      "extraction_status": "review_pending",
      "confidence_overall": 0.72,
      "low_confidence_fields": ["budget_nominal_idr", "implementation_timeline"],
      "started_at": "2026-04-22T10:00:00Z",
      "bd_owner_email": "alice@dealls.com"
    }
  ],
  "total": 14
}
```

## DB Schema

```sql
CREATE TYPE extraction_status AS ENUM (
  'running',
  'accepted',
  'review_pending',
  'low_confidence',
  'incomplete',
  'failed'
);

CREATE TABLE extractions (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id        UUID NOT NULL REFERENCES workspaces(id),
  prospect_id         UUID NOT NULL,
  transcript_url      TEXT NOT NULL,
  transcript_word_count INTEGER,
  product             VARCHAR(20) NOT NULL,        -- 'HRIS' | 'ATS'
  started_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  completed_at        TIMESTAMPTZ,
  status              extraction_status NOT NULL DEFAULT 'running',
  stage1_output       JSONB,                       -- raw Stage 1 JSON
  stage2_output       JSONB,                       -- raw Stage 2 JSON
  stage3_output       JSONB,                       -- {bants_score, bants_percentage, bants_classification, buying_intent}
  confidence_overall  DECIMAL(3,2),
  retry_count         INTEGER NOT NULL DEFAULT 0,
  error_text          TEXT,
  triggered_by        VARCHAR(255),                -- user email OR 'fireflies_webhook' OR 'cron_re_run'
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ext_workspace_status ON extractions(workspace_id, status);
CREATE INDEX idx_ext_prospect ON extractions(prospect_id, started_at DESC);
CREATE INDEX idx_ext_review_queue ON extractions(workspace_id, status, started_at)
  WHERE status IN ('review_pending', 'low_confidence', 'incomplete');
```

`stage3_output` is also denormalised into `prospects.bants_*` columns for query performance — extraction is the source of truth, prospect columns are a cache refreshed on every accepted run.

## Backend MUST

- Cache `system_config[CLAUDE_*_PROMPT]` for ≤ 60s; never read on hot path
- Persist Stage 1 + 2 raw JSON even on failure for debugging
- Retry Claude API once on 429/5xx with exponential backoff (1s, then 4s)
- Log every Claude API call to `claude_api_log` table (latency, tokens_in, tokens_out, cost_usd) for cost monitoring
- Block D0 cron from picking up prospects unless `extraction_status='accepted'`

## Backend MAY

- Batch up to 5 extractions per Claude API parallel call (request rate ≤ 50/min as of 2026-04 contract)
- Cache transcript text in `fireflies_transcripts` table (see `07-fireflies-integration.md`) to avoid re-fetch on `force=true` re-runs
- Pre-warm common confidence thresholds in Redis for the queue dashboard
