package http

import (
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/middleware"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	teamuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/team"
)

// jws composes JWT + workspace-required middleware for the common
// "scoped dashboard endpoint" pattern. Saves repeating wsRequired(jwt(...))
// inline at every route.
func (mw authBundle) jws(h middleware.ErrorHandler) middleware.ErrorHandler {
	return mw.ws(mw.jwt(h))
}

func registerAuthRoutes(api *router.Router, h *handlers, mw authBundle) {
	api.Handle(http.MethodPost, "/auth/login", h.auth.Login)
	api.Handle(http.MethodPost, "/auth/google", h.auth.LoginGoogle)
	api.Handle(http.MethodPost, "/auth/logout", h.auth.Logout)
	api.Handle(http.MethodGet, "/whitelist/check", h.auth.CheckWhitelist)
	api.Handle(http.MethodGet, "/whitelist", mw.jwt(h.auth.ListWhitelist))
	api.Handle(http.MethodPost, "/whitelist", mw.jwt(h.auth.AddWhitelist))
	api.Handle(http.MethodDelete, "/whitelist/{id}", mw.jwt(h.auth.DeleteWhitelist))
	// Session revocation (feat/01 §2c)
	api.Handle(http.MethodPost, "/sessions/revoke", mw.jws(h.session.Revoke))
	api.Handle(http.MethodGet, "/sessions/revoked", mw.jws(h.session.List))
}

func registerCronAndWebhookRoutes(api *router.Router, h *handlers, mw authBundle) {
	// Webhook + handoff + payment (HMAC + signature based)
	webhook := api.Group("/webhook")
	webhook.Handle(http.MethodPost, "/wa", mw.halo(h.wa.Handle))
	webhook.Handle(http.MethodPost, "/checkin-form", h.checkin.Handle)
	api.Handle(http.MethodPost, "/handoff/new-client", mw.hmac(h.handoff.Handle))
	api.Handle(http.MethodPost, "/payment/verify", mw.hmac(h.payment.Handle))
	api.Handle(http.MethodPost, "/webhook/paperid/{workspace_id}", h.paperid.Handle)
	api.Handle(http.MethodPost, "/webhook/fireflies/{workspace_id}", mw.hmac(h.firefliesWebhook.Handle))
	// Cron — OIDC for scheduled tasks, JWT for the on-demand admin retention.
	api.Handle(http.MethodGet, "/cron/run", mw.oidc(h.cron.Run))
	api.Handle(http.MethodGet, "/cron/invoices/overdue", mw.oidc(h.invoiceCron.UpdateOverdue))
	api.Handle(http.MethodGet, "/cron/invoices/escalate", mw.oidc(h.invoiceCron.EscalateStages))
	api.Handle(http.MethodGet, "/cron/analytics/rebuild-snapshots", mw.oidc(h.snapshotCron.Rebuild))
	api.Handle(http.MethodGet, "/cron/sessions/cleanup", mw.oidc(h.session.Cleanup))
	api.Handle(http.MethodGet, "/cron/pdp/retention", mw.jws(h.pdp.RunRetention))
}

func registerWorkspaceRoutes(api *router.Router, h *handlers, mw authBundle) {
	api.Handle(http.MethodGet, "/workspaces", mw.jwt(h.workspace.List))
	// /workspaces/mine MUST be registered before /workspaces/{id} — router
	// matches in registration order and {id} would otherwise capture "mine".
	api.Handle(http.MethodGet, "/workspaces/mine", mw.jwt(h.workspace.Mine))
	api.Handle(http.MethodPost, "/workspaces", mw.jwt(h.workspace.Create))
	api.Handle(http.MethodGet, "/workspaces/{id}", mw.jwt(h.workspace.Get))
	api.Handle(http.MethodPut, "/workspaces/{id}", mw.jwt(h.workspace.Update))
	api.Handle(http.MethodDelete, "/workspaces/{id}", mw.jwt(h.workspace.SoftDelete))
	api.Handle(http.MethodPost, "/workspaces/{id}/switch", mw.jwt(h.workspace.Switch))
	api.Handle(http.MethodGet, "/workspaces/{id}/members", mw.jwt(h.workspace.ListMembers))
	api.Handle(http.MethodPost, "/workspaces/{id}/members/invite", mw.jwt(h.workspace.Invite))
	api.Handle(http.MethodPut, "/workspaces/{id}/members/{member_id}", mw.jwt(h.workspace.UpdateMemberRole))
	api.Handle(http.MethodDelete, "/workspaces/{id}/members/{member_id}", mw.jwt(h.workspace.RemoveMember))
	api.Handle(http.MethodPost, "/workspaces/invitations/{token}/accept", mw.jwt(h.workspace.AcceptInvitation))
	// Workspace theme + holding expansion (feat/02)
	api.Handle(http.MethodGet, "/workspace/theme", mw.jws(h.theme.Get))
	api.Handle(http.MethodPut, "/workspace/theme", mw.jws(h.theme.Upsert))
	api.Handle(http.MethodGet, "/workspace/holding/expand", mw.jws(h.theme.ExpandHolding))
}

