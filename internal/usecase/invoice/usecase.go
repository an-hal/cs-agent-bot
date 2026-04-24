// Package invoice implements the full billing usecase for invoice management:
// create, list, update, delete, mark-paid, send reminder, stats, and
// checker-maker approval flow for destructive operations.
package invoice

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// Usecase is the full-featured invoice billing usecase.
type Usecase interface {
	List(ctx context.Context, wsIDs []string, filter entity.InvoiceFilter, p pagination.Params) ([]entity.Invoice, int64, error)
	Get(ctx context.Context, invoiceID string) (*entity.InvoiceDetail, error)

	// Create returns an ApprovalRequest (202) — actual insert happens in ApplyCreate.
	Create(ctx context.Context, req entity.CreateInvoiceReq) (*entity.ApprovalRequest, error)
	ApplyCreate(ctx context.Context, workspaceID, approvalID, checkerEmail string) (*entity.Invoice, error)

	Update(ctx context.Context, invoiceID string, patch map[string]any) error
	Delete(ctx context.Context, invoiceID string) error

	// MarkPaid returns an ApprovalRequest (202) — actual mark-paid in ApplyMarkPaid.
	MarkPaid(ctx context.Context, invoiceID string, req entity.MarkPaidReq) (*entity.ApprovalRequest, error)
	ApplyMarkPaid(ctx context.Context, workspaceID, approvalID, checkerEmail string) error

	SendReminder(ctx context.Context, invoiceID string, req entity.SendReminderReq) error
	Stats(ctx context.Context, wsIDs []string) (*entity.InvoiceStats, error)
	ByStage(ctx context.Context, wsIDs []string) (map[string]int64, error)
	PaymentLogs(ctx context.Context, invoiceID string) ([]entity.PaymentLog, error)

	// Activity returns a unified timeline for one invoice — payment_logs +
	// mutation history from master_data_mutations that touched this invoice's
	// company_id. Newest first.
	Activity(ctx context.Context, workspaceID, invoiceID string, limit int) ([]InvoiceActivityEntry, error)

	// UpdateStage manually sets the collection_stage on an invoice (AE-only
	// in practice — role-gating enforced by caller). Writes a payment_log
	// entry with event_type=stage_updated + old/new stage.
	UpdateStage(ctx context.Context, workspaceID, invoiceID string, req entity.UpdateStageReq) (*entity.Invoice, error)

	// ConfirmPartial marks one termin as paid on a multi-termin invoice.
	// When all termins reach status=paid the invoice's payment_status flips
	// to Lunas automatically.
	ConfirmPartial(ctx context.Context, workspaceID, invoiceID string, req entity.ConfirmPartialReq) (*entity.Invoice, error)
}

// InvoiceActivityEntry is one row of the unified invoice timeline.
type InvoiceActivityEntry struct {
	Source    string    `json:"source"`    // "payment_log" | "mutation"
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type,omitempty"`
	Actor     string    `json:"actor,omitempty"`
	Summary   string    `json:"summary,omitempty"`
	Detail    map[string]any `json:"detail,omitempty"`
}

type invoiceUsecase struct {
	db              *sql.DB
	invoiceRepo     repository.InvoiceRepository
	lineItemRepo    repository.InvoiceLineItemRepository
	paymentLogRepo  repository.PaymentLogRepository
	seqRepo         repository.InvoiceSequenceRepository
	approvalRepo    repository.ApprovalRequestRepository
	workspaceRepo   repository.WorkspaceRepository
	paperidSvc      PaperIDService
	tracer          tracer.Tracer
	logger          zerolog.Logger
}

// New constructs a fully wired InvoiceUsecase.
func New(
	db *sql.DB,
	invoiceRepo repository.InvoiceRepository,
	lineItemRepo repository.InvoiceLineItemRepository,
	paymentLogRepo repository.PaymentLogRepository,
	seqRepo repository.InvoiceSequenceRepository,
	approvalRepo repository.ApprovalRequestRepository,
	workspaceRepo repository.WorkspaceRepository,
	paperidSvc PaperIDService,
	tr tracer.Tracer,
	logger zerolog.Logger,
) Usecase {
	return &invoiceUsecase{
		db:             db,
		invoiceRepo:    invoiceRepo,
		lineItemRepo:   lineItemRepo,
		paymentLogRepo: paymentLogRepo,
		seqRepo:        seqRepo,
		approvalRepo:   approvalRepo,
		workspaceRepo:  workspaceRepo,
		paperidSvc:     paperidSvc,
		tracer:         tr,
		logger:         logger,
	}
}

