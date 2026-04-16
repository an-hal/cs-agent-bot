package invoice_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	invoiceuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/invoice"
	"github.com/rs/zerolog"
)

// ─── Stub repositories ────────────────────────────────────────────────────────

type stubInvoiceRepo struct {
	byID       map[string]*entity.Invoice
	deleted    []string
	updates    map[string]map[string]interface{}
	overdue    []entity.Invoice
	stats      *entity.InvoiceStats
	statusBulk []string
}

func newStubInvoiceRepo() *stubInvoiceRepo {
	return &stubInvoiceRepo{
		byID:    map[string]*entity.Invoice{},
		updates: map[string]map[string]interface{}{},
		stats:   &entity.InvoiceStats{ByStatus: map[string]int64{}, AmountByStatus: map[string]int64{}, ByCollectionStage: map[string]int64{}},
	}
}

func (s *stubInvoiceRepo) GetActiveByCompanyID(_ context.Context, _ string) (*entity.Invoice, error) {
	return nil, nil
}
func (s *stubInvoiceRepo) GetAllByCompanyID(_ context.Context, _ string) ([]entity.Invoice, error) {
	return nil, nil
}
func (s *stubInvoiceRepo) GetAllPaginated(_ context.Context, _ entity.InvoiceFilter, _ pagination.Params) ([]entity.Invoice, int64, error) {
	out := make([]entity.Invoice, 0, len(s.byID))
	for _, v := range s.byID {
		out = append(out, *v)
	}
	return out, int64(len(out)), nil
}
func (s *stubInvoiceRepo) GetByID(_ context.Context, id string) (*entity.Invoice, error) {
	return s.byID[id], nil
}
func (s *stubInvoiceRepo) UpdateFields(_ context.Context, id string, fields map[string]interface{}) error {
	s.updates[id] = fields
	if inv, ok := s.byID[id]; ok {
		if st, ok := fields["payment_status"].(string); ok {
			inv.PaymentStatus = st
		}
	}
	return nil
}
func (s *stubInvoiceRepo) CreateInvoice(_ context.Context, _ entity.Invoice) error { return nil }
func (s *stubInvoiceRepo) UpdateFlags(_ context.Context, _ string, _ map[string]bool) error {
	return nil
}
func (s *stubInvoiceRepo) Create(_ context.Context, _ *sql.Tx, inv entity.Invoice) error {
	s.byID[inv.InvoiceID] = &inv
	return nil
}
func (s *stubInvoiceRepo) Delete(_ context.Context, id string) error {
	s.deleted = append(s.deleted, id)
	delete(s.byID, id)
	return nil
}
func (s *stubInvoiceRepo) ListOverdue(_ context.Context, _ time.Time) ([]entity.Invoice, error) {
	return s.overdue, nil
}
func (s *stubInvoiceRepo) Stats(_ context.Context, _ []string) (*entity.InvoiceStats, error) {
	return s.stats, nil
}
func (s *stubInvoiceRepo) UpdateStatusBulk(_ context.Context, ids []string, _ string) error {
	s.statusBulk = append(s.statusBulk, ids...)
	return nil
}

type stubLineItemRepo struct {
	items map[string][]entity.InvoiceLineItem
}

func (s *stubLineItemRepo) BulkCreate(_ context.Context, _ *sql.Tx, items []entity.InvoiceLineItem) error {
	if s.items == nil {
		s.items = map[string][]entity.InvoiceLineItem{}
	}
	for _, it := range items {
		s.items[it.InvoiceID] = append(s.items[it.InvoiceID], it)
	}
	return nil
}
func (s *stubLineItemRepo) GetByInvoiceID(_ context.Context, id string) ([]entity.InvoiceLineItem, error) {
	return s.items[id], nil
}
func (s *stubLineItemRepo) DeleteByInvoiceID(_ context.Context, _ *sql.Tx, id string) error {
	delete(s.items, id)
	return nil
}

type stubPaymentLogRepo struct {
	logs []entity.PaymentLog
}

func (s *stubPaymentLogRepo) Append(_ context.Context, l entity.PaymentLog) error {
	s.logs = append(s.logs, l)
	return nil
}
func (s *stubPaymentLogRepo) AppendTx(_ context.Context, _ *sql.Tx, l entity.PaymentLog) error {
	s.logs = append(s.logs, l)
	return nil
}
func (s *stubPaymentLogRepo) GetByInvoiceID(_ context.Context, id string, _ int) ([]entity.PaymentLog, error) {
	out := []entity.PaymentLog{}
	for _, l := range s.logs {
		if l.InvoiceID == id {
			out = append(out, l)
		}
	}
	return out, nil
}
func (s *stubPaymentLogRepo) GetRecentByWorkspace(_ context.Context, _ []string, _ int) ([]entity.PaymentLog, error) {
	return s.logs, nil
}

type stubSeqRepo struct {
	next int
}

func (s *stubSeqRepo) NextSeq(_ context.Context, _ *sql.Tx, _ string, _ int) (int, error) {
	s.next++
	return s.next, nil
}

