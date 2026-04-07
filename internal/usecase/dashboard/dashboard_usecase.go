package dashboard

import (
	"context"
	"fmt"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
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
}

type dashboardUsecase struct {
	workspaceRepo  repository.WorkspaceRepository
	clientRepo     repository.ClientRepository
	invoiceRepo    repository.InvoiceRepository
	escalationRepo repository.EscalationRepository
	logRepo        repository.LogRepository
	tracer         tracer.Tracer
	logger         zerolog.Logger
}

func NewDashboardUsecase(
	workspaceRepo repository.WorkspaceRepository,
	clientRepo repository.ClientRepository,
	invoiceRepo repository.InvoiceRepository,
	escalationRepo repository.EscalationRepository,
	logRepo repository.LogRepository,
	tr tracer.Tracer,
	logger zerolog.Logger,
) DashboardUsecase {
	return &dashboardUsecase{
		workspaceRepo:  workspaceRepo,
		clientRepo:     clientRepo,
		invoiceRepo:    invoiceRepo,
		escalationRepo: escalationRepo,
		logRepo:        logRepo,
		tracer:         tr,
		logger:         logger,
	}
}

func (u *dashboardUsecase) GetWorkspaces(ctx context.Context) ([]entity.Workspace, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetWorkspaces")
	defer span.End()
	return u.workspaceRepo.GetAll(ctx)
}

func (u *dashboardUsecase) GetWorkspaceBySlug(ctx context.Context, slug string) (*entity.Workspace, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetWorkspaceBySlug")
	defer span.End()
	return u.workspaceRepo.GetBySlug(ctx, slug)
}

func (u *dashboardUsecase) GetClients(ctx context.Context, workspaceSlug string, p pagination.Params) ([]entity.Client, int64, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetClients")
	defer span.End()

	ws, err := u.workspaceRepo.GetBySlug(ctx, workspaceSlug)
	if err != nil {
		return nil, 0, fmt.Errorf("get workspace: %w", err)
	}
	if ws == nil {
		return nil, 0, fmt.Errorf("workspace not found: %s", workspaceSlug)
	}

	if ws.IsHolding && len(ws.MemberIDs) > 0 {
		return u.clientRepo.GetAllByWorkspaceIDsPaginated(ctx, ws.MemberIDs, p)
	}
	return u.clientRepo.GetAllByWorkspacePaginated(ctx, workspaceSlug, p)
}

func (u *dashboardUsecase) GetClientsByWorkspaceID(ctx context.Context, workspaceID string, p pagination.Params) (*ClientListResult, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetClientsByWorkspaceID")
	defer span.End()

	total, err := u.clientRepo.CountByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("count clients: %w", err)
	}

	clients, err := u.clientRepo.FetchByWorkspaceID(ctx, workspaceID, p)
	if err != nil {
		return nil, fmt.Errorf("fetch clients: %w", err)
	}

	return &ClientListResult{
		Clients: clients,
		Meta:    pagination.NewMeta(p, total),
	}, nil
}

func (u *dashboardUsecase) GetClient(ctx context.Context, companyID string) (*entity.Client, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetClient")
	defer span.End()
	return u.clientRepo.GetByID(ctx, companyID)
}

func (u *dashboardUsecase) CreateClient(ctx context.Context, client entity.Client) error {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.CreateClient")
	defer span.End()

	if client.SequenceCS == "" {
		client.SequenceCS = "ACTIVE"
	}
	if client.PaymentStatus == "" {
		client.PaymentStatus = "Paid"
	}
	return u.clientRepo.CreateClient(ctx, client)
}

func (u *dashboardUsecase) UpdateClient(ctx context.Context, companyID string, fields map[string]interface{}) error {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.UpdateClient")
	defer span.End()
	return u.clientRepo.UpdateClientFields(ctx, companyID, fields)
}

func (u *dashboardUsecase) DeleteClient(ctx context.Context, companyID string) error {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.DeleteClient")
	defer span.End()
	return u.clientRepo.UpdateClientFields(ctx, companyID, map[string]interface{}{
		"blacklisted": true,
	})
}

func (u *dashboardUsecase) GetClientInvoices(ctx context.Context, companyID string, p pagination.Params) ([]entity.Invoice, int64, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetClientInvoices")
	defer span.End()
	return u.invoiceRepo.GetAllByCompanyIDPaginated(ctx, companyID, p)
}

func (u *dashboardUsecase) GetClientEscalations(ctx context.Context, companyID string, p pagination.Params) ([]entity.Escalation, int64, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetClientEscalations")
	defer span.End()
	return u.escalationRepo.GetByCompanyIDPaginated(ctx, companyID, p)
}

func (u *dashboardUsecase) RecordActivity(ctx context.Context, entry entity.ActivityLog) error {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.RecordActivity")
	defer span.End()
	return u.logRepo.AppendActivity(ctx, entry)
}

func (u *dashboardUsecase) GetActivityLogs(ctx context.Context, filter entity.ActivityFilter) ([]entity.ActivityLog, int, error) {
	ctx, span := u.tracer.Start(ctx, "dashboard.usecase.GetActivityLogs")
	defer span.End()
	return u.logRepo.GetActivities(ctx, filter)
}