func registerNotificationsRoutes(api *router.Router, h *handlers, mw authBundle) {
	api.Handle(http.MethodGet, "/notifications", mw.jws(h.notification.List))
	api.Handle(http.MethodPost, "/notifications", mw.jws(h.notification.Create))
	api.Handle(http.MethodGet, "/notifications/count", mw.jws(h.notification.Count))
	api.Handle(http.MethodPut, "/notifications/{id}/read", mw.jws(h.notification.MarkRead))
	api.Handle(http.MethodPut, "/notifications/read-all", mw.jws(h.notification.MarkAllRead))
}

// registerSharedRoutes is the umbrella for cross-cutting workspace-scoped
// surfaces. Each domain lives in its own helper below.
func registerSharedRoutes(api *router.Router, h *handlers, mw authBundle) {
	registerUserPreferencesRoutes(api, h, mw)
	registerIntegrationRoutes(api, h, mw)
	registerApprovalRoutes(api, h, mw)
	registerManualActionRoutes(api, h, mw)
	registerAuditLogRoutes(api, h, mw)
	registerFirefliesRoutes(api, h, mw)
	registerReactivationRoutes(api, h, mw)
	registerCoachingRoutes(api, h, mw)
	registerRejectionAnalysisRoutes(api, h, mw)
	registerMockRoutes(api, h, mw)
	registerPDPRoutes(api, h, mw)
}

func registerUserPreferencesRoutes(api *router.Router, h *handlers, mw authBundle) {
	api.Handle(http.MethodGet, "/preferences", mw.jws(h.userPreferences.List))
	api.Handle(http.MethodGet, "/preferences/{namespace}", mw.jws(h.userPreferences.Get))
	api.Handle(http.MethodPut, "/preferences/{namespace}", mw.jws(h.userPreferences.Upsert))
	api.Handle(http.MethodDelete, "/preferences/{namespace}", mw.jws(h.userPreferences.Delete))
}

func registerIntegrationRoutes(api *router.Router, h *handlers, mw authBundle) {
	api.Handle(http.MethodGet, "/integrations", mw.jws(h.integrations.List))
	api.Handle(http.MethodGet, "/integrations/{provider}", mw.jws(h.integrations.Get))
	api.Handle(http.MethodPut, "/integrations/{provider}", mw.jws(h.integrations.Upsert))
	api.Handle(http.MethodDelete, "/integrations/{provider}", mw.jws(h.integrations.Delete))
}

func registerApprovalRoutes(api *router.Router, h *handlers, mw authBundle) {
	api.Handle(http.MethodGet, "/approvals", mw.jws(h.approval.List))
	api.Handle(http.MethodGet, "/approvals/{id}", mw.jws(h.approval.Get))
	api.Handle(http.MethodPost, "/approvals/{id}/apply", mw.jws(h.approval.Apply))
	api.Handle(http.MethodPost, "/approvals/{id}/reject", mw.jws(h.approval.Reject))
}

// Manual action queue (feat/06 GUARD).
func registerManualActionRoutes(api *router.Router, h *handlers, mw authBundle) {
	api.Handle(http.MethodGet, "/manual-actions", mw.jws(h.manualAction.List))
	api.Handle(http.MethodGet, "/manual-actions/{id}", mw.jws(h.manualAction.Get))
	api.Handle(http.MethodPatch, "/manual-actions/{id}/mark-sent", mw.jws(h.manualAction.MarkSent))
	api.Handle(http.MethodPatch, "/manual-actions/{id}/skip", mw.jws(h.manualAction.Skip))
}