type stubApprovalRepo struct {
	created  []*entity.ApprovalRequest
	byID     map[string]*entity.ApprovalRequest
	nextID   int
}

func newStubApprovalRepo() *stubApprovalRepo {
	return &stubApprovalRepo{byID: map[string]*entity.ApprovalRequest{}}
}

func (s *stubApprovalRepo) Create(_ context.Context, a *entity.ApprovalRequest) (*entity.ApprovalRequest, error) {
	s.nextID++
	a.ID = "ap-" + time.Now().Format("150405") + "-" + itoa(s.nextID)
	a.Status = entity.ApprovalStatusPending
	s.created = append(s.created, a)
	s.byID[a.ID] = a
	return a, nil
}
func (s *stubApprovalRepo) GetByID(_ context.Context, _ string, id string) (*entity.ApprovalRequest, error) {
	return s.byID[id], nil
}
func (s *stubApprovalRepo) UpdateStatus(_ context.Context, _, _, _, _, _ string) error {
	return nil
}

type stubWorkspaceRepo struct {
	ws *entity.Workspace
}

func (s *stubWorkspaceRepo) GetAll(_ context.Context) ([]entity.Workspace, error) {
	return []entity.Workspace{*s.ws}, nil
}
func (s *stubWorkspaceRepo) GetByID(_ context.Context, _ string) (*entity.Workspace, error) {
	return s.ws, nil
}
func (s *stubWorkspaceRepo) GetBySlug(_ context.Context, _ string) (*entity.Workspace, error) {
	return s.ws, nil
}
func (s *stubWorkspaceRepo) ListForUser(_ context.Context, _ string) ([]entity.Workspace, error) {
	return []entity.Workspace{*s.ws}, nil
}
func (s *stubWorkspaceRepo) Create(_ context.Context, w *entity.Workspace) (*entity.Workspace, error) {
	return w, nil
}
func (s *stubWorkspaceRepo) Update(_ context.Context, _ string, _ repository.WorkspacePatch) (*entity.Workspace, error) {
	return s.ws, nil
}
func (s *stubWorkspaceRepo) SoftDelete(_ context.Context, _ string) error { return nil }


