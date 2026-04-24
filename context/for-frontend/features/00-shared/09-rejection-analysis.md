# Rejection Analysis

Capture + classify rejection replies so coaching + dashboard can surface
common objection patterns. Rule-based classifier runs inline; Claude-backed
analyst can be added later.

## Endpoints

### Run classifier over a reply (stores result)
```
POST /rejection-analysis/analyze
{
  "master_data_id": "uuid",
  "source_channel": "wa",         // wa|email|call_note|meeting
  "source_message": "Bro, ini terlalu mahal, butuh diskon dulu"
}
```
Classifier is rule-based keyword matching. Categories produced:
- `price` — "mahal", "harga", "budget", "biaya", "diskon"
- `authority` — "bos", "atasan", "cfo", "ceo", "direksi", "tanya atasan"
- `timing` — "nanti", "bulan depan", "quarter", "q3", "q4", "masih lama", "belum butuh"
- `feature` — "fitur", "integrasi", "api", "laporan", "belum ada", "kurang"
- `tone` — "ga enak", "marah", "komplain", "kecewa", "jelek" (severity=high)
- `other` — default fallback (severity=low)

### Record pre-classified (human or Claude-driven)
```
POST /rejection-analysis
{
  "master_data_id": "uuid",
  "source_channel": "call_note",
  "source_message": "Nanti saja pak, fokus Q4",
  "rejection_category": "timing",
  "severity": "mid",              // low|mid|high
  "analysis_summary": "Delay until Q4; keep warm",
  "suggested_response": "Set a Q3 check-in reminder",
  "analyst": "human",             // rule|claude|human
  "analyst_version": "ae-v1"
}
```

### List
```
GET /rejection-analysis?master_data_id={id}&category=price&limit=50
```

### Category stats (dashboard widget)
```
GET /rejection-analysis/stats?days=30
```
Returns `{price: 12, timing: 7, authority: 3, ...}` — buckets over last N days.

## Data shape

See [../../05-data-models.md](../../05-data-models.md) for `RejectionAnalysis`
entity.

## FE UX

**Inline widget on client page:** "Rejection patterns" panel showing
recent analyses for that client.

**Dashboard widget:** bar chart of category counts over 30/60/90 days.
Drill-in opens filtered list.

**Classifier accuracy:** keyword-based is ~70% accurate; surface the
"run Claude re-analysis" button for manager-only re-classification.