func registerAuditLogRoutes(api *router.Router, h *handlers, mw authBundle) {
	api.Handle(http.MethodGet, "/audit-logs/workspace-access", mw.jws(h.auditWs.List))
	api.Handle(http.MethodPost, "/audit-logs/workspace-access", mw.jws(h.auditWs.Record))
}

// Dashboard views only — webhook ingestion lives in registerCronAndWebhookRoutes.
func registerFirefliesRoutes(api *router.Router, h *handlers, mw authBundle) {
	api.Handle(http.MethodGet, "/fireflies/transcripts", mw.jws(h.firefliesDash.List))
	api.Handle(http.MethodGet, "/fireflies/transcripts/{id}", mw.jws(h.firefliesDash.Get))
}

func registerReactivationRoutes(api *router.Router, h *handlers, mw authBundle) {
	api.Handle(http.MethodGet, "/reactivation/triggers", mw.jws(h.reactivation.ListTriggers))
	api.Handle(http.MethodPost, "/reactivation/triggers", mw.jws(h.reactivation.UpsertTrigger))
	api.Handle(http.MethodGet, "/reactivation/triggers/{id}", mw.jws(h.reactivation.GetTrigger))
	api.Handle(http.MethodDelete, "/reactivation/triggers/{id}", mw.jws(h.reactivation.DeleteTrigger))
	api.Handle(http.MethodPost, "/master-data/clients/{id}/reactivate", mw.jws(h.reactivation.Reactivate))
	api.Handle(http.MethodGet, "/master-data/clients/{id}/reactivation-history", mw.jws(h.reactivation.History))
}

func registerCoachingRoutes(api *router.Router, h *handlers, mw authBundle) {
	api.Handle(http.MethodGet, "/coaching/sessions", mw.jws(h.coaching.List))
	api.Handle(http.MethodPost, "/coaching/sessions", mw.jws(h.coaching.Create))
	api.Handle(http.MethodGet, "/coaching/sessions/{id}", mw.jws(h.coaching.Get))
	api.Handle(http.MethodPatch, "/coaching/sessions/{id}", mw.jws(h.coaching.Update))
	api.Handle(http.MethodPost, "/coaching/sessions/{id}/submit", mw.jws(h.coaching.Submit))
	api.Handle(http.MethodDelete, "/coaching/sessions/{id}", mw.jws(h.coaching.Delete))
}

func registerRejectionAnalysisRoutes(api *router.Router, h *handlers, mw authBundle) {
	api.Handle(http.MethodGet, "/rejection-analysis", mw.jws(h.rejectionAnal.List))
	api.Handle(http.MethodPost, "/rejection-analysis", mw.jws(h.rejectionAnal.Record))
	api.Handle(http.MethodPost, "/rejection-analysis/analyze", mw.jws(h.rejectionAnal.Analyze))
	api.Handle(http.MethodGet, "/rejection-analysis/stats", mw.jws(h.rejectionAnal.Stats))
}

// Mock outbox + per-provider triggers (FE QA).
func registerMockRoutes(api *router.Router, h *handlers, mw authBundle) {
	api.Handle(http.MethodGet, "/mock/outbox", mw.jwt(h.mock.ListOutbox))
	api.Handle(http.MethodGet, "/mock/outbox/{id}", mw.jwt(h.mock.GetOutboxRecord))
	api.Handle(http.MethodDelete, "/mock/outbox", mw.jwt(h.mock.ClearOutbox))
	api.Handle(http.MethodPost, "/mock/claude/extract", mw.jwt(h.mock.TriggerClaude))
	api.Handle(http.MethodPost, "/mock/fireflies/fetch", mw.jwt(h.mock.TriggerFireflies))
	api.Handle(http.MethodPost, "/mock/haloai/send", mw.jwt(h.mock.TriggerHaloAI))
	api.Handle(http.MethodPost, "/mock/smtp/send", mw.jwt(h.mock.TriggerSMTP))
}

