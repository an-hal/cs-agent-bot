package dashboard_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/middleware"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	ucDashboard "github.com/Sejutacita/cs-agent-bot/internal/usecase/dashboard"
)

// ─── Compile-time interface check ─────────────────────────────────────────────
var _ ucDashboard.DashboardUsecase = (*mockUsecase)(nil)

// ─── mockUsecase ──────────────────────────────────────────────────────────────

type mockUsecase struct {
	// Workspace
	getWorkspacesResult []entity.Workspace
	getWorkspacesErr    error
	getWSBySlugResult   *entity.Workspace
	getWSBySlugErr      error

	// Clients
	getClientsResult []entity.Client
	getClientsTotal  int64
	getClientsErr    error
	getClientResult  *entity.Client
	getClientErr     error
	createClientErr  error
	updateClientErr  error
	deleteClientErr  error

	// Invoices
	getInvoicesResult []entity.Invoice
	getInvoicesTotal  int64
	getInvoicesErr    error

	// Escalations
	getEscalationsResult []entity.Escalation
	getEscalationsTotal  int64
	getEscalationsErr    error

	// Activity
	recordActivityErr error
	recordEntry       entity.ActivityLog
	getLogsResult     []entity.ActivityLog
	getLogsTotal      int
	getLogsErr        error
}

func (m *mockUsecase) GetWorkspaces(context.Context) ([]entity.Workspace, error) {
	return m.getWorkspacesResult, m.getWorkspacesErr
}

func (m *mockUsecase) GetWorkspaceBySlug(_ context.Context, _ string) (*entity.Workspace, error) {
	return m.getWSBySlugResult, m.getWSBySlugErr
}

func (m *mockUsecase) GetClients(_ context.Context, _ string, _ pagination.Params) ([]entity.Client, int64, error) {
	return m.getClientsResult, m.getClientsTotal, m.getClientsErr
}

func (m *mockUsecase) GetClient(_ context.Context, _ string) (*entity.Client, error) {
	return m.getClientResult, m.getClientErr
}

func (m *mockUsecase) CreateClient(_ context.Context, _ entity.Client) error {
	return m.createClientErr
}

func (m *mockUsecase) UpdateClient(_ context.Context, _ string, _ map[string]interface{}) error {
	return m.updateClientErr
}

func (m *mockUsecase) DeleteClient(_ context.Context, _ string) error {
	return m.deleteClientErr
}

func (m *mockUsecase) GetClientInvoices(_ context.Context, _ string, _ pagination.Params) ([]entity.Invoice, int64, error) {
	return m.getInvoicesResult, m.getInvoicesTotal, m.getInvoicesErr
}

func (m *mockUsecase) GetClientEscalations(_ context.Context, _ string, _ pagination.Params) ([]entity.Escalation, int64, error) {
	return m.getEscalationsResult, m.getEscalationsTotal, m.getEscalationsErr
}

func (m *mockUsecase) RecordActivity(_ context.Context, entry entity.ActivityLog) error {
	m.recordEntry = entry
	return m.recordActivityErr
}

func (m *mockUsecase) GetActivityLogs(_ context.Context, _ entity.ActivityFilter) ([]entity.ActivityLog, int, error) {
	return m.getLogsResult, m.getLogsTotal, m.getLogsErr
}

// ─── Test helpers ─────────────────────────────────────────────────────────────

func callHandler(h func(http.ResponseWriter, *http.Request) error, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	_ = h(w, r)
	return w
}

func withJWTUser(r *http.Request, email string) *http.Request {
	ctx := middleware.WithJWTUser(r.Context(), middleware.JWTUser{Email: email})
	return r.WithContext(ctx)
}

// pathParamKey mirrors the unexported contextKey type in the router package.
type pathParamKey string

func withPathParam(r *http.Request, key, value string) *http.Request {
	ctx := context.WithValue(r.Context(), pathParamKey("pathParams"), map[string]string{key: value})
	return r.WithContext(ctx)
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	return out
}
