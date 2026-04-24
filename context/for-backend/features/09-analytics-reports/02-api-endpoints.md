# API Endpoints — Analytics & Reports

## Base URL
```
{BACKEND_API_URL}/api/v1
```

All endpoints require `Authorization: Bearer {token}` header.
Workspace-scoped endpoints require `X-Workspace-ID: {uuid}` header.

---

## 1. Dashboard Overview

### GET `/dashboard/stats`
Quick stats for the dashboard overview page. Lightweight — used on page load.

```
Response 200:
{
  "revenue": {
    "achieved": 3240000000,
    "target": 5000000000,
    "pct": 64.8,
    "upsell": 450000000
  },
  "clients": {
    "total": 15,
    "active": 12,
    "snoozed": 3
  },
  "risk": {
    "high": 2,
    "mid": 4,
    "low": 9
  },
  "contracts": {
    "expiring_30d": 3,
    "expired": 1
  },
  "ae": {
    "quota_attainment": 76.3,
    "deals_won": 12,
    "deals_lost": 3,
    "win_rate": 80,
    "avg_sales_cycle": 28,
    "forecast_accuracy": 82
  },
  "top_accounts": [
    {
      "id": "uuid",
      "name": "PT Maju Digital",
      "revenue": 45000000,
      "status": "expanding",
      "last_contact": "2026-04-01"
    }
  ]
}
```

Holding view: if workspace is holding, aggregate from all members.
Response shape is the same, values are summed/averaged.

---

## 2. Analytics — Full KPI Data

### GET `/analytics/kpi`
Comprehensive KPI data for the Analytics page.

```
Response 200:
{
  "revenue": {
    "achieved": 3240000000,
    "target": 5000000000,
    "pct": 64.8,
    "upsell": 450000000
  },
  "clients": {
    "total": 15,
    "active": 12,
    "snoozed": 3
  },
  "nps": {
    "average": 7.8,
    "score": 42,
    "respondents": 13,
    "total_clients": 15,
    "promoter": 6,
    "passive": 4,
    "detractor": 3
  },
  "attention": {
    "high_risk": 2,
    "expiring_30d": 3
  },
  "ae": {
    "quota_attainment": 76.3,
    "deals_won": 12,
    "deals_lost": 3,
    "win_rate": 80,
    "avg_sales_cycle": 28,
    "forecast_accuracy": 82,
    "upsell_revenue": 450000000
  }
}
```

### GET `/analytics/distributions`
Distribution data for charts (donut, horizontal bars).

```
Response 200:
{
  "risk": {
    "high": 2,
    "mid": 4,
    "low": 9
  },
  "payment_status": {
    "lunas": 8,
    "menunggu": 4,
    "terlambat": 3
  },
  "contract_expiry": {
    "0_30d": 3,
    "31_60d": 4,
    "61_90d": 2,
    "90d_plus": 6
  },
  "nps_distribution": {
    "promoter": 6,
    "passive": 4,
    "detractor": 3
  },
  "usage_score": {
    "0_25": 2,
    "26_50": 3,
    "51_75": 5,
    "76_100": 5,
    "average": 62
  },
  "plan_type_revenue": [
    { "plan": "Enterprise", "revenue": 1800000000 },
    { "plan": "Mid", "revenue": 950000000 },
    { "plan": "Basic", "revenue": 490000000 }
  ],
  "industry_breakdown": [
    { "industry": "Technology", "count": 4 },
    { "industry": "Finance", "count": 3 },
    { "industry": "Retail", "count": 2 }
  ],
  "hc_size_breakdown": [
    { "range": "1-10", "count": 1 },
    { "range": "11-50", "count": 3 },
    { "range": "51-100", "count": 4 },
    { "range": "101-200", "count": 4 },
    { "range": "201-500", "count": 2 },
    { "range": "500+", "count": 1 }
  ],
  "value_tier_breakdown": [
    { "tier": "Tier 1", "count": 3 },
    { "tier": "Tier 2", "count": 7 },
    { "tier": "Tier 3", "count": 5 }
  ],
  "engagement": {
    "bot_active": 12,
    "bot_inactive": 3,
    "nps_replied": 10,
    "checkin_replied": 8,
    "cross_sell_interested": 4,
    "cross_sell_rejected": 1,
    "renewed": 6
  }
}
```

