package http

import (
	"github.com/Sejutacita/cs-agent-bot/config"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/validator"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	analyticsuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/analytics"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/approval"
	auditws "github.com/Sejutacita/cs-agent-bot/internal/usecase/audit_workspace_access"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/auth"
	claudeextractionuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/claude_extraction"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/coaching"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/fireflies"
	firefliesclient "github.com/Sejutacita/cs-agent-bot/internal/usecase/fireflies_client"
	haloaimock "github.com/Sejutacita/cs-agent-bot/internal/usecase/haloai_mock"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/mockoutbox"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/rediscache"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/pdp"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/reactivation"
	smtpclient "github.com/Sejutacita/cs-agent-bot/internal/usecase/smtp_client"
	rejectionanalysisuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/rejection_analysis"
	automationrule "github.com/Sejutacita/cs-agent-bot/internal/usecase/automation_rule"
	collectionuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/collection"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/cron"
	customfield "github.com/Sejutacita/cs-agent-bot/internal/usecase/custom_field"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/dashboard"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/invoice"
	manualaction "github.com/Sejutacita/cs-agent-bot/internal/usecase/manual_action"
	masterdata "github.com/Sejutacita/cs-agent-bot/internal/usecase/master_data"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/messaging"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/notification"
	usecasePayment "github.com/Sejutacita/cs-agent-bot/internal/usecase/payment"
	pipelineview "github.com/Sejutacita/cs-agent-bot/internal/usecase/pipeline_view"
	reportsuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/reports"
	teamuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/team"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/trigger"
	userpreferences "github.com/Sejutacita/cs-agent-bot/internal/usecase/user_preferences"
	workspaceintegration "github.com/Sejutacita/cs-agent-bot/internal/usecase/workspace_integration"
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

	// Shared: user preferences (per-user, per-workspace UI state)
	UserPreferencesUC userpreferences.Usecase

	// Shared: per-workspace integration credentials (HaloAI/Telegram/Paper.id/SMTP)
	WorkspaceIntegrationUC workspaceintegration.Usecase

	// Shared: central approval dispatcher (routes by request_type)
	ApprovalDispatcher approval.Dispatcher

	// Feature 06-workflow-engine/07: manual action queue (GUARD)
	ManualActionUC manualaction.Usecase

	// Wave-1 extensions
	AuditWorkspaceAccessUC auditws.Usecase
	FirefliesUC            fireflies.Usecase
	ClaudeExtractionUC     claudeextractionuc.Usecase
	ReactivationUC         reactivation.Usecase
	CoachingUC             coaching.Usecase
	RejectionAnalysisUC    rejectionanalysisuc.Usecase

	// Mock plumbing (wired when MOCK_EXTERNAL_APIS=true or key absent)
	MockOutbox       *mockoutbox.Outbox
	MockClaudeClient claudeextractionuc.Client
	MockFFClient     firefliesclient.Client
	MockWASender     haloaimock.Sender
	MockSMTPClient   smtpclient.Client

	// PDP compliance (erasure + retention)
	PDPUC pdp.Usecase

	// Analytics cache (Redis 15-min; nil = no caching, handler still works)
	AnalyticsCache rediscache.Cache

	// Wave B2
	WorkspaceThemeRepo repository.WorkspaceThemeRepository
	ActivityFeedRepo   repository.ActivityFeedRepository

	// Wave B3
	TeamActivityRepo    repository.TeamActivityLogRepository
	RevokedSessionsRepo repository.RevokedSessionRepository
}
