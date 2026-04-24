# Analytics & Reports — Implementation Progress

## 2026-04-23 — NPS dual-track + Health Score sync

### FE (shipped)
- **NPS Trend Tracking** — dual-score model (Onboarding NPS @ D30 + Renewal NPS @ T-60) with separate trend lines
- **AE Health Score formula card** — surfaces composition (NPS 30% + Usage 30% + Payment 25% + Engagement 15%) with current value per client
- **Handoff SLA dashboard** — surfaces BD→AE handoff lag distribution + breach count (shared with 06-workflow-engine)

### Backend spec (documented, implementation pending)
- **NPS Survey Timing & Response Rate SLA** referenced from `claude/for-backend/13-kpi-metrics-and-targets.md` — defines D30 + T-60 windows, response rate floor, and per-survey aggregation rules
- AE Health Score formula referenced from same KPI doc — backend must compute server-side (not FE) for consistency across reports

### Open dependencies (backend)
- Implement Health Score computation server-side — exposed via new field on `GET /master-data/clients/{id}` and aggregated in `GET /analytics/kpi`
- Implement NPS dual tracking aggregation — separate `nps_onboarding_*` + `nps_renewal_*` columns in distributions response (currently single `nps_*`)
- Cache invalidation must trigger when health-score inputs change (NPS write, Usage update, payment status flip, engagement event)
- Handoff SLA aggregation — needs join to `bd_escalations` table from 06-workflow-engine

### Cross-refs
- FE: `app/dashboard/[workspace]/analytics/page.tsx`, `components/features/HealthScoreCard.tsx`, `components/features/NPSTrendChart.tsx`
- Handoff SLA shared with 06-workflow-engine (Handoff SLA dashboard)
- KPI source: `claude/for-backend/13-kpi-metrics-and-targets.md` (NPS SLA + Health Score formula)
- Backend gap doc: `claude/for-backend/09-analytics-reports/gap-nps-dual-tracking.md`, `gap-health-score-server-compute.md`

---

> **Overall: 38% complete** (15/40 items done or partial)
> - Frontend/BFF: 72% done (13 done + 2 partial)
> - Backend (Go): 0% done (20 items not started)
> - Optional improvements: 0% done (5 items)

---

## DONE — Frontend/BFF ✅ (13 items)

| # | Item | File | Notes |
|---|------|------|-------|
| 1 | Dashboard overview page | `app/dashboard/[workspace]/page.tsx` | 4 summary cards (Revenue, Quota, Clients, Attention) + AE Feature Card + Data Master Feature Card |
| 2 | Analytics page with full KPI data | `app/dashboard/[workspace]/analytics/page.tsx` | KPI cards, donut charts, horizontal bars. Uses mock data from `lib/mock-data.ts` |
| 3 | Reports page with 5 tabs | `app/dashboard/[workspace]/reports/page.tsx` | Executive Summary, Revenue & Contracts, Client Health, Engagement & Retention, Workspace Comparison (holding only) |
| 4 | Revenue chart with forecast | `components/features/RevenueChart.tsx` | ComposedChart (Recharts): actual + forecast with confidence band, linear regression, brush zoom, per-workspace + holding combined view |
| 5 | Linear regression forecast | `components/features/RevenueChart.tsx` | OLS from 10 months actual data, 6 months extrapolation, confidence band +/-1.5 sigma residual |
| 6 | Workspace-aware data selection | All 3 pages | Dealls / KantorKu / Holding selection. Holding aggregates from both. Weighted averages for NPS, Usage |
| 7 | Risk distribution visualization | Analytics + Reports pages | Donut chart: High/Mid/Low risk with counts |
| 8 | Payment status breakdown | Analytics + Reports pages | Donut chart: Lunas/Menunggu/Terlambat |
| 9 | NPS distribution & by-industry | Analytics + Reports pages | Promoter/Passive/Detractor donut + NPS by industry horizontal bars |
| 10 | Usage score distribution | Analytics + Reports pages | 4 buckets (0-25, 26-50, 51-75, 76-100) + average |
| 11 | Engagement metrics | Reports page (Engagement tab) | Bot active/inactive, NPS replied, checkin replied, cross-sell interested/rejected, renewed |
| 12 | Workspace comparison tab | Reports page | Side-by-side: Dealls vs KantorKu with clients, revenue, win rate, NPS, usage, risk, expiring, cross-sell |
| 13 | Mock metrics API | `app/api/mock/metrics/route.ts` | GET /api/mock/metrics?role=sdr\|bd\|ae\|cs — returns role-specific mock metrics |

## PARTIAL ⚠️ (2 items)

| # | Item | What's Done | What's Missing |
|---|------|-------------|----------------|
| 14 | Dashboard overview stats | 4 summary cards + AE + Data Master feature cards with all key metrics | Spec wants dedicated `GET /dashboard/stats` endpoint response shape. Currently computed from local mock data, not fetched from API |
| 15 | Contract expiry timeline | Reports shows expired, 0-30d, 31-60d, 61-90d buckets | Spec includes >90d bucket. Missing expiring-soon client list with renewal status in analytics page |

## NOT DONE — Backend (Go) Required 🔴 (20 items)

