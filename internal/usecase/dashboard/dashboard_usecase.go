package dashboard

import (
	"context"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// ClientListResult contains paginated client results.
type ClientListResult struct {
	Clients []entity.Client
	Meta    pagination.Meta
}

type DashboardUsecase interface {
	GetWorkspaces(ctx context.Context) ([]entity.Workspace, error)
	GetWorkspaceBySlug(ctx context.Context, slug string) (*entity.Workspace, error)
	GetClients(ctx context.Context, workspaceSlug string, p pagination.Params) ([]entity.Client, int64, error)
	GetClientsByWorkspaceID(ctx context.Context, workspaceID string, p pagination.Params) (*ClientListResult, error)
	GetClient(ctx context.Context, companyID string) (*entity.Client, error)
	CreateClient(ctx context.Context, client entity.Client) error
	UpdateClient(ctx context.Context, companyID string, fields map[string]interface{}) error
	DeleteClient(ctx context.Context, companyID string) error
	GetClientInvoices(ctx context.Context, companyID string, p pagination.Params) ([]entity.Invoice, int64, error)
	GetClientEscalations(ctx context.Context, companyID string, p pagination.Params) ([]entity.Escalation, int64, error)
	RecordActivity(ctx context.Context, entry entity.ActivityLog) error
	GetActivityLogs(ctx context.Context, filter entity.ActivityFilter) ([]entity.ActivityLog, int, error)

	// Invoices (standalone)
	GetInvoices(ctx context.Context, filter entity.InvoiceFilter, p pagination.Params) ([]entity.Invoice, int64, error)
	GetInvoice(ctx context.Context, invoiceID string) (*entity.Invoice, error)
	UpdateInvoice(ctx context.Context, invoiceID string, fields map[string]interface{}) error

	// Templates
	GetTemplates(ctx context.Context, filter entity.TemplateFilter, p pagination.Params) ([]entity.Template, int64, error)
	GetTemplate(ctx context.Context, templateID string) (*entity.Template, error)
	UpdateTemplate(ctx context.Context, templateID string, fields map[string]interface{}) error
}

type dashboardUsecase struct {
	workspaceRepo  repository.WorkspaceRepository
	clientRepo     repository.ClientRepository
	invoiceRepo    repository.InvoiceRepository
	escalationRepo repository.EscalationRepository
	logRepo        repository.LogRepository
	templateRepo   repository.TemplateRepository
	tracer         tracer.Tracer
	logger         zerolog.Logger
}

func NewDashboardUsecase(
	workspaceRepo repository.WorkspaceRepository,
	clientRepo repository.ClientRepository,
	invoiceRepo repository.InvoiceRepository,
	escalationRepo repository.EscalationRepository,
	logRepo repository.LogRepository,
	templateRepo repository.TemplateRepository,
	tr tracer.Tracer,
	logger zerolog.Logger,
) DashboardUsecase {
	return &dashboardUsecase{
		workspaceRepo:  workspaceRepo,
		clientRepo:     clientRepo,
		invoiceRepo:    invoiceRepo,
		escalationRepo: escalationRepo,
		logRepo:        logRepo,
		templateRepo:   templateRepo,
		tracer:         tr,
		logger:         logger,
	}
}

func (u *dashboardUsecase) GetWorkspaces(ctx context.Context) ([]entity.Workspace, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetWorkspaces")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)
	logger.Info().Msg("Fetching all workspaces")

	workspaces, err := u.workspaceRepo.GetAll(ctx)
	if err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to fetch workspaces")
	}
	return workspaces, nil
}

func (u *dashboardUsecase) GetWorkspaceBySlug(ctx context.Context, slug string) (*entity.Workspace, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetWorkspaceBySlug")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	ws, err := u.workspaceRepo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to get workspace by slug")
	}
	return ws, nil
}

