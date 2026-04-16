package dashboard

import (
	"context"
	"io"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/jobstore"
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
	GetClientsByWorkspaceID(ctx context.Context, filter entity.ClientFilter, p pagination.Params) (*ClientListResult, error)
	GetClient(ctx context.Context, companyID string) (*entity.Client, error)
	CreateClient(ctx context.Context, client entity.Client) error
	UpdateClient(ctx context.Context, companyID string, fields map[string]interface{}) error
	DeleteClient(ctx context.Context, companyID string) error
	RecordActivity(ctx context.Context, entry entity.ActivityLog) error
	GetActivityLogs(ctx context.Context, filter entity.ActivityFilter) ([]entity.ActivityLog, int, error)
	GetActivityStats(ctx context.Context, workspaceID string) (entity.ActivityStats, error)
	GetRecentActivities(ctx context.Context, workspaceID string, since time.Time, limit int) ([]entity.ActivityLog, error)
	GetCompanySummary(ctx context.Context, workspaceID, companyID string) (*entity.CompanySummary, error)

	// Escalations (standalone)
	GetEscalations(ctx context.Context, filter entity.EscalationFilter, p pagination.Params) ([]entity.Escalation, int64, error)
	GetEscalation(ctx context.Context, id string) (*entity.Escalation, error)
	ResolveEscalation(ctx context.Context, id, resolvedBy, resolutionNote string) error
	GetEscalationsByCompany(ctx context.Context, workspaceID, companyID string, p pagination.Params) ([]entity.Escalation, int64, error)

	// Invoices (standalone)
	GetInvoices(ctx context.Context, filter entity.InvoiceFilter, p pagination.Params) ([]entity.Invoice, int64, error)
	GetInvoice(ctx context.Context, invoiceID string) (*entity.Invoice, error)
	UpdateInvoice(ctx context.Context, invoiceID string, fields map[string]interface{}) error

	// Templates
	GetTemplates(ctx context.Context, filter entity.TemplateFilter, p pagination.Params) ([]entity.Template, int64, error)
	GetTemplate(ctx context.Context, templateID string) (*entity.Template, error)
	UpdateTemplate(ctx context.Context, templateID string, fields map[string]interface{}) error

	// Background Jobs
	StartImportClients(ctx context.Context, workspaceID, actor, filename string, file io.Reader, updateExisting bool) (*entity.BackgroundJob, error)
	StartExportClients(ctx context.Context, workspaceID, actor string, filter entity.ClientFilter) (*entity.BackgroundJob, error)
	GetBackgroundJob(ctx context.Context, jobID string) (*entity.BackgroundJob, error)
	ListBackgroundJobs(ctx context.Context, workspaceID, jobType, entityType string, p pagination.Params) ([]entity.BackgroundJob, int64, error)
	DownloadJobFile(ctx context.Context, jobID, workspaceID string) (filename string, r io.ReadCloser, err error)
}

type dashboardUsecase struct {
	workspaceRepo  repository.WorkspaceRepository
	clientRepo     repository.ClientRepository
	invoiceRepo    repository.InvoiceRepository
	escalationRepo repository.EscalationRepository
	logRepo        repository.LogRepository
	templateRepo   repository.TemplateRepository
	bgJobRepo      repository.BackgroundJobRepository
	fileStore      jobstore.FileStore
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
	bgJobRepo repository.BackgroundJobRepository,
	fileStore jobstore.FileStore,
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
		bgJobRepo:      bgJobRepo,
		fileStore:      fileStore,
		tracer:         tr,
		logger:         logger,
	}
}

// resolveWorkspaceIDs returns the list of workspace IDs to query.
// If the given workspaceID belongs to a holding workspace, it returns MemberIDs.
// Otherwise it returns a slice containing just the original workspaceID.
func (u *dashboardUsecase) resolveWorkspaceIDs(ctx context.Context, workspaceID string) ([]string, error) {
	ws, err := u.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if ws != nil && ws.IsHolding && len(ws.MemberIDs) > 0 {
		return ws.MemberIDs, nil
	}
	return []string{workspaceID}, nil
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

func (u *dashboardUsecase) GetClientsByWorkspaceID(ctx context.Context, filter entity.ClientFilter, p pagination.Params) (*ClientListResult, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetClientsByWorkspaceID")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)
	logger.Info().Strs("workspace_ids", filter.WorkspaceIDs).Int("offset", p.Offset).Int("limit", p.Limit).Msg("Fetching clients by workspace ID")

	if len(filter.WorkspaceIDs) == 1 {
		wsIDs, err := u.resolveWorkspaceIDs(ctx, filter.WorkspaceIDs[0])
		if err != nil {
			return nil, apperror.WrapInternal(logger, err, "Failed to resolve workspace IDs")
		}
		filter.WorkspaceIDs = wsIDs
	}

	total, err := u.clientRepo.CountByFilter(ctx, filter)
	if err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to count clients")
	}

	clients, err := u.clientRepo.FetchByFilter(ctx, filter, p)
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

	if len(filter.WorkspaceIDs) == 1 {
		wsIDs, err := u.resolveWorkspaceIDs(ctx, filter.WorkspaceIDs[0])
		if err != nil {
			return nil, 0, apperror.WrapInternal(logger, err, "Failed to resolve workspace IDs")
		}
		filter.WorkspaceIDs = wsIDs
	}

	logs, total, err := u.logRepo.GetActivities(ctx, filter)
	if err != nil {
		return nil, 0, apperror.WrapInternal(logger, err, "Failed to fetch activity logs")
	}
	return logs, total, nil
}