// wsShortCode derives a short workspace code from its slug (first 2 chars, uppercase).
func wsShortCode(slug string) string {
	slug = strings.ToUpper(slug)
	if len(slug) >= 2 {
		return slug[:2]
	}
	if len(slug) == 1 {
		return slug + "X"
	}
	return "XX"
}

// ─── List ──────────────────────────────────────────────────────────────────

func (u *invoiceUsecase) List(ctx context.Context, wsIDs []string, filter entity.InvoiceFilter, p pagination.Params) ([]entity.Invoice, int64, error) {
	ctx, span := u.tracer.Start(ctx, "invoice.usecase.List")
	defer span.End()

	filter.WorkspaceIDs = wsIDs
	return u.invoiceRepo.GetAllPaginated(ctx, filter, p)
}

// ─── Get ───────────────────────────────────────────────────────────────────

func (u *invoiceUsecase) Get(ctx context.Context, invoiceID string) (*entity.InvoiceDetail, error) {
	ctx, span := u.tracer.Start(ctx, "invoice.usecase.Get")
	defer span.End()

	inv, err := u.invoiceRepo.GetByID(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, apperror.NotFound("invoice", "")
	}

	lineItems, err := u.lineItemRepo.GetByInvoiceID(ctx, invoiceID)
	if err != nil {
		return nil, err
	}

	logs, err := u.paymentLogRepo.GetByInvoiceID(ctx, invoiceID, 50)
	if err != nil {
		return nil, err
	}

	return &entity.InvoiceDetail{
		Invoice:     *inv,
		LineItems:   lineItems,
		PaymentLogs: logs,
	}, nil
}

// ─── Create (checker-maker) ────────────────────────────────────────────────

// Create validates the request and queues an approval for the actual insert.
func (u *invoiceUsecase) Create(ctx context.Context, req entity.CreateInvoiceReq) (*entity.ApprovalRequest, error) {
	ctx, span := u.tracer.Start(ctx, "invoice.usecase.Create")
	defer span.End()
	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	if req.CompanyID == "" {
		return nil, apperror.ValidationError("company_id is required")
	}
	if req.DueDate.IsZero() {
		return nil, apperror.ValidationError("due_date is required")
	}
	if len(req.LineItems) == 0 {
		return nil, apperror.ValidationError("at least one line_item is required")
	}
	if !entity.IsValidPaymentMethodRoute(req.PaymentMethodRoute) {
		return nil, apperror.ValidationError("payment_method_route must be paper_id or transfer_bank")
	}

	// Compute totals.
	var totalAmount int64
	for _, li := range req.LineItems {
		totalAmount += li.Subtotal
	}

	// Termin validation — when present, sum of termin amounts must equal
	// the line-item total.
	if len(req.TerminBreakdown) > 0 {
		var terminSum int64
		seen := map[int]bool{}
		for _, t := range req.TerminBreakdown {
			if t.TerminNumber <= 0 {
				return nil, apperror.ValidationError("termin_number must be >= 1")
			}
			if seen[t.TerminNumber] {
				return nil, apperror.ValidationError("duplicate termin_number in breakdown")
			}
			seen[t.TerminNumber] = true
			if t.Amount <= 0 {
				return nil, apperror.ValidationError("termin amount must be positive")
			}
			terminSum += t.Amount
		}
		if terminSum != totalAmount {
			return nil, apperror.ValidationError("sum of termin amounts must equal total line_item amount")
		}
	}

	payload := map[string]any{
		"company_id":            req.CompanyID,
		"issue_date":            req.IssueDate,
		"due_date":              req.DueDate,
		"payment_terms":         req.PaymentTerms,
		"notes":                 req.Notes,
		"created_by":            req.CreatedBy,
		"total_amount":          totalAmount,
		"line_item_count":       len(req.LineItems),
		"payment_method_route":  defaultStr(req.PaymentMethodRoute, entity.PaymentMethodRouteTransfer),
		"has_termin":            len(req.TerminBreakdown) > 0,
	}
	desc := fmt.Sprintf("Create invoice for company %s (amount: %d)", req.CompanyID, totalAmount)

	ar, err := u.approvalRepo.Create(ctx, &entity.ApprovalRequest{
		RequestType: "create_invoice",
		Description: desc,
		Payload:     payload,
		MakerEmail:  req.CreatedBy,
	})
	if err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to create invoice approval request")
	}
	return ar, nil
}