func registerPDPRoutes(api *router.Router, h *handlers, mw authBundle) {
	api.Handle(http.MethodGet, "/pdp/erasure-requests", mw.jws(h.pdp.ListErasure))
	api.Handle(http.MethodPost, "/pdp/erasure-requests", mw.jws(h.pdp.CreateErasure))
	api.Handle(http.MethodGet, "/pdp/erasure-requests/{id}", mw.jws(h.pdp.GetErasure))
	api.Handle(http.MethodPost, "/pdp/erasure-requests/{id}/approve", mw.jws(h.pdp.ApproveErasure))
	api.Handle(http.MethodPost, "/pdp/erasure-requests/{id}/reject", mw.jws(h.pdp.RejectErasure))
	api.Handle(http.MethodPost, "/pdp/erasure-requests/{id}/execute", mw.jws(h.pdp.ExecuteErasure))
	api.Handle(http.MethodGet, "/pdp/retention-policies", mw.jws(h.pdp.ListPolicies))
	api.Handle(http.MethodPost, "/pdp/retention-policies", mw.jws(h.pdp.UpsertPolicy))
	api.Handle(http.MethodDelete, "/pdp/retention-policies/{id}", mw.jws(h.pdp.DeletePolicy))
}

func registerActivityAndJobRoutes(api *router.Router, h *handlers, mw authBundle) {
	// Unified activity feed + bot action aggregations (feat/08)
	api.Handle(http.MethodGet, "/activity-log/feed", mw.jws(h.feed.Feed))
	api.Handle(http.MethodGet, "/activity-log/today", mw.jws(h.actionLog.TodayActivity))
	api.Handle(http.MethodGet, "/action-log/recent", mw.jws(h.actionLog.Recent))
	api.Handle(http.MethodGet, "/action-log/today", mw.jws(h.actionLog.Today))
	api.Handle(http.MethodGet, "/action-log/summary", mw.jws(h.actionLog.Summary))
	// Team activity (feat/04)
	api.Handle(http.MethodGet, "/team/activity", mw.jws(h.teamAct.List))
	api.Handle(http.MethodPost, "/team/activity", mw.jws(h.teamAct.Record))
	// Activity-logs (legacy unified endpoint)
	api.Handle(http.MethodGet, "/activity-logs", mw.jws(h.activity.List))
	api.Handle(http.MethodPost, "/activity-logs", mw.jws(h.activity.Record))
	api.Handle(http.MethodGet, "/activity-logs/recent", mw.jws(h.activity.Recent))
	api.Handle(http.MethodGet, "/activity-logs/stats", mw.jws(h.activity.Stats))
	api.Handle(http.MethodGet, "/activity-logs/companies/{company_id}/summary", mw.jws(h.activity.CompanySummary))
	// Background jobs
	api.Handle(http.MethodGet, "/jobs", mw.jws(h.bgJob.ListJobs))
	api.Handle(http.MethodGet, "/jobs/{job_id}", mw.jws(h.bgJob.GetJob))
	api.Handle(http.MethodGet, "/jobs/{job_id}/download", mw.jws(h.bgJob.DownloadJobFile))
}

