package analytics_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/analytics"
	"github.com/rs/zerolog"
)

// ─── mock repos ──────────────────────────────────────────────────────────────

type mockAnalyticsRepo struct {
	dashStats   *entity.DashboardStats
	dashErr     error
	kpi         *entity.KPIData
	kpiErr      error
	dist        *entity.DistributionData
	distErr     error
	engagement  *entity.EngagementData
	engErr      error
	calledWSIDs []string
}

func (m *mockAnalyticsRepo) DashboardStats(_ context.Context, wsIDs []string) (*entity.DashboardStats, error) {
	m.calledWSIDs = wsIDs
	return m.dashStats, m.dashErr
}
func (m *mockAnalyticsRepo) KPI(_ context.Context, wsIDs []string) (*entity.KPIData, error) {
	m.calledWSIDs = wsIDs
	return m.kpi, m.kpiErr
}
func (m *mockAnalyticsRepo) Distributions(_ context.Context, wsIDs []string) (*entity.DistributionData, error) {
	m.calledWSIDs = wsIDs
	return m.dist, m.distErr
}
func (m *mockAnalyticsRepo) Engagement(_ context.Context, wsIDs []string) (*entity.EngagementData, error) {
	m.calledWSIDs = wsIDs
	return m.engagement, m.engErr
}

type mockTargetRepo struct {
	targets []entity.RevenueTarget
	listErr error
	upsertErr error
}

func (m *mockTargetRepo) List(_ context.Context, _ string) ([]entity.RevenueTarget, error) {
	return m.targets, m.listErr
}
func (m *mockTargetRepo) Upsert(_ context.Context, _ entity.RevenueTarget) error {
	return m.upsertErr
}

type mockSnapshotRepo struct {
	snapshots   []entity.RevenueSnapshot
	listErr     error
	upsertErr   error
	rebuildErr  error
	rebuildCalled bool
}

func (m *mockSnapshotRepo) List(_ context.Context, _ string, _ int) ([]entity.RevenueSnapshot, error) {
	return m.snapshots, m.listErr
}
func (m *mockSnapshotRepo) Upsert(_ context.Context, _ entity.RevenueSnapshot) error {
	return m.upsertErr
}
func (m *mockSnapshotRepo) RebuildFromInvoices(_ context.Context, _ string, _ int) error {
	m.rebuildCalled = true
	return m.rebuildErr
}

type mockWorkspaceRepo struct {
	getByIDResult *entity.Workspace
	getByIDErr    error
}