// ─── Helper ───────────────────────────────────────────────────────────────────

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	b := []byte{}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func newTestUsecase(t *testing.T) (invoiceuc.Usecase, *stubInvoiceRepo, *stubPaymentLogRepo, *stubApprovalRepo, *stubWorkspaceRepo) {
	t.Helper()
	invRepo := newStubInvoiceRepo()
	lineRepo := &stubLineItemRepo{}
	plRepo := &stubPaymentLogRepo{}
	seqRepo := &stubSeqRepo{}
	apRepo := newStubApprovalRepo()
	wsRepo := &stubWorkspaceRepo{
		ws: &entity.Workspace{ID: "ws-1", Slug: "dealls", Settings: map[string]any{}},
	}
	uc := invoiceuc.New(
		nil, // db — ApplyCreate will panic if called with nil; tests exercise Create path only.
		invRepo, lineRepo, plRepo, seqRepo, apRepo, wsRepo,
		&invoiceuc.NoopPaperIDService{},
		tracer.NewNoopTracer(),
		zerolog.Nop(),
	)
	return uc, invRepo, plRepo, apRepo, wsRepo
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestCreate_CreatesApprovalRequest(t *testing.T) {
	uc, _, _, apRepo, _ := newTestUsecase(t)

	req := entity.CreateInvoiceReq{
		CompanyID:    "co-1",
		DueDate:      time.Now().Add(30 * 24 * time.Hour),
		PaymentTerms: 30,
		CreatedBy:    "ae@dealls.com",
		LineItems: []entity.InvoiceLineItem{
			{Description: "Monthly retainer", Qty: 1, UnitPrice: 1_000_000, Subtotal: 1_000_000},
		},
	}

	ar, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if ar == nil || ar.ID == "" {
		t.Fatalf("expected approval request with ID, got %v", ar)
	}
	if len(apRepo.created) != 1 {
		t.Fatalf("expected 1 approval, got %d", len(apRepo.created))
	}
	if apRepo.created[0].RequestType != "create_invoice" {
		t.Errorf("unexpected request type: %s", apRepo.created[0].RequestType)
	}
}

func TestCreate_ValidatesRequiredFields(t *testing.T) {
	uc, _, _, _, _ := newTestUsecase(t)

	_, err := uc.Create(context.Background(), entity.CreateInvoiceReq{})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !apperror.IsValidationError(err) {
		t.Errorf("expected validation error, got %v", err)
	}
}

func TestCreate_RejectsEmptyLineItems(t *testing.T) {
	uc, _, _, _, _ := newTestUsecase(t)

	_, err := uc.Create(context.Background(), entity.CreateInvoiceReq{
		CompanyID: "co-1",
		DueDate:   time.Now().Add(24 * time.Hour),
	})
	if err == nil {
		t.Fatal("expected line_items validation error")
	}
}

func TestDelete_BlocksNonBelumBayarStatus(t *testing.T) {
	uc, invRepo, _, _, _ := newTestUsecase(t)

	invRepo.byID["INV-DE-2026-001"] = &entity.Invoice{
		InvoiceID:     "INV-DE-2026-001",
		PaymentStatus: entity.PaymentStatusLunas,
	}

	err := uc.Delete(context.Background(), "INV-DE-2026-001")
	if err == nil {
		t.Fatal("expected error when deleting a Lunas invoice")
	}
	if !apperror.IsBadRequest(err) {
		t.Errorf("expected bad request error, got %v", err)
	}
	if len(invRepo.deleted) != 0 {
		t.Errorf("expected no deletes, got %d", len(invRepo.deleted))
	}
}

func TestDelete_AllowsBelumBayarStatus(t *testing.T) {
	uc, invRepo, _, _, _ := newTestUsecase(t)

	invRepo.byID["INV-DE-2026-001"] = &entity.Invoice{
		InvoiceID:     "INV-DE-2026-001",
		PaymentStatus: entity.PaymentStatusBelumBayar,
	}

	if err := uc.Delete(context.Background(), "INV-DE-2026-001"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(invRepo.deleted) != 1 {
		t.Errorf("expected 1 delete, got %d", len(invRepo.deleted))
	}
}

func TestMarkPaid_CreatesApproval(t *testing.T) {
	uc, invRepo, _, apRepo, _ := newTestUsecase(t)

	invRepo.byID["INV-DE-2026-001"] = &entity.Invoice{
		InvoiceID:     "INV-DE-2026-001",
		WorkspaceID:   "ws-1",
		PaymentStatus: entity.PaymentStatusBelumBayar,
	}

	ar, err := uc.MarkPaid(context.Background(), "INV-DE-2026-001", entity.MarkPaidReq{
		PaymentMethod: "bank_transfer",
		PaymentDate:   time.Now(),
		AmountPaid:    1_000_000,
		Actor:         "ae@dealls.com",
	})
	if err != nil {
		t.Fatalf("MarkPaid returned error: %v", err)
	}
	if ar == nil || ar.RequestType != "mark_invoice_paid" {
		t.Errorf("unexpected approval: %+v", ar)
	}
	if len(apRepo.created) != 1 {
		t.Errorf("expected 1 approval, got %d", len(apRepo.created))
	}
}

func TestMarkPaid_RejectsAlreadyPaid(t *testing.T) {
	uc, invRepo, _, _, _ := newTestUsecase(t)

	invRepo.byID["INV-DE-2026-001"] = &entity.Invoice{
		InvoiceID:     "INV-DE-2026-001",
		WorkspaceID:   "ws-1",
		PaymentStatus: entity.PaymentStatusLunas,
	}

	_, err := uc.MarkPaid(context.Background(), "INV-DE-2026-001", entity.MarkPaidReq{AmountPaid: 1000, Actor: "x@y.z"})
	if err == nil {
		t.Fatal("expected error for already-paid invoice")
	}
}

func TestSendReminder_IncrementsCounterAndLogs(t *testing.T) {
	uc, invRepo, plRepo, _, _ := newTestUsecase(t)

	invRepo.byID["INV-DE-2026-001"] = &entity.Invoice{
		InvoiceID:     "INV-DE-2026-001",
		WorkspaceID:   "ws-1",
		PaymentStatus: entity.PaymentStatusBelumBayar,
		ReminderCount: 2,
	}

	err := uc.SendReminder(context.Background(), "INV-DE-2026-001", entity.SendReminderReq{
		Channel:    "whatsapp",
		TemplateID: "tmpl-1",
		Actor:      "ae@dealls.com",
	})
	if err != nil {
		t.Fatalf("SendReminder returned error: %v", err)
	}

	upd := invRepo.updates["INV-DE-2026-001"]
	if rc, ok := upd["reminder_count"].(int); !ok || rc != 3 {
		t.Errorf("expected reminder_count=3, got %v", upd["reminder_count"])
	}
	if len(plRepo.logs) != 1 || plRepo.logs[0].EventType != entity.EventReminderSent {
		t.Errorf("expected one reminder_sent log, got %+v", plRepo.logs)
	}
}

func TestUpdate_StripsPaymentStatusFromPatch(t *testing.T) {
	uc, invRepo, _, _, _ := newTestUsecase(t)

	invRepo.byID["INV-DE-2026-001"] = &entity.Invoice{
		InvoiceID:     "INV-DE-2026-001",
		WorkspaceID:   "ws-1",
		PaymentStatus: entity.PaymentStatusBelumBayar,
	}

	patch := map[string]any{
		"notes":          "updated note",
		"payment_status": entity.PaymentStatusLunas, // must be dropped
	}
	if err := uc.Update(context.Background(), "INV-DE-2026-001", patch); err != nil {
		t.Fatalf("Update error: %v", err)
	}
	upd := invRepo.updates["INV-DE-2026-001"]
	if _, has := upd["payment_status"]; has {
		t.Errorf("expected payment_status to be stripped, but it was written: %v", upd["payment_status"])
	}
	if upd["notes"] != "updated note" {
		t.Errorf("expected notes to be written, got %v", upd["notes"])
	}
}