// registerDataMasterRoutes registers the legacy /data-master prefix used by
// the original dashboard plus system-config + trigger-rule admin endpoints.
func registerDataMasterRoutes(api *router.Router, h *handlers, mw authBundle) {
	dm := api.Group("/data-master")
	dm.Handle(http.MethodGet, "/clients", mw.jws(h.dashboardClient.List))
	dm.Handle(http.MethodPost, "/clients/import", mw.jws(h.bgJob.ImportClients))
	dm.Handle(http.MethodPost, "/clients/export", mw.jws(h.bgJob.ExportClients))
	dm.Handle(http.MethodGet, "/clients/{company_id}", mw.jws(h.dashboardClient.Get))
	dm.Handle(http.MethodPost, "/clients", mw.jws(h.dashboardClient.Create))
	dm.Handle(http.MethodPut, "/clients/{company_id}", mw.jws(h.dashboardClient.Update))
	dm.Handle(http.MethodDelete, "/clients/{company_id}", mw.jws(h.dashboardClient.Delete))
	dm.Handle(http.MethodGet, "/clients/{company_id}/escalations", mw.jws(h.escalation.ListByCompany))
	dm.Handle(http.MethodGet, "/invoices", mw.jws(h.invoice.List))
	dm.Handle(http.MethodGet, "/invoices/{invoice_id}", mw.jws(h.invoice.Get))
	dm.Handle(http.MethodPut, "/invoices/{invoice_id}", mw.jws(h.invoice.Update))
	dm.Handle(http.MethodGet, "/escalations", mw.jws(h.escalation.List))
	dm.Handle(http.MethodGet, "/escalations/{id}", mw.jws(h.escalation.Get))
	dm.Handle(http.MethodPut, "/escalations/{id}/resolve", mw.jws(h.escalation.Resolve))
	dm.Handle(http.MethodGet, "/message-templates", mw.jws(h.template.List))
	dm.Handle(http.MethodGet, "/message-templates/{template_id}", mw.jws(h.template.Get))
	dm.Handle(http.MethodPut, "/message-templates/{template_id}", mw.jws(h.template.Update))
	// Trigger rules + system config admin (JWT only — no workspace scope)
	dm.Handle(http.MethodGet, "/trigger-rules", mw.jwt(h.triggerRule.List))
	dm.Handle(http.MethodGet, "/trigger-rules/{rule_id}", mw.jwt(h.triggerRule.Get))
	dm.Handle(http.MethodPost, "/trigger-rules", mw.jwt(h.triggerRule.Create))
	dm.Handle(http.MethodPut, "/trigger-rules/{rule_id}", mw.jwt(h.triggerRule.Update))
	dm.Handle(http.MethodDelete, "/trigger-rules/{rule_id}", mw.jwt(h.triggerRule.Delete))
	dm.Handle(http.MethodPost, "/trigger-rules/cache/invalidate", mw.jwt(h.triggerRule.InvalidateCache))
	dm.Handle(http.MethodGet, "/template-variables", mw.jwt(h.triggerRule.ListVariables))
	dm.Handle(http.MethodGet, "/system-config", mw.jwt(h.systemConfig.List))
	dm.Handle(http.MethodPut, "/system-config/{key}", mw.jwt(h.systemConfig.Update))
}

func registerInvoiceRoutes(api *router.Router, h *handlers, mw authBundle) {
	g := api.Group("/invoices")
	g.Handle(http.MethodGet, "", mw.jws(h.invoice.List))
	g.Handle(http.MethodPost, "", mw.jws(h.invoice.Create))
	g.Handle(http.MethodGet, "/stats", mw.jws(h.invoice.Stats))
	g.Handle(http.MethodGet, "/by-stage", mw.jws(h.invoice.ByStage))
	g.Handle(http.MethodGet, "/{invoice_id}", mw.jws(h.invoice.Get))
	g.Handle(http.MethodPut, "/{invoice_id}", mw.jws(h.invoice.Update))
	g.Handle(http.MethodDelete, "/{invoice_id}", mw.jws(h.invoice.Delete))
	g.Handle(http.MethodPost, "/{invoice_id}/mark-paid", mw.jws(h.invoice.MarkPaid))
	g.Handle(http.MethodPost, "/{invoice_id}/send-reminder", mw.jws(h.invoice.SendReminder))
	g.Handle(http.MethodGet, "/{invoice_id}/payment-logs", mw.jws(h.invoice.PaymentLogs))
	g.Handle(http.MethodGet, "/{invoice_id}/pdf", mw.jws(h.invoice.PDF))
	g.Handle(http.MethodGet, "/{invoice_id}/activity", mw.jws(h.invoice.Activity))
	g.Handle(http.MethodPost, "/{invoice_id}/update-stage", mw.jws(h.invoice.UpdateStage))
	g.Handle(http.MethodPut, "/{invoice_id}/confirm-partial", mw.jws(h.invoice.ConfirmPartial))
}

