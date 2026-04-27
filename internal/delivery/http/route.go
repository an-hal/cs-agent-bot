package http

import (
	"net/http"

	authHandler "github.com/Sejutacita/cs-agent-bot/internal/delivery/http/auth"
	cronHandler "github.com/Sejutacita/cs-agent-bot/internal/delivery/http/cron"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/dashboard"
	deliveryHttpDeps "github.com/Sejutacita/cs-agent-bot/internal/delivery/http/deps"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/health"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/middleware"
	paymentHandler "github.com/Sejutacita/cs-agent-bot/internal/delivery/http/payment"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	teamHandler "github.com/Sejutacita/cs-agent-bot/internal/delivery/http/team"
	webhookHandler "github.com/Sejutacita/cs-agent-bot/internal/delivery/http/webhook"

	httpSwagger "github.com/swaggo/http-swagger"
)

// SetupHandler wires global middleware, builds all handlers + auth bundles,
// and registers every route by domain. Each per-domain helper sits below.
func SetupHandler(deps deliveryHttpDeps.Deps) http.Handler {
	h := buildHandlers(deps)
	mw := buildAuthBundle(deps)

	r := router.NewRouter()
	r.SetErrorHandler(middleware.ErrorHandlingMiddleware(deps.ExceptionHandler))
	r.Use(middleware.TracingMiddleware(deps.Tracer))
	r.Use(middleware.RecoveryMiddleware(deps.Logger, deps.ExceptionHandler))
	r.Use(middleware.RequestIDMiddleware())
	r.Use(middleware.LoggingMiddleware(deps.Logger))

	r.Handle(http.MethodGet, "/readiness", h.health.Check)
	r.HandlePrefix(http.MethodGet, "/swagger", httpSwagger.WrapHandler)

	api := r.Group(deps.Cfg.RoutePrefix)
	registerAuthRoutes(api, h, mw)
	registerCronAndWebhookRoutes(api, h, mw)
	registerWorkspaceRoutes(api, h, mw)
	registerNotificationsRoutes(api, h, mw)
	registerSharedRoutes(api, h, mw)
	registerActivityAndJobRoutes(api, h, mw)
	registerDataMasterRoutes(api, h, mw)
	registerInvoiceRoutes(api, h, mw)
	registerMasterDataRoutes(api, h, mw)
	registerTeamRoutes(api, h, mw, deps.TeamUC)
	registerMessagingRoutes(api, h, mw)
	registerWorkflowRoutes(api, h, mw)
	registerAutomationRuleRoutes(api, h, mw)
	registerAnalyticsRoutes(api, h, mw)
	registerReportsRoutes(api, h, mw)
	registerCollectionRoutes(api, h, mw)

	return r
}

// authBundle groups every per-route auth wrapper used across the API. All
// ErrorHandler-shaped wrappers compose freely; the HaloAI signature wrapper
// uses the older http.HandlerFunc shape and is exposed separately.
type authBundle struct {
	oidc, hmac, jwt, ws func(middleware.ErrorHandler) middleware.ErrorHandler
	halo                func(http.HandlerFunc) http.HandlerFunc
}

func buildAuthBundle(deps deliveryHttpDeps.Deps) authBundle {
	return authBundle{
		oidc: middleware.OIDCAuthMiddleware(deps.Cfg.AppURL, deps.Cfg.SchedulerSAEmail, deps.Cfg.Env, deps.Logger),
		halo: middleware.HaloAISignatureMiddleware(deps.Cfg.WAWebhookSecret, deps.Cfg.Env, deps.Logger),
		hmac: middleware.HMACAuthMiddleware(deps.Cfg.HandoffWebhookSecret, deps.Cfg.Env, deps.Logger),
		jwt:  middleware.JWTAuthMiddleware(deps.Cfg.JWTValidateURL, deps.Cfg.Env, deps.Cfg.JWTDevBypassEnabled, deps.Logger),
		ws:   middleware.WorkspaceIDMiddleware(),
	}
}