---

## 3. Revenue Trend & Forecast

### GET `/analytics/revenue-trend`
Monthly revenue data with forecast. Dipakai oleh RevenueChart component.

```
Query params:
  ?months=16                         (optional, default 16 — 10 actual + 6 forecast)
  &workspace=dealls                  (optional: dealls | kantorku | holding)

Response 200:
{
  "workspace": "dealls",
  "target": 5.0,
  "data": [
    {
      "month": "Jul '24",
      "date": "2024-07",
      "actual": 2.05,
      "target": 5.0,
      "forecast": null,
      "forecast_high": null,
      "forecast_low": null,
      "is_forecast": false
    },
    {
      "month": "Apr '25",
      "date": "2025-04",
      "actual": 3.24,
      "target": 5.0,
      "forecast": null,
      "forecast_high": null,
      "forecast_low": null,
      "is_forecast": false
    },
    {
      "month": "May '25",
      "date": "2025-05",
      "actual": null,
      "target": 5.0,
      "forecast": 3.58,
      "forecast_high": 3.92,
      "forecast_low": 3.24,
      "is_forecast": true
    }
  ],
  "regression": {
    "slope": 0.128,
    "intercept": 1.95,
    "r_squared": 0.87,
    "residual_std": 0.22
  },
  "summary": {
    "last_actual": 3.24,
    "growth_pct": 58.0,
    "forecast_end": 4.12
  }
}
```

Holding view:
```
{
  "workspace": "holding",
  "target": 8.0,
  "data": [
    {
      "month": "Jul '24",
      "date": "2024-07",
      "dealls": 2.05,
      "kantorku": 0.82,
      "combined": 2.87,
      "combined_target": 8.0,
      "is_forecast": false
    },
    {
      "month": "May '25",
      "date": "2025-05",
      "dealls_fc": 3.58,
      "kantorku_fc": 1.95,
      "combined_fc": 5.53,
      "forecast_high": 6.12,
      "forecast_low": 4.94,
      "is_forecast": true
    }
  ],
  "per_workspace": {
    "dealls": { "target": 5.0, "last_actual": 3.24, "growth_pct": 58.0 },
    "kantorku": { "target": 3.0, "last_actual": 1.82, "growth_pct": 121.9 }
  }
}
```

### Forecast Computation (backend)
```
1. Query: SELECT date_trunc('month', closed_at) AS month, SUM(final_price) AS revenue
   FROM deals WHERE workspace_id = $1 AND closed_at >= NOW() - INTERVAL '12 months'
   GROUP BY 1 ORDER BY 1

2. Fit linear regression: y = slope * x + intercept
   x = month index (0, 1, 2, ...)
   y = revenue in billions

3. Extrapolate 6 months forward

4. Confidence band = forecast +/- 1.5 * residual_std_dev

5. Cache result for 1 hour (invalidate on new deal close)
```

### GET `/analytics/funnel-forecast`

Backs the `ForecastView` component. Projects next-month SDR→BD→Paid pipeline by blending current-pipeline math with 6-month linear regression.