func registerMasterDataRoutes(api *router.Router, h *handlers, mw authBundle) {
	g := api.Group("/master-data")
	g.Handle(http.MethodGet, "/clients", mw.jws(h.masterData.List))
	g.Handle(http.MethodGet, "/clients/export", mw.jws(h.masterData.Export))
	g.Handle(http.MethodGet, "/clients/template", mw.jws(h.masterData.Template))
	g.Handle(http.MethodPost, "/clients/import", mw.jws(h.masterData.Import))
	g.Handle(http.MethodPost, "/clients/import/preview", mw.jws(h.masterData.ImportPreview))
	// OneSchema-style import wizard endpoints (Phase A + C).
	g.Handle(http.MethodGet, "/import/schema", mw.jws(h.masterData.ImportSchema))
	g.Handle(http.MethodPost, "/import/detect", mw.jws(h.masterData.ImportDetect))
	g.Handle(http.MethodPost, "/import/sessions", mw.jws(h.masterData.CreateImportSession))
	g.Handle(http.MethodGet, "/import/sessions/{id}", mw.jws(h.masterData.GetImportSession))
	g.Handle(http.MethodPatch, "/import/sessions/{id}/cell", mw.jws(h.masterData.PatchImportSessionCell))
	g.Handle(http.MethodPost, "/import/sessions/{id}/submit", mw.jws(h.masterData.SubmitImportSession))
	g.Handle(http.MethodPost, "/clients", mw.jws(h.masterData.Create))
	g.Handle(http.MethodGet, "/clients/{id}", mw.jws(h.masterData.Get))
	g.Handle(http.MethodPut, "/clients/{id}", mw.jws(h.masterData.Patch))
	g.Handle(http.MethodDelete, "/clients/{id}", mw.jws(h.masterData.Delete))
	g.Handle(http.MethodPost, "/clients/{id}/transition", mw.jws(h.masterData.Transition))
	// Multi-stage PIC — see context/new/multi-stage-pic-spec.md.
	g.Handle(http.MethodGet, "/clients/{id}/contacts", mw.jws(h.masterData.ListContacts))
	g.Handle(http.MethodPost, "/clients/{id}/contacts", mw.jws(h.masterData.CreateContact))
	g.Handle(http.MethodPatch, "/clients/{id}/contacts/{contact_id}", mw.jws(h.masterData.PatchContact))
	g.Handle(http.MethodDelete, "/clients/{id}/contacts/{contact_id}", mw.jws(h.masterData.DeleteContact))
	g.Handle(http.MethodPost, "/query", mw.jws(h.masterData.Query))
	g.Handle(http.MethodGet, "/stats", mw.jws(h.masterData.Stats))
	g.Handle(http.MethodGet, "/attention", mw.jws(h.masterData.Attention))
	g.Handle(http.MethodGet, "/mutations", mw.jws(h.masterData.Mutations))
	g.Handle(http.MethodGet, "/field-definitions", mw.jws(h.customField.List))
	g.Handle(http.MethodPost, "/field-definitions", mw.jws(h.customField.Create))
	g.Handle(http.MethodPut, "/field-definitions/reorder", mw.jws(h.customField.Reorder))
	g.Handle(http.MethodPut, "/field-definitions/{id}", mw.jws(h.customField.Update))
	g.Handle(http.MethodDelete, "/field-definitions/{id}", mw.jws(h.customField.Delete))
}

func registerTeamRoutes(api *router.Router, h *handlers, mw authBundle, teamUC teamuc.Usecase) {
	requireTeam := func(action string) func(middleware.ErrorHandler) middleware.ErrorHandler {
		return middleware.RequirePermission(entity.ModuleTeam, action, teamUC)
	}
	guarded := func(action string, hh middleware.ErrorHandler) middleware.ErrorHandler {
		return mw.jws(requireTeam(action)(hh))
	}
	g := api.Group("/team")
	g.Handle(http.MethodGet, "/members", guarded(entity.ActionViewList, h.team.ListMembers))
	g.Handle(http.MethodPost, "/members/invite", guarded(entity.ActionCreate, h.team.InviteMember))
	g.Handle(http.MethodGet, "/members/{id}", guarded(entity.ActionViewDetail, h.team.GetMember))
	g.Handle(http.MethodPut, "/members/{id}", guarded(entity.ActionEdit, h.team.UpdateMember))
	g.Handle(http.MethodPut, "/members/{id}/role", guarded(entity.ActionEdit, h.team.ChangeRole))
	g.Handle(http.MethodPut, "/members/{id}/status", guarded(entity.ActionEdit, h.team.ChangeStatus))
	g.Handle(http.MethodPut, "/members/{id}/workspaces", guarded(entity.ActionEdit, h.team.UpdateMemberWorkspaces))
	g.Handle(http.MethodDelete, "/members/{id}", guarded(entity.ActionDelete, h.team.RemoveMember))
	g.Handle(http.MethodPost, "/invitations/{token}/accept", mw.jwt(h.team.AcceptInvitation))
	g.Handle(http.MethodGet, "/roles", guarded(entity.ActionViewList, h.team.ListRoles))
	g.Handle(http.MethodPost, "/roles", guarded(entity.ActionCreate, h.team.CreateRole))
	g.Handle(http.MethodGet, "/roles/{id}", guarded(entity.ActionViewDetail, h.team.GetRole))
	g.Handle(http.MethodPut, "/roles/{id}", guarded(entity.ActionEdit, h.team.UpdateRole))
	g.Handle(http.MethodPut, "/roles/{id}/permissions", guarded(entity.ActionEdit, h.team.UpdateRolePermissions))
	g.Handle(http.MethodDelete, "/roles/{id}", guarded(entity.ActionDelete, h.team.DeleteRole))
	g.Handle(http.MethodGet, "/permissions/me", mw.jws(h.team.GetMyPermissions))
}

