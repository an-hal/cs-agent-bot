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

	// Per-route auth wrappers
	oidcAuth := middleware.OIDCAuthMiddleware(deps.Cfg.AppURL, deps.Cfg.SchedulerSAEmail, deps.Cfg.Env, deps.Logger)
	haloaiSig := middleware.HaloAISignatureMiddleware(deps.Cfg.WAWebhookSecret, deps.Cfg.Env, deps.Logger)
	hmacAuth := middleware.HMACAuthMiddleware(deps.Cfg.HandoffWebhookSecret, deps.Cfg.Env, deps.Logger)
	jwtAuth := middleware.JWTAuthMiddleware(deps.Cfg.JWTValidateURL, deps.Logger)

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
	api.Handle(http.MethodGet, "/dashboard/workspaces", jwtAuth(workspaceH.List))
	api.Handle(http.MethodGet, "/dashboard/clients", jwtAuth(dashboardH.List))
	api.Handle(http.MethodGet, "/dashboard/clients/{company_id}", jwtAuth(dashboardH.Get))
	api.Handle(http.MethodPost, "/dashboard/clients", jwtAuth(dashboardH.Create))
	api.Handle(http.MethodPut, "/dashboard/clients/{company_id}", jwtAuth(dashboardH.Update))
	api.Handle(http.MethodDelete, "/dashboard/clients/{company_id}", jwtAuth(dashboardH.Delete))
	api.Handle(http.MethodGet, "/dashboard/clients/{company_id}/invoices", jwtAuth(dashboardH.GetInvoices))
	api.Handle(http.MethodGet, "/dashboard/clients/{company_id}/escalations", jwtAuth(dashboardH.GetEscalations))
	api.Handle(http.MethodGet, "/dashboard/workspaces/{workspace_id}/clients", jwtAuth(dashboardH.ListByWorkspaceID))
	api.Handle(http.MethodGet, "/dashboard/activity-logs", jwtAuth(activityH.List))
	api.Handle(http.MethodPost, "/dashboard/activity-logs", jwtAuth(activityH.Record))
	api.Handle(http.MethodGet, "/dashboard/invoices", jwtAuth(invoiceH.List))
	api.Handle(http.MethodGet, "/dashboard/invoices/{invoice_id}", jwtAuth(invoiceH.Get))
	api.Handle(http.MethodPut, "/dashboard/invoices/{invoice_id}", jwtAuth(invoiceH.Update))
	api.Handle(http.MethodGet, "/dashboard/message-templates", jwtAuth(templateH.List))
	api.Handle(http.MethodGet, "/dashboard/message-templates/{template_id}", jwtAuth(templateH.Get))
	api.Handle(http.MethodPut, "/dashboard/message-templates/{template_id}", jwtAuth(templateH.Update))

	// Swagger
	api.HandlePrefix(http.MethodGet, "/swagger/", httpSwagger.WrapHandler)

	// Webhook
	webhook := api.Group("/webhook")
	webhook.Handle(http.MethodPost, "/wa", haloaiSig(waH.Handle))
	webhook.Handle(http.MethodPost, "/checkin-form", checkinH.Handle)

	return r
}