```
Query params:
  ?workspace=dealls            (required)
  &owner=alice@company.com     (optional — project one individual's pipeline)
  &months=6                    (optional, default 6 — historical window for regression)

Response 200:
{
  "workspace": "dealls",
  "owner": null,                            // echoes ?owner= if scoped
  "current_pipeline": {
    "sdr_leads": 42,                        // Stage=LEAD or DORMANT
    "sdr_blasted": 38,                      // leads with first_blast_date set
    "sdr_qualified": 14,                    // sdr_score >= 3 AND verdict_sdr in (QUALIFIED,WARM)
    "sdr_to_meeting": 9,                    // pipeline_status=bd_meeting_scheduled OR Stage in (PROSPECT,CLIENT)
    "sdr_success_rate": 0.237,              // sdr_to_meeting / sdr_blasted
    "bd_prospects": 23,
    "bd_paid": 6,
    "bd_conversion_rate": 0.261,
    "avg_deal_size": 48500000,              // average Final_Price of closed_won + first_payment_paid
    "projected_bd_meetings": 3,             // round(sdr_qualified * sdr_success_rate)
    "projected_paid_deals": 1,              // round(projected_bd_meetings * bd_conversion_rate)
    "projected_revenue": 48500000           // projected_paid_deals * avg_deal_size
  },
  "regression": {                           // 6-month linear regression
    "slope": 2.1,
    "intercept": 4.3,
    "r_squared": 0.82,
    "trend_direction": "up",                // up | down | flat
    "trend_pct_change": 12,                 // slope / mean * 100
    "confidence": "high",                   // high (r²>0.7) | medium (>0.4) | low
    "projected_revenue": 52000000,
    "projected_paid": 1
  },
  "blended": {                              // average of current + regression
    "revenue": 50250000,
    "paid_deals": 1
  },
  "history": [
    { "month": "2025-11", "sdr_blasted": 120, "sdr_qualified": 40, "bd_meetings": 22, "bd_paid": 8, "revenue": 390000000, "avg_deal_size": 48750000 },
    { "month": "2025-12", "sdr_blasted": 134, "sdr_qualified": 46, "bd_meetings": 25, "bd_paid": 9, "revenue": 441000000, "avg_deal_size": 49000000 }
    // ... up to `months` entries
  ],
  "per_owner": [                            // omitted when ?owner= is set
    {
      "owner": "alice@company.com",
      "owner_name": "Alice",
      "leads": 12, "qualified": 5, "meetings": 3, "paid": 2,
      "sdr_rate": 0.25, "bd_rate": 0.667,
      "forecast_paid": 1                    // round(qualified * sdr_rate * bd_rate)
    }
  ]
}
```

**Computation (FE parity):**
Matches `components/features/ForecastView.tsx` exactly so FE can transition from client-side math to server response without display changes.

- `sdr_success_rate = sdr_to_meeting / max(sdr_blasted, 1)`
- `bd_conversion_rate = bd_paid / max(bd_prospects, 1)`
- `projected_bd_meetings = round(sdr_qualified * sdr_success_rate)`
- `projected_paid_deals  = round(projected_bd_meetings * bd_conversion_rate)`
- `projected_revenue     = projected_paid_deals * avg_deal_size`
- Linear regression on `history[].revenue` gives `slope`, `intercept`, `r_squared`
- `trend_direction`: `up` if slope > 0.1×mean, `down` if < -0.1×mean, else `flat`
- `confidence`: `high` r²>0.7, `medium` r²>0.4, `low` otherwise
- `blended.revenue = round((current.projected_revenue + regression.projected_revenue) / 2)`

**Caching:** 15-minute TTL per (workspace, owner). Invalidate on: deal close, stage transition, new blast, bd_meeting_date update.

**When `?owner=` is set:** filter all `current_pipeline` counts to rows where `owner_name = ?owner` (or `owner_email`, depending on schema — FE uses `Owner_Name` today). `per_owner` is omitted from the response.

---

## 4. Reports Data

### GET `/reports/executive`
Executive summary tab data.

```
Response 200:
{
  "kpi": {
    "revenue_achieved": 3240000000,
    "revenue_target": 5000000000,
    "revenue_pct": 64.8,
    "quota_attainment": 76.3,
    "active_clients": 12,
    "total_clients": 15,
    "snoozed_clients": 3,
    "high_risk": 2,
    "mid_risk": 4,
    "low_risk": 9,
    "avg_nps": 7.8,
    "nps_score": 42,
    "nps_respondents": 13,
    "expiring_30d": 3,
    "expired": 1
  },
  "highlights_positive": [
    "Revenue on track: 64.8% quota attainment dengan 12 deals closed",
    "9 klien risiko rendah (60% portofolio)",
    "NPS avg 7.8 — 6 promoter aktif",
    "4 peluang cross-sell terbuka"
  ],
  "highlights_negative": [
    "2 klien high risk perlu perhatian segera",
    "3 kontrak expired / expiring < 30 hari",
    "3 pembayaran terlambat (total Rp 150.000.000)",
    "3 klien dengan usage score < 25"
  ]
}
```

