package reports

import (
	"context"
	"fmt"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/rs/zerolog"
)

// Usecase defines the reports use case contract.
type Usecase interface {
	ExecutiveSummary(ctx context.Context, workspaceID string) (*entity.ExecutiveSummaryResponse, error)
	RevenueContracts(ctx context.Context, workspaceID string) (*entity.RevenueReportResponse, error)
	ClientHealth(ctx context.Context, workspaceID string) (*entity.HealthReportResponse, error)
	EngagementRetention(ctx context.Context, workspaceID string) (*entity.EngagementReportResponse, error)
	WorkspaceComparison(ctx context.Context, workspaceID string) (*entity.WorkspaceComparisonResponse, error)
}

type reportsUsecase struct {
	analyticsRepo repository.AnalyticsRepository
	targetRepo    repository.RevenueTargetRepository
	workspaceRepo repository.WorkspaceRepository
	logger        zerolog.Logger
}

// New creates a new reports Usecase.
func New(
	ar repository.AnalyticsRepository,
	tr repository.RevenueTargetRepository,
	wr repository.WorkspaceRepository,
	logger zerolog.Logger,
) Usecase {
	return &reportsUsecase{
		analyticsRepo: ar,
		targetRepo:    tr,
		workspaceRepo: wr,
		logger:        logger,
	}
}

func (u *reportsUsecase) resolveWorkspaceIDs(ctx context.Context, workspaceID string) ([]string, bool, error) {
	ws, err := u.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil {
		return nil, false, fmt.Errorf("resolve workspace: %w", err)
	}
	if ws.IsHolding && len(ws.MemberIDs) > 0 {
		return ws.MemberIDs, true, nil
	}
	return []string{workspaceID}, false, nil
}

