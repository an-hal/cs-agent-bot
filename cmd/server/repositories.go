package main

import (
	"database/sql"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/pkg/secretvault"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	appTracer "github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// repositories holds every repo instance built once at server start. Keeping
// them as a single struct keeps main() readable and lets downstream wiring
// pick what it needs instead of juggling 50+ free variables.
type repositories struct {
	systemConfig         repository.SystemConfigRepository
	client               repository.ClientRepository
	invoice              repository.InvoiceRepository
	invoiceLineItem      repository.InvoiceLineItemRepository
	paymentLog           repository.PaymentLogRepository
	invoiceSeq           repository.InvoiceSequenceRepository
	flags                repository.FlagsRepository
	convState            repository.ConversationStateRepository
	log                  repository.LogRepository
	escalation           repository.EscalationRepository
	systemConfigKV       repository.ConfigRepository
	workspace            repository.WorkspaceRepository
	workspaceMember      repository.WorkspaceMemberRepository
	workspaceInvitation  repository.WorkspaceInvitationRepository
	notification         repository.NotificationRepository
	template             repository.TemplateRepository
	bgJob                repository.BackgroundJobRepository
	whitelist            repository.WhitelistRepository
	masterData           repository.MasterDataRepository
	customField          repository.CustomFieldDefinitionRepository
	masterDataMutation   repository.MasterDataMutationRepository
	approvalRequest      repository.ApprovalRequestRepository
	importSession        repository.ImportSessionRepository
	clientContact        repository.ClientContactRepository
	role                 repository.RoleRepository
	rolePermission       repository.RolePermissionRepository
	teamMember           repository.TeamMemberRepository
	memberWsAssignment   repository.MemberWorkspaceAssignmentRepository
	messageTemplate      repository.MessageTemplateRepository
	emailTemplate        repository.EmailTemplateRepository
	templateVariable     repository.TemplateVariableRepository
	templateEditLog      repository.TemplateEditLogRepository
	userPreferences      repository.UserPreferencesRepository
	workspaceIntegration repository.WorkspaceIntegrationRepository
	manualAction         repository.ManualActionRepository
	auditWsAccess        repository.AuditWorkspaceAccessRepository
	fireflies            repository.FirefliesTranscriptRepository
	claudeExtraction     repository.ClaudeExtractionRepository
	reactivation         repository.ReactivationRepository
	coaching             repository.CoachingSessionRepository
	rejectionAnalysis    repository.RejectionAnalysisRepository
	pdp                  repository.PDPRepository
	workspaceTheme       repository.WorkspaceThemeRepository
	activityFeed         repository.ActivityFeedRepository
	teamActivity         repository.TeamActivityLogRepository
	revokedSessions      repository.RevokedSessionRepository
	triggerRule          repository.TriggerRuleRepository
	workflow             repository.WorkflowRepository
	automationRule       repository.AutomationRuleRepository
	pipelineView         repository.PipelineViewRepository
	analytics            repository.AnalyticsRepository
	revenueTarget        repository.RevenueTargetRepository
	revenueSnapshot      repository.RevenueSnapshotRepository
	collection           repository.CollectionRepository
	collectionField      repository.CollectionFieldRepository
	collectionRecord     repository.CollectionRecordRepository
	actionLogWorkflow    repository.ActionLogWorkflowRepository
}

func wireRepositories(db *sql.DB, qt time.Duration, tr appTracer.Tracer, logger zerolog.Logger, vault *secretvault.Vault) *repositories {
	return &repositories{
		systemConfig:         repository.NewSystemConfigRepo(db, qt, tr, logger),
		client:               repository.NewClientRepo(db, qt, tr, logger),
		invoice:              repository.NewInvoiceRepo(db, qt, tr, logger),
		invoiceLineItem:      repository.NewInvoiceLineItemRepo(db, qt, tr, logger),
		paymentLog:           repository.NewPaymentLogRepo(db, qt, tr, logger),
		invoiceSeq:           repository.NewInvoiceSequenceRepo(db, qt, tr, logger),
		flags:                repository.NewFlagsRepo(db, qt, tr, logger),
		convState:            repository.NewConversationStateRepo(db, qt, tr, logger),
		log:                  repository.NewLogRepo(db, qt, tr, logger),
		escalation:           repository.NewEscalationRepo(db, qt, tr, logger),
		systemConfigKV:       repository.NewConfigRepo(db, qt, tr, logger),
		workspace:            repository.NewWorkspaceRepo(db, qt, tr, logger),
		workspaceMember:      repository.NewWorkspaceMemberRepo(db, qt, tr, logger),
		workspaceInvitation:  repository.NewWorkspaceInvitationRepo(db, qt, tr, logger),
		notification:         repository.NewNotificationRepo(db, qt, tr, logger),
		template:             repository.NewTemplateRepo(db, qt, tr),
		bgJob:                repository.NewBackgroundJobRepo(db, qt, tr, logger),
		whitelist:            repository.NewWhitelistRepo(db, qt, tr, logger),
		masterData:           repository.NewMasterDataRepo(db, qt, tr, logger),
		customField:          repository.NewCustomFieldDefinitionRepo(db, qt, tr, logger),
		masterDataMutation:   repository.NewMasterDataMutationRepo(db, qt, tr, logger),
		approvalRequest:      repository.NewApprovalRequestRepo(db, qt, tr, logger),
		importSession:        repository.NewImportSessionRepo(db, qt, tr, logger),
		clientContact:        repository.NewClientContactRepo(db, qt, tr, logger),
		role:                 repository.NewRoleRepo(db, qt, tr, logger),
		rolePermission:       repository.NewRolePermissionRepo(db, qt, tr, logger),
		teamMember:           repository.NewTeamMemberRepo(db, qt, tr, logger),
		memberWsAssignment:   repository.NewMemberWorkspaceAssignmentRepo(db, qt, tr, logger),
		messageTemplate:      repository.NewMessageTemplateRepo(db, qt, tr, logger),
		emailTemplate:        repository.NewEmailTemplateRepo(db, qt, tr, logger),
		templateVariable:     repository.NewTemplateVariableRepo(db, qt, tr, logger),
		templateEditLog:      repository.NewTemplateEditLogRepo(db, qt, tr, logger),
		userPreferences:      repository.NewUserPreferencesRepo(db, qt, tr, logger),
		workspaceIntegration: repository.NewWorkspaceIntegrationRepoWithVault(db, qt, tr, logger, vault),
		manualAction:         repository.NewManualActionRepo(db, qt, tr, logger),
		auditWsAccess:        repository.NewAuditWorkspaceAccessRepo(db, qt, tr, logger),
		fireflies:            repository.NewFirefliesTranscriptRepo(db, qt, tr, logger),
		claudeExtraction:     repository.NewClaudeExtractionRepo(db, qt, tr, logger),
		reactivation:         repository.NewReactivationRepo(db, qt, tr, logger),
		coaching:             repository.NewCoachingSessionRepo(db, qt, tr, logger),
		rejectionAnalysis:    repository.NewRejectionAnalysisRepo(db, qt, tr, logger),
		pdp:                  repository.NewPDPRepo(db, qt, tr, logger),
		workspaceTheme:       repository.NewWorkspaceThemeRepo(db, qt, tr, logger),
		activityFeed:         repository.NewActivityFeedRepo(db, qt, tr, logger),
		teamActivity:         repository.NewTeamActivityLogRepo(db, qt, tr, logger),
		revokedSessions:      repository.NewRevokedSessionsRepo(db, qt, tr, logger),
		triggerRule:          repository.NewTriggerRuleRepo(db, qt, tr, logger),
		workflow:             repository.NewWorkflowRepo(db, qt, tr, logger),
		automationRule:       repository.NewAutomationRuleRepo(db, qt, tr, logger),
		pipelineView:         repository.NewPipelineViewRepo(db, qt, tr, logger),
		analytics:            repository.NewAnalyticsRepo(db, qt, tr, logger),
		revenueTarget:        repository.NewRevenueTargetRepo(db, qt, tr, logger),
		revenueSnapshot:      repository.NewRevenueSnapshotRepo(db, qt, tr, logger),
		collection:           repository.NewCollectionRepo(db, qt, tr, logger),
		collectionField:      repository.NewCollectionFieldRepo(db, qt, tr, logger),
		collectionRecord:     repository.NewCollectionRecordRepo(db, qt, tr, logger),
		actionLogWorkflow:    repository.NewActionLogWorkflowRepo(db, qt, tr, logger),
	}
}