### GET `/reports/revenue`
Revenue & Contract tab data.

```
Response 200:
{
  "total_contract_value": 4750000000,
  "overdue_value": 450000000,
  "plan_type_revenue": [
    { "plan": "Enterprise", "revenue": 1800000000 },
    { "plan": "Mid", "revenue": 950000000 }
  ],
  "top_clients": [
    {
      "company_id": "DE-001",
      "company_name": "PT Maju Digital",
      "final_price": 45000000,
      "plan_type": "Enterprise",
      "payment_status": "Lunas",
      "days_to_expiry": 45,
      "risk_flag": "Low"
    }
  ],
  "contract_expiry": {
    "expired": 1,
    "0_30d": 3,
    "31_60d": 4,
    "61_90d": 2
  },
  "payment_breakdown": {
    "lunas": 8,
    "menunggu": 4,
    "terlambat": 3
  },
  "at_risk_clients": [
    {
      "company_id": "DE-007",
      "company_name": "PT Alam Indah",
      "risk_flag": "High",
      "payment_status": "Terlambat",
      "days_to_expiry": -5,
      "final_price": 30000000
    }
  ],
  "expiring_soon": [
    {
      "company_id": "DE-003",
      "company_name": "PT Garuda Nusantara",
      "days_to_expiry": 12,
      "contract_end": "2026-04-17",
      "final_price": 25000000,
      "renewed": false
    }
  ]
}
```

### GET `/reports/health`
Client Health tab data.

```
Response 200:
{
  "risk_distribution": { "high": 2, "mid": 4, "low": 9 },
  "payment_distribution": { "lunas": 8, "menunggu": 4, "terlambat": 3 },
  "nps_distribution": { "promoter": 6, "passive": 4, "detractor": 3, "no_response": 2 },
  "nps_by_industry": [
    { "industry": "Technology", "avg_nps": 8.5 },
    { "industry": "Finance", "avg_nps": 7.2 }
  ],
  "usage_distribution": {
    "0_25": 2, "26_50": 3, "51_75": 5, "76_100": 5,
    "average": 62
  },
  "payment_issue_clients": [
    {
      "company_id": "DE-014",
      "company_name": "PT Surya Gemilang",
      "payment_status": "Terlambat",
      "days_overdue": 15,
      "final_price": 35000000
    }
  ]
}
```

### GET `/reports/engagement`
Engagement & Retention tab data.

```
Response 200:
{
  "bot_active": 12,
  "bot_inactive": 3,
  "nps_replied": 10,
  "nps_not_replied": 5,
  "checkin_replied": 8,
  "checkin_not_replied": 7,
  "cross_sell_interested": 4,
  "cross_sell_rejected": 1,
  "cross_sell_pending": 10,
  "renewed": 6,
  "not_renewed": 9,
  "cross_sell_clients": [
    {
      "company_id": "DE-001",
      "company_name": "PT Maju Digital",
      "current_plan": "Mid",
      "interested_in": "Enterprise"
    }
  ]
}
```

### GET `/reports/comparison`
Workspace comparison data — only available for holding workspace.

```
Response 200:
{
  "workspaces": [
    {
      "id": "uuid",
      "slug": "dealls",
      "name": "Dealls",
      "stats": {
        "clients": 15,
        "active_clients": 12,
        "total_value": 2930000000,
        "revenue_achieved": 3240000000,
        "revenue_target": 5000000000,
        "revenue_pct": 64.8,
        "deals_won": 12,
        "win_rate": 80,
        "avg_nps": 7.8,
        "avg_usage": 65,
        "high_risk": 2,
        "expiring_30d": 2,
        "payment_lunas_pct": 53,
        "cross_sell_interested": 3,
        "forecast_accuracy": 82
      }
    },
    {
      "id": "uuid",
      "slug": "kantorku",
      "name": "KantorKu",
      "stats": { ... same shape ... }
    }
  ],
  "combined": {
    "clients": 45,
    "total_value": 4750000000,
    "revenue_achieved": 5060000000,
    "revenue_target": 8000000000
  }
}
```

