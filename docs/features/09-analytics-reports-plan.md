# Plan — feat/09-analytics-reports

> **Branch base**: `master` &nbsp;&nbsp;|&nbsp;&nbsp; **Migration range**: `20260414000800`–`000899` &nbsp;&nbsp;|&nbsp;&nbsp; **Spec dir**: `~/dealls/project-bumi-dashboard/context/for-backend/features/09-analytics-reports/`

## Scope

Aggregated KPI computation, dashboard overview, analytics deep-dive, revenue trend + 6-month forecast (linear regression), report tabs (executive summary, revenue, client health, engagement, workspace comparison). Reuses `background_jobs` + `pkg/jobstore` + `pkg/xlsxexport` for async exports.

**Read first**: `01-overview.md`, `02-api-endpoints.md`, `03-progress.md`, `00-shared/04-integrations.md`.

> **Existing repo has** `background_jobs` table + `pkg/jobstore` + `pkg/xlsxexport`. **REUSE for any export endpoint.** Do not roll new export infra.

## Migrations

| # | File | Purpose |
|---|---|---|
| 800 | `create_revenue_targets.{up,down}.sql` | `(workspace_id, year, month, target_amount BIGINT, created_by, created_at, updated_at)` UNIQUE(ws, year, month). Used by quota attainment + forecast targeting. |
| 801 | `create_revenue_snapshots.{up,down}.sql` | Materialized monthly aggregates `(workspace_id, year, month, revenue_actual BIGINT, deals_won INT, deals_lost INT, computed_at)` UNIQUE(ws, year, month). Refreshed nightly by cron. Used to keep forecast queries fast. |
| 802 | `create_report_jobs_extension.{up,down}.sql` | If needed, ALTER `background_jobs` to add `report_type VARCHAR(50)` for tracking which report kind was exported. Otherwise reuse generic columns. |

> **No new core data tables**. This feature is read-mostly; all numbers come from feat/03 master_data, feat/07 invoices, and feat/08 logs.

## Entities

`internal/entity/analytics.go`:
```go
type DashboardStats struct {
    Revenue struct {
        Achieved int64   `json:"achieved"`
        Target   int64   `json:"target"`
        Pct      float64 `json:"pct"`
        Upsell   int64   `json:"upsell"`
    } `json:"revenue"`
    Clients struct {
        Total   int `json:"total"`
        Active  int `json:"active"`
        Snoozed int `json:"snoozed"`
    } `json:"clients"`
    Risk struct {
        High, Mid, Low int
    } `json:"risk"`
    Contracts struct {
        Expiring30d int `json:"expiring_30d"`
        Expired     int `json:"expired"`
    } `json:"contracts"`
    AE struct {
        QuotaAttainment  float64 `json:"quota_attainment"`
        DealsWon         int     `json:"deals_won"`
        DealsLost        int     `json:"deals_lost"`
        WinRate          float64 `json:"win_rate"`
        AvgSalesCycle    int     `json:"avg_sales_cycle"`
        ForecastAccuracy float64 `json:"forecast_accuracy"`
    } `json:"ae"`
    TopAccounts []TopAccount `json:"top_accounts"`
}

type RevenueDataPoint struct {
    Month        string  `json:"month"`        // "Jul '24"
    Date         string  `json:"date"`         // "2024-07"
    Actual       *float64 `json:"actual"`      // billions IDR
    Target       float64 `json:"target"`
    Forecast     *float64 `json:"forecast"`
    ForecastHigh *float64 `json:"forecast_high"`
    ForecastLow  *float64 `json:"forecast_low"`
}

type RevenueTarget struct {
    WorkspaceID  uuid.UUID
    Year         int
    Month        int
    TargetAmount int64
}

type RevenueSnapshot struct {
    WorkspaceID   uuid.UUID
    Year          int
    Month         int
    RevenueActual int64
    DealsWon      int
    DealsLost     int
    ComputedAt    time.Time
}
```

## Repositories

```
internal/repository/
  analytics_repo.go            // DashboardStats, KPI, Distributions — pure SQL aggregates over master_data + invoices + escalations
  revenue_target_repo.go       // CRUD
  revenue_snapshot_repo.go     // List, Upsert, RebuildAll(ws)
```

## Usecases

`internal/usecase/analytics/usecase.go`:
- `DashboardStats(ctx, ws)` — single SQL call returning the response shape. Holding workspace expands to `member_ids`.
- `KPI(ctx, ws)` — full KPI block per spec §2
- `Distributions(ctx, ws)` — donut + horizontal bar data
- `Engagement(ctx, ws)` — bot/NPS/checkin/cross-sell/renewed counts

`internal/usecase/analytics/forecast.go`:
- `RevenueTrend(ctx, ws, monthsTotal int)` — fetches snapshots + targets, runs OLS linear regression (slope+intercept), extrapolates next 6 months, computes residual std dev for confidence band (`±1.5σ`). Returns full `[]RevenueDataPoint` (10 actual + 6 forecast).
- `ForecastAccuracy(ctx, ws)` — last month's forecast vs actual diff

