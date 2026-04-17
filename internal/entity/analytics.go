package entity

import "time"

// DashboardStats is the response shape for GET /dashboard/stats.
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
		High int `json:"high"`
		Mid  int `json:"mid"`
		Low  int `json:"low"`
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

// TopAccount is a high-value client for the dashboard.
type TopAccount struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Revenue     int64  `json:"revenue"`
	Status      string `json:"status"`
	LastContact string `json:"last_contact"`
}

// KPIData is the response shape for GET /analytics/kpi.
type KPIData struct {
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
	NPS struct {
		Average     float64 `json:"average"`
		Score       int     `json:"score"`
		Respondents int     `json:"respondents"`
		Total       int     `json:"total_clients"`
		Promoter    int     `json:"promoter"`
		Passive     int     `json:"passive"`
		Detractor   int     `json:"detractor"`
	} `json:"nps"`
	Attention struct {
		HighRisk    int `json:"high_risk"`
		Expiring30d int `json:"expiring_30d"`
	} `json:"attention"`
	AE struct {
		QuotaAttainment  float64 `json:"quota_attainment"`
		DealsWon         int     `json:"deals_won"`
		DealsLost        int     `json:"deals_lost"`
		WinRate          float64 `json:"win_rate"`
		AvgSalesCycle    int     `json:"avg_sales_cycle"`
		ForecastAccuracy float64 `json:"forecast_accuracy"`
		UpsellRevenue    int64   `json:"upsell_revenue"`
	} `json:"ae"`
}

// DistributionData is the response shape for GET /analytics/distributions.
type DistributionData struct {
	Risk struct {
		High int `json:"high"`
		Mid  int `json:"mid"`
		Low  int `json:"low"`
	} `json:"risk"`
	PaymentStatus struct {
		Lunas    int `json:"lunas"`
		Menunggu int `json:"menunggu"`
		Terlambat int `json:"terlambat"`
	} `json:"payment_status"`
	ContractExpiry struct {
		D0_30   int `json:"0_30d"`
		D31_60  int `json:"31_60d"`
		D61_90  int `json:"61_90d"`
		D90Plus int `json:"90d_plus"`
	} `json:"contract_expiry"`
	NPSDistribution struct {
		Promoter  int `json:"promoter"`
		Passive   int `json:"passive"`
		Detractor int `json:"detractor"`
	} `json:"nps_distribution"`
	UsageScore struct {
		R0_25   int     `json:"0_25"`
		R26_50  int     `json:"26_50"`
		R51_75  int     `json:"51_75"`
		R76_100 int     `json:"76_100"`
		Average float64 `json:"average"`
	} `json:"usage_score"`
	PlanTypeRevenue []PlanRevenue    `json:"plan_type_revenue"`
	IndustryBreakdown []IndustryCount `json:"industry_breakdown"`
	HCSizeBreakdown   []HCSizeCount   `json:"hc_size_breakdown"`
	ValueTierBreakdown []ValueTierCount `json:"value_tier_breakdown"`
	Engagement EngagementData `json:"engagement"`
}

// PlanRevenue is one plan type and its revenue.
type PlanRevenue struct {
	Plan    string `json:"plan"`
	Revenue int64  `json:"revenue"`
}

// IndustryCount is one industry and its client count.
type IndustryCount struct {
	Industry string `json:"industry"`
	Count    int    `json:"count"`
}

// HCSizeCount is one headcount range and its count.
type HCSizeCount struct {
	Range string `json:"range"`
	Count int    `json:"count"`
}

// ValueTierCount is one tier and its count.
type ValueTierCount struct {
	Tier  string `json:"tier"`
	Count int    `json:"count"`
}

// EngagementData is the engagement metrics block.
type EngagementData struct {
	BotActive           int `json:"bot_active"`
	BotInactive         int `json:"bot_inactive"`
	NPSReplied          int `json:"nps_replied"`
	CheckinReplied      int `json:"checkin_replied"`
	CrossSellInterested int `json:"cross_sell_interested"`
	CrossSellRejected   int `json:"cross_sell_rejected"`
	Renewed             int `json:"renewed"`
}

// RevenueDataPoint is one month of revenue data (actual or forecast).
type RevenueDataPoint struct {
	Month        string   `json:"month"`
	Date         string   `json:"date"`
	Actual       *float64 `json:"actual"`
	Target       float64  `json:"target"`
	Forecast     *float64 `json:"forecast"`
	ForecastHigh *float64 `json:"forecast_high"`
	ForecastLow  *float64 `json:"forecast_low"`
	IsForecast   bool     `json:"is_forecast"`
}

// RevenueTrendResponse is the response shape for GET /analytics/revenue-trend.
type RevenueTrendResponse struct {
	Workspace string             `json:"workspace"`
	Target    float64            `json:"target"`
	Data      []RevenueDataPoint `json:"data"`
	Regression struct {
		Slope       float64 `json:"slope"`
		Intercept   float64 `json:"intercept"`
		RSquared    float64 `json:"r_squared"`
		ResidualStd float64 `json:"residual_std"`
	} `json:"regression"`
	Summary struct {
		LastActual  float64 `json:"last_actual"`
		GrowthPct   float64 `json:"growth_pct"`
		ForecastEnd float64 `json:"forecast_end"`
	} `json:"summary"`
}