func registerMessagingRoutes(api *router.Router, h *handlers, mw authBundle) {
	g := api.Group("/templates")
	g.Handle(http.MethodGet, "/messages", mw.jws(h.messaging.ListMessages))
	g.Handle(http.MethodPost, "/messages", mw.jws(h.messaging.CreateMessage))
	g.Handle(http.MethodGet, "/messages/{id}", mw.jws(h.messaging.GetMessage))
	g.Handle(http.MethodPut, "/messages/{id}", mw.jws(h.messaging.UpdateMessage))
	g.Handle(http.MethodDelete, "/messages/{id}", mw.jws(h.messaging.DeleteMessage))
	g.Handle(http.MethodGet, "/emails", mw.jws(h.messaging.ListEmails))
	g.Handle(http.MethodPost, "/emails", mw.jws(h.messaging.CreateEmail))
	g.Handle(http.MethodGet, "/emails/{id}", mw.jws(h.messaging.GetEmail))
	g.Handle(http.MethodPut, "/emails/{id}", mw.jws(h.messaging.UpdateEmail))
	g.Handle(http.MethodDelete, "/emails/{id}", mw.jws(h.messaging.DeleteEmail))
	g.Handle(http.MethodPost, "/preview", mw.jws(h.messaging.Preview))
	g.Handle(http.MethodGet, "/edit-logs", mw.jws(h.messaging.ListEditLogs))
	g.Handle(http.MethodGet, "/edit-logs/{template_id}", mw.jws(h.messaging.GetEditLogsForTemplate))
	g.Handle(http.MethodGet, "/variables", mw.jws(h.messaging.ListVariables))
}

func registerWorkflowRoutes(api *router.Router, h *handlers, mw authBundle) {
	g := api.Group("/workflows")
	g.Handle(http.MethodGet, "", mw.jws(h.workflow.List))
	g.Handle(http.MethodPost, "", mw.jws(h.workflow.Create))
	g.Handle(http.MethodGet, "/by-slug/{slug}", mw.jws(h.workflow.GetBySlug))
	g.Handle(http.MethodGet, "/{id}", mw.jws(h.workflow.Get))
	g.Handle(http.MethodPut, "/{id}", mw.jws(h.workflow.Update))
	g.Handle(http.MethodDelete, "/{id}", mw.jws(h.workflow.Delete))
	g.Handle(http.MethodPut, "/{id}/canvas", mw.jws(h.workflow.SaveCanvas))
	g.Handle(http.MethodGet, "/{id}/steps", mw.jws(h.workflow.GetSteps))
	g.Handle(http.MethodPut, "/{id}/steps", mw.jws(h.workflow.SaveSteps))
	g.Handle(http.MethodGet, "/{id}/steps/{stepKey}", mw.jws(h.workflow.GetStep))
	g.Handle(http.MethodPut, "/{id}/steps/{stepKey}", mw.jws(h.workflow.UpdateStep))
	g.Handle(http.MethodGet, "/{id}/config", mw.jws(h.workflow.GetConfig))
	g.Handle(http.MethodPut, "/{id}/tabs", mw.jws(h.workflow.SaveTabs))
	g.Handle(http.MethodPut, "/{id}/stats", mw.jws(h.workflow.SaveStats))
	g.Handle(http.MethodPut, "/{id}/columns", mw.jws(h.workflow.SaveColumns))
	g.Handle(http.MethodGet, "/{id}/data", mw.jws(h.pipelineView.GetData))
}

