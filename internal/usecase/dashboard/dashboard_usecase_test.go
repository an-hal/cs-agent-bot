package dashboard_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/dashboard"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// ─── noop tracer ──────────────────────────────────────────────────────────────

type noopTracer struct{ t trace.Tracer }

func newNoopTracer() noopTracer {
	return noopTracer{t: noop.NewTracerProvider().Tracer("test")}
}

func (n noopTracer) Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return n.t.Start(ctx, name, opts...)
}

func (n noopTracer) Shutdown(_ context.Context) error { return nil }

// ─── mock repos ───────────────────────────────────────────────────────────────

// mockWorkspaceRepo
type mockWorkspaceRepo struct {
	getAllResult    []entity.Workspace
	getAllErr       error
	getBySlugResult *entity.Workspace
	getBySlugErr    error
}

func (m *mockWorkspaceRepo) GetAll(context.Context) ([]entity.Workspace, error) {
	return m.getAllResult, m.getAllErr
}
func (m *mockWorkspaceRepo) GetBySlug(_ context.Context, _ string) (*entity.Workspace, error) {
	return m.getBySlugResult, m.getBySlugErr
}

// mockClientRepo
type mockClientRepo struct {
	repository.ClientRepository

	getByIDResult      *entity.Client
	getByIDErr         error
	createErr          error
	updateFieldsErr    error
	paginatedResult    []entity.Client
	paginatedTotal     int64
	paginatedErr       error
	paginatedIDsResult []entity.Client
	paginatedIDsTotal  int64
	paginatedIDsErr    error
	countByWSIDResult  int64
	countByWSIDErr     error
	fetchByWSIDResult  []entity.Client
	fetchByWSIDErr     error
}

func (m *mockClientRepo) GetByID(_ context.Context, _ string) (*entity.Client, error) {
	return m.getByIDResult, m.getByIDErr
}
func (m *mockClientRepo) CreateClient(_ context.Context, _ entity.Client) error {
	return m.createErr
}
func (m *mockClientRepo) CountByWorkspaceID(_ context.Context, _ string) (int64, error) {
	return m.countByWSIDResult, m.countByWSIDErr
}
func (m *mockClientRepo) FetchByWorkspaceID(_ context.Context, _ string, _ pagination.Params) ([]entity.Client, error) {
	return m.fetchByWSIDResult, m.fetchByWSIDErr
}
func (m *mockClientRepo) UpdateClientFields(_ context.Context, _ string, _ map[string]interface{}) error {
	return m.updateFieldsErr
}
func (m *mockClientRepo) GetAllByWorkspacePaginated(_ context.Context, _ string, _ pagination.Params) ([]entity.Client, int64, error) {
	return m.paginatedResult, m.paginatedTotal, m.paginatedErr
}
func (m *mockClientRepo) GetAllByWorkspaceIDsPaginated(_ context.Context, _ []string, _ pagination.Params) ([]entity.Client, int64, error) {
	return m.paginatedIDsResult, m.paginatedIDsTotal, m.paginatedIDsErr
}

// mockInvoiceRepo
type mockInvoiceRepo struct {
	repository.InvoiceRepository

	paginatedResult     []entity.Invoice
	paginatedTotal      int64
	paginatedErr        error
	getAllPaginated      []entity.Invoice
	getAllPaginatedTotal int64
	getAllPaginatedErr   error
	getByIDResult       *entity.Invoice
	getByIDErr          error
	updateFieldsErr     error
}

func (m *mockInvoiceRepo) GetAllByCompanyIDPaginated(_ context.Context, _ string, _ pagination.Params) ([]entity.Invoice, int64, error) {
	return m.paginatedResult, m.paginatedTotal, m.paginatedErr
}
func (m *mockInvoiceRepo) GetAllPaginated(_ context.Context, _ entity.InvoiceFilter, _ pagination.Params) ([]entity.Invoice, int64, error) {
	return m.getAllPaginated, m.getAllPaginatedTotal, m.getAllPaginatedErr
}
func (m *mockInvoiceRepo) GetByID(_ context.Context, _ string) (*entity.Invoice, error) {
	return m.getByIDResult, m.getByIDErr
}
func (m *mockInvoiceRepo) UpdateFields(_ context.Context, _ string, _ map[string]interface{}) error {
	return m.updateFieldsErr
}

// mockEscalationRepo
type mockEscalationRepo struct {
	repository.EscalationRepository

	paginatedResult []entity.Escalation
	paginatedTotal  int64
	paginatedErr    error
}