---

## 5. KPI Endpoints (Computed Metrics)

These endpoints return pre-computed KPI values for dashboard cards + Reports page. Each computes against the workspace's current data + an optional date range. Cached 15 min (Redis TTL, invalidated on new activity).

Common query parameters for every endpoint below:

| Param       | Required | Description                                      |
|-------------|----------|--------------------------------------------------|
| `workspace` | yes      | Workspace slug or UUID (e.g. `dealls`)           |
| `from`      | no       | ISO date `YYYY-MM-DD` — start of window          |
| `to`        | no       | ISO date `YYYY-MM-DD` — end of window (inclusive)|

Common response shape for every KPI endpoint:

```
{
  "metric": "reply_rate_sdr",
  "target": 0.15,                  // target value (fraction or absolute)
  "actual": 0.187,                 // observed value
  "variance_pct": 24.7,            // (actual - target) / target * 100
  "status": "above_target",        // above_target | on_target | below_target
  "chart_data": [                  // time series for sparkline / trend
    { "date": "2026-04-01", "value": 0.181 },
    { "date": "2026-04-02", "value": 0.192 }
  ]
}
```

`status` rule: `above_target` if `actual >= target` (or `<= target` for "lower is better" metrics like DSO, avg_sales_cycle, overdue_rate); `on_target` if within ±2% of target; else `below_target`.

### SDR KPI Endpoints

#### GET `/analytics/kpi/sdr/reply-rate`

```
Query: ?workspace=... &from=YYYY-MM-DD &to=YYYY-MM-DD

Formula:
  COUNT(action_log WHERE role='sdr' AND replied=TRUE AND status='delivered')
/ COUNT(action_log WHERE role='sdr' AND status='delivered')

Response 200:
{
  "metric": "reply_rate_sdr",
  "target": 0.15,                  // 15%
  "actual": 0.187,                 // 18.7%
  "variance_pct": 24.7,
  "status": "above_target",
  "chart_data": [{ "date": "2026-04-01", "value": 0.181 }]
}
```

#### GET `/analytics/kpi/sdr/qualified-rate`
Formula: `COUNT(verdict_sdr IN ('QUALIFIED','WARM')) / COUNT(total leads)`
Target: `0.40` (40%). Response shape as above with `"metric": "qualified_rate_sdr"`.

#### GET `/analytics/kpi/sdr/meeting-booked-rate`
Formula: `COUNT(pipeline_status='bd_meeting_scheduled') / COUNT(qualified)`
Target: `0.70` (70% of qualified). Response shape as above with `"metric": "meeting_booked_rate_sdr"`.

#### GET `/analytics/kpi/sdr/leads-contacted-per-day`
Formula: avg daily count of `first_blast_date` events over the window.
Target: `20` (≥20 leads/day). Response shape as above with `"metric": "leads_contacted_per_day_sdr"`.

### BD KPI Endpoints

#### GET `/analytics/kpi/bd/post-call-sla`
Formula: `COUNT(action_log.review_timestamp − bd_meeting_date ≤ 30m) / COUNT(bd_meeting_date)`
Target: `0.95`. Response shape as above with `"metric": "post_call_sla_bd"`.

#### GET `/analytics/kpi/bd/win-rate`
Formula: `COUNT(Stage='CLIENT') / COUNT(Stage IN ('PROSPECT','CLIENT'))`
Target: `0.25`. Response shape as above with `"metric": "win_rate_bd"`.

#### GET `/analytics/kpi/bd/first-payment-rate`
Formula: `COUNT(invoice.paid=TRUE) / COUNT(invoice WHERE created_by=bd)`
Target: `0.85`. Response shape as above with `"metric": "first_payment_rate_bd"`.

#### GET `/analytics/kpi/bd/avg-sales-cycle`
Formula: avg days between `first_blast_date` and `last_payment_date` for rows where `Payment_Status='Lunas'`.
Target: `≤30` days (lower is better). Response shape as above with `"metric": "avg_sales_cycle_bd"`.

