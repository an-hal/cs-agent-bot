package analytics

import (
	"context"
	"fmt"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/rs/zerolog"
)

// Usecase defines the analytics use case contract.
type Usecase interface {
	DashboardStats(ctx context.Context, workspaceID string) (*entity.DashboardStats, error)
	KPI(ctx context.Context, workspaceID string) (*entity.KPIData, error)
	Distributions(ctx context.Context, workspaceID string) (*entity.DistributionData, error)
	Engagement(ctx context.Context, workspaceID string) (*entity.EngagementData, error)
	RevenueTrend(ctx context.Context, workspaceID string, months int) (*entity.RevenueTrendResponse, error)
	ForecastAccuracy(ctx context.Context, workspaceID string) (float64, error)
	RebuildSnapshots(ctx context.Context, workspaceID string) error
}

type analyticsUsecase struct {
	analyticsRepo repository.AnalyticsRepository
	targetRepo    repository.RevenueTargetRepository
	snapshotRepo  repository.RevenueSnapshotRepository
	workspaceRepo repository.WorkspaceRepository
	logger        zerolog.Logger
}

// New creates a new analytics Usecase.
func New(
	ar repository.AnalyticsRepository,
	tr repository.RevenueTargetRepository,
	sr repository.RevenueSnapshotRepository,
	wr repository.WorkspaceRepository,
	logger zerolog.Logger,
) Usecase {
	return &analyticsUsecase{
		analyticsRepo: ar,
		targetRepo:    tr,
		snapshotRepo:  sr,
		workspaceRepo: wr,
		logger:        logger,
	}
}

func (u *analyticsUsecase) resolveWorkspaceIDs(ctx context.Context, workspaceID string) ([]string, error) {
	ws, err := u.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace: %w", err)
	}
	if ws.IsHolding && len(ws.MemberIDs) > 0 {
		return ws.MemberIDs, nil
	}
	return []string{workspaceID}, nil
}

func (u *analyticsUsecase) DashboardStats(ctx context.Context, workspaceID string) (*entity.DashboardStats, error) {
	wsIDs, err := u.resolveWorkspaceIDs(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	stats, err := u.analyticsRepo.DashboardStats(ctx, wsIDs)
	if err != nil {
		return nil, err
	}

	// Get revenue target for the current month.
	targets, err := u.targetRepo.List(ctx, workspaceID)
	if err != nil {
		u.logger.Warn().Err(err).Msg("Failed to load revenue targets")
	} else {
		for _, t := range targets {
			stats.Revenue.Target += t.TargetAmount
		}
	}

	if stats.Revenue.Target > 0 {
		stats.Revenue.Pct = float64(stats.Revenue.Achieved) / float64(stats.Revenue.Target) * 100
	}

	// AE stats from snapshot data.
	snapshots, err := u.snapshotRepo.List(ctx, workspaceID, 18)
	if err != nil {
		u.logger.Warn().Err(err).Msg("Failed to load revenue snapshots")
	} else {
		totalWon, totalLost := 0, 0
		for _, s := range snapshots {
			totalWon += s.DealsWon
			totalLost += s.DealsLost
		}
		stats.AE.DealsWon = totalWon
		stats.AE.DealsLost = totalLost
		if totalWon+totalLost > 0 {
			stats.AE.WinRate = float64(totalWon) / float64(totalWon+totalLost) * 100
		}
		if stats.Revenue.Target > 0 {
			stats.AE.QuotaAttainment = stats.Revenue.Pct
		}
	}

	return stats, nil
}

func (u *analyticsUsecase) KPI(ctx context.Context, workspaceID string) (*entity.KPIData, error) {
	wsIDs, err := u.resolveWorkspaceIDs(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	kpi, err := u.analyticsRepo.KPI(ctx, wsIDs)
	if err != nil {
		return nil, err
	}

	targets, err := u.targetRepo.List(ctx, workspaceID)
	if err != nil {
		u.logger.Warn().Err(err).Msg("Failed to load revenue targets for KPI")
	} else {
		for _, t := range targets {
			kpi.Revenue.Target += t.TargetAmount
		}
	}

	if kpi.Revenue.Target > 0 {
		kpi.Revenue.Pct = float64(kpi.Revenue.Achieved) / float64(kpi.Revenue.Target) * 100
		kpi.AE.QuotaAttainment = kpi.Revenue.Pct
	}

	return kpi, nil
}

func (u *analyticsUsecase) Distributions(ctx context.Context, workspaceID string) (*entity.DistributionData, error) {
	wsIDs, err := u.resolveWorkspaceIDs(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return u.analyticsRepo.Distributions(ctx, wsIDs)
}

func (u *analyticsUsecase) Engagement(ctx context.Context, workspaceID string) (*entity.EngagementData, error) {
	wsIDs, err := u.resolveWorkspaceIDs(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return u.analyticsRepo.Engagement(ctx, wsIDs)
}

func (u *analyticsUsecase) RebuildSnapshots(ctx context.Context, workspaceID string) error {
	return u.snapshotRepo.RebuildFromInvoices(ctx, workspaceID, 18)
}
