# Analytics & Reports — Backend Implementation Guide

## Context
Analytics dan Reports menyajikan data agregat dari Master Data dan Action Logs.
Tujuannya: memberikan visibilitas operasional, revenue tracking, health monitoring,
dan forecast ke stakeholder (dari AE officer sampai holding management).

Halaman yang di-cover:
1. **Dashboard Overview** (`/dashboard/[workspace]`) — ringkasan cepat semua metrik
2. **Analytics** (`/dashboard/[workspace]/analytics`) — deep-dive dengan charts interaktif
3. **Reports** (`/dashboard/[workspace]/reports`) — laporan komprehensif per tab

## Arsitektur: Real-time vs Aggregated

```
┌──────────────┐     ┌───────────────────┐     ┌──────────────┐
│  master_data  │────>│ Aggregation Layer │────>│  Frontend    │
│  action_logs  │     │ (computed on-fly  │     │  Charts &    │
│  mutation_logs│     │  or pre-cached)   │     │  KPI Cards   │
└──────────────┘     └───────────────────┘     └──────────────┘
```

### Real-time Metrics (computed on request)
- Total klien, klien aktif, by stage
- Risk distribution (High/Mid/Low)
- Payment status breakdown
- Contract expiry urgency
- NPS average & distribution
- Usage score distribution

### Aggregated Metrics (pre-computed, cached or materialized)
- Revenue trend per bulan (historical + target)
- Forecast (linear regression dari historical data)
- Deals won/lost per periode
- Cross-workspace comparison (holding view)
- Month-over-month growth rates

## KPI Definitions

### Revenue & AE Performance
| KPI                | Definisi                                                    | Source              |
|--------------------|-------------------------------------------------------------|---------------------|
| Revenue (MTD)      | Total `final_price` dari deals closed bulan ini              | master_data + deals |
| Revenue Target     | Target revenue per workspace per bulan                       | workspace_config    |
| Quota Attainment   | (Revenue MTD / Target) * 100                                | computed            |
| Deals Won          | Count deals dengan status closed-won bulan ini               | deals/pipeline      |
| Deals Lost         | Count deals dengan status closed-lost bulan ini              | deals/pipeline      |
| Win Rate           | (Deals Won / (Won + Lost)) * 100                           | computed            |
| Avg Sales Cycle    | Rata-rata hari dari lead masuk sampai deal closed            | deals/pipeline      |
| Forecast Accuracy  | Seberapa akurat forecast bulan lalu vs actual                | computed            |
| Upsell Revenue     | Revenue dari upsell/cross-sell deals                         | deals/pipeline      |

### Client Health
| KPI                | Definisi                                                    | Source              |
|--------------------|-------------------------------------------------------------|---------------------|
| Total Klien        | Count master_data records in workspace                       | master_data         |
| Klien Aktif        | Count where stage = 'ACTIVE'                                | master_data         |
| High Risk          | Count where risk_flag = 'High'                              | master_data         |
| NPS Average        | Avg of NPS_Score where NPS_Score is not null                 | master_data (custom)|
| NPS Score          | ((Promoters - Detractors) / Total respondents) * 100         | computed            |
| Usage Score Avg    | Avg of Usage_Score                                          | master_data (custom)|
| Contract Expiring  | Count where days_to_expiry between 0 and 30                 | master_data         |
| Payment Overdue    | Count where payment_status in ('Terlambat', 'Belum bayar')  | master_data         |

### Engagement
| KPI                | Definisi                                                    | Source              |
|--------------------|-------------------------------------------------------------|---------------------|
| Bot Active         | Count where bot_active = true                                | master_data         |
| NPS Replied        | Count where custom_fields.nps_replied = true                 | master_data (custom)|
| Checkin Replied    | Count where custom_fields.checkin_replied = true             | master_data (custom)|
| Cross-sell Interest| Count where custom_fields.cross_sell_interested = true       | master_data (custom)|
| Renewed            | Count where renewed = true                                   | master_data         |

## Revenue Forecast Model

### Metodologi
- **Linear regression (OLS)** dari data aktual bulanan
- Input: 10+ bulan data revenue aktual
- Output: projected revenue untuk 6 bulan ke depan
- **Confidence band**: +/- 1.5 * standard deviation dari residuals

### Data Structure (per bulan)
```
{
  month: "2025-04",
  actual: 3.24,          // miliar IDR (null jika belum ada)
  target: 5.0,           // miliar IDR
  forecast: 3.58,        // miliar IDR (null jika bulan lalu)
  forecast_high: 3.92,   // upper confidence bound
  forecast_low: 3.24     // lower confidence bound
}
```

### Computation
Backend harus:
1. Query monthly aggregated revenue dari deals/invoices
2. Fit linear regression (slope + intercept)
3. Extrapolate 6 bulan ke depan
4. Compute residual std dev untuk confidence band
5. Return array of data points (actual + forecast)

### Holding View
Untuk holding workspace:
- Return revenue data per child workspace + combined
- Forecast dihitung per-workspace kemudian di-combine
- Confidence band dari combined residuals

## Report Tabs (Reports page)

### Tab 1: Ringkasan Eksekutif
- KPI cards: Revenue, Quota, Active Clients, High Risk, NPS, Expiring Contracts
- Highlights Positif & Negatif (auto-generated dari threshold rules)
- Data flow: aggregated stats dari master_data

### Tab 2: Revenue & Kontrak
- Revenue by plan type (bar chart)
- Top clients by contract value (table)
- Contract expiry timeline (0-30d, 31-60d, 61-90d, >90d)
- Payment status breakdown
- At-risk client list

### Tab 3: Kesehatan Klien
- Risk distribution (donut chart)
- Payment status distribution
- NPS distribution (Promoter/Passive/Detractor)
- NPS by industry (horizontal bar)
- Usage score distribution

### Tab 4: Engagement & Retensi
- Bot activity rates
- NPS reply rates
- Checkin reply rates
- Cross-sell pipeline
- Renewal rates

### Tab 5: Perbandingan Workspace (holding only)
- Side-by-side comparison: Dealls vs KantorKu
- Metrics: clients, revenue, win rate, NPS, usage, risk, cross-sell, expiring

## Dashboard Overview Page
Simplified version showing:
- 4 summary cards (Revenue, Active Clients, High Risk, Expiring)
- AE Feature Card (revenue progress, quota, deals, top accounts)
- Data Master Feature Card (stage distribution, risk, payment, contract)
- Quick links to Analytics, Reports, Pipeline modules

## Workspace-aware Logic
- Single workspace: show data for that workspace only
- Holding (Sejutacita): aggregate from all member workspaces
  - Revenue = sum of all workspace revenues
  - Averages (NPS, Usage) = weighted average by client count
  - Counts = sum across workspaces
  - Comparison tab shows per-workspace breakdown
