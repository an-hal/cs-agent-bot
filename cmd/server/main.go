// @title           Customer Service Agent Bot API
// @version         1.0
// @description     WhatsApp automation bot for Kantorku.id HRIS SaaS

// @contact.name   Dealls Engineering
// @contact.url    https://github.com/Sejutacita

// @host      api-dev.sejutacita.id
// @BasePath  /v1/cs-agent-bot

package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/Sejutacita/cs-agent-bot/config"
	_ "github.com/Sejutacita/cs-agent-bot/docs"
	deliveryHttp "github.com/Sejutacita/cs-agent-bot/internal/delivery/http"
	deliveryHttpDeps "github.com/Sejutacita/cs-agent-bot/internal/delivery/http/deps"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/conditiondsl"
	pkgDatabase "github.com/Sejutacita/cs-agent-bot/internal/pkg/database"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/jobstore"
	pkgLogger "github.com/Sejutacita/cs-agent-bot/internal/pkg/logger"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/rediscache"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/secretvault"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	pkgValidator "github.com/Sejutacita/cs-agent-bot/internal/pkg/validator"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/workday"
	appTracer "github.com/Sejutacita/cs-agent-bot/internal/tracer"
	analyticsuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/analytics"
	approvaluc "github.com/Sejutacita/cs-agent-bot/internal/usecase/approval"
	auditwsuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/audit_workspace_access"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/auth"
	automationrule "github.com/Sejutacita/cs-agent-bot/internal/usecase/automation_rule"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/classifier"
	claudeextractionuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/claude_extraction"
	coachinguc "github.com/Sejutacita/cs-agent-bot/internal/usecase/coaching"
	collectionuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/collection"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/cron"
	customfielduc "github.com/Sejutacita/cs-agent-bot/internal/usecase/custom_field"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/dashboard"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/escalation"
	firefliesuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/fireflies"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/haloai"
	invoiceuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/invoice"
	manualactionuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/manual_action"
	masterdatauc "github.com/Sejutacita/cs-agent-bot/internal/usecase/master_data"
	messaginguc "github.com/Sejutacita/cs-agent-bot/internal/usecase/messaging"
	notificationuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/notification"
	usecasePayment "github.com/Sejutacita/cs-agent-bot/internal/usecase/payment"
	pdpuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/pdp"
	pipelineview "github.com/Sejutacita/cs-agent-bot/internal/usecase/pipeline_view"
	reactivationuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/reactivation"
	rejectionanalysisuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/rejection_analysis"
	reportsuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/reports"
	teamuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/team"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/telegram"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/template"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/trigger"
	userpreferencesuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/user_preferences"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/webhook"
	workflowuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/workflow"
	workspaceuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/workspace"
	workspaceintegrationuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/workspace_integration"
	"github.com/joho/godotenv"
)

const LiveFile = "/tmp/app_server_live"
const version = "v1.0.0"

func main() {
	if _, err := os.Create(LiveFile); err != nil {
		log.Fatal("fail to create live file :: ", err)
	}
	_ = godotenv.Load()

	cfg := config.LoadConfig()
	if cfg.TracerServiceVersion == "" {
		cfg.TracerServiceVersion = version
	}

	logger := pkgLogger.InitLogger(cfg.Env, cfg.LogLevel)

	tracerInstance := mustInitTracer(cfg, logger)
	defer shutdownTracer(tracerInstance, logger)

	redisClient := pkgDatabase.NewRedisClient(cfg, logger)
	rdb := redisClient.InitRedis()
	db := mustOpenPostgres(cfg, logger)

	// Optional secret vault for workspace_integrations.config. Absent key →
	// plaintext (dev default); set CONFIG_ENCRYPTION_KEY in prod.
	configVault := buildSecretVault(cfg, logger)

	repos := wireRepositories(db, cfg.DBQueryTimeout, tracerInstance, logger, configVault)
	hydrateConfigFromDB(cfg, repos.systemConfig, logger)
	cfg.ValidateCriticalAfterHydration()

	fileStore, err := jobstore.NewLocalFileStore(cfg.ExportStoragePath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize file store")
	}
	if markErr := repos.bgJob.MarkOrphansFailed(context.Background()); markErr != nil {
		logger.Warn().Err(markErr).Msg("Failed to mark orphaned jobs as failed")
	}

	mocks := buildMockClients(cfg, logger)
	deps := assembleDeps(cfg, db, rdb, repos, mocks, fileStore, tracerInstance, logger)

	runHTTPServer(cfg.Port, deliveryHttp.SetupHandler(deps), rdb, logger)
}