// handlers holds every HTTP handler built from the Deps struct. Built once
// per server start so route registration helpers can address them by name.
type handlers struct {
	health           *health.HealthHandler
	cron             *cronHandler.CronHandler
	wa               *webhookHandler.WAWebhookHandler
	checkin          *webhookHandler.CheckinFormHTTPHandler
	handoff          *webhookHandler.HandoffHTTPHandler
	payment          *paymentHandler.VerifyPaymentHTTPHandler
	dashboardClient  *dashboard.ClientHandler
	workspace        *dashboard.WorkspaceHandler
	notification     *dashboard.NotificationHandler
	userPreferences  *dashboard.UserPreferencesHandler
	integrations     *dashboard.WorkspaceIntegrationHandler
	approval         *dashboard.ApprovalHandler
	manualAction     *dashboard.ManualActionHandler
	auditWs          *dashboard.AuditWorkspaceAccessHandler
	firefliesDash    *dashboard.FirefliesHandler
	firefliesWebhook *webhookHandler.FirefliesWebhookHandler
	reactivation     *dashboard.ReactivationHandler
	coaching         *dashboard.CoachingHandler
	rejectionAnal    *dashboard.RejectionAnalysisHandler
	mock             *dashboard.MockHandler
	pdp              *dashboard.PDPHandler
	theme            *dashboard.WorkspaceThemeHandler
	feed             *dashboard.ActivityFeedHandler
	actionLog        *dashboard.ActionLogHandler
	teamAct          *dashboard.TeamActivityHandler
	session          *dashboard.SessionHandler
	activity         *dashboard.ActivityHandler
	invoice          *dashboard.InvoiceHandler
	invoiceCron      *dashboard.InvoiceCronHandler
	paperid          *webhookHandler.PaperIDWebhookHandler
	template         *dashboard.TemplateHandler
	messaging        *dashboard.MessagingHandler
	workflow         *dashboard.WorkflowHandler
	pipelineView     *dashboard.PipelineViewHandler
	automationRule   *dashboard.AutomationRuleHandler
	escalation       *dashboard.EscalationHandler
	bgJob            *dashboard.BackgroundJobHandler
	analytics        *dashboard.AnalyticsHandler
	reports          *dashboard.ReportsHandler
	revenueTarget    *dashboard.RevenueTargetHandler
	snapshotCron     *dashboard.SnapshotCronHandler
	collection       *dashboard.CollectionHandler
	collectionRec    *dashboard.CollectionRecordHandler
	triggerRule      *dashboard.TriggerRuleHandler
	systemConfig     *dashboard.SystemConfigHandler
	masterData       *dashboard.MasterDataHandler
	customField      *dashboard.CustomFieldHandler
	auth             *authHandler.AuthHandler
	team             *teamHandler.Handler
}