func (m *mockEscalationRepo) GetByCompanyIDPaginated(_ context.Context, _ string, _ pagination.Params) ([]entity.Escalation, int64, error) {
	return m.paginatedResult, m.paginatedTotal, m.paginatedErr
}

// mockLogRepo
type mockLogRepo struct {
	repository.LogRepository

	appendActivityCalled bool
	appendActivityEntry  entity.ActivityLog
	appendActivityErr    error

	getActivitiesResult []entity.ActivityLog
	getActivitiesTotal  int
	getActivitiesErr    error
}

func (m *mockLogRepo) AppendActivity(_ context.Context, entry entity.ActivityLog) error {
	m.appendActivityCalled = true
	m.appendActivityEntry = entry
	return m.appendActivityErr
}
func (m *mockLogRepo) GetActivities(_ context.Context, _ entity.ActivityFilter) ([]entity.ActivityLog, int, error) {
	return m.getActivitiesResult, m.getActivitiesTotal, m.getActivitiesErr
}

// mockTemplateRepo
type mockTemplateRepo struct {
	repository.TemplateRepository

	getTemplateResult  *entity.Template
	getTemplateErr     error
	getAllPaginated     []entity.Template
	getAllPaginatedTotal int64
	getAllPaginatedErr  error
	updateFieldsErr    error
}

func (m *mockTemplateRepo) GetTemplate(_ context.Context, _ string) (*entity.Template, error) {
	return m.getTemplateResult, m.getTemplateErr
}
func (m *mockTemplateRepo) GetAllPaginated(_ context.Context, _ entity.TemplateFilter, _ pagination.Params) ([]entity.Template, int64, error) {
	return m.getAllPaginated, m.getAllPaginatedTotal, m.getAllPaginatedErr
}
func (m *mockTemplateRepo) UpdateFields(_ context.Context, _ string, _ map[string]interface{}) error {
	return m.updateFieldsErr
}

// ─── helpers ──────────────────────────────────────────────────────────────────

type ucDeps struct {
	wsRepo   *mockWorkspaceRepo
	cRepo    *mockClientRepo
	iRepo    *mockInvoiceRepo
	eRepo    *mockEscalationRepo
	logRepo  *mockLogRepo
	tRepo    *mockTemplateRepo
}

func newTestUC(deps ucDeps) dashboard.DashboardUsecase {
	ws := deps.wsRepo
	if ws == nil {
		ws = &mockWorkspaceRepo{}
	}
	c := deps.cRepo
	if c == nil {
		c = &mockClientRepo{}
	}
	i := deps.iRepo
	if i == nil {
		i = &mockInvoiceRepo{}
	}
	e := deps.eRepo
	if e == nil {
		e = &mockEscalationRepo{}
	}
	l := deps.logRepo
	if l == nil {
		l = &mockLogRepo{}
	}
	tmpl := deps.tRepo
	if tmpl == nil {
		tmpl = &mockTemplateRepo{}
	}
	return dashboard.NewDashboardUsecase(ws, c, i, e, l, tmpl, newNoopTracer(), zerolog.Nop())
}

var defaultParams = pagination.Params{Offset: 0, Limit: 10}

// ─── GetWorkspaces ────────────────────────────────────────────────────────────

func TestGetWorkspaces_Success(t *testing.T) {
	ws := &mockWorkspaceRepo{getAllResult: []entity.Workspace{{Slug: "dealls"}}}
	uc := newTestUC(ucDeps{wsRepo: ws})

	result, err := uc.GetWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0].Slug != "dealls" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestGetWorkspaces_Error(t *testing.T) {
	ws := &mockWorkspaceRepo{getAllErr: errors.New("fail")}
	uc := newTestUC(ucDeps{wsRepo: ws})

	_, err := uc.GetWorkspaces(context.Background())
	if err == nil {
		t.Error("expected error")
	}
}

// ─── GetClients ───────────────────────────────────────────────────────────────

func TestGetClients_BySlug(t *testing.T) {
	ws := &mockWorkspaceRepo{getBySlugResult: &entity.Workspace{ID: "1", Slug: "dealls"}}
	c := &mockClientRepo{paginatedResult: []entity.Client{{CompanyID: "C01"}}, paginatedTotal: 1}
	uc := newTestUC(ucDeps{wsRepo: ws, cRepo: c})

	clients, total, err := uc.GetClients(context.Background(), "dealls", defaultParams)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 || len(clients) != 1 {
		t.Errorf("expected 1 client, got %d (total=%d)", len(clients), total)
	}
}

