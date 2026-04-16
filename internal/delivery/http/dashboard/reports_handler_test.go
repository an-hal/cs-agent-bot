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

// ─── mockReportsUsecase ──────────────────────────────────────────────────────

type mockReportsUsecase struct {
	execSummary     *entity.ExecutiveSummaryResponse
	execErr         error
	revContracts    *entity.RevenueReportResponse
	revErr          error
	healthReport    *entity.HealthReportResponse
	healthErr       error
	engReport       *entity.EngagementReportResponse
	engErr          error
	wsComparison    *entity.WorkspaceComparisonResponse
	wsCompErr       error
}

func (m *mockReportsUsecase) ExecutiveSummary(_ context.Context, _ string) (*entity.ExecutiveSummaryResponse, error) {
	return m.execSummary, m.execErr
}
func (m *mockReportsUsecase) RevenueContracts(_ context.Context, _ string) (*entity.RevenueReportResponse, error) {
	return m.revContracts, m.revErr
}
func (m *mockReportsUsecase) ClientHealth(_ context.Context, _ string) (*entity.HealthReportResponse, error) {
	return m.healthReport, m.healthErr
}
func (m *mockReportsUsecase) EngagementRetention(_ context.Context, _ string) (*entity.EngagementReportResponse, error) {
	return m.engReport, m.engErr
}
func (m *mockReportsUsecase) WorkspaceComparison(_ context.Context, _ string) (*entity.WorkspaceComparisonResponse, error) {
	return m.wsComparison, m.wsCompErr
}

// ─── ExecutiveSummary ────────────────────────────────────────────────────────

func TestExecutiveSummaryHandler_ReturnsOK(t *testing.T) {
	mock := &mockReportsUsecase{
		execSummary: &entity.ExecutiveSummaryResponse{
			KPI:                map[string]interface{}{"revenue_achieved": 3_240_000_000},
			HighlightsPositive: []string{"Revenue on track"},
			HighlightsNegative: []string{"2 high risk"},
		},
	}
	h := handler.NewReportsHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/reports/executive-summary", nil)
	w := callHandler(h.ExecutiveSummary, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d — body: %s", w.Code, w.Body)
	}
	body := decodeBody(t, w)
	if body["status"] != "success" {
		t.Errorf("expected status=success, got %v", body["status"])
	}
}

func TestExecutiveSummaryHandler_Error(t *testing.T) {
	mock := &mockReportsUsecase{execErr: errors.New("fail")}
	h := handler.NewReportsHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/reports/executive-summary", nil)
	w := httptest.NewRecorder()
	err := h.ExecutiveSummary(w, r)
	if err == nil {
		t.Error("expected error")
	}
}

// ─── RevenueContracts ────────────────────────────────────────────────────────

func TestRevenueContractsHandler_ReturnsOK(t *testing.T) {
	mock := &mockReportsUsecase{
		revContracts: &entity.RevenueReportResponse{
			TotalContractValue: 4_750_000_000,
			PlanTypeRevenue:    []entity.PlanRevenue{},
			TopClients:         []entity.ReportClient{},
			AtRiskClients:      []entity.ReportClient{},
			ExpiringSoon:       []entity.ReportClient{},
			ContractExpiry:     map[string]int{},
			PaymentBreakdown:   map[string]int{},
		},
	}
	h := handler.NewReportsHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/reports/revenue-contracts", nil)
	w := callHandler(h.RevenueContracts, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRevenueContractsHandler_Error(t *testing.T) {
	mock := &mockReportsUsecase{revErr: errors.New("fail")}
	h := handler.NewReportsHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/reports/revenue-contracts", nil)
	w := httptest.NewRecorder()
	err := h.RevenueContracts(w, r)
	if err == nil {
		t.Error("expected error")
	}
}

// ─── ClientHealth ────────────────────────────────────────────────────────────

func TestClientHealthHandler_ReturnsOK(t *testing.T) {
	mock := &mockReportsUsecase{
		healthReport: &entity.HealthReportResponse{
			RiskDistribution:    map[string]int{"high": 2},
			PaymentDistribution: map[string]int{},
			NPSDistribution:    map[string]int{},
			NPSByIndustry:      []entity.IndustryNPS{},
			UsageDistribution:   map[string]any{},
			PaymentIssueClients: []entity.ReportClient{},
		},
	}
	h := handler.NewReportsHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/reports/client-health", nil)
	w := callHandler(h.ClientHealth, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// ─── EngagementRetention ─────────────────────────────────────────────────────

func TestEngagementRetentionHandler_ReturnsOK(t *testing.T) {
	mock := &mockReportsUsecase{
		engReport: &entity.EngagementReportResponse{
			BotActive:        12,
			CrossSellClients: []entity.CrossSellClient{},
		},
	}
	h := handler.NewReportsHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/reports/engagement-retention", nil)
	w := callHandler(h.EngagementRetention, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// ─── WorkspaceComparison ─────────────────────────────────────────────────────

func TestWorkspaceComparisonHandler_ReturnsOK(t *testing.T) {
	mock := &mockReportsUsecase{
		wsComparison: &entity.WorkspaceComparisonResponse{
			Workspaces: []entity.WorkspaceStats{
				{ID: "ws-1", Slug: "dealls", Name: "Dealls", Stats: map[string]any{"clients": 15}},
			},
			Combined: map[string]any{"clients": 15},
		},
	}
	h := handler.NewReportsHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/reports/workspace-comparison", nil)
	w := callHandler(h.WorkspaceComparison, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestWorkspaceComparisonHandler_Error(t *testing.T) {
	mock := &mockReportsUsecase{wsCompErr: errors.New("not holding")}
	h := handler.NewReportsHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/reports/workspace-comparison", nil)
	w := httptest.NewRecorder()
	err := h.WorkspaceComparison(w, r)
	if err == nil {
		t.Error("expected error")
	}
}

// ─── Export ──────────────────────────────────────────────────────────────────

func TestExportHandler_ReturnsOK(t *testing.T) {
	mock := &mockReportsUsecase{}
	h := handler.NewReportsHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodPost, "/reports/export?tab=executive&format=xlsx", nil)
	w := callHandler(h.Export, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