### AE KPI Endpoints

#### GET `/analytics/kpi/ae/nrr`
Formula: `(retained_mrr + expansion_mrr − churn_mrr) / starting_mrr * 100`
Target: `≥110` (percent). Response shape as above with `"metric": "nrr_ae"`.

#### GET `/analytics/kpi/ae/renewal-rate`
Formula: `COUNT(renewed=TRUE) / COUNT(eligible_for_renewal)`
Target: `0.75`. Response shape as above with `"metric": "renewal_rate_ae"`.

#### GET `/analytics/kpi/ae/avg-nps`
Formula: `AVG(NPS_Score WHERE workspace)`
Target: `≥7`. Response shape as above with `"metric": "avg_nps_ae"`.

#### GET `/analytics/kpi/ae/dso`
Formula: avg days from invoice issue date to payment date.
Target: `≤21` days (lower is better). Response shape as above with `"metric": "dso_ae"`.

#### GET `/analytics/kpi/ae/overdue-rate`
Formula: `COUNT(Payment_Status='Terlambat') / COUNT(total)`
Target: `≤0.10` (lower is better). Response shape as above with `"metric": "overdue_rate_ae"`.

#### GET `/analytics/kpi/ae/cross-sell-rate`
Formula: `COUNT(cross_sell_interested=TRUE) / COUNT(total)`
Target: `≥0.15`. Response shape as above with `"metric": "cross_sell_rate_ae"`.

### Batch Endpoint (for dashboard loads)

#### GET `/analytics/kpi/bundle`
Returns an array of all role's KPIs in one call (avoids N round trips on dashboard mount).

```
Query params:
  ?role=sdr                        (sdr | bd | ae — required)
  &workspace=dealls                (required)
  &from=YYYY-MM-DD                 (optional)
  &to=YYYY-MM-DD                   (optional)

Response 200:
{
  "role": "sdr",
  "workspace": "dealls",
  "kpis": [
    {
      "metric": "reply_rate_sdr",
      "target": 0.15,
      "actual": 0.187,
      "variance_pct": 24.7,
      "status": "above_target",
      "chart_data": [{ "date": "2026-04-01", "value": 0.181 }]
    },
    {
      "metric": "qualified_rate_sdr",
      "target": 0.40,
      "actual": 0.38,
      "variance_pct": -5.0,
      "status": "below_target",
      "chart_data": [{ "date": "2026-04-01", "value": 0.37 }]
    }
    // ... one entry per KPI for the requested role
  ]
}
```

### Go Service Signature

