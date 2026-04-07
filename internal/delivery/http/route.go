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
	healthH := health.NewHealthHandler(deps.Logger, deps.Tracer)
	cronH := cronHandler.NewCronHandler(deps.CronRunner, deps.Logger)
	waH := webhookHandler.NewWAWebhookHandler(deps.ReplyHandler, deps.Logger)
	checkinH := webhookHandler.NewCheckinFormHTTPHandler(deps.CheckinHandler, deps.Logger)
	handoffH := webhookHandler.NewHandoffHTTPHandler(deps.HandoffHandler, deps.Logger)
	paymentVerifyH := paymentHandler.NewVerifyPaymentHTTPHandler(deps.PaymentVerifier, deps.Logger)

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

	// Cron — OIDC auth
	wrappedCronRun := middleware.WrapErrorHandler(cronH.Run, deps.ExceptionHandler)
	r.Handle(http.MethodGet, "/cron/run", oidcAuth(wrappedCronRun))

	// Webhook — HaloAI signature verification; immediate 200 response
	r.Handle(http.MethodPost, "/webhook/wa", haloaiSig(waH.Handle))

	// Webhook — checkin form (no auth)
	r.Handle(http.MethodPost, "/webhook/checkin-form", checkinH.Handle)

	// API — BD handoff with HMAC auth
	wrappedHandoff := middleware.WrapErrorHandler(handoffH.Handle, deps.ExceptionHandler)
	r.Handle(http.MethodPost, "/api/handoff/new-client", hmacAuth(wrappedHandoff))

	// API — Payment verification with HMAC auth
	wrappedPaymentVerify := middleware.WrapErrorHandler(paymentVerifyH.Handle, deps.ExceptionHandler)
	r.Handle(http.MethodPost, "/api/payment/verify", hmacAuth(wrappedPaymentVerify))

	// Dashboard API — JWT auth
	dashboardH := dashboard.NewClientHandler(deps.DashboardUsecase)
	workspaceH := dashboard.NewWorkspaceHandler(deps.DashboardUsecase)

	wrappedWorkspaceList := middleware.WrapErrorHandler(workspaceH.List, deps.ExceptionHandler)
	r.Handle(http.MethodGet, "/api/dashboard/workspaces", jwtAuth(wrappedWorkspaceList))

	wrappedClientList := middleware.WrapErrorHandler(dashboardH.List, deps.ExceptionHandler)
	r.Handle(http.MethodGet, "/api/dashboard/clients", jwtAuth(wrappedClientList))
	wrappedClientGet := middleware.WrapErrorHandler(dashboardH.Get, deps.ExceptionHandler)
	r.Handle(http.MethodGet, "/api/dashboard/clients/{company_id}", jwtAuth(wrappedClientGet))
	wrappedClientCreate := middleware.WrapErrorHandler(dashboardH.Create, deps.ExceptionHandler)
	r.Handle(http.MethodPost, "/api/dashboard/clients", jwtAuth(wrappedClientCreate))
	wrappedClientUpdate := middleware.WrapErrorHandler(dashboardH.Update, deps.ExceptionHandler)
	r.Handle(http.MethodPut, "/api/dashboard/clients/{company_id}", jwtAuth(wrappedClientUpdate))
	wrappedClientDelete := middleware.WrapErrorHandler(dashboardH.Delete, deps.ExceptionHandler)
	r.Handle(http.MethodDelete, "/api/dashboard/clients/{company_id}", jwtAuth(wrappedClientDelete))
	wrappedClientInvoices := middleware.WrapErrorHandler(dashboardH.GetInvoices, deps.ExceptionHandler)
	r.Handle(http.MethodGet, "/api/dashboard/clients/{company_id}/invoices", jwtAuth(wrappedClientInvoices))
	wrappedClientEscalations := middleware.WrapErrorHandler(dashboardH.GetEscalations, deps.ExceptionHandler)
	r.Handle(http.MethodGet, "/api/dashboard/clients/{company_id}/escalations", jwtAuth(wrappedClientEscalations))

	// Swagger
	api := r.Group(deps.Cfg.RoutePrefix)
	api.HandlePrefix(http.MethodGet, "/swagger/", httpSwagger.WrapHandler)

	return r
}