func (u *reportsUsecase) ExecutiveSummary(ctx context.Context, workspaceID string) (*entity.ExecutiveSummaryResponse, error) {
	wsIDs, _, err := u.resolveWorkspaceIDs(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	kpi, err := u.analyticsRepo.KPI(ctx, wsIDs)
	if err != nil {
		return nil, fmt.Errorf("executive summary kpi: %w", err)
	}

	targets, err := u.targetRepo.List(ctx, workspaceID)
	if err != nil {
		u.logger.Warn().Err(err).Msg("Failed to load revenue targets for executive summary")
	} else {
		for _, t := range targets {
			kpi.Revenue.Target += t.TargetAmount
		}
	}
	if kpi.Revenue.Target > 0 {
		kpi.Revenue.Pct = float64(kpi.Revenue.Achieved) / float64(kpi.Revenue.Target) * 100
	}

	dist, err := u.analyticsRepo.Distributions(ctx, wsIDs)
	if err != nil {
		return nil, fmt.Errorf("executive summary distributions: %w", err)
	}

	kpiMap := map[string]interface{}{
		"revenue_achieved": kpi.Revenue.Achieved,
		"revenue_target":   kpi.Revenue.Target,
		"revenue_pct":      kpi.Revenue.Pct,
		"quota_attainment": kpi.Revenue.Pct,
		"active_clients":   kpi.Clients.Active,
		"total_clients":    kpi.Clients.Total,
		"snoozed_clients":  kpi.Clients.Snoozed,
		"high_risk":        kpi.Attention.HighRisk,
		"mid_risk":         dist.Risk.Mid,
		"low_risk":         dist.Risk.Low,
		"avg_nps":          kpi.NPS.Average,
		"nps_score":        kpi.NPS.Score,
		"nps_respondents":  kpi.NPS.Respondents,
		"expiring_30d":     kpi.Attention.Expiring30d,
		"expired":          dist.ContractExpiry.D0_30,
	}

	pos, neg := generateHighlights(kpi, dist)

	return &entity.ExecutiveSummaryResponse{
		KPI:                kpiMap,
		HighlightsPositive: pos,
		HighlightsNegative: neg,
	}, nil
}

func (u *reportsUsecase) RevenueContracts(ctx context.Context, workspaceID string) (*entity.RevenueReportResponse, error) {
	wsIDs, _, err := u.resolveWorkspaceIDs(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	dist, err := u.analyticsRepo.Distributions(ctx, wsIDs)
	if err != nil {
		return nil, fmt.Errorf("revenue contracts distributions: %w", err)
	}

	stats, err := u.analyticsRepo.DashboardStats(ctx, wsIDs)
	if err != nil {
		return nil, fmt.Errorf("revenue contracts stats: %w", err)
	}

	resp := &entity.RevenueReportResponse{
		TotalContractValue: stats.Revenue.Achieved,
		PlanTypeRevenue:    dist.PlanTypeRevenue,
		ContractExpiry: map[string]int{
			"expired": dist.ContractExpiry.D0_30,
			"0_30d":   dist.ContractExpiry.D0_30,
			"31_60d":  dist.ContractExpiry.D31_60,
			"61_90d":  dist.ContractExpiry.D61_90,
		},
		PaymentBreakdown: map[string]int{
			"lunas":     dist.PaymentStatus.Lunas,
			"menunggu":  dist.PaymentStatus.Menunggu,
			"terlambat": dist.PaymentStatus.Terlambat,
		},
		TopClients:    []entity.ReportClient{},
		AtRiskClients: []entity.ReportClient{},
		ExpiringSoon:  []entity.ReportClient{},
	}

	return resp, nil
}

func (u *reportsUsecase) ClientHealth(ctx context.Context, workspaceID string) (*entity.HealthReportResponse, error) {
	wsIDs, _, err := u.resolveWorkspaceIDs(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	dist, err := u.analyticsRepo.Distributions(ctx, wsIDs)
	if err != nil {
		return nil, fmt.Errorf("client health distributions: %w", err)
	}

	kpi, err := u.analyticsRepo.KPI(ctx, wsIDs)
	if err != nil {
		return nil, fmt.Errorf("client health kpi: %w", err)
	}

	noResponse := kpi.Clients.Total - kpi.NPS.Respondents
	if noResponse < 0 {
		noResponse = 0
	}

	resp := &entity.HealthReportResponse{
		RiskDistribution: map[string]int{
			"high": dist.Risk.High,
			"mid":  dist.Risk.Mid,
			"low":  dist.Risk.Low,
		},
		PaymentDistribution: map[string]int{
			"lunas":     dist.PaymentStatus.Lunas,
			"menunggu":  dist.PaymentStatus.Menunggu,
			"terlambat": dist.PaymentStatus.Terlambat,
		},
		NPSDistribution: map[string]int{
			"promoter":    dist.NPSDistribution.Promoter,
			"passive":     dist.NPSDistribution.Passive,
			"detractor":   dist.NPSDistribution.Detractor,
			"no_response": noResponse,
		},
		NPSByIndustry: []entity.IndustryNPS{},
		UsageDistribution: map[string]any{
			"0_25":    dist.UsageScore.R0_25,
			"26_50":   dist.UsageScore.R26_50,
			"51_75":   dist.UsageScore.R51_75,
			"76_100":  dist.UsageScore.R76_100,
			"average": dist.UsageScore.Average,
		},
		PaymentIssueClients: []entity.ReportClient{},
	}

	return resp, nil
}

func (u *reportsUsecase) EngagementRetention(ctx context.Context, workspaceID string) (*entity.EngagementReportResponse, error) {
	wsIDs, _, err := u.resolveWorkspaceIDs(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	eng, err := u.analyticsRepo.Engagement(ctx, wsIDs)
	if err != nil {
		return nil, fmt.Errorf("engagement retention: %w", err)
	}

	kpi, err := u.analyticsRepo.KPI(ctx, wsIDs)
	if err != nil {
		return nil, fmt.Errorf("engagement kpi: %w", err)
	}

	resp := &entity.EngagementReportResponse{
		BotActive:           eng.BotActive,
		BotInactive:         eng.BotInactive,
		NPSReplied:          eng.NPSReplied,
		NPSNotReplied:       kpi.Clients.Total - eng.NPSReplied,
		CheckinReplied:      eng.CheckinReplied,
		CheckinNotReplied:   kpi.Clients.Total - eng.CheckinReplied,
		CrossSellInterested: eng.CrossSellInterested,
		CrossSellRejected:   eng.CrossSellRejected,
		CrossSellPending:    kpi.Clients.Total - eng.CrossSellInterested - eng.CrossSellRejected,
		Renewed:             eng.Renewed,
		NotRenewed:          kpi.Clients.Total - eng.Renewed,
		CrossSellClients:    []entity.CrossSellClient{},
	}

	if resp.NPSNotReplied < 0 {
		resp.NPSNotReplied = 0
	}
	if resp.CheckinNotReplied < 0 {
		resp.CheckinNotReplied = 0
	}
	if resp.CrossSellPending < 0 {
		resp.CrossSellPending = 0
	}
	if resp.NotRenewed < 0 {
		resp.NotRenewed = 0
	}

	return resp, nil
}

func (u *reportsUsecase) WorkspaceComparison(ctx context.Context, workspaceID string) (*entity.WorkspaceComparisonResponse, error) {
	ws, err := u.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("workspace comparison: %w", err)
	}
	if !ws.IsHolding || len(ws.MemberIDs) == 0 {
		return nil, fmt.Errorf("workspace comparison: not a holding workspace")
	}

	resp := &entity.WorkspaceComparisonResponse{
		Workspaces: []entity.WorkspaceStats{},
		Combined:   map[string]any{},
	}

	var totalClients int
	var totalRevenue int64

	for _, memberID := range ws.MemberIDs {
		memberWS, err := u.workspaceRepo.GetByID(ctx, memberID)
		if err != nil {
			u.logger.Warn().Err(err).Str("member_id", memberID).Msg("Failed to load member workspace")
			continue
		}

		stats, err := u.analyticsRepo.DashboardStats(ctx, []string{memberID})
		if err != nil {
			u.logger.Warn().Err(err).Str("member_id", memberID).Msg("Failed to load member stats")
			continue
		}

		kpi, err := u.analyticsRepo.KPI(ctx, []string{memberID})
		if err != nil {
			u.logger.Warn().Err(err).Str("member_id", memberID).Msg("Failed to load member KPI")
			continue
		}

		wsStats := entity.WorkspaceStats{
			ID:   memberID,
			Slug: memberWS.Slug,
			Name: memberWS.Name,
			Stats: map[string]any{
				"clients":              stats.Clients.Total,
				"active_clients":       stats.Clients.Active,
				"total_value":          stats.Revenue.Achieved,
				"revenue_achieved":     stats.Revenue.Achieved,
				"revenue_target":       stats.Revenue.Target,
				"revenue_pct":         stats.Revenue.Pct,
				"deals_won":           stats.AE.DealsWon,
				"win_rate":            stats.AE.WinRate,
				"avg_nps":             kpi.NPS.Average,
				"avg_usage":           0,
				"high_risk":           stats.Risk.High,
				"expiring_30d":        stats.Contracts.Expiring30d,
				"cross_sell_interested": 0,
				"forecast_accuracy":    stats.AE.ForecastAccuracy,
			},
		}
		resp.Workspaces = append(resp.Workspaces, wsStats)

		totalClients += stats.Clients.Total
		totalRevenue += stats.Revenue.Achieved
	}

	resp.Combined = map[string]any{
		"clients":          totalClients,
		"total_value":      totalRevenue,
		"revenue_achieved": totalRevenue,
	}

	return resp, nil
}