```go
type KPIPoint struct {
    Date  string  `json:"date"`
    Value float64 `json:"value"`
}

type KPIResult struct {
    Metric      string     `json:"metric"`
    Target      float64    `json:"target"`
    Actual      float64    `json:"actual"`
    VariancePct float64    `json:"variance_pct"`
    Status      string     `json:"status"`      // above_target|on_target|below_target
    ChartData   []KPIPoint `json:"chart_data"`
}

type KPIService struct {
    db    *sqlx.DB
    cache *redis.Client
}

// SDR
func (s *KPIService) ReplyRateSDR(ctx context.Context, wsID uuid.UUID, from, to time.Time) (*KPIResult, error)       { /* ... */ }
func (s *KPIService) QualifiedRateSDR(ctx context.Context, wsID uuid.UUID, from, to time.Time) (*KPIResult, error)   { /* ... */ }
func (s *KPIService) MeetingBookedRateSDR(ctx context.Context, wsID uuid.UUID, from, to time.Time) (*KPIResult, error){ /* ... */ }
func (s *KPIService) LeadsContactedPerDaySDR(ctx context.Context, wsID uuid.UUID, from, to time.Time) (*KPIResult, error){ /* ... */ }

// BD
func (s *KPIService) PostCallSLABD(ctx context.Context, wsID uuid.UUID, from, to time.Time) (*KPIResult, error)      { /* ... */ }
func (s *KPIService) WinRateBD(ctx context.Context, wsID uuid.UUID, from, to time.Time) (*KPIResult, error)          { /* ... */ }
func (s *KPIService) FirstPaymentRateBD(ctx context.Context, wsID uuid.UUID, from, to time.Time) (*KPIResult, error) { /* ... */ }
func (s *KPIService) AvgSalesCycleBD(ctx context.Context, wsID uuid.UUID, from, to time.Time) (*KPIResult, error)    { /* ... */ }

// AE
func (s *KPIService) NRRAE(ctx context.Context, wsID uuid.UUID, from, to time.Time) (*KPIResult, error)              { /* ... */ }
func (s *KPIService) RenewalRateAE(ctx context.Context, wsID uuid.UUID, from, to time.Time) (*KPIResult, error)      { /* ... */ }
func (s *KPIService) AvgNPSAE(ctx context.Context, wsID uuid.UUID, from, to time.Time) (*KPIResult, error)           { /* ... */ }
func (s *KPIService) DSOAE(ctx context.Context, wsID uuid.UUID, from, to time.Time) (*KPIResult, error)              { /* ... */ }
func (s *KPIService) OverdueRateAE(ctx context.Context, wsID uuid.UUID, from, to time.Time) (*KPIResult, error)      { /* ... */ }
func (s *KPIService) CrossSellRateAE(ctx context.Context, wsID uuid.UUID, from, to time.Time) (*KPIResult, error)    { /* ... */ }

// Batch
func (s *KPIService) Bundle(ctx context.Context, role string, wsID uuid.UUID, from, to time.Time) ([]KPIResult, error) { /* ... */ }
```

All methods cache via Redis with a 15-minute TTL.

### Caching Rules

- Redis key: `kpi:{workspace_id}:{metric}:{from}:{to}`
- TTL: `900s` (15 minutes)
- Invalidate on: new `action_log` insert, new `invoice`, `stage_transition`

> **Footnote — Data Readiness**
>
> All formulas above assume real backend data. Currently the FE's KPI cards (see
> `components/features/*Section.tsx`) render mock data. Backend Phase 1 must ship
> Master Data + Invoice + Activity Log APIs before baseline metrics can be established.

---

## 6. Export

### GET `/reports/export`
Export report as Excel file.

```
Query params:
  ?tab=executive                     (executive | revenue | health | engagement | comparison)
  &format=xlsx                       (xlsx | csv | pdf)

Response headers:
  Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
  Content-Disposition: attachment; filename="report-executive-dealls-2026-04.xlsx"
```

Requires: `reports.export` permission.

---

## Design Notes

### Caching Strategy
| Endpoint                    | TTL     | Invalidation                           |
|-----------------------------|---------|-----------------------------------------|
| `/dashboard/stats`          | 5 min   | On master_data or deal write            |
| `/analytics/kpi`            | 5 min   | On master_data or deal write            |
| `/analytics/distributions`  | 10 min  | On master_data write                    |
| `/analytics/revenue-trend`  | 1 hour  | On new deal close                       |
| `/reports/*`                | 15 min  | On master_data or deal write            |

Cache key: `{endpoint}:{workspace_id}` — per workspace.
Holding cache: separate key, invalidated when any member workspace changes.

### Performance
- Distribution queries iterate all master_data records in workspace.
  For < 10K records this is fine with proper indexes.
- Revenue trend requires join with deals/invoices table — ensure monthly aggregate index.
- Holding view queries are 2x (one per member workspace) — use concurrent queries.

### Forecast Re-computation
- Recompute forecast whenever a new monthly revenue data point is available
  (i.e., at month-end or when a significant deal closes)
- Store regression parameters in a cache/table for quick retrieval
- Frontend polls revenue-trend infrequently (page load only, no polling)

### Access Control
| Endpoint              | Required Permission                          |
|-----------------------|----------------------------------------------|
| `/dashboard/stats`    | `dashboard.view_list`                        |
| `/analytics/*`        | `analytics.view_list`                        |
| `/reports/*`          | `reports.view_list`                          |
| `/reports/export`     | `reports.export`                             |
| `/reports/comparison` | `reports.view_list` + workspace.is_holding   |