func buildHandlers(deps deliveryHttpDeps.Deps) *handlers {
	return &handlers{
		health:           health.NewHealthHandler(deps.Logger, deps.Tracer),
		cron:             cronHandler.NewCronHandler(deps.CronRunner, deps.Logger),
		wa:               webhookHandler.NewWAWebhookHandler(deps.ReplyHandler, deps.Logger),
		checkin:          webhookHandler.NewCheckinFormHTTPHandler(deps.CheckinHandler, deps.Logger),
		handoff:          webhookHandler.NewHandoffHTTPHandler(deps.HandoffHandler, deps.Logger),
		payment:          paymentHandler.NewVerifyPaymentHTTPHandler(deps.PaymentVerifier, deps.Logger),
		dashboardClient:  dashboard.NewClientHandler(deps.DashboardUsecase, deps.Logger, deps.Tracer),
		workspace:        dashboard.NewWorkspaceHandler(deps.WorkspaceUC, deps.Logger, deps.Tracer),
		notification:     dashboard.NewNotificationHandler(deps.NotificationUC, deps.Logger, deps.Tracer),
		userPreferences:  dashboard.NewUserPreferencesHandler(deps.UserPreferencesUC, deps.Logger, deps.Tracer),
		integrations:     dashboard.NewWorkspaceIntegrationHandler(deps.WorkspaceIntegrationUC, deps.Logger, deps.Tracer),
		approval:         dashboard.NewApprovalHandler(deps.ApprovalDispatcher, deps.Logger, deps.Tracer),
		manualAction:     dashboard.NewManualActionHandler(deps.ManualActionUC, deps.Logger, deps.Tracer),
		auditWs:          dashboard.NewAuditWorkspaceAccessHandler(deps.AuditWorkspaceAccessUC, deps.Logger, deps.Tracer),
		firefliesDash:    dashboard.NewFirefliesHandler(deps.FirefliesUC, deps.Logger, deps.Tracer),
		firefliesWebhook: webhookHandler.NewFirefliesWebhookHandler(deps.FirefliesUC, deps.Logger),
		reactivation:     dashboard.NewReactivationHandler(deps.ReactivationUC, deps.Logger, deps.Tracer),
		coaching:         dashboard.NewCoachingHandler(deps.CoachingUC, deps.Logger, deps.Tracer),
		rejectionAnal:    dashboard.NewRejectionAnalysisHandler(deps.RejectionAnalysisUC, deps.Logger, deps.Tracer),
		mock:             dashboard.NewMockHandler(deps.MockOutbox, deps.MockClaudeClient, deps.MockFFClient, deps.MockWASender, deps.MockSMTPClient, deps.Logger, deps.Tracer),
		pdp:              dashboard.NewPDPHandler(deps.PDPUC, deps.Logger, deps.Tracer),
		theme:            dashboard.NewWorkspaceThemeHandler(deps.WorkspaceThemeRepo, deps.Logger, deps.Tracer),
		feed:             dashboard.NewActivityFeedHandler(deps.ActivityFeedRepo, deps.Logger, deps.Tracer),
		actionLog:        dashboard.NewActionLogHandler(deps.LogRepo, deps.Logger, deps.Tracer),
		teamAct:          dashboard.NewTeamActivityHandler(deps.TeamActivityRepo, deps.Logger, deps.Tracer),
		session:          dashboard.NewSessionHandler(deps.RevokedSessionsRepo, deps.Logger, deps.Tracer),
		activity:         dashboard.NewActivityHandler(deps.DashboardUsecase, deps.Logger, deps.Tracer),
		invoice:          dashboard.NewInvoiceHandler(deps.DashboardUsecase, deps.InvoiceUC, deps.Logger, deps.Tracer),
		invoiceCron:      dashboard.NewInvoiceCronHandler(deps.InvoiceCron, deps.Logger, deps.Tracer),
		paperid:          webhookHandler.NewPaperIDWebhookHandler(deps.PaperIDSvc, deps.Logger),
		template:         dashboard.NewTemplateHandler(deps.DashboardUsecase, deps.Logger, deps.Tracer),
		messaging:        dashboard.NewMessagingHandler(deps.MessagingUC, deps.Logger, deps.Tracer),
		workflow:         dashboard.NewWorkflowHandler(deps.WorkflowUC, deps.Logger, deps.Tracer),
		pipelineView:     dashboard.NewPipelineViewHandler(deps.WorkflowUC, deps.PipelineViewUC, deps.Logger, deps.Tracer),
		automationRule:   dashboard.NewAutomationRuleHandler(deps.AutomationRuleUC, deps.Logger, deps.Tracer),
		escalation:       dashboard.NewEscalationHandler(deps.DashboardUsecase, deps.Logger, deps.Tracer),
		bgJob:            dashboard.NewBackgroundJobHandler(deps.DashboardUsecase, deps.Logger, deps.Tracer),
		analytics:        dashboard.NewAnalyticsHandler(deps.AnalyticsUC, deps.Logger, deps.Tracer).WithCache(deps.AnalyticsCache),
		reports:          dashboard.NewReportsHandler(deps.ReportsUC, deps.Logger, deps.Tracer),
		revenueTarget:    dashboard.NewRevenueTargetHandler(deps.RevenueTargetRepo, deps.Logger, deps.Tracer),
		snapshotCron:     dashboard.NewSnapshotCronHandler(deps.WorkspaceRepo, deps.RevenueSnapshotRepo, deps.Logger, deps.Tracer),
		collection:       dashboard.NewCollectionHandler(deps.CollectionUC, deps.Logger, deps.Tracer),
		collectionRec:    dashboard.NewCollectionRecordHandler(deps.CollectionUC, deps.Logger, deps.Tracer),
		triggerRule:      dashboard.NewTriggerRuleHandler(deps.TriggerRuleRepo, deps.LogRepo, deps.RuleEngine, deps.Logger, deps.Tracer),
		systemConfig:     dashboard.NewSystemConfigHandler(deps.SystemConfigRepo, deps.LogRepo, deps.Logger, deps.Tracer),
		masterData:       dashboard.NewMasterDataHandler(deps.MasterDataUC, deps.Logger, deps.Tracer),
		customField:      dashboard.NewCustomFieldHandler(deps.CustomFieldUC, deps.Logger, deps.Tracer),
		auth:             authHandler.NewAuthHandler(deps.AuthUsecase, deps.Cfg.Env, deps.Logger, deps.Tracer),
		team:             teamHandler.NewHandler(deps.TeamUC, deps.Logger, deps.Tracer),
	}
}