// mustInitTracer fails fast on tracer init errors — the service can't run
// without observability.
func mustInitTracer(cfg *config.AppConfig, logger zerolog.Logger) appTracer.Tracer {
	tr, err := appTracer.New(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize tracer")
	}
	return tr
}

func shutdownTracer(tr appTracer.Tracer, logger zerolog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tr.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("Failed to shutdown tracer")
	}
}

func mustOpenPostgres(cfg *config.AppConfig, logger zerolog.Logger) *sql.DB {
	pgClient := pkgDatabase.NewPostgresClient(cfg, logger)
	db := pgClient.InitPostgresDB()
	if db == nil {
		logger.Fatal().Msg("PostgreSQL connection required. Set DB_ENABLED=true")
	}
	pgCtx, pgCancel := context.WithCancel(context.Background())
	_ = pgCancel // intentionally leaked — process-lifetime context
	if pgClient.IsStatsLoggingEnabled() {
		pgClient.StartStatsLogger(pgCtx, db)
	}
	return db
}

func buildSecretVault(cfg *config.AppConfig, logger zerolog.Logger) *secretvault.Vault {
	v, err := secretvault.New(cfg.ConfigEncryptionKey)
	if err != nil {
		logger.Warn().Err(err).Msg("CONFIG_ENCRYPTION_KEY invalid — falling back to plaintext storage")
		return nil
	}
	if v == nil {
		logger.Info().Msg("CONFIG_ENCRYPTION_KEY not set — workspace integration secrets stored in plaintext (dev mode)")
	}
	return v
}

// hydrateConfigFromDB pulls overrides from the system_config table; falls
// back to env-var defaults on error so the service still starts.
func hydrateConfigFromDB(cfg *config.AppConfig, sysRepo repository.SystemConfigRepository, logger zerolog.Logger) {
	dbValues, err := sysRepo.GetAll(context.Background())
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to load system_config from DB; using env vars only")
		return
	}
	cfg.HydrateFromDB(dbValues)
	logger.Info().Msg("Config hydrated from system_config table")
}