Implementation: pure Go OLS — no ML library. ~50 lines:
```go
// y = mx + b
// m = Σ((xi - x̄)(yi - ȳ)) / Σ((xi - x̄)²)
// b = ȳ - m*x̄
```

`internal/usecase/analytics/snapshot_cron.go`:
- `RebuildSnapshots(ctx, ws)` — recomputes `revenue_snapshots` rows for the past 18 months from `invoices` (`payment_status='Lunas'` group by year-month). Run nightly via Cloud Scheduler.

`internal/usecase/reports/usecase.go`:
- `ExecutiveSummary(ctx, ws)`, `RevenueContracts(ctx, ws)`, `ClientHealth(ctx, ws)`, `EngagementRetention(ctx, ws)`, `WorkspaceComparison(ctx, holdingWS)` — five report tabs. Each returns a JSON blob ready for the frontend.
- `ExportReport(ctx, ws, reportType, format)` — creates `background_jobs` row, kicks off goroutine that uses `pkg/xlsxexport` (or `pdfexport` if added) to render the report, writes to `EXPORT_STORAGE_PATH/reports/{job_id}.{ext}`.

`internal/usecase/reports/highlights.go`:
- Auto-generates "Highlights Positif & Negatif" from threshold rules:
  - Positive: revenue ≥ 90% target, NPS ≥ 8, win rate ≥ 75%
  - Negative: high risk count > 5, expiring 30d > 10, NPS < 5
  - Returns array of strings ready for display

## HTTP routes

```go
api.Handle(GET, "/dashboard/stats",                    wsRequired(jwtAuth(dashH.Stats)))

an := api.Group("/analytics")
an.Handle(GET, "/kpi",                                 wsRequired(jwtAuth(anH.KPI)))
an.Handle(GET, "/distributions",                       wsRequired(jwtAuth(anH.Distributions)))
an.Handle(GET, "/engagement",                          wsRequired(jwtAuth(anH.Engagement)))
an.Handle(GET, "/revenue-trend",                       wsRequired(jwtAuth(anH.RevenueTrend)))
an.Handle(GET, "/forecast-accuracy",                   wsRequired(jwtAuth(anH.ForecastAccuracy)))

rep := api.Group("/reports")
rep.Handle(GET, "/executive-summary",                  wsRequired(jwtAuth(repH.ExecutiveSummary)))
rep.Handle(GET, "/revenue-contracts",                  wsRequired(jwtAuth(repH.RevenueContracts)))
rep.Handle(GET, "/client-health",                      wsRequired(jwtAuth(repH.ClientHealth)))
rep.Handle(GET, "/engagement-retention",               wsRequired(jwtAuth(repH.EngagementRetention)))
rep.Handle(GET, "/workspace-comparison",               wsRequired(jwtAuth(repH.WorkspaceComparison)))
rep.Handle(POST, "/export",                            wsRequired(jwtAuth(repH.Export)))   // bg job

api.Handle(GET, "/revenue-targets",                    wsRequired(jwtAuth(targetH.List)))
api.Handle(PUT, "/revenue-targets",                    wsRequired(jwtAuth(targetH.Upsert)))

// Cron snapshot rebuild
api.Handle(GET, "/cron/analytics/rebuild-snapshots",   oidcAuth(snapH.Rebuild))
```

## Tests

- `usecase/analytics/usecase_test.go` — DashboardStats numeric correctness on fixture, holding aggregation, Pct = 0 when target=0 (no div-by-zero)
- `usecase/analytics/forecast_test.go` — OLS implementation matches a known dataset (slope+intercept within ε), 6-month extrapolation count, confidence band sanity (±1.5σ)
- `usecase/reports/highlights_test.go` — threshold rules fire correctly across boundaries
- `usecase/reports/export_test.go` — background job creation, file written to expected path, status transitions

## Risks / business-rule conflicts with CLAUDE.md

- **Read-only feature**: doesn't write to `clients`, `invoices`, or any audit table. The only writes are to `revenue_targets` (admin-only) and `revenue_snapshots` (cron). No conflict with rules 1–3.
- **Workspace isolation**: every aggregate query MUST include `workspace_id` filter. Holding workspaces use `WHERE workspace_id = ANY(member_ids)`. Add a `_workspace_isolation_test.go` that proves no cross-workspace leak.
- **Forecast assumptions**: linear regression only. Document that this is naive — for a v2, swap in seasonal decomposition. The CI test asserts behavior on a known dataset; do NOT regression-test the forecast by retraining each run (use a fixed seed/fixed dataset).
- **Background job storage path**: reuse `EXPORT_STORAGE_PATH` from existing config; sub-folder `/reports/`.

## File checklist

- [ ] migrations 800–802
- [ ] entities (analytics, revenue_target, revenue_snapshot)
- [ ] repos + mocks (3)
- [ ] usecases: analytics (kpi/distributions/engagement), analytics/forecast (OLS), analytics/snapshot_cron, reports (5 tabs + export + highlights)
- [ ] handlers: dashboard, analytics, reports, revenue_target, snapshot_cron
- [ ] route.go + deps.go + main.go wiring
- [ ] swag regen
- [ ] `make lint && make unit-test` green; explicit forecast accuracy test + workspace isolation test
- [ ] commit + push `feat/09-analytics-reports`
