package dashboard_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	handler "github.com/Sejutacita/cs-agent-bot/internal/delivery/http/dashboard"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

// ─── mockAnalyticsUsecase ────────────────────────────────────────────────────

type mockAnalyticsUsecase struct {
	dashStats        *entity.DashboardStats
	dashErr          error
	kpiData          *entity.KPIData
	kpiErr           error
	distData         *entity.DistributionData
	distErr          error
	engData          *entity.EngagementData
	engErr           error
	trendData        *entity.RevenueTrendResponse
	trendErr         error
	forecastAccuracy float64
	forecastErr      error
	rebuildErr       error
}

func (m *mockAnalyticsUsecase) DashboardStats(_ context.Context, _ string) (*entity.DashboardStats, error) {
	return m.dashStats, m.dashErr
}
func (m *mockAnalyticsUsecase) KPI(_ context.Context, _ string) (*entity.KPIData, error) {
	return m.kpiData, m.kpiErr
}
func (m *mockAnalyticsUsecase) Distributions(_ context.Context, _ string) (*entity.DistributionData, error) {
	return m.distData, m.distErr
}
func (m *mockAnalyticsUsecase) Engagement(_ context.Context, _ string) (*entity.EngagementData, error) {
	return m.engData, m.engErr
}
func (m *mockAnalyticsUsecase) RevenueTrend(_ context.Context, _ string, _ int) (*entity.RevenueTrendResponse, error) {
	return m.trendData, m.trendErr
}
func (m *mockAnalyticsUsecase) ForecastAccuracy(_ context.Context, _ string) (float64, error) {
	return m.forecastAccuracy, m.forecastErr
}
func (m *mockAnalyticsUsecase) RebuildSnapshots(_ context.Context, _ string) error {
	return m.rebuildErr
}

func defaultDashStats() *entity.DashboardStats {
	s := &entity.DashboardStats{}
	s.Revenue.Achieved = 3_240_000_000
	s.Revenue.Target = 5_000_000_000
	s.Revenue.Pct = 64.8
	s.Clients.Total = 15
	s.Clients.Active = 12
	s.Risk.High = 2
	s.TopAccounts = []entity.TopAccount{{ID: "1", Name: "PT Test", Revenue: 45_000_000}}
	return s
}

// ─── Stats ───────────────────────────────────────────────────────────────────

func TestStats_ReturnsOK(t *testing.T) {
	mock := &mockAnalyticsUsecase{dashStats: defaultDashStats()}
	h := handler.NewAnalyticsHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/stats", nil)
	w := callHandler(h.Stats, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d — body: %s", w.Code, w.Body)
	}
	body := decodeBody(t, w)
	if body["status"] != "success" {
		t.Errorf("expected status=success, got %v", body["status"])
	}
}

func TestStats_UsecaseError(t *testing.T) {
	mock := &mockAnalyticsUsecase{dashErr: errors.New("db down")}
	h := handler.NewAnalyticsHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/stats", nil)
	w := httptest.NewRecorder()
	err := h.Stats(w, r)
	if err == nil {
		t.Error("expected error")
	}
}

// ─── KPI ──────────────────────────────────────────────────────────────────────

func TestKPI_ReturnsOK(t *testing.T) {
	kpi := &entity.KPIData{}
	kpi.Revenue.Achieved = 3_240_000_000
	kpi.Clients.Total = 15
	kpi.NPS.Average = 7.8
	mock := &mockAnalyticsUsecase{kpiData: kpi}
	h := handler.NewAnalyticsHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/analytics/kpi", nil)
	w := callHandler(h.KPI, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestKPI_Error(t *testing.T) {
	mock := &mockAnalyticsUsecase{kpiErr: errors.New("fail")}
	h := handler.NewAnalyticsHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/analytics/kpi", nil)
	w := httptest.NewRecorder()
	err := h.KPI(w, r)
	if err == nil {
		t.Error("expected error")
	}
}

// ─── Distributions ───────────────────────────────────────────────────────────

func TestDistributions_ReturnsOK(t *testing.T) {
	dist := &entity.DistributionData{}
	dist.Risk.High = 2
	dist.PlanTypeRevenue = []entity.PlanRevenue{}
	dist.IndustryBreakdown = []entity.IndustryCount{}
	dist.HCSizeBreakdown = []entity.HCSizeCount{}
	dist.ValueTierBreakdown = []entity.ValueTierCount{}
	mock := &mockAnalyticsUsecase{distData: dist}
	h := handler.NewAnalyticsHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/analytics/distributions", nil)
	w := callHandler(h.Distributions, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestDistributions_Error(t *testing.T) {
	mock := &mockAnalyticsUsecase{distErr: errors.New("fail")}
	h := handler.NewAnalyticsHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/analytics/distributions", nil)
	w := httptest.NewRecorder()
	err := h.Distributions(w, r)
	if err == nil {
		t.Error("expected error")
	}
}

// ─── Engagement ──────────────────────────────────────────────────────────────

func TestEngagement_ReturnsOK(t *testing.T) {
	eng := &entity.EngagementData{BotActive: 12, Renewed: 6}
	mock := &mockAnalyticsUsecase{engData: eng}
	h := handler.NewAnalyticsHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/analytics/engagement", nil)
	w := callHandler(h.Engagement, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// ─── RevenueTrend ────────────────────────────────────────────────────────────

func TestRevenueTrend_DefaultMonths(t *testing.T) {
	trend := &entity.RevenueTrendResponse{
		Workspace: "ws-1",
		Data:      []entity.RevenueDataPoint{},
	}
	mock := &mockAnalyticsUsecase{trendData: trend}
	h := handler.NewAnalyticsHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/analytics/revenue-trend", nil)
	w := callHandler(h.RevenueTrend, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRevenueTrend_CustomMonths(t *testing.T) {
	trend := &entity.RevenueTrendResponse{Data: []entity.RevenueDataPoint{}}
	mock := &mockAnalyticsUsecase{trendData: trend}
	h := handler.NewAnalyticsHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/analytics/revenue-trend?months=20", nil)
	w := callHandler(h.RevenueTrend, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRevenueTrend_Error(t *testing.T) {
	mock := &mockAnalyticsUsecase{trendErr: errors.New("fail")}
	h := handler.NewAnalyticsHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/analytics/revenue-trend", nil)
	w := httptest.NewRecorder()
	err := h.RevenueTrend(w, r)
	if err == nil {
		t.Error("expected error")
	}
}

// ─── ForecastAccuracy ────────────────────────────────────────────────────────

func TestForecastAccuracy_ReturnsOK(t *testing.T) {
	mock := &mockAnalyticsUsecase{forecastAccuracy: 82.3}
	h := handler.NewAnalyticsHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/analytics/forecast-accuracy", nil)
	w := callHandler(h.ForecastAccuracy, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := decodeBody(t, w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data map in response")
	}
	if data["accuracy"] != 82.3 {
		t.Errorf("accuracy = %v, want 82.3", data["accuracy"])
	}
}

func TestForecastAccuracy_Error(t *testing.T) {
	mock := &mockAnalyticsUsecase{forecastErr: errors.New("fail")}
	h := handler.NewAnalyticsHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/analytics/forecast-accuracy", nil)
	w := httptest.NewRecorder()
	err := h.ForecastAccuracy(w, r)
	if err == nil {
		t.Error("expected error")
	}
}