// ApplyCreate executes the invoice insert after approval.
func (u *invoiceUsecase) ApplyCreate(ctx context.Context, workspaceID, approvalID, checkerEmail string) (*entity.Invoice, error) {
	ctx, span := u.tracer.Start(ctx, "invoice.usecase.ApplyCreate")
	defer span.End()
	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	ar, err := u.approvalRepo.GetByID(ctx, workspaceID, approvalID)
	if err != nil {
		return nil, err
	}
	if ar == nil {
		return nil, apperror.NotFound("approval_request", "")
	}
	if ar.RequestType != "create_invoice" {
		return nil, apperror.BadRequest("approval is not a create_invoice request")
	}
	if ar.Status != entity.ApprovalStatusPending {
		return nil, apperror.BadRequest("approval is not pending")
	}

	// Resolve workspace for ID generation.
	ws, err := u.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil || ws == nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to resolve workspace")
	}

	tx, err := u.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to begin transaction")
	}
	defer tx.Rollback()

	year := time.Now().Year()
	seq, err := u.seqRepo.NextSeq(ctx, tx, workspaceID, year)
	if err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to generate invoice sequence")
	}

	shortCode := wsShortCode(ws.Slug)
	invoiceID := fmt.Sprintf("INV-%s-%d-%03d", shortCode, year, seq)

	companyID, _ := ar.Payload["company_id"].(string)
	notes, _ := ar.Payload["notes"].(string)
	createdBy, _ := ar.Payload["created_by"].(string)
	paymentTerms := 30
	if pt, ok := ar.Payload["payment_terms"].(float64); ok {
		paymentTerms = int(pt)
	}

	var issueDate, dueDate time.Time
	if isStr, ok := ar.Payload["issue_date"].(string); ok {
		issueDate, _ = time.Parse(time.RFC3339, isStr)
	}
	if ddStr, ok := ar.Payload["due_date"].(string); ok {
		dueDate, _ = time.Parse(time.RFC3339, ddStr)
	}
	if issueDate.IsZero() {
		issueDate = time.Now().UTC()
	}

	inv := entity.Invoice{
		InvoiceID:       invoiceID,
		CompanyID:       companyID,
		WorkspaceID:     workspaceID,
		IssueDate:       issueDate,
		DueDate:         dueDate,
		PaymentStatus:   entity.PaymentStatusBelumBayar,
		CollectionStage: entity.CollectionStage0,
		Notes:           notes,
		PaymentTerms:    paymentTerms,
		CreatedBy:       createdBy,
	}

	if err := u.invoiceRepo.Create(ctx, tx, inv); err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to insert invoice")
	}

	if err := tx.Commit(); err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to commit invoice transaction")
	}

	// Append creation audit log (best-effort, outside transaction).
	_ = u.paymentLogRepo.Append(ctx, entity.PaymentLog{
		WorkspaceID: workspaceID,
		InvoiceID:   invoiceID,
		EventType:   entity.EventInvoiceCreated,
		NewStatus:   entity.PaymentStatusBelumBayar,
		Actor:       checkerEmail,
		Notes:       fmt.Sprintf("Invoice created via approval %s", approvalID),
		Timestamp:   time.Now().UTC(),
	})

	// Best-effort Paper.id integration.
	if u.paperidSvc != nil {
		paperURL, paperRef, perr := u.paperidSvc.Create(ctx, *ws, inv)
		if perr != nil {
			u.logger.Warn().Err(perr).Str("invoice_id", invoiceID).Msg("Paper.id create failed (non-blocking)")
		} else if paperURL != "" {
			_ = u.invoiceRepo.UpdateFields(ctx, invoiceID, map[string]interface{}{
				"paper_id_url": paperURL,
				"paper_id_ref": paperRef,
			})
		}
	}

	return &inv, nil
}

