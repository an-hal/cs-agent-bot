package reports_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/reports"
	"github.com/rs/zerolog"
)

// ─── mock repos ──────────────────────────────────────────────────────────────

type mockAnalyticsRepo struct {
	dashStats  *entity.DashboardStats
	dashErr    error
	kpi        *entity.KPIData
	kpiErr     error
	dist       *entity.DistributionData
	distErr    error
	engagement *entity.EngagementData
	engErr     error
}

func (m *mockAnalyticsRepo) DashboardStats(_ context.Context, _ []string) (*entity.DashboardStats, error) {
	return m.dashStats, m.dashErr
}
func (m *mockAnalyticsRepo) KPI(_ context.Context, _ []string) (*entity.KPIData, error) {
	return m.kpi, m.kpiErr
}
func (m *mockAnalyticsRepo) Distributions(_ context.Context, _ []string) (*entity.DistributionData, error) {
	return m.dist, m.distErr
}
func (m *mockAnalyticsRepo) Engagement(_ context.Context, _ []string) (*entity.EngagementData, error) {
	return m.engagement, m.engErr
}

type mockTargetRepo struct {
	targets []entity.RevenueTarget
	listErr error
}

func (m *mockTargetRepo) List(_ context.Context, _ string) ([]entity.RevenueTarget, error) {
	return m.targets, m.listErr
}
func (m *mockTargetRepo) Upsert(_ context.Context, _ entity.RevenueTarget) error { return nil }

type mockWorkspaceRepo struct {
	ws  *entity.Workspace
	err error
}

func (m *mockWorkspaceRepo) GetAll(context.Context) ([]entity.Workspace, error) {
	if m.ws != nil {
		return []entity.Workspace{*m.ws}, nil
	}
	return nil, nil
}
func (m *mockWorkspaceRepo) GetByID(_ context.Context, _ string) (*entity.Workspace, error) {
	return m.ws, m.err
}
func (m *mockWorkspaceRepo) GetBySlug(context.Context, string) (*entity.Workspace, error) {
	return nil, nil
}
func (m *mockWorkspaceRepo) ListForUser(context.Context, string) ([]entity.Workspace, error) {
	return nil, nil
}
func (m *mockWorkspaceRepo) Create(context.Context, *entity.Workspace) (*entity.Workspace, error) {
	return nil, nil
}
func (m *mockWorkspaceRepo) Update(context.Context, string, repository.WorkspacePatch) (*entity.Workspace, error) {
	return nil, nil
}
func (m *mockWorkspaceRepo) SoftDelete(context.Context, string) error { return nil }

var _ repository.AnalyticsRepository = (*mockAnalyticsRepo)(nil)
var _ repository.RevenueTargetRepository = (*mockTargetRepo)(nil)
var _ repository.WorkspaceRepository = (*mockWorkspaceRepo)(nil)

func defaultKPI() *entity.KPIData {
	kpi := &entity.KPIData{}
	kpi.Revenue.Achieved = 3_240_000_000
	kpi.Clients.Total = 15
	kpi.Clients.Active = 12
	kpi.Clients.Snoozed = 3
	kpi.NPS.Average = 7.8
	kpi.NPS.Score = 42
	kpi.NPS.Respondents = 13
	kpi.NPS.Promoter = 6
	kpi.NPS.Passive = 4
	kpi.NPS.Detractor = 3
	kpi.Attention.HighRisk = 2
	kpi.Attention.Expiring30d = 3
	return kpi
}

func defaultDist() *entity.DistributionData {
	d := &entity.DistributionData{}
	d.Risk.High = 2
	d.Risk.Mid = 4
	d.Risk.Low = 9
	d.PaymentStatus.Lunas = 8
	d.PaymentStatus.Menunggu = 4
	d.PaymentStatus.Terlambat = 3
	d.ContractExpiry.D0_30 = 3
	d.ContractExpiry.D31_60 = 4
	d.ContractExpiry.D61_90 = 2
	d.NPSDistribution.Promoter = 6
	d.NPSDistribution.Passive = 4
	d.NPSDistribution.Detractor = 3
	d.UsageScore.R0_25 = 2
	d.UsageScore.R26_50 = 3
	d.UsageScore.R51_75 = 5
	d.UsageScore.R76_100 = 5
	d.UsageScore.Average = 62
	d.Engagement.BotActive = 12
	d.Engagement.CrossSellInterested = 4
	d.Engagement.Renewed = 6
	d.PlanTypeRevenue = []entity.PlanRevenue{{Plan: "Enterprise", Revenue: 1_800_000_000}}
	d.IndustryBreakdown = []entity.IndustryCount{{Industry: "Tech", Count: 4}}
	d.HCSizeBreakdown = []entity.HCSizeCount{{Range: "51-100", Count: 4}}
	d.ValueTierBreakdown = []entity.ValueTierCount{{Tier: "Tier 1", Count: 3}}
	return d
}

