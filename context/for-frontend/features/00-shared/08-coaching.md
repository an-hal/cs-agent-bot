# Coaching Sessions

Peer-review + manager-review scoring for BDs on their discovery / renewal
conversations. Lead (or coach) creates a session linked to a claude_extraction
or master_data record, scores 4 dimensions (1–5), and submits.

## Endpoints

All require `Authorization: Bearer {jwt}` + `X-Workspace-ID`.

### List sessions
```
GET /coaching/sessions?bd=bd@example.com&status=submitted&limit=50
```
Query: `bd`, `coach`, `status`, `limit`, `offset`.

### Create session (draft)
```
POST /coaching/sessions
{
  "bd_email": "bd@kantorku.id",
  "session_type": "peer_review",      // peer_review|self_review|manager_review
  "master_data_id": "uuid",           // optional prospect context
  "claude_extraction_id": "uuid"      // optional extraction this critiques
}
```

### Get one
```
GET /coaching/sessions/{id}
```

### Update scores + notes
```
PATCH /coaching/sessions/{id}
{
  "bants_clarity_score": 4,
  "discovery_depth_score": 5,
  "tone_fit_score": 3,
  "next_step_clarity_score": 4,
  "strengths": "Strong discovery questions",
  "improvements": "Close with concrete next step",
  "action_items": "Send proposal by Fri"
}
```
- Each score 1–5. Out of range → 422.
- Overall score auto-computed as avg of non-nil scores.
- PATCH can be called multiple times while `status=draft`.

### Submit
```
POST /coaching/sessions/{id}/submit
```
- Requires at least one score set (`overall_score` non-nil).
- Status moves `draft → submitted`.
- After submit, further PATCH is blocked (reject with 400).

### Delete (cleanup)
```
DELETE /coaching/sessions/{id}
```

## Status lifecycle

```
draft → submitted → reviewed
          ↑
       (can't edit; only delete)
```

`reviewed` is set externally (manager marks done) — no dedicated transition
endpoint currently.

## FE UX

**Coach's view:**
1. Lead opens a BD's Fireflies transcript + Claude extraction side-by-side.
2. Clicks "Start coaching" → POST /coaching/sessions with linkage.
3. Fills in the 4 scores + 3 text fields progressively.
4. Submits — row moves to BD's inbox.

**BD's view:**
- GET /coaching/sessions?bd=me@example.com → list of feedback received.
- Show overall score, strengths, improvements, action items.
- Acknowledge + mark `reviewed` (future endpoint).

**Manager's view:**
- Aggregate across BDs — avg overall score per BD over time.
- Filter by session_type.
- Link to claude_extractions for context.
