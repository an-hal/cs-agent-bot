package dashboard_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/middleware"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	appTracer "github.com/Sejutacita/cs-agent-bot/internal/tracer"
	ucDashboard "github.com/Sejutacita/cs-agent-bot/internal/usecase/dashboard"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// ─── test helpers: noop tracer + logger ───────────────────────────────────────

type testTracer struct{ t trace.Tracer }

func newTestTracer() appTracer.Tracer {
	return testTracer{t: noop.NewTracerProvider().Tracer("test")}
}

func (n testTracer) Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return n.t.Start(ctx, name, opts...)
}

func (n testTracer) Shutdown(_ context.Context) error { return nil }

var (
	testLogger = zerolog.Nop()
	testTr     = newTestTracer()
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
	getClientsResult       []entity.Client
	getClientsTotal        int64
	getClientsErr          error
	getClientsByWSIDResult *ucDashboard.ClientListResult
	getClientsByWSIDErr    error
	getClientResult        *entity.Client
	getClientErr           error
	createClientErr        error
	updateClientErr        error
	deleteClientErr        error

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

	// Invoices (standalone)
	getStandaloneInvoicesResult []entity.Invoice
	getStandaloneInvoicesTotal  int64
	getStandaloneInvoicesErr    error
	getInvoiceResult            *entity.Invoice
	getInvoiceErr               error
	updateInvoiceErr            error

	// Templates
	getTemplatesResult []entity.Template
	getTemplatesTotal  int64
	getTemplatesErr    error
	getTemplateResult  *entity.Template
	getTemplateErr     error
	updateTemplateErr  error
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

func (m *mockUsecase) GetClientsByWorkspaceID(_ context.Context, _ string, _ entity.ClientFilter, _ pagination.Params) (*ucDashboard.ClientListResult, error) {
	return m.getClientsByWSIDResult, m.getClientsByWSIDErr
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

func (m *mockUsecase) GetEscalations(_ context.Context, _ entity.EscalationFilter, _ pagination.Params) ([]entity.Escalation, int64, error) {
	return m.getEscalationsResult, m.getEscalationsTotal, m.getEscalationsErr
}

func (m *mockUsecase) GetInvoices(_ context.Context, _ entity.InvoiceFilter, _ pagination.Params) ([]entity.Invoice, int64, error) {
	return m.getStandaloneInvoicesResult, m.getStandaloneInvoicesTotal, m.getStandaloneInvoicesErr
}

func (m *mockUsecase) GetInvoice(_ context.Context, _ string) (*entity.Invoice, error) {
	return m.getInvoiceResult, m.getInvoiceErr
}

func (m *mockUsecase) UpdateInvoice(_ context.Context, _ string, _ map[string]interface{}) error {
	return m.updateInvoiceErr
}

func (m *mockUsecase) GetTemplates(_ context.Context, _ entity.TemplateFilter, _ pagination.Params) ([]entity.Template, int64, error) {
	return m.getTemplatesResult, m.getTemplatesTotal, m.getTemplatesErr
}

func (m *mockUsecase) GetTemplate(_ context.Context, _ string) (*entity.Template, error) {
	return m.getTemplateResult, m.getTemplateErr
}

func (m *mockUsecase) UpdateTemplate(_ context.Context, _ string, _ map[string]interface{}) error {
	return m.updateTemplateErr
}

// ─── Test helpers ─────────────────────────────────────────────────────────────

// callHandler invokes the handler and applies error handling like the middleware does.
// If the handler returns an apperror, it writes the appropriate HTTP response.
func callHandler(h func(http.ResponseWriter, *http.Request) error, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	err := h(w, r)
	if err != nil {
		handler := response.NewHTTPExceptionHandler(zerolog.Nop(), false)
		handler.HandleError(w, r, err)
	}
	return w
}

func withJWTUser(r *http.Request, email string) *http.Request {
	ctx := middleware.WithJWTUser(r.Context(), middleware.JWTUser{Email: email})
	return r.WithContext(ctx)
}

func withPathParam(r *http.Request, key, value string) *http.Request {
	ctx := context.WithValue(r.Context(), router.ParamKey, map[string]string{key: value})
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