### Critical (blocks real data)

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 16 | GET `/dashboard/stats` | 02-api-endpoints | Quick stats: revenue (achieved/target/pct/upsell), clients (total/active/snoozed), risk, contracts, ae (quota/deals/win_rate/cycle/forecast), top_accounts |
| 17 | GET `/analytics/kpi` | 02-api-endpoints | Comprehensive KPI: revenue, clients, nps (avg/score/respondents/promoter/passive/detractor), attention, ae performance |
| 18 | GET `/analytics/distributions` | 02-api-endpoints | Risk, payment, contract_expiry, nps, usage_score, plan_type_revenue, industry_breakdown, hc_size_breakdown, value_tier_breakdown, engagement stats |

### High Priority

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 19 | GET `/analytics/revenue-trend` | 02-api-endpoints | Monthly revenue with forecast. Data array: month, actual, target, forecast, forecast_high/low, is_forecast. Plus regression params + summary |
| 20 | Revenue trend holding view | 02-api-endpoints | Per-workspace breakdown (dealls/kantorku columns) + combined forecast + per_workspace summary |
| 21 | Forecast computation (backend) | 01-overview | Query monthly aggregated revenue from deals, fit linear regression, extrapolate 6 months, compute confidence band, cache 1 hour |
| 22 | GET `/reports/executive` | 02-api-endpoints | KPI summary + auto-generated highlights_positive and highlights_negative arrays |
| 23 | GET `/reports/revenue` | 02-api-endpoints | Total contract value, overdue value, plan_type_revenue, top_clients, contract_expiry, payment_breakdown, at_risk_clients, expiring_soon |
| 24 | GET `/reports/health` | 02-api-endpoints | Risk distribution, payment distribution, NPS distribution + by industry, usage distribution, payment_issue_clients |
| 25 | GET `/reports/engagement` | 02-api-endpoints | Bot active/inactive, NPS replied, checkin replied, cross-sell pipeline, renewed/not_renewed, cross_sell_clients list |
| 26 | GET `/reports/comparison` | 02-api-endpoints | Workspace comparison: per-workspace stats array (clients, revenue, deals, NPS, usage, risk, expiring) + combined totals. Holding only |

### Medium Priority

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 27 | GET `/reports/export` | 02-api-endpoints | Export report as XLSX/CSV/PDF. Query: tab + format. Requires `reports.export` permission |
| 28 | Caching strategy | 02-api-endpoints | `/dashboard/stats` 5 min, `/analytics/kpi` 5 min, `/analytics/distributions` 10 min, `/analytics/revenue-trend` 1 hour, `/reports/*` 15 min |
| 29 | Cache invalidation | 02-api-endpoints | Invalidate on master_data write, deal write, or deal close. Per-workspace cache keys. Holding cache separate |
| 30 | Highlights auto-generation | 01-overview | Threshold-based rules to generate positive/negative highlights for executive summary |
| 31 | Forecast re-computation trigger | 01-overview | Recompute when new monthly data point available or significant deal closes. Store regression params |

### Low Priority

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 32 | HC size breakdown | 02-api-endpoints | Distribution: 1-10, 11-50, 51-100, 101-200, 201-500, 500+ |
| 33 | Value tier breakdown | 02-api-endpoints | Distribution by Tier 1/2/3 |
| 34 | Holding weighted averages | 01-overview | NPS, Usage averages weighted by client count per workspace |
| 35 | Concurrent queries for holding | 02-api-endpoints | Run per-workspace queries in parallel for holding view performance |

## NOT DONE — Optional Frontend Improvements 🟡 (5 items)

| # | Item | Priority | Description |
|---|------|----------|-------------|
| 36 | Replace mock data with API calls | High | All 3 pages use local mock data. Need to fetch from backend endpoints once available |
| 37 | Export button UI | High | Reports page has no export button yet. Needs download trigger for XLSX/CSV/PDF |
| 38 | Revenue trend from API | Medium | RevenueChart currently computes forecast client-side from hardcoded data. Should fetch from `/analytics/revenue-trend` |
| 39 | Loading states for analytics | Low | No skeleton/loading states when switching workspaces — data is instant from mock but will need loading from API |
| 40 | Drill-down from KPI cards | Low | Click on a KPI card to navigate to filtered view (e.g., click "High Risk" to see those clients) |

---

## Recommended Implementation Order (Backend)

```
Week 1: #16 GET /dashboard/stats + #17 GET /analytics/kpi + #18 GET /analytics/distributions
Week 2: #19 GET /analytics/revenue-trend + #21 forecast computation + #20 holding view
Week 3: #22 reports/executive + #23 reports/revenue + #24 reports/health + #25 reports/engagement
Week 4: #26 reports/comparison + #27 reports/export + #30 highlights auto-generation
Week 5: #28-29 caching + #31 forecast trigger + #32-35 remaining distributions
```

## Dependency Chain

```
master_data queries ──→ /dashboard/stats ──→ cache (5 min)
                   ──→ /analytics/kpi    ──→ cache (5 min)
                   ──→ /analytics/distributions ──→ cache (10 min)

deals/invoices queries ──→ monthly aggregation ──→ linear regression
                                                       │
                                                       └──→ /analytics/revenue-trend ──→ cache (1 hour)

/analytics/* ──→ /reports/executive (KPI + highlights)
             ──→ /reports/revenue   (contract + payment focus)
             ──→ /reports/health    (risk + NPS + usage focus)
             ──→ /reports/engagement (bot + reply + cross-sell focus)
             ──→ /reports/comparison (holding only — per-workspace)
                       │
                       └──→ /reports/export (XLSX/CSV/PDF generation)
```