// ─── Escalations (standalone) ────────────────────────────────────────────────

func (u *dashboardUsecase) GetEscalations(ctx context.Context, filter entity.EscalationFilter, p pagination.Params) ([]entity.Escalation, int64, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetEscalations")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	if len(filter.WorkspaceIDs) == 1 {
		wsIDs, err := u.resolveWorkspaceIDs(ctx, filter.WorkspaceIDs[0])
		if err != nil {
			return nil, 0, apperror.WrapInternal(logger, err, "Failed to resolve workspace IDs")
		}
		filter.WorkspaceIDs = wsIDs
	}

	escalations, total, err := u.escalationRepo.GetAllPaginated(ctx, filter, p)
	if err != nil {
		return nil, 0, apperror.WrapInternal(logger, err, "Failed to fetch escalations")
	}
	return escalations, total, nil
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

	if len(filter.WorkspaceIDs) == 1 {
		wsIDs, err := u.resolveWorkspaceIDs(ctx, filter.WorkspaceIDs[0])
		if err != nil {
			return nil, 0, apperror.WrapInternal(logger, err, "Failed to resolve workspace IDs")
		}
		filter.WorkspaceIDs = wsIDs
	}

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
	"template_name":   true,
	"wa_content":      true,
	"language":        true,
	"channel":         true,
	"email_subject":   true,
	"email_body_html": true,
	"email_body_text": true,
	"active":          true,
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

// ─── Activity Stats / Recent / Summary ──────────────────────────────────────

func (u *dashboardUsecase) GetActivityStats(ctx context.Context, workspaceID string) (entity.ActivityStats, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetActivityStats")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	wsIDs, err := u.resolveWorkspaceIDs(ctx, workspaceID)
	if err != nil {
		return entity.ActivityStats{}, apperror.WrapInternal(logger, err, "Failed to resolve workspace IDs")
	}

	stats, err := u.logRepo.GetActivityStats(ctx, wsIDs)
	if err != nil {
		return entity.ActivityStats{}, apperror.WrapInternal(logger, err, "Failed to fetch activity stats")
	}
	return stats, nil
}

func (u *dashboardUsecase) GetRecentActivities(ctx context.Context, workspaceID string, since time.Time, limit int) ([]entity.ActivityLog, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetRecentActivities")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	wsIDs, err := u.resolveWorkspaceIDs(ctx, workspaceID)
	if err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to resolve workspace IDs")
	}

	logs, err := u.logRepo.GetRecentActivities(ctx, wsIDs, since, limit)
	if err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to fetch recent activities")
	}
	return logs, nil
}

func (u *dashboardUsecase) GetCompanySummary(ctx context.Context, workspaceID, companyID string) (*entity.CompanySummary, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetCompanySummary")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	wsIDs, err := u.resolveWorkspaceIDs(ctx, workspaceID)
	if err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to resolve workspace IDs")
	}

	summary, err := u.logRepo.GetCompanySummary(ctx, wsIDs, companyID)
	if err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to fetch company summary")
	}
	return summary, nil
}

// ─── Escalation: Get / Resolve / ListByCompany ─────────────────────────────

func (u *dashboardUsecase) GetEscalation(ctx context.Context, id string) (*entity.Escalation, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetEscalation")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	esc, err := u.escalationRepo.GetByID(ctx, id)
	if err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to get escalation")
	}
	if esc == nil {
		return nil, apperror.NotFound("escalation", "Escalation not found")
	}
	return esc, nil
}

func (u *dashboardUsecase) ResolveEscalation(ctx context.Context, id, resolvedBy, resolutionNote string) error {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.ResolveEscalation")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	if err := u.escalationRepo.Resolve(ctx, id, resolvedBy, resolutionNote); err != nil {
		return apperror.WrapInternal(logger, err, "Failed to resolve escalation")
	}
	return nil
}

func (u *dashboardUsecase) GetEscalationsByCompany(ctx context.Context, workspaceID, companyID string, p pagination.Params) ([]entity.Escalation, int64, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetEscalationsByCompany")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	wsIDs, err := u.resolveWorkspaceIDs(ctx, workspaceID)
	if err != nil {
		return nil, 0, apperror.WrapInternal(logger, err, "Failed to resolve workspace IDs")
	}

	filter := entity.EscalationFilter{
		WorkspaceIDs: wsIDs,
		CompanyID:    companyID,
	}

	escalations, total, err := u.escalationRepo.GetAllPaginated(ctx, filter, p)
	if err != nil {
		return nil, 0, apperror.WrapInternal(logger, err, "Failed to fetch company escalations")
	}
	return escalations, total, nil
}