func TestGetClients_ByHolding(t *testing.T) {
	ws := &mockWorkspaceRepo{getBySlugResult: &entity.Workspace{ID: "h", Slug: "holding", IsHolding: true, MemberIDs: []string{"1", "2"}}}
	c := &mockClientRepo{paginatedIDsResult: []entity.Client{{CompanyID: "C01"}, {CompanyID: "C02"}}, paginatedIDsTotal: 2}
	uc := newTestUC(ucDeps{wsRepo: ws, cRepo: c})

	clients, total, err := uc.GetClients(context.Background(), "holding", defaultParams)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 || len(clients) != 2 {
		t.Errorf("expected 2 clients, got %d (total=%d)", len(clients), total)
	}
}

func TestGetClients_WorkspaceNotFound(t *testing.T) {
	ws := &mockWorkspaceRepo{getBySlugResult: nil}
	uc := newTestUC(ucDeps{wsRepo: ws})

	_, _, err := uc.GetClients(context.Background(), "nope", defaultParams)
	if err == nil {
		t.Error("expected error for missing workspace")
	}
}

// ─── GetClientsByWorkspaceID ──────────────────────────────────────────────────

func TestGetClientsByWorkspaceID_Success(t *testing.T) {
	c := &mockClientRepo{
		countByWSIDResult: 2,
		fetchByWSIDResult: []entity.Client{{CompanyID: "C01"}, {CompanyID: "C02"}},
	}
	uc := newTestUC(ucDeps{cRepo: c})

	result, err := uc.GetClientsByWorkspaceID(context.Background(), "ws-1", defaultParams)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Clients) != 2 {
		t.Errorf("expected 2 clients, got %d", len(result.Clients))
	}
	if result.Meta.Total != 2 {
		t.Errorf("expected total=2, got %d", result.Meta.Total)
	}
}