func (u *dashboardUsecase) GetClients(ctx context.Context, workspaceSlug string, p pagination.Params) ([]entity.Client, int64, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetClients")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)
	logger.Info().Str("workspace", workspaceSlug).Int("offset", p.Offset).Int("limit", p.Limit).Msg("Fetching clients")

	ws, err := u.workspaceRepo.GetBySlug(ctx, workspaceSlug)
	if err != nil {
		return nil, 0, apperror.WrapInternal(logger, err, "Failed to get workspace")
	}
	if ws == nil {
		return nil, 0, apperror.WrapNotFound(logger, nil, "workspace", "Workspace not found: "+workspaceSlug)
	}

	if ws.IsHolding && len(ws.MemberIDs) > 0 {
		clients, total, fetchErr := u.clientRepo.GetAllByWorkspaceIDsPaginated(ctx, ws.MemberIDs, p)
		if fetchErr != nil {
			return nil, 0, apperror.WrapInternal(logger, fetchErr, "Failed to fetch clients by workspace IDs")
		}
		return clients, total, nil
	}

	clients, total, fetchErr := u.clientRepo.GetAllByWorkspacePaginated(ctx, workspaceSlug, p)
	if fetchErr != nil {
		return nil, 0, apperror.WrapInternal(logger, fetchErr, "Failed to fetch clients by workspace")
	}
	return clients, total, nil
}

func (u *dashboardUsecase) GetClientsByWorkspaceID(ctx context.Context, workspaceID string, p pagination.Params) (*ClientListResult, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetClientsByWorkspaceID")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)
	logger.Info().Str("workspace_id", workspaceID).Int("offset", p.Offset).Int("limit", p.Limit).Msg("Fetching clients by workspace ID")

	total, err := u.clientRepo.CountByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to count clients")
	}

	clients, err := u.clientRepo.FetchByWorkspaceID(ctx, workspaceID, p)
	if err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to fetch clients")
	}

	return &ClientListResult{
		Clients: clients,
		Meta:    pagination.NewMeta(p, total),
	}, nil
}

func (u *dashboardUsecase) GetClient(ctx context.Context, companyID string) (*entity.Client, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetClient")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	client, err := u.clientRepo.GetByID(ctx, companyID)
	if err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to get client")
	}
	return client, nil
}

func (u *dashboardUsecase) CreateClient(ctx context.Context, client entity.Client) error {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.CreateClient")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)
	logger.Info().Str("company_id", client.CompanyID).Msg("Creating client")

	if client.SequenceCS == "" {
		client.SequenceCS = "ACTIVE"
	}
	if client.PaymentStatus == "" {
		client.PaymentStatus = "Paid"
	}

	if err := u.clientRepo.CreateClient(ctx, client); err != nil {
		return apperror.WrapInternal(logger, err, "Failed to create client")
	}
	return nil
}

func (u *dashboardUsecase) UpdateClient(ctx context.Context, companyID string, fields map[string]interface{}) error {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.UpdateClient")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)
	logger.Info().Str("company_id", companyID).Msg("Updating client")

	if err := u.clientRepo.UpdateClientFields(ctx, companyID, fields); err != nil {
		return apperror.WrapInternal(logger, err, "Failed to update client")
	}
	return nil
}

func (u *dashboardUsecase) DeleteClient(ctx context.Context, companyID string) error {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.DeleteClient")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)
	logger.Info().Str("company_id", companyID).Msg("Deleting client (soft)")

	if err := u.clientRepo.UpdateClientFields(ctx, companyID, map[string]interface{}{
		"blacklisted": true,
	}); err != nil {
		return apperror.WrapInternal(logger, err, "Failed to delete client")
	}
	return nil
}

func (u *dashboardUsecase) GetClientInvoices(ctx context.Context, companyID string, p pagination.Params) ([]entity.Invoice, int64, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetClientInvoices")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	invoices, total, err := u.invoiceRepo.GetAllByCompanyIDPaginated(ctx, companyID, p)
	if err != nil {
		return nil, 0, apperror.WrapInternal(logger, err, "Failed to fetch invoices")
	}
	return invoices, total, nil
}