// RevenueTarget is a monthly revenue target for a workspace.
type RevenueTarget struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspace_id"`
	Year         int       `json:"year"`
	Month        int       `json:"month"`
	TargetAmount int64     `json:"target_amount"`
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// RevenueSnapshot is a materialized monthly aggregate.
type RevenueSnapshot struct {
	ID            string    `json:"id"`
	WorkspaceID   string    `json:"workspace_id"`
	Year          int       `json:"year"`
	Month         int       `json:"month"`
	RevenueActual int64     `json:"revenue_actual"`
	DealsWon      int       `json:"deals_won"`
	DealsLost     int       `json:"deals_lost"`
	ComputedAt    time.Time `json:"computed_at"`
}

// ExecutiveSummaryResponse is the response shape for GET /reports/executive.
type ExecutiveSummaryResponse struct {
	KPI                map[string]interface{} `json:"kpi"`
	HighlightsPositive []string               `json:"highlights_positive"`
	HighlightsNegative []string               `json:"highlights_negative"`
}

// RevenueReportResponse is the response shape for GET /reports/revenue.
type RevenueReportResponse struct {
	TotalContractValue int64              `json:"total_contract_value"`
	OverdueValue       int64              `json:"overdue_value"`
	PlanTypeRevenue    []PlanRevenue      `json:"plan_type_revenue"`
	TopClients         []ReportClient     `json:"top_clients"`
	ContractExpiry     map[string]int     `json:"contract_expiry"`
	PaymentBreakdown   map[string]int     `json:"payment_breakdown"`
	AtRiskClients      []ReportClient     `json:"at_risk_clients"`
	ExpiringSoon       []ReportClient     `json:"expiring_soon"`
}

// ReportClient is a client row in report tables.
type ReportClient struct {
	CompanyID     string `json:"company_id"`
	CompanyName   string `json:"company_name"`
	FinalPrice    int64  `json:"final_price"`
	PlanType      string `json:"plan_type,omitempty"`
	PaymentStatus string `json:"payment_status,omitempty"`
	DaysToExpiry  int    `json:"days_to_expiry"`
	RiskFlag      string `json:"risk_flag,omitempty"`
	ContractEnd   string `json:"contract_end,omitempty"`
	Renewed       bool   `json:"renewed,omitempty"`
	DaysOverdue   int    `json:"days_overdue,omitempty"`
}

// HealthReportResponse is the response shape for GET /reports/health.
type HealthReportResponse struct {
	RiskDistribution    map[string]int      `json:"risk_distribution"`
	PaymentDistribution map[string]int      `json:"payment_distribution"`
	NPSDistribution     map[string]int      `json:"nps_distribution"`
	NPSByIndustry       []IndustryNPS       `json:"nps_by_industry"`
	UsageDistribution   map[string]any      `json:"usage_distribution"`
	PaymentIssueClients []ReportClient      `json:"payment_issue_clients"`
}

// IndustryNPS is one industry and its average NPS.
type IndustryNPS struct {
	Industry string  `json:"industry"`
	AvgNPS   float64 `json:"avg_nps"`
}

// EngagementReportResponse is the response shape for GET /reports/engagement.
type EngagementReportResponse struct {
	BotActive           int              `json:"bot_active"`
	BotInactive         int              `json:"bot_inactive"`
	NPSReplied          int              `json:"nps_replied"`
	NPSNotReplied       int              `json:"nps_not_replied"`
	CheckinReplied      int              `json:"checkin_replied"`
	CheckinNotReplied   int              `json:"checkin_not_replied"`
	CrossSellInterested int              `json:"cross_sell_interested"`
	CrossSellRejected   int              `json:"cross_sell_rejected"`
	CrossSellPending    int              `json:"cross_sell_pending"`
	Renewed             int              `json:"renewed"`
	NotRenewed          int              `json:"not_renewed"`
	CrossSellClients    []CrossSellClient `json:"cross_sell_clients"`
}

// CrossSellClient is one cross-sell opportunity.
type CrossSellClient struct {
	CompanyID    string `json:"company_id"`
	CompanyName  string `json:"company_name"`
	CurrentPlan  string `json:"current_plan"`
	InterestedIn string `json:"interested_in"`
}

// WorkspaceComparisonResponse is the response shape for GET /reports/comparison.
type WorkspaceComparisonResponse struct {
	Workspaces []WorkspaceStats `json:"workspaces"`
	Combined   map[string]any   `json:"combined"`
}

// WorkspaceStats is one workspace in the comparison.
type WorkspaceStats struct {
	ID    string         `json:"id"`
	Slug  string         `json:"slug"`
	Name  string         `json:"name"`
	Stats map[string]any `json:"stats"`
}

// ExportReportRequest is the request body for POST /reports/export.
type ExportReportRequest struct {
	Tab    string `json:"tab"`
	Format string `json:"format"`
}
