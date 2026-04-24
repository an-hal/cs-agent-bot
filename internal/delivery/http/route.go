package http

import (
	"net/http"

	authHandler "github.com/Sejutacita/cs-agent-bot/internal/delivery/http/auth"
	cronHandler "github.com/Sejutacita/cs-agent-bot/internal/delivery/http/cron"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/dashboard"
	teamHandler "github.com/Sejutacita/cs-agent-bot/internal/delivery/http/team"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
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
	workspaceH := dashboard.NewWorkspaceHandler(deps.WorkspaceUC, deps.Logger, deps.Tracer)
	notificationH := dashboard.NewNotificationHandler(deps.NotificationUC, deps.Logger, deps.Tracer)
	userPreferencesH := dashboard.NewUserPreferencesHandler(deps.UserPreferencesUC, deps.Logger, deps.Tracer)
	integrationsH := dashboard.NewWorkspaceIntegrationHandler(deps.WorkspaceIntegrationUC, deps.Logger, deps.Tracer)
	approvalH := dashboard.NewApprovalHandler(deps.ApprovalDispatcher, deps.Logger, deps.Tracer)
	manualActionH := dashboard.NewManualActionHandler(deps.ManualActionUC, deps.Logger, deps.Tracer)
	auditWsH := dashboard.NewAuditWorkspaceAccessHandler(deps.AuditWorkspaceAccessUC, deps.Logger, deps.Tracer)
	firefliesDashH := dashboard.NewFirefliesHandler(deps.FirefliesUC, deps.Logger, deps.Tracer)
	firefliesWebhookH := webhookHandler.NewFirefliesWebhookHandler(deps.FirefliesUC, deps.Logger)
	reactivationH := dashboard.NewReactivationHandler(deps.ReactivationUC, deps.Logger, deps.Tracer)
	coachingH := dashboard.NewCoachingHandler(deps.CoachingUC, deps.Logger, deps.Tracer)
	rejectionAnalysisH := dashboard.NewRejectionAnalysisHandler(deps.RejectionAnalysisUC, deps.Logger, deps.Tracer)
	mockH := dashboard.NewMockHandler(deps.MockOutbox, deps.MockClaudeClient, deps.MockFFClient, deps.MockWASender, deps.MockSMTPClient, deps.Logger, deps.Tracer)
	pdpH := dashboard.NewPDPHandler(deps.PDPUC, deps.Logger, deps.Tracer)
	themeH := dashboard.NewWorkspaceThemeHandler(deps.WorkspaceThemeRepo, deps.Logger, deps.Tracer)
	feedH := dashboard.NewActivityFeedHandler(deps.ActivityFeedRepo, deps.Logger, deps.Tracer)
	actionLogH := dashboard.NewActionLogHandler(deps.LogRepo, deps.Logger, deps.Tracer)
	teamActH := dashboard.NewTeamActivityHandler(deps.TeamActivityRepo, deps.Logger, deps.Tracer)
	sessionH := dashboard.NewSessionHandler(deps.RevokedSessionsRepo, deps.Logger, deps.Tracer)
	activityH := dashboard.NewActivityHandler(deps.DashboardUsecase, deps.Logger, deps.Tracer)
	invoiceH := dashboard.NewInvoiceHandler(deps.DashboardUsecase, deps.InvoiceUC, deps.Logger, deps.Tracer)
	invoiceCronH := dashboard.NewInvoiceCronHandler(deps.InvoiceCron, deps.Logger, deps.Tracer)
	paperidH := webhookHandler.NewPaperIDWebhookHandler(deps.PaperIDSvc, deps.Logger)
	templateH := dashboard.NewTemplateHandler(deps.DashboardUsecase, deps.Logger, deps.Tracer)
	messagingH := dashboard.NewMessagingHandler(deps.MessagingUC, deps.Logger, deps.Tracer)
	workflowH := dashboard.NewWorkflowHandler(deps.WorkflowUC, deps.Logger, deps.Tracer)
	pipelineViewH := dashboard.NewPipelineViewHandler(deps.WorkflowUC, deps.PipelineViewUC, deps.Logger, deps.Tracer)
	automationRuleH := dashboard.NewAutomationRuleHandler(deps.AutomationRuleUC, deps.Logger, deps.Tracer)
	escalationH := dashboard.NewEscalationHandler(deps.DashboardUsecase, deps.Logger, deps.Tracer)
	bgJobH := dashboard.NewBackgroundJobHandler(deps.DashboardUsecase, deps.Logger, deps.Tracer)
	analyticsH := dashboard.NewAnalyticsHandler(deps.AnalyticsUC, deps.Logger, deps.Tracer).WithCache(deps.AnalyticsCache)
	reportsH := dashboard.NewReportsHandler(deps.ReportsUC, deps.Logger, deps.Tracer)
	revenueTargetH := dashboard.NewRevenueTargetHandler(deps.RevenueTargetRepo, deps.Logger, deps.Tracer)
	snapshotCronH := dashboard.NewSnapshotCronHandler(deps.WorkspaceRepo, deps.RevenueSnapshotRepo, deps.Logger, deps.Tracer)
	collectionH := dashboard.NewCollectionHandler(deps.CollectionUC, deps.Logger, deps.Tracer)
	collectionRecordH := dashboard.NewCollectionRecordHandler(deps.CollectionUC, deps.Logger, deps.Tracer)
	triggerRuleH := dashboard.NewTriggerRuleHandler(deps.TriggerRuleRepo, deps.LogRepo, deps.RuleEngine, deps.Logger, deps.Tracer)
	systemConfigH := dashboard.NewSystemConfigHandler(deps.SystemConfigRepo, deps.LogRepo, deps.Logger, deps.Tracer)
	masterDataH := dashboard.NewMasterDataHandler(deps.MasterDataUC, deps.Logger, deps.Tracer)
	customFieldH := dashboard.NewCustomFieldHandler(deps.CustomFieldUC, deps.Logger, deps.Tracer)
	authH := authHandler.NewAuthHandler(deps.AuthUsecase, deps.Cfg.Env, deps.Logger, deps.Tracer)
	teamH := teamHandler.NewHandler(deps.TeamUC, deps.Logger, deps.Tracer)

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
	api.Handle(http.MethodPost, "/auth/login", authH.Login)
	api.Handle(http.MethodPost, "/auth/google", authH.LoginGoogle)
	api.Handle(http.MethodPost, "/auth/logout", authH.Logout)
	api.Handle(http.MethodGet, "/whitelist/check", authH.CheckWhitelist)
	api.Handle(http.MethodGet, "/whitelist", jwtAuth(authH.ListWhitelist))
	api.Handle(http.MethodPost, "/whitelist", jwtAuth(authH.AddWhitelist))
	api.Handle(http.MethodDelete, "/whitelist/{id}", jwtAuth(authH.DeleteWhitelist))

	api.Handle(http.MethodGet, "/cron/run", oidcAuth(cronH.Run))
	api.Handle(http.MethodPost, "/handoff/new-client", hmacAuth(handoffH.Handle))
	api.Handle(http.MethodPost, "/payment/verify", hmacAuth(paymentVerifyH.Handle))
	api.Handle(http.MethodGet, "/workspaces", jwtAuth(workspaceH.List))
	api.Handle(http.MethodPost, "/workspaces", jwtAuth(workspaceH.Create))
	api.Handle(http.MethodGet, "/workspaces/{id}", jwtAuth(workspaceH.Get))
	api.Handle(http.MethodPut, "/workspaces/{id}", jwtAuth(workspaceH.Update))
	api.Handle(http.MethodDelete, "/workspaces/{id}", jwtAuth(workspaceH.SoftDelete))
	api.Handle(http.MethodPost, "/workspaces/{id}/switch", jwtAuth(workspaceH.Switch))
	api.Handle(http.MethodGet, "/workspaces/{id}/members", jwtAuth(workspaceH.ListMembers))
	api.Handle(http.MethodPost, "/workspaces/{id}/members/invite", jwtAuth(workspaceH.Invite))
	api.Handle(http.MethodPut, "/workspaces/{id}/members/{member_id}", jwtAuth(workspaceH.UpdateMemberRole))
	api.Handle(http.MethodDelete, "/workspaces/{id}/members/{member_id}", jwtAuth(workspaceH.RemoveMember))
	api.Handle(http.MethodPost, "/workspaces/invitations/{token}/accept", jwtAuth(workspaceH.AcceptInvitation))
	api.Handle(http.MethodGet, "/notifications", wsRequired(jwtAuth(notificationH.List)))
	api.Handle(http.MethodPost, "/notifications", wsRequired(jwtAuth(notificationH.Create)))
	api.Handle(http.MethodGet, "/notifications/count", wsRequired(jwtAuth(notificationH.Count)))
	api.Handle(http.MethodPut, "/notifications/{id}/read", wsRequired(jwtAuth(notificationH.MarkRead)))
	api.Handle(http.MethodPut, "/notifications/read-all", wsRequired(jwtAuth(notificationH.MarkAllRead)))

	// User preferences (shared) — per-user, per-workspace UI state
	api.Handle(http.MethodGet, "/preferences", wsRequired(jwtAuth(userPreferencesH.List)))
	api.Handle(http.MethodGet, "/preferences/{namespace}", wsRequired(jwtAuth(userPreferencesH.Get)))
	api.Handle(http.MethodPut, "/preferences/{namespace}", wsRequired(jwtAuth(userPreferencesH.Upsert)))
	api.Handle(http.MethodDelete, "/preferences/{namespace}", wsRequired(jwtAuth(userPreferencesH.Delete)))

	// Workspace integrations (shared) — per-workspace HaloAI/Telegram/Paper.id/SMTP credentials
	api.Handle(http.MethodGet, "/integrations", wsRequired(jwtAuth(integrationsH.List)))
	api.Handle(http.MethodGet, "/integrations/{provider}", wsRequired(jwtAuth(integrationsH.Get)))
	api.Handle(http.MethodPut, "/integrations/{provider}", wsRequired(jwtAuth(integrationsH.Upsert)))
	api.Handle(http.MethodDelete, "/integrations/{provider}", wsRequired(jwtAuth(integrationsH.Delete)))

	// Central approval dispatcher (routes by request_type)
	api.Handle(http.MethodPost, "/approvals/{id}/apply", wsRequired(jwtAuth(approvalH.Apply)))

	// Manual action queue (feat/06 GUARD)
	api.Handle(http.MethodGet, "/manual-actions", wsRequired(jwtAuth(manualActionH.List)))
	api.Handle(http.MethodGet, "/manual-actions/{id}", wsRequired(jwtAuth(manualActionH.Get)))
	api.Handle(http.MethodPatch, "/manual-actions/{id}/mark-sent", wsRequired(jwtAuth(manualActionH.MarkSent)))
	api.Handle(http.MethodPatch, "/manual-actions/{id}/skip", wsRequired(jwtAuth(manualActionH.Skip)))

	// Audit logs — cross-workspace access (feat/01 §2c + feat/08)
	api.Handle(http.MethodGet, "/audit-logs/workspace-access", wsRequired(jwtAuth(auditWsH.List)))
	api.Handle(http.MethodPost, "/audit-logs/workspace-access", wsRequired(jwtAuth(auditWsH.Record)))

	// Fireflies (feat/00-shared/07) — dashboard views + webhook receiver
	api.Handle(http.MethodGet, "/fireflies/transcripts", wsRequired(jwtAuth(firefliesDashH.List)))
	api.Handle(http.MethodGet, "/fireflies/transcripts/{id}", wsRequired(jwtAuth(firefliesDashH.Get)))
	api.Handle(http.MethodPost, "/webhook/fireflies/{workspace_id}", hmacAuth(firefliesWebhookH.Handle))

	// Reactivation triggers + events (feat/03 master-data)
	api.Handle(http.MethodGet, "/reactivation/triggers", wsRequired(jwtAuth(reactivationH.ListTriggers)))
	api.Handle(http.MethodPost, "/reactivation/triggers", wsRequired(jwtAuth(reactivationH.UpsertTrigger)))
	api.Handle(http.MethodGet, "/reactivation/triggers/{id}", wsRequired(jwtAuth(reactivationH.GetTrigger)))
	api.Handle(http.MethodDelete, "/reactivation/triggers/{id}", wsRequired(jwtAuth(reactivationH.DeleteTrigger)))
	api.Handle(http.MethodPost, "/master-data/clients/{id}/reactivate", wsRequired(jwtAuth(reactivationH.Reactivate)))
	api.Handle(http.MethodGet, "/master-data/clients/{id}/reactivation-history", wsRequired(jwtAuth(reactivationH.History)))

	// Coaching sessions (feat/00-shared/11)
	api.Handle(http.MethodGet, "/coaching/sessions", wsRequired(jwtAuth(coachingH.List)))
	api.Handle(http.MethodPost, "/coaching/sessions", wsRequired(jwtAuth(coachingH.Create)))
	api.Handle(http.MethodGet, "/coaching/sessions/{id}", wsRequired(jwtAuth(coachingH.Get)))
	api.Handle(http.MethodPatch, "/coaching/sessions/{id}", wsRequired(jwtAuth(coachingH.Update)))
	api.Handle(http.MethodPost, "/coaching/sessions/{id}/submit", wsRequired(jwtAuth(coachingH.Submit)))
	api.Handle(http.MethodDelete, "/coaching/sessions/{id}", wsRequired(jwtAuth(coachingH.Delete)))

	// Rejection analysis (feat/05-messaging)
	api.Handle(http.MethodGet, "/rejection-analysis", wsRequired(jwtAuth(rejectionAnalysisH.List)))
	api.Handle(http.MethodPost, "/rejection-analysis", wsRequired(jwtAuth(rejectionAnalysisH.Record)))
	api.Handle(http.MethodPost, "/rejection-analysis/analyze", wsRequired(jwtAuth(rejectionAnalysisH.Analyze)))
	api.Handle(http.MethodGet, "/rejection-analysis/stats", wsRequired(jwtAuth(rejectionAnalysisH.Stats)))

	// Mock external-API endpoints (FE QA) — JWT only, no workspace header needed.
	api.Handle(http.MethodGet, "/mock/outbox", jwtAuth(mockH.ListOutbox))
	api.Handle(http.MethodGet, "/mock/outbox/{id}", jwtAuth(mockH.GetOutboxRecord))
	api.Handle(http.MethodDelete, "/mock/outbox", jwtAuth(mockH.ClearOutbox))
	api.Handle(http.MethodPost, "/mock/claude/extract", jwtAuth(mockH.TriggerClaude))
	api.Handle(http.MethodPost, "/mock/fireflies/fetch", jwtAuth(mockH.TriggerFireflies))
	api.Handle(http.MethodPost, "/mock/haloai/send", jwtAuth(mockH.TriggerHaloAI))
	api.Handle(http.MethodPost, "/mock/smtp/send", jwtAuth(mockH.TriggerSMTP))

	// PDP compliance (feat/00-shared/08)
	api.Handle(http.MethodGet, "/pdp/erasure-requests", wsRequired(jwtAuth(pdpH.ListErasure)))
	api.Handle(http.MethodPost, "/pdp/erasure-requests", wsRequired(jwtAuth(pdpH.CreateErasure)))
	api.Handle(http.MethodGet, "/pdp/erasure-requests/{id}", wsRequired(jwtAuth(pdpH.GetErasure)))
	api.Handle(http.MethodPost, "/pdp/erasure-requests/{id}/approve", wsRequired(jwtAuth(pdpH.ApproveErasure)))
	api.Handle(http.MethodPost, "/pdp/erasure-requests/{id}/reject", wsRequired(jwtAuth(pdpH.RejectErasure)))
	api.Handle(http.MethodPost, "/pdp/erasure-requests/{id}/execute", wsRequired(jwtAuth(pdpH.ExecuteErasure)))
	api.Handle(http.MethodGet, "/pdp/retention-policies", wsRequired(jwtAuth(pdpH.ListPolicies)))
	api.Handle(http.MethodPost, "/pdp/retention-policies", wsRequired(jwtAuth(pdpH.UpsertPolicy)))
	api.Handle(http.MethodDelete, "/pdp/retention-policies/{id}", wsRequired(jwtAuth(pdpH.DeletePolicy)))
	api.Handle(http.MethodGet, "/cron/pdp/retention", wsRequired(jwtAuth(pdpH.RunRetention)))

	// Workspace theme + holding expansion (feat/02)
	api.Handle(http.MethodGet, "/workspace/theme", wsRequired(jwtAuth(themeH.Get)))
	api.Handle(http.MethodPut, "/workspace/theme", wsRequired(jwtAuth(themeH.Upsert)))
	api.Handle(http.MethodGet, "/workspace/holding/expand", wsRequired(jwtAuth(themeH.ExpandHolding)))

	// Unified activity feed (feat/08)
	api.Handle(http.MethodGet, "/activity-log/feed", wsRequired(jwtAuth(feedH.Feed)))
	api.Handle(http.MethodGet, "/activity-log/today", wsRequired(jwtAuth(actionLogH.TodayActivity)))

	// Bot-action aggregation (dashboard sidebar — feat/08 §bot activity)
	api.Handle(http.MethodGet, "/action-log/recent", wsRequired(jwtAuth(actionLogH.Recent)))
	api.Handle(http.MethodGet, "/action-log/today", wsRequired(jwtAuth(actionLogH.Today)))
	api.Handle(http.MethodGet, "/action-log/summary", wsRequired(jwtAuth(actionLogH.Summary)))

	// Team activity logs (feat/04)
	api.Handle(http.MethodGet, "/team/activity", wsRequired(jwtAuth(teamActH.List)))
	api.Handle(http.MethodPost, "/team/activity", wsRequired(jwtAuth(teamActH.Record)))

	// Session revocation (feat/01 §2c)
	api.Handle(http.MethodPost, "/sessions/revoke", wsRequired(jwtAuth(sessionH.Revoke)))
	api.Handle(http.MethodGet, "/sessions/revoked", wsRequired(jwtAuth(sessionH.List)))
	api.Handle(http.MethodGet, "/cron/sessions/cleanup", oidcAuth(sessionH.Cleanup))
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

	// Full-featured invoice routes.
	invoices := api.Group("/invoices")
	invoices.Handle(http.MethodGet, "", wsRequired(jwtAuth(invoiceH.List)))
	invoices.Handle(http.MethodPost, "", wsRequired(jwtAuth(invoiceH.Create)))
	invoices.Handle(http.MethodGet, "/stats", wsRequired(jwtAuth(invoiceH.Stats)))
	invoices.Handle(http.MethodGet, "/by-stage", wsRequired(jwtAuth(invoiceH.ByStage)))
	invoices.Handle(http.MethodGet, "/{invoice_id}", wsRequired(jwtAuth(invoiceH.Get)))
	invoices.Handle(http.MethodPut, "/{invoice_id}", wsRequired(jwtAuth(invoiceH.Update)))
	invoices.Handle(http.MethodDelete, "/{invoice_id}", wsRequired(jwtAuth(invoiceH.Delete)))
	invoices.Handle(http.MethodPost, "/{invoice_id}/mark-paid", wsRequired(jwtAuth(invoiceH.MarkPaid)))
	invoices.Handle(http.MethodPost, "/{invoice_id}/send-reminder", wsRequired(jwtAuth(invoiceH.SendReminder)))
	invoices.Handle(http.MethodGet, "/{invoice_id}/payment-logs", wsRequired(jwtAuth(invoiceH.PaymentLogs)))
	invoices.Handle(http.MethodGet, "/{invoice_id}/pdf", wsRequired(jwtAuth(invoiceH.PDF)))
	invoices.Handle(http.MethodGet, "/{invoice_id}/activity", wsRequired(jwtAuth(invoiceH.Activity)))
	invoices.Handle(http.MethodPost, "/{invoice_id}/update-stage", wsRequired(jwtAuth(invoiceH.UpdateStage)))
	invoices.Handle(http.MethodPut, "/{invoice_id}/confirm-partial", wsRequired(jwtAuth(invoiceH.ConfirmPartial)))

	// Paper.id webhook (HMAC verified inside the usecase — no JWT).
	api.Handle(http.MethodPost, "/webhook/paperid/{workspace_id}", paperidH.Handle)

	// Invoice cron endpoints.
	api.Handle(http.MethodGet, "/cron/invoices/overdue", oidcAuth(invoiceCronH.UpdateOverdue))
	api.Handle(http.MethodGet, "/cron/invoices/escalate", oidcAuth(invoiceCronH.EscalateStages))
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

	masterData := api.Group("/master-data")
	masterData.Handle(http.MethodGet, "/clients", wsRequired(jwtAuth(masterDataH.List)))
	masterData.Handle(http.MethodGet, "/clients/export", wsRequired(jwtAuth(masterDataH.Export)))
	masterData.Handle(http.MethodGet, "/clients/template", wsRequired(jwtAuth(masterDataH.Template)))
	masterData.Handle(http.MethodPost, "/clients/import", wsRequired(jwtAuth(masterDataH.Import)))
	masterData.Handle(http.MethodPost, "/clients/import/preview", wsRequired(jwtAuth(masterDataH.ImportPreview)))
	masterData.Handle(http.MethodPost, "/clients", wsRequired(jwtAuth(masterDataH.Create)))
	masterData.Handle(http.MethodGet, "/clients/{id}", wsRequired(jwtAuth(masterDataH.Get)))
	masterData.Handle(http.MethodPut, "/clients/{id}", wsRequired(jwtAuth(masterDataH.Patch)))
	masterData.Handle(http.MethodDelete, "/clients/{id}", wsRequired(jwtAuth(masterDataH.Delete)))
	masterData.Handle(http.MethodPost, "/clients/{id}/transition", wsRequired(jwtAuth(masterDataH.Transition)))
	masterData.Handle(http.MethodPost, "/query", wsRequired(jwtAuth(masterDataH.Query)))
	masterData.Handle(http.MethodGet, "/stats", wsRequired(jwtAuth(masterDataH.Stats)))
	masterData.Handle(http.MethodGet, "/attention", wsRequired(jwtAuth(masterDataH.Attention)))
	masterData.Handle(http.MethodGet, "/mutations", wsRequired(jwtAuth(masterDataH.Mutations)))
	masterData.Handle(http.MethodGet, "/field-definitions", wsRequired(jwtAuth(customFieldH.List)))
	masterData.Handle(http.MethodPost, "/field-definitions", wsRequired(jwtAuth(customFieldH.Create)))
	masterData.Handle(http.MethodPut, "/field-definitions/reorder", wsRequired(jwtAuth(customFieldH.Reorder)))
	masterData.Handle(http.MethodPut, "/field-definitions/{id}", wsRequired(jwtAuth(customFieldH.Update)))
	masterData.Handle(http.MethodDelete, "/field-definitions/{id}", wsRequired(jwtAuth(customFieldH.Delete)))

	// Team / RBAC
	requireTeam := func(action string) func(middleware.ErrorHandler) middleware.ErrorHandler {
		return middleware.RequirePermission(entity.ModuleTeam, action, deps.TeamUC)
	}
	team := api.Group("/team")
	team.Handle(http.MethodGet, "/members", wsRequired(jwtAuth(requireTeam(entity.ActionViewList)(teamH.ListMembers))))
	team.Handle(http.MethodPost, "/members/invite", wsRequired(jwtAuth(requireTeam(entity.ActionCreate)(teamH.InviteMember))))
	team.Handle(http.MethodGet, "/members/{id}", wsRequired(jwtAuth(requireTeam(entity.ActionViewDetail)(teamH.GetMember))))
	team.Handle(http.MethodPut, "/members/{id}", wsRequired(jwtAuth(requireTeam(entity.ActionEdit)(teamH.UpdateMember))))
	team.Handle(http.MethodPut, "/members/{id}/role", wsRequired(jwtAuth(requireTeam(entity.ActionEdit)(teamH.ChangeRole))))
	team.Handle(http.MethodPut, "/members/{id}/status", wsRequired(jwtAuth(requireTeam(entity.ActionEdit)(teamH.ChangeStatus))))
	team.Handle(http.MethodPut, "/members/{id}/workspaces", wsRequired(jwtAuth(requireTeam(entity.ActionEdit)(teamH.UpdateMemberWorkspaces))))
	team.Handle(http.MethodDelete, "/members/{id}", wsRequired(jwtAuth(requireTeam(entity.ActionDelete)(teamH.RemoveMember))))
	team.Handle(http.MethodPost, "/invitations/{token}/accept", jwtAuth(teamH.AcceptInvitation))
	team.Handle(http.MethodGet, "/roles", wsRequired(jwtAuth(requireTeam(entity.ActionViewList)(teamH.ListRoles))))
	team.Handle(http.MethodPost, "/roles", wsRequired(jwtAuth(requireTeam(entity.ActionCreate)(teamH.CreateRole))))
	team.Handle(http.MethodGet, "/roles/{id}", wsRequired(jwtAuth(requireTeam(entity.ActionViewDetail)(teamH.GetRole))))
	team.Handle(http.MethodPut, "/roles/{id}", wsRequired(jwtAuth(requireTeam(entity.ActionEdit)(teamH.UpdateRole))))
	team.Handle(http.MethodPut, "/roles/{id}/permissions", wsRequired(jwtAuth(requireTeam(entity.ActionEdit)(teamH.UpdateRolePermissions))))
	team.Handle(http.MethodDelete, "/roles/{id}", wsRequired(jwtAuth(requireTeam(entity.ActionDelete)(teamH.DeleteRole))))
	team.Handle(http.MethodGet, "/permissions/me", wsRequired(jwtAuth(teamH.GetMyPermissions)))

	// Messaging templates (feat/05-messaging) — workspace-scoped.
	tpl := api.Group("/templates")
	tpl.Handle(http.MethodGet, "/messages", wsRequired(jwtAuth(messagingH.ListMessages)))
	tpl.Handle(http.MethodPost, "/messages", wsRequired(jwtAuth(messagingH.CreateMessage)))
	tpl.Handle(http.MethodGet, "/messages/{id}", wsRequired(jwtAuth(messagingH.GetMessage)))
	tpl.Handle(http.MethodPut, "/messages/{id}", wsRequired(jwtAuth(messagingH.UpdateMessage)))
	tpl.Handle(http.MethodDelete, "/messages/{id}", wsRequired(jwtAuth(messagingH.DeleteMessage)))
	tpl.Handle(http.MethodGet, "/emails", wsRequired(jwtAuth(messagingH.ListEmails)))
	tpl.Handle(http.MethodPost, "/emails", wsRequired(jwtAuth(messagingH.CreateEmail)))
	tpl.Handle(http.MethodGet, "/emails/{id}", wsRequired(jwtAuth(messagingH.GetEmail)))
	tpl.Handle(http.MethodPut, "/emails/{id}", wsRequired(jwtAuth(messagingH.UpdateEmail)))
	tpl.Handle(http.MethodDelete, "/emails/{id}", wsRequired(jwtAuth(messagingH.DeleteEmail)))
	tpl.Handle(http.MethodPost, "/preview", wsRequired(jwtAuth(messagingH.Preview)))
	tpl.Handle(http.MethodGet, "/edit-logs", wsRequired(jwtAuth(messagingH.ListEditLogs)))
	tpl.Handle(http.MethodGet, "/edit-logs/{template_id}", wsRequired(jwtAuth(messagingH.GetEditLogsForTemplate)))
	tpl.Handle(http.MethodGet, "/variables", wsRequired(jwtAuth(messagingH.ListVariables)))

	// Workflow engine (feat/06) — workspace-scoped.
	wf := api.Group("/workflows")
	wf.Handle(http.MethodGet, "", wsRequired(jwtAuth(workflowH.List)))
	wf.Handle(http.MethodPost, "", wsRequired(jwtAuth(workflowH.Create)))
	wf.Handle(http.MethodGet, "/by-slug/{slug}", wsRequired(jwtAuth(workflowH.GetBySlug)))
	wf.Handle(http.MethodGet, "/{id}", wsRequired(jwtAuth(workflowH.Get)))
	wf.Handle(http.MethodPut, "/{id}", wsRequired(jwtAuth(workflowH.Update)))
	wf.Handle(http.MethodDelete, "/{id}", wsRequired(jwtAuth(workflowH.Delete)))
	wf.Handle(http.MethodPut, "/{id}/canvas", wsRequired(jwtAuth(workflowH.SaveCanvas)))
	wf.Handle(http.MethodGet, "/{id}/steps", wsRequired(jwtAuth(workflowH.GetSteps)))
	wf.Handle(http.MethodPut, "/{id}/steps", wsRequired(jwtAuth(workflowH.SaveSteps)))
	wf.Handle(http.MethodGet, "/{id}/steps/{stepKey}", wsRequired(jwtAuth(workflowH.GetStep)))
	wf.Handle(http.MethodPut, "/{id}/steps/{stepKey}", wsRequired(jwtAuth(workflowH.UpdateStep)))
	wf.Handle(http.MethodGet, "/{id}/config", wsRequired(jwtAuth(workflowH.GetConfig)))
	wf.Handle(http.MethodPut, "/{id}/tabs", wsRequired(jwtAuth(workflowH.SaveTabs)))
	wf.Handle(http.MethodPut, "/{id}/stats", wsRequired(jwtAuth(workflowH.SaveStats)))
	wf.Handle(http.MethodPut, "/{id}/columns", wsRequired(jwtAuth(workflowH.SaveColumns)))
	wf.Handle(http.MethodGet, "/{id}/data", wsRequired(jwtAuth(pipelineViewH.GetData)))

	// Automation rules (feat/06) — workspace-scoped.
	ar := api.Group("/automation-rules")
	ar.Handle(http.MethodGet, "", wsRequired(jwtAuth(automationRuleH.List)))
	ar.Handle(http.MethodPost, "", wsRequired(jwtAuth(automationRuleH.Create)))
	ar.Handle(http.MethodGet, "/change-logs", wsRequired(jwtAuth(automationRuleH.ListChangeLogs)))
	ar.Handle(http.MethodGet, "/{id}", wsRequired(jwtAuth(automationRuleH.Get)))
	ar.Handle(http.MethodPut, "/{id}", wsRequired(jwtAuth(automationRuleH.Update)))
	ar.Handle(http.MethodDelete, "/{id}", wsRequired(jwtAuth(automationRuleH.Delete)))

	// Dashboard overview stats (feat/09)
	api.Handle(http.MethodGet, "/dashboard/stats", wsRequired(jwtAuth(analyticsH.Stats)))

	// Analytics (feat/09)
	an := api.Group("/analytics")
	an.Handle(http.MethodGet, "/kpi", wsRequired(jwtAuth(analyticsH.KPI)))
	an.Handle(http.MethodGet, "/kpi/bundle", wsRequired(jwtAuth(analyticsH.Bundle)))
	an.Handle(http.MethodGet, "/distributions", wsRequired(jwtAuth(analyticsH.Distributions)))
	an.Handle(http.MethodGet, "/engagement", wsRequired(jwtAuth(analyticsH.Engagement)))
	an.Handle(http.MethodGet, "/revenue-trend", wsRequired(jwtAuth(analyticsH.RevenueTrend)))
	an.Handle(http.MethodGet, "/forecast-accuracy", wsRequired(jwtAuth(analyticsH.ForecastAccuracy)))

	// Reports (feat/09)
	rep := api.Group("/reports")
	rep.Handle(http.MethodGet, "/executive-summary", wsRequired(jwtAuth(reportsH.ExecutiveSummary)))
	rep.Handle(http.MethodGet, "/revenue-contracts", wsRequired(jwtAuth(reportsH.RevenueContracts)))
	rep.Handle(http.MethodGet, "/client-health", wsRequired(jwtAuth(reportsH.ClientHealth)))
	rep.Handle(http.MethodGet, "/engagement-retention", wsRequired(jwtAuth(reportsH.EngagementRetention)))
	rep.Handle(http.MethodGet, "/workspace-comparison", wsRequired(jwtAuth(reportsH.WorkspaceComparison)))
	rep.Handle(http.MethodPost, "/export", wsRequired(jwtAuth(reportsH.Export)))

	// Revenue targets (feat/09)
	api.Handle(http.MethodGet, "/revenue-targets", wsRequired(jwtAuth(revenueTargetH.List)))
	api.Handle(http.MethodPut, "/revenue-targets", wsRequired(jwtAuth(revenueTargetH.Upsert)))

	// Cron — snapshot rebuild (feat/09)
	api.Handle(http.MethodGet, "/cron/analytics/rebuild-snapshots", oidcAuth(snapshotCronH.Rebuild))

	// Collections (feat/10) — user-defined tables
	col := api.Group("/collections")
	col.Handle(http.MethodGet, "", wsRequired(jwtAuth(collectionH.List)))
	col.Handle(http.MethodPost, "", wsRequired(jwtAuth(collectionH.Create)))
	col.Handle(http.MethodGet, "/{id}", wsRequired(jwtAuth(collectionH.Get)))
	col.Handle(http.MethodPatch, "/{id}", wsRequired(jwtAuth(collectionH.Update)))
	col.Handle(http.MethodDelete, "/{id}", wsRequired(jwtAuth(collectionH.Delete)))
	col.Handle(http.MethodPost, "/{id}/fields", wsRequired(jwtAuth(collectionH.AddField)))
	col.Handle(http.MethodPatch, "/{id}/fields/{field_id}", wsRequired(jwtAuth(collectionH.UpdateField)))
	col.Handle(http.MethodDelete, "/{id}/fields/{field_id}", wsRequired(jwtAuth(collectionH.DeleteField)))
	col.Handle(http.MethodPost, "/approvals/{approval_id}/approve", wsRequired(jwtAuth(collectionH.ApplyApproval)))
	col.Handle(http.MethodGet, "/{id}/records", wsRequired(jwtAuth(collectionRecordH.List)))
	col.Handle(http.MethodGet, "/{id}/records/distinct", wsRequired(jwtAuth(collectionRecordH.Distinct)))
	col.Handle(http.MethodPost, "/{id}/records", wsRequired(jwtAuth(collectionRecordH.Create)))
	col.Handle(http.MethodPatch, "/{id}/records/{record_id}", wsRequired(jwtAuth(collectionRecordH.Update)))
	col.Handle(http.MethodDelete, "/{id}/records/{record_id}", wsRequired(jwtAuth(collectionRecordH.Delete)))
	col.Handle(http.MethodPost, "/{id}/records/bulk", wsRequired(jwtAuth(collectionRecordH.Bulk)))

	// Swagger
	r.HandlePrefix(http.MethodGet, "/swagger", httpSwagger.WrapHandler)

	// Webhook
	webhook := api.Group("/webhook")
	webhook.Handle(http.MethodPost, "/wa", haloaiSig(waH.Handle))
	webhook.Handle(http.MethodPost, "/checkin-form", checkinH.Handle)

	return r
}