func (u *dashboardUsecase) GetClientEscalations(ctx context.Context, companyID string, p pagination.Params) ([]entity.Escalation, int64, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetClientEscalations")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	escalations, total, err := u.escalationRepo.GetByCompanyIDPaginated(ctx, companyID, p)
	if err != nil {
		return nil, 0, apperror.WrapInternal(logger, err, "Failed to fetch escalations")
	}
	return escalations, total, nil
}

func (u *dashboardUsecase) RecordActivity(ctx context.Context, entry entity.ActivityLog) error {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.RecordActivity")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	if err := u.logRepo.AppendActivity(ctx, entry); err != nil {
		return apperror.WrapInternal(logger, err, "Failed to record activity")
	}
	return nil
}

func (u *dashboardUsecase) GetActivityLogs(ctx context.Context, filter entity.ActivityFilter) ([]entity.ActivityLog, int, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetActivityLogs")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	logs, total, err := u.logRepo.GetActivities(ctx, filter)
	if err != nil {
		return nil, 0, apperror.WrapInternal(logger, err, "Failed to fetch activity logs")
	}
	return logs, total, nil
}

// ─── Invoices (standalone) ───────────────────────────────────────────────────

var invoiceEditableFields = map[string]bool{
	"notes":          true,
	"link_invoice":   true,
	"payment_status": true,
	"amount_paid":    true,
	"paid_at":        true,
	"amount":         true,
	"issue_date":     true,
}

func (u *dashboardUsecase) GetInvoices(ctx context.Context, filter entity.InvoiceFilter, p pagination.Params) ([]entity.Invoice, int64, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetInvoices")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	invoices, total, err := u.invoiceRepo.GetAllPaginated(ctx, filter, p)
	if err != nil {
		return nil, 0, apperror.WrapInternal(logger, err, "Failed to fetch invoices")
	}
	return invoices, total, nil
}

func (u *dashboardUsecase) GetInvoice(ctx context.Context, invoiceID string) (*entity.Invoice, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetInvoice")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	inv, err := u.invoiceRepo.GetByID(ctx, invoiceID)
	if err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to get invoice")
	}
	return inv, nil
}

func (u *dashboardUsecase) UpdateInvoice(ctx context.Context, invoiceID string, fields map[string]interface{}) error {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.UpdateInvoice")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	for key := range fields {
		if !invoiceEditableFields[key] {
			return apperror.BadRequest("Field '" + key + "' is not editable")
		}
	}

	if err := u.invoiceRepo.UpdateFields(ctx, invoiceID, fields); err != nil {
		return apperror.WrapInternal(logger, err, "Failed to update invoice")
	}
	return nil
}

// ─── Templates ───────────────────────────────────────────────────────────────

var templateEditableFields = map[string]bool{
	"template_name":    true,
	"template_content": true,
	"language":         true,
	"active":           true,
}

func (u *dashboardUsecase) GetTemplates(ctx context.Context, filter entity.TemplateFilter, p pagination.Params) ([]entity.Template, int64, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetTemplates")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	templates, total, err := u.templateRepo.GetAllPaginated(ctx, filter, p)
	if err != nil {
		return nil, 0, apperror.WrapInternal(logger, err, "Failed to fetch templates")
	}
	return templates, total, nil
}

func (u *dashboardUsecase) GetTemplate(ctx context.Context, templateID string) (*entity.Template, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetTemplate")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	tmpl, err := u.templateRepo.GetTemplate(ctx, templateID)
	if err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to get template")
	}
	return tmpl, nil
}

func (u *dashboardUsecase) UpdateTemplate(ctx context.Context, templateID string, fields map[string]interface{}) error {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.UpdateTemplate")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	for key := range fields {
		if !templateEditableFields[key] {
			return apperror.BadRequest("Field '" + key + "' is not editable")
		}
	}

	if err := u.templateRepo.UpdateFields(ctx, templateID, fields); err != nil {
		return apperror.WrapInternal(logger, err, "Failed to update template")
	}
	return nil
}