// assembleDeps stitches every concrete dependency into the HTTP Deps struct.
// Big function but flat — no logic, just dependency wiring.
func assembleDeps(
	cfg *config.AppConfig,
	db *sql.DB,
	rdb *redis.Client,
	repos *repositories,
	mocks *mockClients,
	fileStore jobstore.FileStore,
	tr appTracer.Tracer,
	logger zerolog.Logger,
) deliveryHttpDeps.Deps {
	templateResolver := template.NewTemplateResolver(repos.systemConfigKV, logger)
	haloaiClient := haloai.NewHaloAIClient(cfg.HaloAIAPIURL, cfg.WAAPIToken, cfg.HaloAIBusinessID, cfg.HaloAIChannelID, logger)
	telegramNotifier := telegram.NewTelegramNotifier(cfg.TelegramBotToken, cfg.TelegramAELeadID, templateResolver, logger)

	paymentVerifier := usecasePayment.NewPaymentVerifier(repos.client, repos.flags, repos.log, repos.escalation, telegramNotifier, haloaiClient, templateResolver, logger)
	escalationHandler := escalation.NewEscalationHandler(repos.flags, repos.log, repos.escalation, telegramNotifier, logger)
	triggerService := trigger.NewTriggerService(repos.client, repos.invoice, repos.flags, repos.convState, repos.log, repos.systemConfigKV, repos.escalation, templateResolver, haloaiClient, telegramNotifier, escalationHandler, cfg, logger)
	actionExecutor := trigger.NewActionExecutor(repos.client, repos.invoice, repos.flags, repos.convState, repos.log, repos.systemConfigKV, templateResolver, haloaiClient, telegramNotifier, escalationHandler, cfg, logger)
	ruleEngine := trigger.NewRuleEngine(repos.triggerRule, actionExecutor, logger)

	cronRunner := cron.NewCronRunner(repos.client, repos.flags, repos.convState, repos.invoice, repos.log, repos.bgJob, repos.workspace, triggerService, logger)
	cronRunner.(cron.CronRunnerWithRuleEngine).WithRuleEngine(ruleEngine, cfg.UseDynamicRules)

	replyHandler := webhook.NewReplyHandler(repos.invoice, repos.client, repos.flags, repos.convState, repos.log, classifier.NewReplyClassifier(), escalationHandler, haloaiClient, telegramNotifier, logger)
	checkinHandler := webhook.NewCheckinFormHandler(repos.client, repos.flags, repos.log, telegramNotifier, logger)
	handoffHandler := webhook.NewHandoffHandler(repos.client, repos.flags, repos.log, logger)

	dashboardUsecase := dashboard.NewDashboardUsecase(repos.workspace, repos.client, repos.invoice, repos.escalation, repos.log, repos.template, repos.bgJob, fileStore, tr, logger)
	authUsecase := auth.NewAuthUsecase(repos.whitelist, auth.NewAuthProxyClient(cfg.AuthProxyURL), auth.NewGoogleTokenVerifier(cfg.GoogleClientID), cfg.SessionSecret, logger)
	workspaceUsecase := workspaceuc.New(repos.workspace, repos.workspaceMember, repos.workspaceInvitation, repos.rolePermission, nil, nil)
	notificationUsecase := notificationuc.New(repos.notification)
	userPreferencesUsecase := userpreferencesuc.New(repos.userPreferences)
	workspaceIntegrationUsecase := workspaceintegrationuc.NewWithApproval(repos.workspaceIntegration, repos.approvalRequest)
	// Manual action queue — Telegram/Activity/MasterData hooks degrade gracefully with nil.
	manualActionUsecase := manualactionuc.New(repos.manualAction, nil, nil, nil, logger)
	auditWsAccessUsecase := auditwsuc.New(repos.auditWsAccess)

	claudeExtractionUsecase := claudeextractionuc.New(repos.claudeExtraction, mocks.claude, logger)
	firefliesBridge := claudeextractionuc.NewFirefliesBridge(claudeExtractionUsecase, repos.fireflies, logger)
	firefliesUsecase := firefliesuc.New(repos.fireflies, firefliesBridge, logger)
	reactivationUsecase := reactivationuc.NewWithMutation(repos.reactivation, repos.masterDataMutation)
	coachingUsecase := coachinguc.New(repos.coaching)
	rejectionAnalysisUsecase := rejectionanalysisuc.New(repos.rejectionAnalysis)
	// PDP: real DB-backed enforcer + executor. Retention deletes/anonymizes;
	// erasure scrubs subject-owned rows across the scope whitelist.
	pdpUsecase := pdpuc.New(repos.pdp, pdpuc.NewSQLEnforcer(db, logger), pdpuc.NewSQLErasureExecutor(db, logger))
	masterDataUsecase := masterdatauc.New(repos.masterData, repos.customField, repos.masterDataMutation, repos.approvalRequest, repos.importSession)
	customFieldUsecase := customfielduc.New(repos.customField)
	teamUsecase := teamuc.New(repos.role, repos.rolePermission, repos.teamMember, repos.memberWsAssignment, repos.approvalRequest, repos.whitelist, teamuc.Options{})
	messagingUsecase := messaginguc.New(repos.messageTemplate, repos.emailTemplate, repos.templateVariable, repos.templateEditLog, logger)

	workflowUsecase := workflowuc.New(repos.workflow, logger)
	automationRuleUsecase := automationrule.New(repos.automationRule, repos.approvalRequest, logger)
	pipelineViewUsecase := pipelineview.New(repos.pipelineView, logger)

	invoiceUsecase := invoiceuc.New(db, repos.invoice, repos.invoiceLineItem, repos.paymentLog, repos.invoiceSeq, repos.approvalRequest, repos.workspace, nil, tr, logger)
	invoicePaperIDSvc := invoiceuc.NewPaperIDService(invoiceUsecase)
	invoiceCron := invoiceuc.NewCronInvoice(invoiceUsecase)

	analyticsUsecase := analyticsuc.New(repos.analytics, repos.revenueTarget, repos.revenueSnapshot, repos.workspace, logger)
	reportsUsecase := reportsuc.New(repos.analytics, repos.revenueTarget, repos.workspace, logger)
	collectionUsecase := collectionuc.New(repos.collection, repos.collectionField, repos.collectionRecord, repos.approvalRequest, repos.log, repos.masterData, tr, logger)

	// Workflow + maintenance runners aren't wired into the cron path yet —
	// constructed here so DI surface and tests stay aligned with main wiring.
	workdayProvider := workday.NewProvider(cfg.GoogleCalendarAPIKey)
	stageHandler := cron.NewStageHandler(repos.masterData, repos.actionLogWorkflow, logger)
	manualEnq := &cronManualActionAdapter{uc: manualActionUsecase}
	actionDispatcher := cron.NewChannelDispatcherWith(stageHandler, cron.DispatcherOptions{
		ManualEnqueuer: manualEnq,
		WASender:       mocks.waSenderForCron,
	}, logger)
	_ = cron.NewWorkflowRunner(workflowUsecase, automationRuleUsecase, conditiondsl.NewEvaluator(workdayProvider), repos.actionLogWorkflow, repos.masterData, actionDispatcher, stageHandler, workdayProvider, cfg.UseWorkflowEngine, logger)
	_ = cron.NewMaintenanceRunner(repos.masterData, logger)
	if cfg.UseWorkflowEngine {
		logger.Info().Msg("Workflow engine enabled (USE_WORKFLOW_ENGINE=true)")
	}

	approvalDispatcher := approvaluc.NewWithExtras(
		repos.approvalRequest,
		invoiceUsecase,
		masterDataUsecase,
		collectionUsecase,
		automationRuleUsecase,
		&stageApproverAdapter{uc: masterDataUsecase},
		&integrationApproverAdapter{uc: workspaceIntegrationUsecase},
		logger,
	)

	return deliveryHttpDeps.Deps{
		Cfg:                    cfg,
		Logger:                 logger,
		Validator:              pkgValidator.New(),
		Tracer:                 tr,
		ExceptionHandler:       response.NewHTTPExceptionHandler(logger, cfg.EnableStackTrace),
		CronRunner:             cronRunner,
		ReplyHandler:           replyHandler,
		CheckinHandler:         checkinHandler,
		HandoffHandler:         handoffHandler,
		PaymentVerifier:        paymentVerifier,
		DashboardUsecase:       dashboardUsecase,
		LogRepo:                repos.log,
		TriggerRuleRepo:        repos.triggerRule,
		SystemConfigRepo:       repos.systemConfig,
		RuleEngine:             ruleEngine,
		AuthUsecase:            authUsecase,
		WorkspaceUC:            workspaceUsecase,
		NotificationUC:         notificationUsecase,
		MasterDataUC:           masterDataUsecase,
		CustomFieldUC:          customFieldUsecase,
		TeamUC:                 teamUsecase,
		MessagingUC:            messagingUsecase,
		WorkflowUC:             workflowUsecase,
		AutomationRuleUC:       automationRuleUsecase,
		PipelineViewUC:         pipelineViewUsecase,
		InvoiceUC:              invoiceUsecase,
		InvoiceCron:            invoiceCron,
		PaperIDSvc:             invoicePaperIDSvc,
		AnalyticsUC:            analyticsUsecase,
		ReportsUC:              reportsUsecase,
		RevenueTargetRepo:      repos.revenueTarget,
		RevenueSnapshotRepo:    repos.revenueSnapshot,
		WorkspaceRepo:          repos.workspace,
		CollectionUC:           collectionUsecase,
		UserPreferencesUC:      userPreferencesUsecase,
		WorkspaceIntegrationUC: workspaceIntegrationUsecase,
		ApprovalDispatcher:     approvalDispatcher,
		ManualActionUC:         manualActionUsecase,
		AuditWorkspaceAccessUC: auditWsAccessUsecase,
		FirefliesUC:            firefliesUsecase,
		ClaudeExtractionUC:     claudeExtractionUsecase,
		ReactivationUC:         reactivationUsecase,
		CoachingUC:             coachingUsecase,
		RejectionAnalysisUC:    rejectionAnalysisUsecase,
		MockOutbox:             mocks.outbox,
		MockClaudeClient:       mocks.claudeForHandler,
		MockFFClient:           mocks.firefliesForHandler,
		MockWASender:           mocks.waForHandler,
		MockSMTPClient:         mocks.smtpForHandler,
		PDPUC:                  pdpUsecase,
		AnalyticsCache:         rediscache.New(rdb, "csagent:analytics", logger),
		WorkspaceThemeRepo:     repos.workspaceTheme,
		ActivityFeedRepo:       repos.activityFeed,
		TeamActivityRepo:       repos.teamActivity,
		RevokedSessionsRepo:    repos.revokedSessions,
	}
}