// ─── Update ────────────────────────────────────────────────────────────────

func (u *invoiceUsecase) Update(ctx context.Context, invoiceID string, patch map[string]any) error {
	ctx, span := u.tracer.Start(ctx, "invoice.usecase.Update")
	defer span.End()

	inv, err := u.invoiceRepo.GetByID(ctx, invoiceID)
	if err != nil {
		return err
	}
	if inv == nil {
		return apperror.NotFound("invoice", "")
	}

	// Guard: payment_status must not be set directly by application code.
	delete(patch, "payment_status")
	patch["updated_at"] = time.Now().UTC()

	return u.invoiceRepo.UpdateFields(ctx, invoiceID, patch)
}

// ─── Delete ────────────────────────────────────────────────────────────────

func (u *invoiceUsecase) Delete(ctx context.Context, invoiceID string) error {
	ctx, span := u.tracer.Start(ctx, "invoice.usecase.Delete")
	defer span.End()

	inv, err := u.invoiceRepo.GetByID(ctx, invoiceID)
	if err != nil {
		return err
	}
	if inv == nil {
		return apperror.NotFound("invoice", "")
	}
	if inv.PaymentStatus != entity.PaymentStatusBelumBayar {
		return apperror.BadRequest(fmt.Sprintf(
			"invoice cannot be deleted in status %q — only %q invoices may be deleted",
			inv.PaymentStatus, entity.PaymentStatusBelumBayar,
		))
	}
	return u.invoiceRepo.Delete(ctx, invoiceID)
}

// ─── MarkPaid (checker-maker) ──────────────────────────────────────────────

// MarkPaid queues an approval request for marking an invoice as paid.
func (u *invoiceUsecase) MarkPaid(ctx context.Context, invoiceID string, req entity.MarkPaidReq) (*entity.ApprovalRequest, error) {
	ctx, span := u.tracer.Start(ctx, "invoice.usecase.MarkPaid")
	defer span.End()
	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	inv, err := u.invoiceRepo.GetByID(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, apperror.NotFound("invoice", "")
	}
	if inv.PaymentStatus == entity.PaymentStatusLunas {
		return nil, apperror.BadRequest("invoice is already paid")
	}

	payload := map[string]any{
		"invoice_id":     invoiceID,
		"workspace_id":   inv.WorkspaceID,
		"old_status":     inv.PaymentStatus,
		"payment_method": req.PaymentMethod,
		"payment_date":   req.PaymentDate,
		"amount_paid":    req.AmountPaid,
		"notes":          req.Notes,
		"actor":          req.Actor,
	}
	desc := fmt.Sprintf("Mark invoice %s as paid (amount: %d, method: %s)", invoiceID, req.AmountPaid, req.PaymentMethod)

	ar, err := u.approvalRepo.Create(ctx, &entity.ApprovalRequest{
		WorkspaceID: inv.WorkspaceID,
		RequestType: "mark_invoice_paid",
		Description: desc,
		Payload:     payload,
		MakerEmail:  req.Actor,
	})
	if err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to create mark-paid approval request")
	}
	return ar, nil
}