func registerAutomationRuleRoutes(api *router.Router, h *handlers, mw authBundle) {
	g := api.Group("/automation-rules")
	g.Handle(http.MethodGet, "", mw.jws(h.automationRule.List))
	g.Handle(http.MethodPost, "", mw.jws(h.automationRule.Create))
	g.Handle(http.MethodGet, "/change-logs", mw.jws(h.automationRule.ListChangeLogs))
	g.Handle(http.MethodGet, "/{id}", mw.jws(h.automationRule.Get))
	g.Handle(http.MethodPut, "/{id}", mw.jws(h.automationRule.Update))
	g.Handle(http.MethodDelete, "/{id}", mw.jws(h.automationRule.Delete))
}

func registerAnalyticsRoutes(api *router.Router, h *handlers, mw authBundle) {
	api.Handle(http.MethodGet, "/dashboard/stats", mw.jws(h.analytics.Stats))
	g := api.Group("/analytics")
	g.Handle(http.MethodGet, "/kpi", mw.jws(h.analytics.KPI))
	g.Handle(http.MethodGet, "/kpi/bundle", mw.jws(h.analytics.Bundle))
	g.Handle(http.MethodGet, "/distributions", mw.jws(h.analytics.Distributions))
	g.Handle(http.MethodGet, "/engagement", mw.jws(h.analytics.Engagement))
	g.Handle(http.MethodGet, "/revenue-trend", mw.jws(h.analytics.RevenueTrend))
	g.Handle(http.MethodGet, "/forecast-accuracy", mw.jws(h.analytics.ForecastAccuracy))
	api.Handle(http.MethodGet, "/revenue-targets", mw.jws(h.revenueTarget.List))
	api.Handle(http.MethodPut, "/revenue-targets", mw.jws(h.revenueTarget.Upsert))
}

func registerReportsRoutes(api *router.Router, h *handlers, mw authBundle) {
	g := api.Group("/reports")
	g.Handle(http.MethodGet, "/executive-summary", mw.jws(h.reports.ExecutiveSummary))
	g.Handle(http.MethodGet, "/revenue-contracts", mw.jws(h.reports.RevenueContracts))
	g.Handle(http.MethodGet, "/client-health", mw.jws(h.reports.ClientHealth))
	g.Handle(http.MethodGet, "/engagement-retention", mw.jws(h.reports.EngagementRetention))
	g.Handle(http.MethodGet, "/workspace-comparison", mw.jws(h.reports.WorkspaceComparison))
	g.Handle(http.MethodPost, "/export", mw.jws(h.reports.Export))
}

func registerCollectionRoutes(api *router.Router, h *handlers, mw authBundle) {
	g := api.Group("/collections")
	g.Handle(http.MethodGet, "", mw.jws(h.collection.List))
	g.Handle(http.MethodPost, "", mw.jws(h.collection.Create))
	g.Handle(http.MethodGet, "/{id}", mw.jws(h.collection.Get))
	g.Handle(http.MethodPatch, "/{id}", mw.jws(h.collection.Update))
	g.Handle(http.MethodDelete, "/{id}", mw.jws(h.collection.Delete))
	g.Handle(http.MethodPost, "/{id}/fields", mw.jws(h.collection.AddField))
	g.Handle(http.MethodPatch, "/{id}/fields/{field_id}", mw.jws(h.collection.UpdateField))
	g.Handle(http.MethodDelete, "/{id}/fields/{field_id}", mw.jws(h.collection.DeleteField))
	g.Handle(http.MethodPost, "/approvals/{approval_id}/approve", mw.jws(h.collection.ApplyApproval))
	g.Handle(http.MethodGet, "/{id}/records", mw.jws(h.collectionRec.List))
	g.Handle(http.MethodGet, "/{id}/records/distinct", mw.jws(h.collectionRec.Distinct))
	g.Handle(http.MethodPost, "/{id}/records", mw.jws(h.collectionRec.Create))
	g.Handle(http.MethodPatch, "/{id}/records/{record_id}", mw.jws(h.collectionRec.Update))
	g.Handle(http.MethodDelete, "/{id}/records/{record_id}", mw.jws(h.collectionRec.Delete))
	g.Handle(http.MethodPost, "/{id}/records/bulk", mw.jws(h.collectionRec.Bulk))
}
