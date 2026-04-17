package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// AnalyticsRepository describes data access for aggregated analytics queries.
type AnalyticsRepository interface {
	DashboardStats(ctx context.Context, workspaceIDs []string) (*entity.DashboardStats, error)
	KPI(ctx context.Context, workspaceIDs []string) (*entity.KPIData, error)
	Distributions(ctx context.Context, workspaceIDs []string) (*entity.DistributionData, error)
	Engagement(ctx context.Context, workspaceIDs []string) (*entity.EngagementData, error)
}

type analyticsRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

// NewAnalyticsRepo creates an AnalyticsRepository backed by PostgreSQL.
func NewAnalyticsRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) AnalyticsRepository {
	return &analyticsRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *analyticsRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

func (r *analyticsRepo) DashboardStats(ctx context.Context, workspaceIDs []string) (*entity.DashboardStats, error) {
	ctx, span := r.tracer.Start(ctx, "analytics.repository.DashboardStats")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	wsPlaceholders, wsArgs := buildINPlaceholders(workspaceIDs, 1)

	q := fmt.Sprintf(`
		SELECT
			COALESCE(SUM(final_price), 0)                                           AS total_revenue,
			COUNT(*)                                                                  AS total_clients,
			COUNT(*) FILTER (WHERE stage = 'CLIENT')                                  AS active_clients,
			COUNT(*) FILTER (WHERE sequence_status = 'SNOOZED')                       AS snoozed_clients,
			COUNT(*) FILTER (WHERE risk_flag = 'High')                                AS high_risk,
			COUNT(*) FILTER (WHERE risk_flag = 'Mid')                                 AS mid_risk,
			COUNT(*) FILTER (WHERE risk_flag = 'Low')                                 AS low_risk,
			COUNT(*) FILTER (WHERE days_to_expiry IS NOT NULL AND days_to_expiry BETWEEN 0 AND 30)
			                                                                          AS expiring_30d,
			COUNT(*) FILTER (WHERE days_to_expiry IS NOT NULL AND days_to_expiry < 0)  AS expired
		FROM clients
		WHERE workspace_id IN (%s)
	`, wsPlaceholders)

	var stats entity.DashboardStats
	err := r.db.QueryRowContext(ctx, q, wsArgs...).Scan(
		&stats.Revenue.Achieved,
		&stats.Clients.Total,
		&stats.Clients.Active,
		&stats.Clients.Snoozed,
		&stats.Risk.High,
		&stats.Risk.Mid,
		&stats.Risk.Low,
		&stats.Contracts.Expiring30d,
		&stats.Contracts.Expired,
	)
	if err != nil {
		return nil, fmt.Errorf("analytics dashboard stats: %w", err)
	}

	// Top accounts by revenue (top 5).
	topQ := fmt.Sprintf(`
		SELECT id, company_name, final_price,
			CASE
				WHEN risk_flag = 'Low' AND final_price > 0 THEN 'expanding'
				WHEN risk_flag = 'High' THEN 'at_risk'
				ELSE 'stable'
			END AS status,
			COALESCE(TO_CHAR(last_interaction_date, 'YYYY-MM-DD'), '') AS last_contact
		FROM clients
		WHERE workspace_id IN (%s) AND stage = 'CLIENT'
		ORDER BY final_price DESC
		LIMIT 5
	`, wsPlaceholders)

	rows, err := r.db.QueryContext(ctx, topQ, wsArgs...)
	if err != nil {
		return nil, fmt.Errorf("analytics top accounts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var a entity.TopAccount
		if err := rows.Scan(&a.ID, &a.Name, &a.Revenue, &a.Status, &a.LastContact); err != nil {
			return nil, fmt.Errorf("analytics top account scan: %w", err)
		}
		stats.TopAccounts = append(stats.TopAccounts, a)
	}
	if stats.TopAccounts == nil {
		stats.TopAccounts = []entity.TopAccount{}
	}

	return &stats, rows.Err()
}

func (r *analyticsRepo) KPI(ctx context.Context, workspaceIDs []string) (*entity.KPIData, error) {
	ctx, span := r.tracer.Start(ctx, "analytics.repository.KPI")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	wsPlaceholders, wsArgs := buildINPlaceholders(workspaceIDs, 1)

	q := fmt.Sprintf(`
		SELECT
			COALESCE(SUM(final_price), 0)                                            AS revenue,
			COUNT(*)                                                                   AS total,
			COUNT(*) FILTER (WHERE stage = 'CLIENT')                                   AS active,
			COUNT(*) FILTER (WHERE sequence_status = 'SNOOZED')                        AS snoozed,
			COALESCE(AVG(CASE WHEN (custom_fields->>'nps_score')::numeric IS NOT NULL
				THEN (custom_fields->>'nps_score')::numeric END), 0)                   AS nps_avg,
			COUNT(*) FILTER (WHERE (custom_fields->>'nps_score')::numeric IS NOT NULL) AS nps_respondents,
			COUNT(*) FILTER (WHERE (custom_fields->>'nps_score')::numeric >= 9)        AS promoter,
			COUNT(*) FILTER (WHERE (custom_fields->>'nps_score')::numeric BETWEEN 7 AND 8) AS passive,
			COUNT(*) FILTER (WHERE (custom_fields->>'nps_score')::numeric <= 6)        AS detractor,
			COUNT(*) FILTER (WHERE risk_flag = 'High')                                 AS high_risk,
			COUNT(*) FILTER (WHERE days_to_expiry IS NOT NULL AND days_to_expiry BETWEEN 0 AND 30)
			                                                                           AS expiring_30d
		FROM clients
		WHERE workspace_id IN (%s)
	`, wsPlaceholders)

	var kpi entity.KPIData
	var npsAvg float64
	var npsRespondents, promoter, passive, detractor int
	err := r.db.QueryRowContext(ctx, q, wsArgs...).Scan(
		&kpi.Revenue.Achieved,
		&kpi.Clients.Total,
		&kpi.Clients.Active,
		&kpi.Clients.Snoozed,
		&npsAvg,
		&npsRespondents,
		&promoter,
		&passive,
		&detractor,
		&kpi.Attention.HighRisk,
		&kpi.Attention.Expiring30d,
	)
	if err != nil {
		return nil, fmt.Errorf("analytics kpi: %w", err)
	}

	kpi.NPS.Average = npsAvg
	kpi.NPS.Respondents = npsRespondents
	kpi.NPS.Total = kpi.Clients.Total
	kpi.NPS.Promoter = promoter
	kpi.NPS.Passive = passive
	kpi.NPS.Detractor = detractor
	if npsRespondents > 0 {
		kpi.NPS.Score = int(float64(promoter-detractor) / float64(npsRespondents) * 100)
	}

	return &kpi, nil
}

func (r *analyticsRepo) Distributions(ctx context.Context, workspaceIDs []string) (*entity.DistributionData, error) {
	ctx, span := r.tracer.Start(ctx, "analytics.repository.Distributions")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	wsPlaceholders, wsArgs := buildINPlaceholders(workspaceIDs, 1)

	q := fmt.Sprintf(`
		SELECT
			COUNT(*) FILTER (WHERE risk_flag = 'High')     AS risk_high,
			COUNT(*) FILTER (WHERE risk_flag = 'Mid')      AS risk_mid,
			COUNT(*) FILTER (WHERE risk_flag = 'Low')      AS risk_low,
			COUNT(*) FILTER (WHERE payment_status = 'Lunas')    AS pay_lunas,
			COUNT(*) FILTER (WHERE payment_status = 'Menunggu') AS pay_menunggu,
			COUNT(*) FILTER (WHERE payment_status IN ('Terlambat','Belum bayar')) AS pay_terlambat,
			COUNT(*) FILTER (WHERE days_to_expiry IS NOT NULL AND days_to_expiry BETWEEN 0 AND 30)  AS exp_0_30,
			COUNT(*) FILTER (WHERE days_to_expiry IS NOT NULL AND days_to_expiry BETWEEN 31 AND 60) AS exp_31_60,
			COUNT(*) FILTER (WHERE days_to_expiry IS NOT NULL AND days_to_expiry BETWEEN 61 AND 90) AS exp_61_90,
			COUNT(*) FILTER (WHERE days_to_expiry IS NOT NULL AND days_to_expiry > 90)              AS exp_90_plus,
			COUNT(*) FILTER (WHERE (custom_fields->>'nps_score')::numeric >= 9)        AS nps_promoter,
			COUNT(*) FILTER (WHERE (custom_fields->>'nps_score')::numeric BETWEEN 7 AND 8) AS nps_passive,
			COUNT(*) FILTER (WHERE (custom_fields->>'nps_score')::numeric <= 6)        AS nps_detractor,
			COUNT(*) FILTER (WHERE (custom_fields->>'usage_score')::numeric BETWEEN 0 AND 25)   AS usage_0_25,
			COUNT(*) FILTER (WHERE (custom_fields->>'usage_score')::numeric BETWEEN 26 AND 50)  AS usage_26_50,
			COUNT(*) FILTER (WHERE (custom_fields->>'usage_score')::numeric BETWEEN 51 AND 75)  AS usage_51_75,
			COUNT(*) FILTER (WHERE (custom_fields->>'usage_score')::numeric BETWEEN 76 AND 100) AS usage_76_100,
			COALESCE(AVG(CASE WHEN (custom_fields->>'usage_score')::numeric IS NOT NULL
				THEN (custom_fields->>'usage_score')::numeric END), 0)                  AS usage_avg,
			COUNT(*) FILTER (WHERE bot_active = TRUE)   AS bot_active,
			COUNT(*) FILTER (WHERE bot_active = FALSE)  AS bot_inactive,
			COUNT(*) FILTER (WHERE (custom_fields->>'nps_replied')::boolean = TRUE)    AS nps_replied,
			COUNT(*) FILTER (WHERE (custom_fields->>'checkin_replied')::boolean = TRUE) AS checkin_replied,
			COUNT(*) FILTER (WHERE (custom_fields->>'cross_sell_interested')::boolean = TRUE)  AS cross_sell_interested,
			COUNT(*) FILTER (WHERE (custom_fields->>'cross_sell_interested')::boolean = FALSE) AS cross_sell_rejected,
			COUNT(*) FILTER (WHERE renewed = TRUE) AS renewed
		FROM clients
		WHERE workspace_id IN (%s)
	`, wsPlaceholders)

	var d entity.DistributionData
	err := r.db.QueryRowContext(ctx, q, wsArgs...).Scan(
		&d.Risk.High, &d.Risk.Mid, &d.Risk.Low,
		&d.PaymentStatus.Lunas, &d.PaymentStatus.Menunggu, &d.PaymentStatus.Terlambat,
		&d.ContractExpiry.D0_30, &d.ContractExpiry.D31_60, &d.ContractExpiry.D61_90, &d.ContractExpiry.D90Plus,
		&d.NPSDistribution.Promoter, &d.NPSDistribution.Passive, &d.NPSDistribution.Detractor,
		&d.UsageScore.R0_25, &d.UsageScore.R26_50, &d.UsageScore.R51_75, &d.UsageScore.R76_100,
		&d.UsageScore.Average,
		&d.Engagement.BotActive, &d.Engagement.BotInactive,
		&d.Engagement.NPSReplied, &d.Engagement.CheckinReplied,
		&d.Engagement.CrossSellInterested, &d.Engagement.CrossSellRejected,
		&d.Engagement.Renewed,
	)
	if err != nil {
		return nil, fmt.Errorf("analytics distributions: %w", err)
	}

	// Plan type revenue breakdown.
	planQ := fmt.Sprintf(`
		SELECT COALESCE(custom_fields->>'plan_type', 'Unknown') AS plan, COALESCE(SUM(final_price), 0) AS revenue
		FROM clients WHERE workspace_id IN (%s) AND stage = 'CLIENT'
		GROUP BY 1 ORDER BY revenue DESC
	`, wsPlaceholders)
	planRows, err := r.db.QueryContext(ctx, planQ, wsArgs...)
	if err != nil {
		return nil, fmt.Errorf("analytics plan revenue: %w", err)
	}
	defer planRows.Close()
	for planRows.Next() {
		var p entity.PlanRevenue
		if err := planRows.Scan(&p.Plan, &p.Revenue); err != nil {
			return nil, fmt.Errorf("analytics plan scan: %w", err)
		}
		d.PlanTypeRevenue = append(d.PlanTypeRevenue, p)
	}
	if d.PlanTypeRevenue == nil {
		d.PlanTypeRevenue = []entity.PlanRevenue{}
	}

	// Industry breakdown.
	indQ := fmt.Sprintf(`
		SELECT COALESCE(custom_fields->>'industry', 'Unknown') AS industry, COUNT(*) AS cnt
		FROM clients WHERE workspace_id IN (%s)
		GROUP BY 1 ORDER BY cnt DESC
	`, wsPlaceholders)
	indRows, err := r.db.QueryContext(ctx, indQ, wsArgs...)
	if err != nil {
		return nil, fmt.Errorf("analytics industry: %w", err)
	}
	defer indRows.Close()
	for indRows.Next() {
		var ic entity.IndustryCount
		if err := indRows.Scan(&ic.Industry, &ic.Count); err != nil {
			return nil, fmt.Errorf("analytics industry scan: %w", err)
		}
		d.IndustryBreakdown = append(d.IndustryBreakdown, ic)
	}
	if d.IndustryBreakdown == nil {
		d.IndustryBreakdown = []entity.IndustryCount{}
	}

	// HC size breakdown.
	hcQ := fmt.Sprintf(`
		SELECT
			CASE
				WHEN (custom_fields->>'hc_size')::int BETWEEN 1 AND 10 THEN '1-10'
				WHEN (custom_fields->>'hc_size')::int BETWEEN 11 AND 50 THEN '11-50'
				WHEN (custom_fields->>'hc_size')::int BETWEEN 51 AND 100 THEN '51-100'
				WHEN (custom_fields->>'hc_size')::int BETWEEN 101 AND 200 THEN '101-200'
				WHEN (custom_fields->>'hc_size')::int BETWEEN 201 AND 500 THEN '201-500'
				WHEN (custom_fields->>'hc_size')::int > 500 THEN '500+'
				ELSE 'Unknown'
			END AS hc_range,
			COUNT(*) AS cnt
		FROM clients WHERE workspace_id IN (%s)
			AND custom_fields->>'hc_size' IS NOT NULL
			AND custom_fields->>'hc_size' ~ '^\d+$'
		GROUP BY 1 ORDER BY MIN((custom_fields->>'hc_size')::int)
	`, wsPlaceholders)
	hcRows, err := r.db.QueryContext(ctx, hcQ, wsArgs...)
	if err != nil {
		return nil, fmt.Errorf("analytics hc size: %w", err)
	}
	defer hcRows.Close()
	for hcRows.Next() {
		var hc entity.HCSizeCount
		if err := hcRows.Scan(&hc.Range, &hc.Count); err != nil {
			return nil, fmt.Errorf("analytics hc scan: %w", err)
		}
		d.HCSizeBreakdown = append(d.HCSizeBreakdown, hc)
	}
	if d.HCSizeBreakdown == nil {
		d.HCSizeBreakdown = []entity.HCSizeCount{}
	}

	// Value tier breakdown.
	vtQ := fmt.Sprintf(`
		SELECT COALESCE(custom_fields->>'value_tier', 'Unknown') AS tier, COUNT(*) AS cnt
		FROM clients WHERE workspace_id IN (%s)
			AND custom_fields->>'value_tier' IS NOT NULL
		GROUP BY 1 ORDER BY tier
	`, wsPlaceholders)
	vtRows, err := r.db.QueryContext(ctx, vtQ, wsArgs...)
	if err != nil {
		return nil, fmt.Errorf("analytics value tier: %w", err)
	}
	defer vtRows.Close()
	for vtRows.Next() {
		var vt entity.ValueTierCount
		if err := vtRows.Scan(&vt.Tier, &vt.Count); err != nil {
			return nil, fmt.Errorf("analytics vt scan: %w", err)
		}
		d.ValueTierBreakdown = append(d.ValueTierBreakdown, vt)
	}
	if d.ValueTierBreakdown == nil {
		d.ValueTierBreakdown = []entity.ValueTierCount{}
	}

	return &d, nil
}

func (r *analyticsRepo) Engagement(ctx context.Context, workspaceIDs []string) (*entity.EngagementData, error) {
	ctx, span := r.tracer.Start(ctx, "analytics.repository.Engagement")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	wsPlaceholders, wsArgs := buildINPlaceholders(workspaceIDs, 1)

	q := fmt.Sprintf(`
		SELECT
			COUNT(*) FILTER (WHERE bot_active = TRUE)   AS bot_active,
			COUNT(*) FILTER (WHERE bot_active = FALSE)  AS bot_inactive,
			COUNT(*) FILTER (WHERE (custom_fields->>'nps_replied')::boolean = TRUE)    AS nps_replied,
			COUNT(*) FILTER (WHERE (custom_fields->>'checkin_replied')::boolean = TRUE) AS checkin_replied,
			COUNT(*) FILTER (WHERE (custom_fields->>'cross_sell_interested')::boolean = TRUE)  AS cross_sell_interested,
			COUNT(*) FILTER (WHERE (custom_fields->>'cross_sell_interested')::boolean = FALSE) AS cross_sell_rejected,
			COUNT(*) FILTER (WHERE renewed = TRUE) AS renewed
		FROM clients
		WHERE workspace_id IN (%s)
	`, wsPlaceholders)

	var e entity.EngagementData
	err := r.db.QueryRowContext(ctx, q, wsArgs...).Scan(
		&e.BotActive, &e.BotInactive,
		&e.NPSReplied, &e.CheckinReplied,
		&e.CrossSellInterested, &e.CrossSellRejected,
		&e.Renewed,
	)
	if err != nil {
		return nil, fmt.Errorf("analytics engagement: %w", err)
	}
	return &e, nil
}

// buildINPlaceholders creates "$1,$2,$3" style placeholders and corresponding args.
func buildINPlaceholders(ids []string, startIdx int) (string, []interface{}) {
	args := make([]interface{}, len(ids))
	placeholders := ""
	for i, id := range ids {
		if i > 0 {
			placeholders += ","
		}
		placeholders += fmt.Sprintf("$%d", startIdx+i)
		args[i] = id
	}
	return placeholders, args
}