func TestGetClientsByWorkspaceID_CountError(t *testing.T) {
	c := &mockClientRepo{countByWSIDErr: errors.New("count fail")}
	uc := newTestUC(ucDeps{cRepo: c})

	_, err := uc.GetClientsByWorkspaceID(context.Background(), "ws-1", defaultParams)
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetClientsByWorkspaceID_FetchError(t *testing.T) {
	c := &mockClientRepo{countByWSIDResult: 5, fetchByWSIDErr: errors.New("fetch fail")}
	uc := newTestUC(ucDeps{cRepo: c})

	_, err := uc.GetClientsByWorkspaceID(context.Background(), "ws-1", defaultParams)
	if err == nil {
		t.Error("expected error")
	}
}

// ─── GetClient ────────────────────────────────────────────────────────────────

func TestGetClient_Success(t *testing.T) {
	c := &mockClientRepo{getByIDResult: &entity.Client{CompanyID: "C01"}}
	uc := newTestUC(ucDeps{cRepo: c})

	result, err := uc.GetClient(context.Background(), "C01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.CompanyID != "C01" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestGetClient_Error(t *testing.T) {
	c := &mockClientRepo{getByIDErr: errors.New("fail")}
	uc := newTestUC(ucDeps{cRepo: c})

	_, err := uc.GetClient(context.Background(), "C01")
	if err == nil {
		t.Error("expected error")
	}
}

// ─── CreateClient ─────────────────────────────────────────────────────────────

func TestCreateClient_SetsDefaults(t *testing.T) {
	c := &mockClientRepo{}
	uc := newTestUC(ucDeps{cRepo: c})

	err := uc.CreateClient(context.Background(), entity.Client{CompanyID: "C01"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateClient_Error(t *testing.T) {
	c := &mockClientRepo{createErr: errors.New("dup")}
	uc := newTestUC(ucDeps{cRepo: c})

	err := uc.CreateClient(context.Background(), entity.Client{CompanyID: "C01"})
	if err == nil {
		t.Error("expected error")
	}
}

// ─── UpdateClient ─────────────────────────────────────────────────────────────

func TestUpdateClient_Success(t *testing.T) {
	c := &mockClientRepo{}
	uc := newTestUC(ucDeps{cRepo: c})

	err := uc.UpdateClient(context.Background(), "C01", map[string]interface{}{"notes": "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateClient_Error(t *testing.T) {
	c := &mockClientRepo{updateFieldsErr: errors.New("fail")}
	uc := newTestUC(ucDeps{cRepo: c})

	err := uc.UpdateClient(context.Background(), "C01", map[string]interface{}{"notes": "x"})
	if err == nil {
		t.Error("expected error")
	}
}

// ─── DeleteClient ─────────────────────────────────────────────────────────────

func TestDeleteClient_Success(t *testing.T) {
	c := &mockClientRepo{}
	uc := newTestUC(ucDeps{cRepo: c})

	err := uc.DeleteClient(context.Background(), "C01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteClient_Error(t *testing.T) {
	c := &mockClientRepo{updateFieldsErr: errors.New("fail")}
	uc := newTestUC(ucDeps{cRepo: c})

	err := uc.DeleteClient(context.Background(), "C01")
	if err == nil {
		t.Error("expected error")
	}
}

// ─── GetClientInvoices ────────────────────────────────────────────────────────

func TestGetClientInvoices_Success(t *testing.T) {
	i := &mockInvoiceRepo{paginatedResult: []entity.Invoice{{InvoiceID: "INV-1"}}, paginatedTotal: 1}
	uc := newTestUC(ucDeps{iRepo: i})

	invoices, total, err := uc.GetClientInvoices(context.Background(), "C01", defaultParams)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 || len(invoices) != 1 {
		t.Errorf("expected 1 invoice, got %d (total=%d)", len(invoices), total)
	}
}

func TestGetClientInvoices_Error(t *testing.T) {
	i := &mockInvoiceRepo{paginatedErr: errors.New("fail")}
	uc := newTestUC(ucDeps{iRepo: i})

	_, _, err := uc.GetClientInvoices(context.Background(), "C01", defaultParams)
	if err == nil {
		t.Error("expected error")
	}
}

// ─── GetClientEscalations ────────────────────────────────────────────────────

func TestGetClientEscalations_Success(t *testing.T) {
	e := &mockEscalationRepo{paginatedResult: []entity.Escalation{{EscalationID: "E1"}}, paginatedTotal: 1}
	uc := newTestUC(ucDeps{eRepo: e})

	escs, total, err := uc.GetClientEscalations(context.Background(), "C01", defaultParams)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 || len(escs) != 1 {
		t.Errorf("expected 1 escalation, got %d (total=%d)", len(escs), total)
	}
}

func TestGetClientEscalations_Error(t *testing.T) {
	e := &mockEscalationRepo{paginatedErr: errors.New("fail")}
	uc := newTestUC(ucDeps{eRepo: e})

	_, _, err := uc.GetClientEscalations(context.Background(), "C01", defaultParams)
	if err == nil {
		t.Error("expected error")
	}
}

// ─── RecordActivity ───────────────────────────────────────────────────────────

func TestRecordActivity_DelegatesToRepo(t *testing.T) {
	l := &mockLogRepo{}
	uc := newTestUC(ucDeps{logRepo: l})

	entry := entity.ActivityLog{
		Category: entity.ActivityCategoryData, ActorType: entity.ActivityActorHuman,
		Actor: "user@example.com", Action: "edit_client", Target: "PT Maju", RefID: "C01",
	}
	if err := uc.RecordActivity(context.Background(), entry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !l.appendActivityCalled {
		t.Fatal("expected AppendActivity to be called")
	}
	if l.appendActivityEntry.Actor != "user@example.com" {
		t.Errorf("actor mismatch: got %q", l.appendActivityEntry.Actor)
	}
}

func TestRecordActivity_PropagatesError(t *testing.T) {
	repoErr := errors.New("db unavailable")
	l := &mockLogRepo{appendActivityErr: repoErr}
	uc := newTestUC(ucDeps{logRepo: l})

	err := uc.RecordActivity(context.Background(), entity.ActivityLog{Action: "add_client"})
	if !errors.Is(err, repoErr) {
		t.Errorf("expected repo error, got: %v", err)
	}
}

// ─── GetActivityLogs ──────────────────────────────────────────────────────────

func TestGetActivityLogs_Success(t *testing.T) {
	now := time.Now()
	l := &mockLogRepo{
		getActivitiesResult: []entity.ActivityLog{{ID: 1, Category: "bot", Action: "RENEWAL"}},
		getActivitiesTotal:  1,
	}
	uc := newTestUC(ucDeps{logRepo: l})

	logs, total, err := uc.GetActivityLogs(context.Background(), entity.ActivityFilter{
		WorkspaceID: "dealls", Category: entity.ActivityCategoryBot, Since: &now, Limit: 25, Offset: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 || len(logs) != 1 {
		t.Errorf("expected 1 log, got %d (total=%d)", len(logs), total)
	}
}

func TestGetActivityLogs_PropagatesError(t *testing.T) {
	l := &mockLogRepo{getActivitiesErr: errors.New("query failed")}
	uc := newTestUC(ucDeps{logRepo: l})

	_, _, err := uc.GetActivityLogs(context.Background(), entity.ActivityFilter{Limit: 10})
	if err == nil {
		t.Error("expected error")
	}
}