// ApplyMarkPaid executes the mark-paid transition after approval.
//
// NOTE: This is the only application-code path that writes payment_status directly.
// All other writes happen via Paper.id webhook (see paperid.go).
func (u *invoiceUsecase) ApplyMarkPaid(ctx context.Context, workspaceID, approvalID, checkerEmail string) error {
	ctx, span := u.tracer.Start(ctx, "invoice.usecase.ApplyMarkPaid")
	defer span.End()
	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	ar, err := u.approvalRepo.GetByID(ctx, workspaceID, approvalID)
	if err != nil {
		return err
	}
	if ar == nil {
		return apperror.NotFound("approval_request", "")
	}
	if ar.RequestType != "mark_invoice_paid" {
		return apperror.BadRequest("approval is not a mark_invoice_paid request")
	}
	if ar.Status != entity.ApprovalStatusPending {
		return apperror.BadRequest("approval is not pending")
	}

	invoiceID, _ := ar.Payload["invoice_id"].(string)
	oldStatus, _ := ar.Payload["old_status"].(string)
	paymentMethod, _ := ar.Payload["payment_method"].(string)
	notes, _ := ar.Payload["notes"].(string)
	_, _ = ar.Payload["actor"].(string) // actor stored in payload; checker-email is used for audit
	var amountPaid int64
	if ap, ok := ar.Payload["amount_paid"].(float64); ok {
		amountPaid = int64(ap)
	}

	now := time.Now().UTC()
	fields := map[string]interface{}{
		"payment_status": entity.PaymentStatusLunas,
		"paid_at":        now,
		"amount_paid":    amountPaid,
		"payment_method": paymentMethod,
		"updated_at":     now,
	}

	if err := u.invoiceRepo.UpdateFields(ctx, invoiceID, fields); err != nil {
		return apperror.WrapInternal(logger, err, "Failed to update invoice to paid")
	}

	_ = u.paymentLogRepo.Append(ctx, entity.PaymentLog{
		WorkspaceID:   workspaceID,
		InvoiceID:     invoiceID,
		EventType:     entity.EventManualMarkPaid,
		AmountPaid:    &amountPaid,
		PaymentMethod: paymentMethod,
		OldStatus:     oldStatus,
		NewStatus:     entity.PaymentStatusLunas,
		Actor:         checkerEmail,
		Notes:         notes,
		Timestamp:     now,
	})

	return nil
}

// ─── SendReminder ──────────────────────────────────────────────────────────

func (u *invoiceUsecase) SendReminder(ctx context.Context, invoiceID string, req entity.SendReminderReq) error {
	ctx, span := u.tracer.Start(ctx, "invoice.usecase.SendReminder")
	defer span.End()
	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	inv, err := u.invoiceRepo.GetByID(ctx, invoiceID)
	if err != nil {
		return err
	}
	if inv == nil {
		return apperror.NotFound("invoice", "")
	}
	if inv.PaymentStatus == entity.PaymentStatusLunas {
		return apperror.BadRequest("invoice is already paid — no reminder needed")
	}

	now := time.Now().UTC()
	err = u.invoiceRepo.UpdateFields(ctx, invoiceID, map[string]interface{}{
		"reminder_count":     inv.ReminderCount + 1,
		"last_reminder_date": now,
		"updated_at":         now,
	})
	if err != nil {
		return apperror.WrapInternal(logger, err, "Failed to update reminder count")
	}

	_ = u.paymentLogRepo.Append(ctx, entity.PaymentLog{
		WorkspaceID: inv.WorkspaceID,
		InvoiceID:   invoiceID,
		EventType:   entity.EventReminderSent,
		Actor:       req.Actor,
		Notes:       fmt.Sprintf("channel=%s template=%s", req.Channel, req.TemplateID),
		Timestamp:   now,
	})

	return nil
}

// ─── Stats / ByStage ───────────────────────────────────────────────────────

func (u *invoiceUsecase) Stats(ctx context.Context, wsIDs []string) (*entity.InvoiceStats, error) {
	ctx, span := u.tracer.Start(ctx, "invoice.usecase.Stats")
	defer span.End()
	return u.invoiceRepo.Stats(ctx, wsIDs)
}

func (u *invoiceUsecase) ByStage(ctx context.Context, wsIDs []string) (map[string]int64, error) {
	ctx, span := u.tracer.Start(ctx, "invoice.usecase.ByStage")
	defer span.End()
	stats, err := u.invoiceRepo.Stats(ctx, wsIDs)
	if err != nil {
		return nil, err
	}
	return stats.ByCollectionStage, nil
}

// ─── PaymentLogs ───────────────────────────────────────────────────────────

func (u *invoiceUsecase) PaymentLogs(ctx context.Context, invoiceID string) ([]entity.PaymentLog, error) {
	ctx, span := u.tracer.Start(ctx, "invoice.usecase.PaymentLogs")
	defer span.End()
	return u.paymentLogRepo.GetByInvoiceID(ctx, invoiceID, 100)
}
