# feat/09 — Analytics & Reports

KPI dashboard widgets + detailed reports + revenue trend + forecast + cache.

## Status

**✅ 92%** — all endpoints live, per-role projection + Redis 15-min cache
wired. Complex per-role formulas (vs simple projection) pending alignment.

## KPI endpoints

### Full KPI
```
GET /analytics/kpi
```
Returns the full `KPIData` payload — all metrics, no filtering.

### Batch bundle (preferred for dashboard)
```
GET /analytics/kpi/bundle?role=ae&months=6
```

Single round-trip for:
- `kpi` — full KPIData
- `role_kpi` — filtered to the role's relevant metrics
- `distributions`
- `engagement`
- `revenue_trend` (months param)
- `forecast_accuracy`

Response shape:
```json
{
  "data": {
    "role": "ae",
    "kpi": { /* full KPIData */ },
    "role_kpi": {
      "role": "ae",
      "metrics": {
        "active_clients": 123,
        "renewal_rate": 0.85,
        "churn_rate": 0.08,
        "mrr": 450000000,
        "expansion_rate": 0.12,
        "overdue_invoices_count": 7
      }
    },
    "distributions": { /* ... */ },
    "engagement": { /* ... */ },
    "revenue_trend": {
      "months": ["2026-01", ..., "2026-04"],
      "actuals":   [320000000, 360000000, 380000000, 410000000],
      "forecasts": [ /* future projection */ ]
    },
    "forecast_accuracy": 0.92
  }
}
```

**Redis cache** — same `(workspace_id, role, months)` tuple serves cached
for 15 min. Cache miss: BE computes + stores + returns (with message
`"Analytics bundle"`). Cache hit: returns immediately with message
`"Analytics bundle (cached)"`.

## Per-role metric layouts

| Role | Metrics surfaced |
|---|---|
| `sdr` | `leads_this_month`, `qualified_rate`, `avg_response_time_hours`, `active_prospects` |
| `bd` | `prospects_in_pipeline`, `win_rate`, `avg_deal_cycle_days`, `closed_won_this_month` |
| `ae` | `active_clients`, `renewal_rate`, `churn_rate`, `mrr`, `expansion_rate`, `overdue_invoices_count` |
| `admin` (or unknown) | full KPI payload (no filtering) |

## Individual analytics endpoints

```
GET /analytics/distributions           # donut + bar chart data
GET /analytics/engagement              # engagement metrics (NPS, reply rate, etc.)
GET /analytics/revenue-trend?months=6
GET /analytics/forecast-accuracy
GET /dashboard/stats                   # lightweight overview stats
```

## Revenue targets

```
GET /revenue-targets                   # list
PUT /revenue-targets                   # upsert
  {"period": "2026-Q2", "target_amount": 2500000000}
```

## Snapshot rebuild (cron)

```
GET /cron/analytics/rebuild-snapshots      # OIDC
```

Rebuilds revenue snapshots (used for forecast + trend charts). Safe to
re-run; idempotent.

## Reports

```
GET /reports/executive-summary
GET /reports/revenue-contracts
GET /reports/client-health
GET /reports/engagement-retention
GET /reports/workspace-comparison         # cross-workspace for admins
POST /reports/export                       # async export via background jobs
```

Report export body:
```json
{"report": "executive-summary", "format": "xlsx"}
```

Returns job id → poll via `/jobs/{job_id}` → download via
`/jobs/{job_id}/download` when status=completed.

## FE UX

**Dashboard home:**
- Call `/analytics/kpi/bundle?role={user_role}&months=6` once on load
- Render role-specific tiles from `role_kpi.metrics`
- Chart from `revenue_trend`
- Widgets from `distributions` + `engagement`
- Cache hit indicator: toast "data from cache" for a second (subtle)

**Role switcher** (admin):
- Dropdown on dashboard: "View as SDR / BD / AE / Admin"
- Same bundle endpoint, different `role` param
- Re-fetch on switch (cache key includes role)

**Reports page:**
- List of available reports (5 types)
- Filter date range
- Export button → creates background job → polls progress → auto-download

**Holding view** (cross-workspace):
- Use `GET /workspace/holding/expand` to get sibling workspace_ids
- Fan-out `/analytics/kpi/bundle` per workspace, merge client-side
- Dedicated `/reports/workspace-comparison` for side-by-side comparison