func singleWS() *mockWorkspaceRepo {
	return &mockWorkspaceRepo{ws: &entity.Workspace{ID: "ws-1", Slug: "dealls", IsHolding: false}}
}

func newUC(ar *mockAnalyticsRepo, tr *mockTargetRepo, wr *mockWorkspaceRepo) reports.Usecase {
	return reports.New(ar, tr, wr, zerolog.Nop())
}

// ─── ExecutiveSummary ────────────────────────────────────────────────────────

func TestExecutiveSummary_Success(t *testing.T) {
	ar := &mockAnalyticsRepo{kpi: defaultKPI(), dist: defaultDist()}
	tr := &mockTargetRepo{targets: []entity.RevenueTarget{{TargetAmount: 5_000_000_000}}}
	uc := newUC(ar, tr, singleWS())

	result, err := uc.ExecutiveSummary(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.KPI == nil {
		t.Fatal("KPI map should not be nil")
	}
	if len(result.HighlightsPositive) == 0 {
		t.Error("expected at least one positive highlight")
	}
	if len(result.HighlightsNegative) == 0 {
		t.Error("expected at least one negative highlight")
	}
}

func TestExecutiveSummary_KPIError(t *testing.T) {
	ar := &mockAnalyticsRepo{kpiErr: errors.New("fail")}
	uc := newUC(ar, &mockTargetRepo{}, singleWS())

	_, err := uc.ExecutiveSummary(context.Background(), "ws-1")
	if err == nil {
		t.Error("expected error")
	}
}

func TestExecutiveSummary_DistError(t *testing.T) {
	ar := &mockAnalyticsRepo{kpi: defaultKPI(), distErr: errors.New("fail")}
	uc := newUC(ar, &mockTargetRepo{}, singleWS())

	_, err := uc.ExecutiveSummary(context.Background(), "ws-1")
	if err == nil {
		t.Error("expected error")
	}
}

func TestExecutiveSummary_WorkspaceError(t *testing.T) {
	ar := &mockAnalyticsRepo{}
	wr := &mockWorkspaceRepo{err: errors.New("not found")}
	uc := newUC(ar, &mockTargetRepo{}, wr)

	_, err := uc.ExecutiveSummary(context.Background(), "ws-bad")
	if err == nil {
		t.Error("expected error when workspace not found")
	}
}

// ─── RevenueContracts ────────────────────────────────────────────────────────

func TestRevenueContracts_Success(t *testing.T) {
	stats := &entity.DashboardStats{}
	stats.Revenue.Achieved = 4_750_000_000
	ar := &mockAnalyticsRepo{dist: defaultDist(), dashStats: stats}
	uc := newUC(ar, &mockTargetRepo{}, singleWS())

	result, err := uc.RevenueContracts(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalContractValue != 4_750_000_000 {
		t.Errorf("total = %d, want 4750000000", result.TotalContractValue)
	}
	if len(result.PlanTypeRevenue) != 1 {
		t.Errorf("plan revenue = %d entries, want 1", len(result.PlanTypeRevenue))
	}
	if result.PaymentBreakdown["lunas"] != 8 {
		t.Errorf("lunas = %d, want 8", result.PaymentBreakdown["lunas"])
	}
}

func TestRevenueContracts_DistError(t *testing.T) {
	ar := &mockAnalyticsRepo{distErr: errors.New("fail")}
	uc := newUC(ar, &mockTargetRepo{}, singleWS())

	_, err := uc.RevenueContracts(context.Background(), "ws-1")
	if err == nil {
		t.Error("expected error")
	}
}

// ─── ClientHealth ────────────────────────────────────────────────────────────

func TestClientHealth_Success(t *testing.T) {
	ar := &mockAnalyticsRepo{dist: defaultDist(), kpi: defaultKPI()}
	uc := newUC(ar, &mockTargetRepo{}, singleWS())

	result, err := uc.ClientHealth(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RiskDistribution["high"] != 2 {
		t.Errorf("high risk = %d, want 2", result.RiskDistribution["high"])
	}
	if result.NPSDistribution["no_response"] != 2 {
		t.Errorf("no_response = %d, want 2 (15 - 13)", result.NPSDistribution["no_response"])
	}
	if result.UsageDistribution["average"].(float64) != 62 {
		t.Errorf("usage avg = %v, want 62", result.UsageDistribution["average"])
	}
}

// ─── EngagementRetention ─────────────────────────────────────────────────────

func TestEngagementRetention_Success(t *testing.T) {
	eng := &entity.EngagementData{
		BotActive: 12, BotInactive: 3,
		NPSReplied: 10, CheckinReplied: 8,
		CrossSellInterested: 4, CrossSellRejected: 1,
		Renewed: 6,
	}
	kpi := defaultKPI()
	ar := &mockAnalyticsRepo{engagement: eng, kpi: kpi}
	uc := newUC(ar, &mockTargetRepo{}, singleWS())

	result, err := uc.EngagementRetention(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.BotActive != 12 {
		t.Errorf("bot_active = %d, want 12", result.BotActive)
	}
	if result.NPSNotReplied != 5 {
		t.Errorf("nps_not_replied = %d, want 5 (15 - 10)", result.NPSNotReplied)
	}
	if result.CrossSellPending != 10 {
		t.Errorf("cross_sell_pending = %d, want 10 (15 - 4 - 1)", result.CrossSellPending)
	}
	if result.NotRenewed != 9 {
		t.Errorf("not_renewed = %d, want 9 (15 - 6)", result.NotRenewed)
	}
}

func TestEngagementRetention_NegativeClampedToZero(t *testing.T) {
	eng := &entity.EngagementData{NPSReplied: 20} // more than total
	kpi := &entity.KPIData{}
	kpi.Clients.Total = 10
	ar := &mockAnalyticsRepo{engagement: eng, kpi: kpi}
	uc := newUC(ar, &mockTargetRepo{}, singleWS())

	result, err := uc.EngagementRetention(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NPSNotReplied != 0 {
		t.Errorf("nps_not_replied = %d, want 0 (clamped)", result.NPSNotReplied)
	}
}

// ─── WorkspaceComparison ─────────────────────────────────────────────────────

func TestWorkspaceComparison_NonHoldingReturnsError(t *testing.T) {
	uc := newUC(&mockAnalyticsRepo{}, &mockTargetRepo{}, singleWS())

	_, err := uc.WorkspaceComparison(context.Background(), "ws-1")
	if err == nil {
		t.Error("expected error for non-holding workspace")
	}
}

func TestWorkspaceComparison_HoldingSuccess(t *testing.T) {
	holdWS := &entity.Workspace{
		ID: "ws-hold", Slug: "sejutacita", IsHolding: true,
		MemberIDs: []string{"ws-1"},
	}
	memberWS := &entity.Workspace{ID: "ws-1", Slug: "dealls", Name: "Dealls"}

	wr := &mockWorkspaceRepo{}
	// GetByID will be called twice: once for holding, once for member.
	// We simulate by returning the holding on first call, member on second.
	// For simplicity, always return the same workspace.
	wr.ws = holdWS

	stats := &entity.DashboardStats{}
	stats.Clients.Total = 15
	stats.Revenue.Achieved = 3_000_000_000
	kpi := defaultKPI()
	ar := &mockAnalyticsRepo{dashStats: stats, kpi: kpi}

	// Override workspace repo to handle the holding → member chain.
	callCount := 0
	customWR := &seqWorkspaceRepo{
		workspaces: []*entity.Workspace{holdWS, memberWS},
		callCount:  &callCount,
	}

	uc := reports.New(ar, &mockTargetRepo{}, customWR, zerolog.Nop())
	result, err := uc.WorkspaceComparison(context.Background(), "ws-hold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Workspaces) != 1 {
		t.Errorf("workspaces = %d, want 1", len(result.Workspaces))
	}
	if result.Workspaces[0].Slug != "dealls" {
		t.Errorf("slug = %q, want dealls", result.Workspaces[0].Slug)
	}
}

// seqWorkspaceRepo returns different workspaces on successive GetByID calls.
type seqWorkspaceRepo struct {
	workspaces []*entity.Workspace
	callCount  *int
}

func (s *seqWorkspaceRepo) GetByID(_ context.Context, _ string) (*entity.Workspace, error) {
	idx := *s.callCount
	*s.callCount++
	if idx < len(s.workspaces) {
		return s.workspaces[idx], nil
	}
	return s.workspaces[len(s.workspaces)-1], nil
}
func (s *seqWorkspaceRepo) GetAll(context.Context) ([]entity.Workspace, error) { return nil, nil }
func (s *seqWorkspaceRepo) GetBySlug(context.Context, string) (*entity.Workspace, error) {
	return nil, nil
}
func (s *seqWorkspaceRepo) ListForUser(context.Context, string) ([]entity.Workspace, error) {
	return nil, nil
}
func (s *seqWorkspaceRepo) Create(context.Context, *entity.Workspace) (*entity.Workspace, error) {
	return nil, nil
}
func (s *seqWorkspaceRepo) Update(context.Context, string, repository.WorkspacePatch) (*entity.Workspace, error) {
	return nil, nil
}
func (s *seqWorkspaceRepo) SoftDelete(context.Context, string) error { return nil }
