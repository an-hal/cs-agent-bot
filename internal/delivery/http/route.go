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
	api.Handle(http.MethodGet, "/activity-logs", wsRequired(jwtAuth(activityH.List)))
	api.Handle(http.MethodPost, "/activity-logs", wsRequired(jwtAuth(activityH.Record)))
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
	dataMaster.Handle(http.MethodGet, "/invoices", wsRequired(jwtAuth(invoiceH.List)))
	dataMaster.Handle(http.MethodGet, "/invoices/{invoice_id}", wsRequired(jwtAuth(invoiceH.Get)))
	dataMaster.Handle(http.MethodPut, "/invoices/{invoice_id}", wsRequired(jwtAuth(invoiceH.Update)))
	dataMaster.Handle(http.MethodGet, "/escalations", wsRequired(jwtAuth(escalationH.List)))

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

	// Swagger
	r.HandlePrefix(http.MethodGet, "/swagger", httpSwagger.WrapHandler)

	// Webhook
	webhook := api.Group("/webhook")
	webhook.Handle(http.MethodPost, "/wa", haloaiSig(waH.Handle))
	webhook.Handle(http.MethodPost, "/checkin-form", checkinH.Handle)

	return r
}
