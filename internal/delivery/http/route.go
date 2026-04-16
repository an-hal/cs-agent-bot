package http

import (
	"net/http"

	cronHandler "github.com/Sejutacita/cs-agent-bot/internal/delivery/http/cron"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/dashboard"
	deliveryHttpDeps "github.com/Sejutacita/cs-agent-bot/internal/delivery/http/deps"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/health"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/middleware"
	paymentHandler "github.com/Sejutacita/cs-agent-bot/internal/delivery/http/payment"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	webhookHandler "github.com/Sejutacita/cs-agent-bot/internal/delivery/http/webhook"

	httpSwagger "github.com/swaggo/http-swagger"
)

func SetupHandler(deps deliveryHttpDeps.Deps) http.Handler {
	// Initialize handlers
	healthH := health.NewHealthHandler(deps.Logger, deps.Tracer)
	cronH := cronHandler.NewCronHandler(deps.CronRunner, deps.Logger)
	waH := webhookHandler.NewWAWebhookHandler(deps.ReplyHandler, deps.Logger)
	checkinH := webhookHandler.NewCheckinFormHTTPHandler(deps.CheckinHandler, deps.Logger)
	handoffH := webhookHandler.NewHandoffHTTPHandler(deps.HandoffHandler, deps.Logger)
	paymentVerifyH := paymentHandler.NewVerifyPaymentHTTPHandler(deps.PaymentVerifier, deps.Logger)
	dashboardH := dashboard.NewClientHandler(deps.DashboardUsecase, deps.Logger, deps.Tracer)
	workspaceH := dashboard.NewWorkspaceHandler(deps.DashboardUsecase, deps.Logger, deps.Tracer)
	activityH := dashboard.NewActivityHandler(deps.DashboardUsecase, deps.Logger, deps.Tracer)
	invoiceH := dashboard.NewInvoiceHandler(deps.DashboardUsecase, deps.Logger, deps.Tracer)
	templateH := dashboard.NewTemplateHandler(deps.DashboardUsecase, deps.Logger, deps.Tracer)
	escalationH := dashboard.NewEscalationHandler(deps.DashboardUsecase, deps.Logger, deps.Tracer)
	bgJobH := dashboard.NewBackgroundJobHandler(deps.DashboardUsecase, deps.Logger, deps.Tracer)
	triggerRuleH := dashboard.NewTriggerRuleHandler(deps.TriggerRuleRepo, deps.LogRepo, deps.RuleEngine, deps.Logger, deps.Tracer)
	systemConfigH := dashboard.NewSystemConfigHandler(deps.SystemConfigRepo, deps.LogRepo, deps.Logger, deps.Tracer)

	// Per-route auth wrappers
	oidcAuth := middleware.OIDCAuthMiddleware(deps.Cfg.AppURL, deps.Cfg.SchedulerSAEmail, deps.Cfg.Env, deps.Logger)
	haloaiSig := middleware.HaloAISignatureMiddleware(deps.Cfg.WAWebhookSecret, deps.Cfg.Env, deps.Logger)
	hmacAuth := middleware.HMACAuthMiddleware(deps.Cfg.HandoffWebhookSecret, deps.Cfg.Env, deps.Logger)
	jwtAuth := middleware.JWTAuthMiddleware(deps.Cfg.JWTValidateURL, deps.Logger)
	wsRequired := middleware.WorkspaceIDMiddleware()

	r := router.NewRouter()

	r.SetErrorHandler(middleware.ErrorHandlingMiddleware(deps.ExceptionHandler))

	// Apply global middleware
	r.Use(middleware.TracingMiddleware(deps.Tracer))
	r.Use(middleware.RecoveryMiddleware(deps.Logger, deps.ExceptionHandler))
	r.Use(middleware.RequestIDMiddleware())
	r.Use(middleware.LoggingMiddleware(deps.Logger))

	// Health check
	r.Handle(http.MethodGet, "/readiness", healthH.Check)

	// API routes
	api := r.Group(deps.Cfg.RoutePrefix)
	api.Handle(http.MethodGet, "/cron/run", oidcAuth(cronH.Run))
	api.Handle(http.MethodPost, "/handoff/new-client", hmacAuth(handoffH.Handle))
	api.Handle(http.MethodPost, "/payment/verify", hmacAuth(paymentVerifyH.Handle))
	api.Handle(http.MethodGet, "/workspaces", jwtAuth(workspaceH.List))
	api.Handle(http.MethodGet, "/activity-logs", wsRequired(jwtAuth(activityH.List)))
	api.Handle(http.MethodPost, "/activity-logs", wsRequired(jwtAuth(activityH.Record)))
	api.Handle(http.MethodGet, "/activity-logs/recent", wsRequired(jwtAuth(activityH.Recent)))
	api.Handle(http.MethodGet, "/activity-logs/stats", wsRequired(jwtAuth(activityH.Stats)))
	api.Handle(http.MethodGet, "/activity-logs/companies/{company_id}/summary", wsRequired(jwtAuth(activityH.CompanySummary)))
	api.Handle(http.MethodGet, "/jobs", wsRequired(jwtAuth(bgJobH.ListJobs)))
	api.Handle(http.MethodGet, "/jobs/{job_id}", wsRequired(jwtAuth(bgJobH.GetJob)))
	api.Handle(http.MethodGet, "/jobs/{job_id}/download", wsRequired(jwtAuth(bgJobH.DownloadJobFile)))

	dataMaster := api.Group("/data-master")
	dataMaster.Handle(http.MethodGet, "/clients", wsRequired(jwtAuth(dashboardH.List)))
	dataMaster.Handle(http.MethodPost, "/clients/import", wsRequired(jwtAuth(bgJobH.ImportClients)))
	dataMaster.Handle(http.MethodPost, "/clients/export", wsRequired(jwtAuth(bgJobH.ExportClients)))
	dataMaster.Handle(http.MethodGet, "/clients/{company_id}", wsRequired(jwtAuth(dashboardH.Get)))
	dataMaster.Handle(http.MethodPost, "/clients", wsRequired(jwtAuth(dashboardH.Create)))
	dataMaster.Handle(http.MethodPut, "/clients/{company_id}", wsRequired(jwtAuth(dashboardH.Update)))
	dataMaster.Handle(http.MethodDelete, "/clients/{company_id}", wsRequired(jwtAuth(dashboardH.Delete)))
	dataMaster.Handle(http.MethodGet, "/clients/{company_id}/escalations", wsRequired(jwtAuth(escalationH.ListByCompany)))
	dataMaster.Handle(http.MethodGet, "/invoices", wsRequired(jwtAuth(invoiceH.List)))
	dataMaster.Handle(http.MethodGet, "/invoices/{invoice_id}", wsRequired(jwtAuth(invoiceH.Get)))
	dataMaster.Handle(http.MethodPut, "/invoices/{invoice_id}", wsRequired(jwtAuth(invoiceH.Update)))
	dataMaster.Handle(http.MethodGet, "/escalations", wsRequired(jwtAuth(escalationH.List)))
	dataMaster.Handle(http.MethodGet, "/escalations/{id}", wsRequired(jwtAuth(escalationH.Get)))
	dataMaster.Handle(http.MethodPut, "/escalations/{id}/resolve", wsRequired(jwtAuth(escalationH.Resolve)))
	dataMaster.Handle(http.MethodGet, "/message-templates", wsRequired(jwtAuth(templateH.List)))
	dataMaster.Handle(http.MethodGet, "/message-templates/{template_id}", wsRequired(jwtAuth(templateH.Get)))
	dataMaster.Handle(http.MethodPut, "/message-templates/{template_id}", wsRequired(jwtAuth(templateH.Update)))
	dataMaster.Handle(http.MethodGet, "/trigger-rules", jwtAuth(triggerRuleH.List))
	dataMaster.Handle(http.MethodGet, "/trigger-rules/{rule_id}", jwtAuth(triggerRuleH.Get))
	dataMaster.Handle(http.MethodPost, "/trigger-rules", jwtAuth(triggerRuleH.Create))
	dataMaster.Handle(http.MethodPut, "/trigger-rules/{rule_id}", jwtAuth(triggerRuleH.Update))
	dataMaster.Handle(http.MethodDelete, "/trigger-rules/{rule_id}", jwtAuth(triggerRuleH.Delete))
	dataMaster.Handle(http.MethodPost, "/trigger-rules/cache/invalidate", jwtAuth(triggerRuleH.InvalidateCache))
	dataMaster.Handle(http.MethodGet, "/template-variables", jwtAuth(triggerRuleH.ListVariables))
	dataMaster.Handle(http.MethodGet, "/system-config", jwtAuth(systemConfigH.List))
	dataMaster.Handle(http.MethodPut, "/system-config/{key}", jwtAuth(systemConfigH.Update))

	// Swagger
	r.HandlePrefix(http.MethodGet, "/swagger", httpSwagger.WrapHandler)

	// Webhook
	webhook := api.Group("/webhook")
	webhook.Handle(http.MethodPost, "/wa", haloaiSig(waH.Handle))
	webhook.Handle(http.MethodPost, "/checkin-form", checkinH.Handle)

	return r
}