func (m *mockWorkspaceRepo) GetAll(context.Context) ([]entity.Workspace, error) { return nil, nil }
func (m *mockWorkspaceRepo) GetByID(_ context.Context, _ string) (*entity.Workspace, error) {
	return m.getByIDResult, m.getByIDErr
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

var _ repository.WorkspaceRepository = (*mockWorkspaceRepo)(nil)
var _ repository.AnalyticsRepository = (*mockAnalyticsRepo)(nil)
var _ repository.RevenueTargetRepository = (*mockTargetRepo)(nil)
var _ repository.RevenueSnapshotRepository = (*mockSnapshotRepo)(nil)

func newUC(ar *mockAnalyticsRepo, tr *mockTargetRepo, sr *mockSnapshotRepo, wr *mockWorkspaceRepo) analytics.Usecase {
	return analytics.New(ar, tr, sr, wr, zerolog.Nop())
}

func singleWorkspace() *mockWorkspaceRepo {
	return &mockWorkspaceRepo{
		getByIDResult: &entity.Workspace{ID: "ws-1", Slug: "dealls", IsHolding: false},
	}
}

func holdingWorkspace() *mockWorkspaceRepo {
	return &mockWorkspaceRepo{
		getByIDResult: &entity.Workspace{
			ID: "ws-hold", Slug: "sejutacita", IsHolding: true,
			MemberIDs: []string{"ws-1", "ws-2"},
		},
	}
}

// ─── DashboardStats ──────────────────────────────────────────────────────────

func TestDashboardStats_SingleWorkspace(t *testing.T) {
	ar := &mockAnalyticsRepo{
		dashStats: &entity.DashboardStats{},
	}
	ar.dashStats.Revenue.Achieved = 3_000_000_000
	ar.dashStats.Clients.Total = 15
	ar.dashStats.Clients.Active = 12
	ar.dashStats.Risk.High = 2

	tr := &mockTargetRepo{
		targets: []entity.RevenueTarget{{TargetAmount: 5_000_000_000}},
	}
	sr := &mockSnapshotRepo{
		snapshots: []entity.RevenueSnapshot{
			{DealsWon: 8, DealsLost: 2},
			{DealsWon: 4, DealsLost: 1},
		},
	}

	uc := newUC(ar, tr, sr, singleWorkspace())
	stats, err := uc.DashboardStats(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.Revenue.Achieved != 3_000_000_000 {
		t.Errorf("revenue achieved = %d, want 3000000000", stats.Revenue.Achieved)
	}
	if stats.Revenue.Target != 5_000_000_000 {
		t.Errorf("revenue target = %d, want 5000000000", stats.Revenue.Target)
	}
	if stats.Revenue.Pct != 60 {
		t.Errorf("revenue pct = %f, want 60", stats.Revenue.Pct)
	}
	if stats.AE.DealsWon != 12 {
		t.Errorf("deals won = %d, want 12", stats.AE.DealsWon)
	}
	if stats.AE.DealsLost != 3 {
		t.Errorf("deals lost = %d, want 3", stats.AE.DealsLost)
	}
	// WinRate = 12 / (12+3) * 100 = 80
	if stats.AE.WinRate != 80 {
		t.Errorf("win rate = %f, want 80", stats.AE.WinRate)
	}
}

func TestDashboardStats_HoldingExpandsMemberIDs(t *testing.T) {
	ar := &mockAnalyticsRepo{
		dashStats: &entity.DashboardStats{},
	}
	tr := &mockTargetRepo{}
	sr := &mockSnapshotRepo{}
	uc := newUC(ar, tr, sr, holdingWorkspace())

	_, err := uc.DashboardStats(context.Background(), "ws-hold")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ar.calledWSIDs) != 2 {
		t.Errorf("expected 2 workspace IDs for holding, got %d", len(ar.calledWSIDs))
	}
	if ar.calledWSIDs[0] != "ws-1" || ar.calledWSIDs[1] != "ws-2" {
		t.Errorf("expected [ws-1, ws-2], got %v", ar.calledWSIDs)
	}
}

func TestDashboardStats_ZeroTargetNoDivByZero(t *testing.T) {
	ar := &mockAnalyticsRepo{dashStats: &entity.DashboardStats{}}
	ar.dashStats.Revenue.Achieved = 1_000_000
	tr := &mockTargetRepo{targets: []entity.RevenueTarget{}} // no target
	sr := &mockSnapshotRepo{}
	uc := newUC(ar, tr, sr, singleWorkspace())

	stats, err := uc.DashboardStats(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.Revenue.Pct != 0 {
		t.Errorf("pct should be 0 when target=0, got %f", stats.Revenue.Pct)
	}
}

func TestDashboardStats_RepoError(t *testing.T) {
	ar := &mockAnalyticsRepo{dashErr: errors.New("db down")}
	tr := &mockTargetRepo{}
	sr := &mockSnapshotRepo{}
	uc := newUC(ar, tr, sr, singleWorkspace())

	_, err := uc.DashboardStats(context.Background(), "ws-1")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestDashboardStats_WorkspaceResolveError(t *testing.T) {
	ar := &mockAnalyticsRepo{}
	tr := &mockTargetRepo{}
	sr := &mockSnapshotRepo{}
	wr := &mockWorkspaceRepo{getByIDErr: errors.New("not found")}
	uc := newUC(ar, tr, sr, wr)

	_, err := uc.DashboardStats(context.Background(), "ws-bad")
	if err == nil {
		t.Error("expected error when workspace not found")
	}
}

// ─── KPI ──────────────────────────────────────────────────────────────────────

func TestKPI_CalculatesRevenuePct(t *testing.T) {
	kpi := &entity.KPIData{}
	kpi.Revenue.Achieved = 4_000_000_000
	ar := &mockAnalyticsRepo{kpi: kpi}
	tr := &mockTargetRepo{targets: []entity.RevenueTarget{{TargetAmount: 5_000_000_000}}}
	sr := &mockSnapshotRepo{}

	uc := newUC(ar, tr, sr, singleWorkspace())
	result, err := uc.KPI(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Revenue.Pct != 80 {
		t.Errorf("revenue pct = %f, want 80", result.Revenue.Pct)
	}
	if result.AE.QuotaAttainment != 80 {
		t.Errorf("quota attainment = %f, want 80", result.AE.QuotaAttainment)
	}
}

func TestKPI_RepoError(t *testing.T) {
	ar := &mockAnalyticsRepo{kpiErr: errors.New("fail")}
	uc := newUC(ar, &mockTargetRepo{}, &mockSnapshotRepo{}, singleWorkspace())

	_, err := uc.KPI(context.Background(), "ws-1")
	if err == nil {
		t.Error("expected error")
	}
}

// ─── Distributions ───────────────────────────────────────────────────────────

func TestDistributions_Success(t *testing.T) {
	dist := &entity.DistributionData{}
	dist.Risk.High = 2
	dist.Risk.Mid = 4
	dist.Risk.Low = 9
	ar := &mockAnalyticsRepo{dist: dist}
	uc := newUC(ar, &mockTargetRepo{}, &mockSnapshotRepo{}, singleWorkspace())

	result, err := uc.Distributions(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Risk.High != 2 || result.Risk.Mid != 4 || result.Risk.Low != 9 {
		t.Errorf("risk = %+v, want 2/4/9", result.Risk)
	}
}

func TestDistributions_RepoError(t *testing.T) {
	ar := &mockAnalyticsRepo{distErr: errors.New("fail")}
	uc := newUC(ar, &mockTargetRepo{}, &mockSnapshotRepo{}, singleWorkspace())

	_, err := uc.Distributions(context.Background(), "ws-1")
	if err == nil {
		t.Error("expected error")
	}
}

// ─── Engagement ──────────────────────────────────────────────────────────────

func TestEngagement_Success(t *testing.T) {
	eng := &entity.EngagementData{BotActive: 12, BotInactive: 3, Renewed: 6}
	ar := &mockAnalyticsRepo{engagement: eng}
	uc := newUC(ar, &mockTargetRepo{}, &mockSnapshotRepo{}, singleWorkspace())

	result, err := uc.Engagement(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.BotActive != 12 {
		t.Errorf("bot_active = %d, want 12", result.BotActive)
	}
}

// ─── RevenueTrend ────────────────────────────────────────────────────────────

func TestRevenueTrend_ProducesForecastPoints(t *testing.T) {
	snapshots := make([]entity.RevenueSnapshot, 10)
	for i := range snapshots {
		snapshots[i] = entity.RevenueSnapshot{
			WorkspaceID: "ws-1", Year: 2025, Month: i + 1,
			RevenueActual: int64((i + 1) * 500_000_000),
		}
	}
	sr := &mockSnapshotRepo{snapshots: snapshots}
	tr := &mockTargetRepo{targets: []entity.RevenueTarget{{TargetAmount: 5_000_000_000}}}

	uc := newUC(&mockAnalyticsRepo{}, tr, sr, singleWorkspace())
	result, err := uc.RevenueTrend(context.Background(), "ws-1", 16)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 10 actual + 6 forecast = 16 data points.
	if len(result.Data) != 16 {
		t.Errorf("data points = %d, want 16", len(result.Data))
	}

	// First 10 should not be forecast.
	for i := 0; i < 10; i++ {
		if result.Data[i].IsForecast {
			t.Errorf("data[%d] should not be forecast", i)
		}
		if result.Data[i].Actual == nil {
			t.Errorf("data[%d] should have actual value", i)
		}
	}

	// Last 6 should be forecast.
	for i := 10; i < 16; i++ {
		if !result.Data[i].IsForecast {
			t.Errorf("data[%d] should be forecast", i)
		}
		if result.Data[i].Forecast == nil {
			t.Errorf("data[%d] should have forecast value", i)
		}
		if result.Data[i].ForecastHigh == nil || result.Data[i].ForecastLow == nil {
			t.Errorf("data[%d] should have confidence band", i)
		}
	}

	// Regression should have positive slope (increasing data).
	if result.Regression.Slope <= 0 {
		t.Errorf("slope = %f, should be positive for increasing data", result.Regression.Slope)
	}
	if result.Regression.RSquared <= 0 {
		t.Errorf("r_squared = %f, should be positive", result.Regression.RSquared)
	}
}

func TestRevenueTrend_EmptySnapshotsReturnsForecastOnly(t *testing.T) {
	sr := &mockSnapshotRepo{snapshots: []entity.RevenueSnapshot{}}
	tr := &mockTargetRepo{}
	uc := newUC(&mockAnalyticsRepo{}, tr, sr, singleWorkspace())

	result, err := uc.RevenueTrend(context.Background(), "ws-1", 16)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With no actual data, only forecast points are generated (6).
	if len(result.Data) != 6 {
		t.Errorf("expected 6 forecast-only data points, got %d", len(result.Data))
	}
	for _, dp := range result.Data {
		if !dp.IsForecast {
			t.Errorf("all points should be forecast when no actual data")
		}
	}
}

func TestRevenueTrend_SnapshotRepoError(t *testing.T) {
	sr := &mockSnapshotRepo{listErr: errors.New("fail")}
	uc := newUC(&mockAnalyticsRepo{}, &mockTargetRepo{}, sr, singleWorkspace())

	_, err := uc.RevenueTrend(context.Background(), "ws-1", 16)
	if err == nil {
		t.Error("expected error")
	}
}

// ─── ForecastAccuracy ────────────────────────────────────────────────────────

func TestForecastAccuracy_CalculatesAccuracy(t *testing.T) {
	// Linear data: y = 1B * (i+1). Forecast for month 10 = ~5.5B, actual = 5B.
	snapshots := make([]entity.RevenueSnapshot, 10)
	for i := range snapshots {
		snapshots[i] = entity.RevenueSnapshot{
			Year: 2025, Month: i + 1,
			RevenueActual: int64((i + 1) * 1_000_000_000),
		}
	}
	sr := &mockSnapshotRepo{snapshots: snapshots}
	uc := newUC(&mockAnalyticsRepo{}, &mockTargetRepo{}, sr, singleWorkspace())

	accuracy, err := uc.ForecastAccuracy(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With perfectly linear data, accuracy should be very high.
	if accuracy < 90 {
		t.Errorf("accuracy = %f, expected > 90 for linear data", accuracy)
	}
}

func TestForecastAccuracy_TooFewSnapshots(t *testing.T) {
	sr := &mockSnapshotRepo{snapshots: []entity.RevenueSnapshot{{}, {}}}
	uc := newUC(&mockAnalyticsRepo{}, &mockTargetRepo{}, sr, singleWorkspace())

	accuracy, err := uc.ForecastAccuracy(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if accuracy != 0 {
		t.Errorf("expected 0 accuracy with < 3 snapshots, got %f", accuracy)
	}
}

// ─── RebuildSnapshots ────────────────────────────────────────────────────────

func TestRebuildSnapshots_CallsRepo(t *testing.T) {
	sr := &mockSnapshotRepo{}
	uc := newUC(&mockAnalyticsRepo{}, &mockTargetRepo{}, sr, singleWorkspace())

	err := uc.RebuildSnapshots(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sr.rebuildCalled {
		t.Error("expected RebuildFromInvoices to be called")
	}
}

func TestRebuildSnapshots_RepoError(t *testing.T) {
	sr := &mockSnapshotRepo{rebuildErr: errors.New("fail")}
	uc := newUC(&mockAnalyticsRepo{}, &mockTargetRepo{}, sr, singleWorkspace())

	err := uc.RebuildSnapshots(context.Background(), "ws-1")
	if err == nil {
		t.Error("expected error")
	}
}
