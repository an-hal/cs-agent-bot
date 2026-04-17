package http

import (
	"github.com/Sejutacita/cs-agent-bot/config"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/validator"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	analyticsuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/analytics"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/auth"
	automationrule "github.com/Sejutacita/cs-agent-bot/internal/usecase/automation_rule"
	collectionuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/collection"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/cron"
	customfield "github.com/Sejutacita/cs-agent-bot/internal/usecase/custom_field"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/dashboard"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/invoice"
	masterdata "github.com/Sejutacita/cs-agent-bot/internal/usecase/master_data"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/messaging"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/notification"
	usecasePayment "github.com/Sejutacita/cs-agent-bot/internal/usecase/payment"
	pipelineview "github.com/Sejutacita/cs-agent-bot/internal/usecase/pipeline_view"
	reportsuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/reports"
	teamuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/team"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/trigger"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/webhook"
	workflowuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/workflow"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/workspace"
	"github.com/rs/zerolog"
)

type Deps struct {
	Cfg              *config.AppConfig
	Logger           zerolog.Logger
	Validator        *validator.Validator
	Tracer           tracer.Tracer
	ExceptionHandler *response.HTTPExceptionHandler
	CronRunner       cron.CronRunner
	ReplyHandler     webhook.ReplyHandler
	CheckinHandler   webhook.CheckinFormHandler
	HandoffHandler   webhook.HandoffHandler
	PaymentVerifier  usecasePayment.PaymentVerifier
	DashboardUsecase dashboard.DashboardUsecase
	LogRepo          repository.LogRepository
	TriggerRuleRepo  repository.TriggerRuleRepository
	SystemConfigRepo repository.SystemConfigRepository
	RuleEngine       *trigger.RuleEngine
	AuthUsecase      auth.AuthUsecase
	WorkspaceUC      workspace.Usecase
	NotificationUC   notification.Usecase
	MasterDataUC     masterdata.Usecase
	CustomFieldUC    customfield.Usecase
	TeamUC           teamuc.Usecase
	MessagingUC      messaging.Usecase
	WorkflowUC       workflowuc.Usecase
	AutomationRuleUC automationrule.Usecase
	PipelineViewUC   pipelineview.Usecase
	InvoiceUC        invoice.Usecase
	InvoiceCron      invoice.CronInvoice
	PaperIDSvc       invoice.PaperIDService

	// Analytics & Reports (feat/09)
	AnalyticsUC         analyticsuc.Usecase
	ReportsUC           reportsuc.Usecase
	RevenueTargetRepo   repository.RevenueTargetRepository
	RevenueSnapshotRepo repository.RevenueSnapshotRepository
	WorkspaceRepo       repository.WorkspaceRepository

	// Collections (feat/10)
	CollectionUC collectionuc.Usecase
}
