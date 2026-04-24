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
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Sejutacita/cs-agent-bot/config"
	_ "github.com/Sejutacita/cs-agent-bot/docs"
	deliveryHttp "github.com/Sejutacita/cs-agent-bot/internal/delivery/http"
	deliveryHttpDeps "github.com/Sejutacita/cs-agent-bot/internal/delivery/http/deps"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	pkgDatabase "github.com/Sejutacita/cs-agent-bot/internal/pkg/database"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/jobstore"
	pkgLogger "github.com/Sejutacita/cs-agent-bot/internal/pkg/logger"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/conditiondsl"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/workday"
	pkgValidator "github.com/Sejutacita/cs-agent-bot/internal/pkg/validator"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	appTracer "github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/auth"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/classifier"
	collectionuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/collection"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/cron"
	customfielduc "github.com/Sejutacita/cs-agent-bot/internal/usecase/custom_field"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/dashboard"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/escalation"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/haloai"
	analyticsuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/analytics"
	approvaluc "github.com/Sejutacita/cs-agent-bot/internal/usecase/approval"
	auditwsuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/audit_workspace_access"
	claudeclient "github.com/Sejutacita/cs-agent-bot/internal/usecase/claude_client"
	claudeextractionuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/claude_extraction"
	coachinguc "github.com/Sejutacita/cs-agent-bot/internal/usecase/coaching"
	firefliesuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/fireflies"
	pdpuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/pdp"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/rediscache"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/secretvault"
	firefliesclient "github.com/Sejutacita/cs-agent-bot/internal/usecase/fireflies_client"
	haloaimock "github.com/Sejutacita/cs-agent-bot/internal/usecase/haloai_mock"
	mockoutboxpkg "github.com/Sejutacita/cs-agent-bot/internal/usecase/mockoutbox"
	reactivationuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/reactivation"
	rejectionanalysisuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/rejection_analysis"
	smtpclient "github.com/Sejutacita/cs-agent-bot/internal/usecase/smtp_client"
	invoiceuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/invoice"
	manualactionuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/manual_action"
	reportsuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/reports"
	masterdatauc "github.com/Sejutacita/cs-agent-bot/internal/usecase/master_data"
	automationrule "github.com/Sejutacita/cs-agent-bot/internal/usecase/automation_rule"
	messaginguc "github.com/Sejutacita/cs-agent-bot/internal/usecase/messaging"
	notificationuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/notification"
	pipelineview "github.com/Sejutacita/cs-agent-bot/internal/usecase/pipeline_view"
	teamuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/team"
	userpreferencesuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/user_preferences"
	workspaceintegrationuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/workspace_integration"
	usecasePayment "github.com/Sejutacita/cs-agent-bot/internal/usecase/payment"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/telegram"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/template"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/trigger"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/webhook"
	workflowuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/workflow"
	workspaceuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/workspace"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

const LiveFile = "/tmp/app_server_live"
const version = "v1.0.0"

func main() {
	_, err := os.Create(LiveFile)
	if err != nil {
		log.Fatal("fail to create live file :: ", err)
	}

	_ = godotenv.Load()

	cfg := config.LoadConfig()

	if cfg.TracerServiceVersion == "" {
		cfg.TracerServiceVersion = version
	}

	logger := pkgLogger.InitLogger(cfg.Env, cfg.LogLevel)

	tracerInstance, err := appTracer.New(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize tracer")
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tracerInstance.Shutdown(ctx); err != nil {
			logger.Error().Err(err).Msg("Failed to shutdown tracer")
		}
	}()

	redisClient := pkgDatabase.NewRedisClient(cfg, logger)
	rdb := redisClient.InitRedis()

	pgClient := pkgDatabase.NewPostgresClient(cfg, logger)
	db := pgClient.InitPostgresDB()
	if db == nil {
		logger.Fatal().Msg("PostgreSQL connection required. Set DB_ENABLED=true")
	}

	pgCtx, pgCancel := context.WithCancel(context.Background())
	defer pgCancel()
	if pgClient.IsStatsLoggingEnabled() {
		pgClient.StartStatsLogger(pgCtx, db)
	}

	queryTimeout := cfg.DBQueryTimeout

	// Hydrate config from system_config table. DB-sourced values override env vars
	// when non-empty, preserving env-var fallback for backward compatibility.
	systemConfigRepo := repository.NewSystemConfigRepo(db, queryTimeout, tracerInstance, logger)
	if dbValues, err := systemConfigRepo.GetAll(context.Background()); err != nil {
		logger.Warn().Err(err).Msg("Failed to load system_config from DB; using env vars only")
	} else {
		cfg.HydrateFromDB(dbValues)
		logger.Info().Msg("Config hydrated from system_config table")
	}
	cfg.ValidateCriticalAfterHydration()

	clientRepo := repository.NewClientRepo(db, queryTimeout, tracerInstance, logger)
	invoiceRepo := repository.NewInvoiceRepo(db, queryTimeout, tracerInstance, logger)
	invoiceLineItemRepo := repository.NewInvoiceLineItemRepo(db, queryTimeout, tracerInstance, logger)
	paymentLogRepo := repository.NewPaymentLogRepo(db, queryTimeout, tracerInstance, logger)
	invoiceSeqRepo := repository.NewInvoiceSequenceRepo(db, queryTimeout, tracerInstance, logger)
	flagsRepo := repository.NewFlagsRepo(db, queryTimeout, tracerInstance, logger)
	convStateRepo := repository.NewConversationStateRepo(db, queryTimeout, tracerInstance, logger)
	logRepo := repository.NewLogRepo(db, queryTimeout, tracerInstance, logger)
	escalationRepo := repository.NewEscalationRepo(db, queryTimeout, tracerInstance, logger)
	configRepo := repository.NewConfigRepo(db, queryTimeout, tracerInstance, logger)
	workspaceRepo := repository.NewWorkspaceRepo(db, queryTimeout, tracerInstance, logger)
	workspaceMemberRepo := repository.NewWorkspaceMemberRepo(db, queryTimeout, tracerInstance, logger)
	workspaceInvitationRepo := repository.NewWorkspaceInvitationRepo(db, queryTimeout, tracerInstance, logger)
	notificationRepo := repository.NewNotificationRepo(db, queryTimeout, tracerInstance, logger)
	templateRepo := repository.NewTemplateRepo(db, queryTimeout, tracerInstance)
	bgJobRepo := repository.NewBackgroundJobRepo(db, queryTimeout, tracerInstance, logger)
	whitelistRepo := repository.NewWhitelistRepo(db, queryTimeout, tracerInstance, logger)
	masterDataRepo := repository.NewMasterDataRepo(db, queryTimeout, tracerInstance, logger)
	customFieldRepo := repository.NewCustomFieldDefinitionRepo(db, queryTimeout, tracerInstance, logger)
	masterDataMutationRepo := repository.NewMasterDataMutationRepo(db, queryTimeout, tracerInstance, logger)
	approvalRequestRepo := repository.NewApprovalRequestRepo(db, queryTimeout, tracerInstance, logger)
	roleRepo := repository.NewRoleRepo(db, queryTimeout, tracerInstance, logger)
	rolePermissionRepo := repository.NewRolePermissionRepo(db, queryTimeout, tracerInstance, logger)
	teamMemberRepo := repository.NewTeamMemberRepo(db, queryTimeout, tracerInstance, logger)
	memberWorkspaceAssignmentRepo := repository.NewMemberWorkspaceAssignmentRepo(db, queryTimeout, tracerInstance, logger)
	messageTemplateRepo := repository.NewMessageTemplateRepo(db, queryTimeout, tracerInstance, logger)
	emailTemplateRepo := repository.NewEmailTemplateRepo(db, queryTimeout, tracerInstance, logger)
	templateVariableRepo := repository.NewTemplateVariableRepo(db, queryTimeout, tracerInstance, logger)
	templateEditLogRepo := repository.NewTemplateEditLogRepo(db, queryTimeout, tracerInstance, logger)
	userPreferencesRepo := repository.NewUserPreferencesRepo(db, queryTimeout, tracerInstance, logger)
	// Optional secret vault for workspace_integrations.config. Absent key →
	// plaintext (dev default); set CONFIG_ENCRYPTION_KEY in prod.
	configVault, vaultErr := secretvault.New(cfg.ConfigEncryptionKey)
	if vaultErr != nil {
		logger.Warn().Err(vaultErr).Msg("CONFIG_ENCRYPTION_KEY invalid — falling back to plaintext storage")
		configVault = nil
	} else if configVault == nil {
		logger.Info().Msg("CONFIG_ENCRYPTION_KEY not set — workspace integration secrets stored in plaintext (dev mode)")
	}
	workspaceIntegrationRepo := repository.NewWorkspaceIntegrationRepoWithVault(db, queryTimeout, tracerInstance, logger, configVault)
	manualActionRepo := repository.NewManualActionRepo(db, queryTimeout, tracerInstance, logger)
	auditWsAccessRepo := repository.NewAuditWorkspaceAccessRepo(db, queryTimeout, tracerInstance, logger)
	firefliesRepo := repository.NewFirefliesTranscriptRepo(db, queryTimeout, tracerInstance, logger)
	claudeExtractionRepo := repository.NewClaudeExtractionRepo(db, queryTimeout, tracerInstance, logger)
	reactivationRepo := repository.NewReactivationRepo(db, queryTimeout, tracerInstance, logger)
	coachingRepo := repository.NewCoachingSessionRepo(db, queryTimeout, tracerInstance, logger)
	rejectionAnalysisRepo := repository.NewRejectionAnalysisRepo(db, queryTimeout, tracerInstance, logger)
	pdpRepo := repository.NewPDPRepo(db, queryTimeout, tracerInstance, logger)
	workspaceThemeRepo := repository.NewWorkspaceThemeRepo(db, queryTimeout, tracerInstance, logger)
	activityFeedRepo := repository.NewActivityFeedRepo(db, queryTimeout, tracerInstance, logger)
	teamActivityRepo := repository.NewTeamActivityLogRepo(db, queryTimeout, tracerInstance, logger)
	revokedSessionsRepo := repository.NewRevokedSessionsRepo(db, queryTimeout, tracerInstance, logger)

	fileStore, err := jobstore.NewLocalFileStore(cfg.ExportStoragePath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize file store")
	}

	// Mark any jobs that were left in 'processing' by a previous crash.
	if markErr := bgJobRepo.MarkOrphansFailed(context.Background()); markErr != nil {
		logger.Warn().Err(markErr).Msg("Failed to mark orphaned jobs as failed")
	}

	templateResolver := template.NewTemplateResolver(configRepo, logger)

	haloaiClient := haloai.NewHaloAIClient(cfg.HaloAIAPIURL, cfg.WAAPIToken, cfg.HaloAIBusinessID, cfg.HaloAIChannelID, logger)
	telegramNotifier := telegram.NewTelegramNotifier(cfg.TelegramBotToken, cfg.TelegramAELeadID, templateResolver, logger)

	paymentVerifier := usecasePayment.NewPaymentVerifier(
		clientRepo,
		flagsRepo,
		logRepo,
		escalationRepo,
		telegramNotifier,
		haloaiClient,
		templateResolver,
		logger,
	)

	replyClassifier := classifier.NewReplyClassifier()

	escalationHandler := escalation.NewEscalationHandler(
		flagsRepo,
		logRepo,
		escalationRepo,
		telegramNotifier,
		logger,
	)

	triggerService := trigger.NewTriggerService(
		clientRepo,
		invoiceRepo,
		flagsRepo,
		convStateRepo,
		logRepo,
		configRepo,
		escalationRepo,
		templateResolver,
		haloaiClient,
		telegramNotifier,
		escalationHandler,
		cfg,
		logger,
	)

	triggerRuleRepo := repository.NewTriggerRuleRepo(db, queryTimeout, tracerInstance, logger)

	actionExecutor := trigger.NewActionExecutor(
		clientRepo,
		invoiceRepo,
		flagsRepo,
		convStateRepo,
		logRepo,
		configRepo,
		templateResolver,
		haloaiClient,
		telegramNotifier,
		escalationHandler,
		cfg,
		logger,
	)

	ruleEngine := trigger.NewRuleEngine(triggerRuleRepo, actionExecutor, logger)

	cronRunner := cron.NewCronRunner(
		clientRepo,
		flagsRepo,
		convStateRepo,
		invoiceRepo,
		logRepo,
		bgJobRepo,
		workspaceRepo,
		triggerService,
		logger,
	)

	// Enable dynamic rule engine if configured
	cronRunner.(cron.CronRunnerWithRuleEngine).WithRuleEngine(ruleEngine, cfg.UseDynamicRules)

	replyHandler := webhook.NewReplyHandler(
		invoiceRepo,
		clientRepo,
		flagsRepo,
		convStateRepo,
		logRepo,
		replyClassifier,
		escalationHandler,
		haloaiClient,
		telegramNotifier,
		logger,
	)

	checkinHandler := webhook.NewCheckinFormHandler(
		clientRepo,
		flagsRepo,
		logRepo,
		telegramNotifier,
		logger,
	)

	handoffHandler := webhook.NewHandoffHandler(
		clientRepo,
		flagsRepo,
		logRepo,
		logger,
	)

	dashboardUsecase := dashboard.NewDashboardUsecase(
		workspaceRepo,
		clientRepo,
		invoiceRepo,
		escalationRepo,
		logRepo,
		templateRepo,
		bgJobRepo,
		fileStore,
		tracerInstance,
		logger,
	)

	authProxyClient := auth.NewAuthProxyClient(cfg.AuthProxyURL)
	googleVerifier := auth.NewGoogleTokenVerifier(cfg.GoogleClientID)
	authUsecase := auth.NewAuthUsecase(whitelistRepo, authProxyClient, googleVerifier, cfg.SessionSecret, logger)
	workspaceUsecase := workspaceuc.New(workspaceRepo, workspaceMemberRepo, workspaceInvitationRepo, nil, nil)
	notificationUsecase := notificationuc.New(notificationRepo)
	userPreferencesUsecase := userpreferencesuc.New(userPreferencesRepo)
	workspaceIntegrationUsecase := workspaceintegrationuc.NewWithApproval(workspaceIntegrationRepo, approvalRequestRepo)
	// Manual action queue — Telegram/Activity/MasterData hooks not wired yet;
	// the usecase degrades gracefully with nil hooks.
	manualActionUsecase := manualactionuc.New(manualActionRepo, nil, nil, nil, logger)
	auditWsAccessUsecase := auditwsuc.New(auditWsAccessRepo)

	// Shared mock outbox (in-memory ring buffer, viewable at /mock/outbox).
	mockOutbox := mockoutboxpkg.New(200)

	// External API clients — use mock impls when MOCK_EXTERNAL_APIS=true OR
	// when the corresponding API key is absent (keeps dev + staging working
	// without real credentials). Real clients are wired in when creds exist
	// and the mock flag is off.
	useMockClaude := cfg.MockExternalAPIs || cfg.ClaudeAPIKey == ""
	useMockFireflies := cfg.MockExternalAPIs || cfg.FirefliesAPIKey == ""
	useMockSMTP := cfg.MockExternalAPIs || cfg.SMTPHost == ""
	useMockHaloAI := cfg.MockExternalAPIs || cfg.WAAPIToken == ""

	var claudeAPIClient claudeextractionuc.Client
	if useMockClaude {
		claudeAPIClient = claudeclient.NewMockClient(claudeclient.MockConfig{Outbox: mockOutbox}, logger)
	} else {
		claudeAPIClient = claudeclient.NewClient(claudeclient.Config{
			APIKey:              cfg.ClaudeAPIKey,
			Model:               cfg.ClaudeModel,
			ExtractionPromptKey: cfg.ClaudeExtractPrompt,
			BANTSPromptKey:      cfg.ClaudeBANTSPrompt,
			Timeout:             cfg.ClaudeTimeoutSecs,
		}, logger)
	}

	// Fireflies + SMTP clients reserved for pull-mode transcript fetch and
	// email template delivery. Mock impls record to the outbox.
	if useMockFireflies {
		_ = firefliesclient.NewMockClient(firefliesclient.MockConfig{Outbox: mockOutbox}, logger)
	} else {
		_ = firefliesclient.NewClient(firefliesclient.Config{
			APIKey:     cfg.FirefliesAPIKey,
			GraphQLURL: cfg.FirefliesGraphQLURL,
		}, logger)
	}
	if useMockSMTP {
		_ = smtpclient.NewMockClient(smtpclient.MockConfig{Outbox: mockOutbox}, logger)
	} else {
		_ = smtpclient.NewClient(smtpclient.Config{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword,
			FromAddr: cfg.SMTPFromAddr,
			UseTLS:   cfg.SMTPUseTLS,
		}, logger)
	}

	// HaloAI WA sender for the cron dispatcher — mock records to outbox, real
	// one would use the existing haloai.Client (not wired here; see TODO).
	var waSenderForCron cron.WASender
	var mockWAForHandler haloaimock.Sender
	if useMockHaloAI {
		mockWAForHandler = haloaimock.NewSender(mockOutbox, logger)
		waSenderForCron = &mockWASenderAdapter{mock: mockWAForHandler}
	}

	// Standalone mock clients for the /mock HTTP endpoints (FE QA). Always
	// mock regardless of MOCK_EXTERNAL_APIS so the /mock surface still works
	// when production mode is on — real calls go via real clients elsewhere.
	mockClaudeForHandler := claudeclient.NewMockClient(claudeclient.MockConfig{Outbox: mockOutbox}, logger)
	mockFFForHandler := firefliesclient.NewMockClient(firefliesclient.MockConfig{Outbox: mockOutbox}, logger)
	if mockWAForHandler == nil {
		mockWAForHandler = haloaimock.NewSender(mockOutbox, logger)
	}
	mockSMTPForHandler := smtpclient.NewMockClient(smtpclient.MockConfig{Outbox: mockOutbox}, logger)

	claudeExtractionUsecase := claudeextractionuc.New(claudeExtractionRepo, claudeAPIClient, logger)
	firefliesBridge := claudeextractionuc.NewFirefliesBridge(claudeExtractionUsecase, firefliesRepo, logger)
	firefliesUsecase := firefliesuc.New(firefliesRepo, firefliesBridge, logger)
	reactivationUsecase := reactivationuc.NewWithMutation(reactivationRepo, masterDataMutationRepo)
	coachingUsecase := coachinguc.New(coachingRepo)
	rejectionAnalysisUsecase := rejectionanalysisuc.New(rejectionAnalysisRepo)
	// PDP: real DB-backed enforcer + executor. Retention policies actually
	// delete/anonymize rows; erasure requests scrub all subject-owned rows
	// across the scope whitelist.
	pdpUsecase := pdpuc.New(pdpRepo,
		pdpuc.NewSQLEnforcer(db, logger),
		pdpuc.NewSQLErasureExecutor(db, logger),
	)
	masterDataUsecase := masterdatauc.New(masterDataRepo, customFieldRepo, masterDataMutationRepo, approvalRequestRepo)
	customFieldUsecase := customfielduc.New(customFieldRepo)
	teamUsecase := teamuc.New(
		roleRepo,
		rolePermissionRepo,
		teamMemberRepo,
		memberWorkspaceAssignmentRepo,
		approvalRequestRepo,
		whitelistRepo,
		teamuc.Options{},
	)
	messagingUsecase := messaginguc.New(
		messageTemplateRepo,
		emailTemplateRepo,
		templateVariableRepo,
		templateEditLogRepo,
		logger,
	)

	// Workflow engine (feat/06) — gated by USE_WORKFLOW_ENGINE.
	workflowRepo := repository.NewWorkflowRepo(db, queryTimeout, tracerInstance, logger)
	automationRuleRepo := repository.NewAutomationRuleRepo(db, queryTimeout, tracerInstance, logger)
	pipelineViewRepo := repository.NewPipelineViewRepo(db, queryTimeout, tracerInstance, logger)

	workflowUsecase := workflowuc.New(workflowRepo, logger)
	automationRuleUsecase := automationrule.New(automationRuleRepo, approvalRequestRepo, logger)
	pipelineViewUsecase := pipelineview.New(pipelineViewRepo, logger)

	// Invoice billing (feat/07).
	invoiceUsecase := invoiceuc.New(
		db,
		invoiceRepo,
		invoiceLineItemRepo,
		paymentLogRepo,
		invoiceSeqRepo,
		approvalRequestRepo,
		workspaceRepo,
		nil, // paperidSvc wired after invoiceUsecase (circular dep avoidance).
		tracerInstance,
		logger,
	)
	invoicePaperIDSvc := invoiceuc.NewPaperIDService(invoiceUsecase)
	invoiceCron := invoiceuc.NewCronInvoice(invoiceUsecase)

	// Analytics & Reports (feat/09).
	analyticsRepo := repository.NewAnalyticsRepo(db, queryTimeout, tracerInstance, logger)
	revenueTargetRepo := repository.NewRevenueTargetRepo(db, queryTimeout, tracerInstance, logger)
	revenueSnapshotRepo := repository.NewRevenueSnapshotRepo(db, queryTimeout, tracerInstance, logger)
	analyticsUsecase := analyticsuc.New(analyticsRepo, revenueTargetRepo, revenueSnapshotRepo, workspaceRepo, logger)
	reportsUsecase := reportsuc.New(analyticsRepo, revenueTargetRepo, workspaceRepo, logger)

	// Collections (feat/10) — user-defined generic tables.
	collectionRepo := repository.NewCollectionRepo(db, queryTimeout, tracerInstance, logger)
	collectionFieldRepo := repository.NewCollectionFieldRepo(db, queryTimeout, tracerInstance, logger)
	collectionRecordRepo := repository.NewCollectionRecordRepo(db, queryTimeout, tracerInstance, logger)
	collectionUsecase := collectionuc.New(
		collectionRepo,
		collectionFieldRepo,
		collectionRecordRepo,
		approvalRequestRepo,
		logRepo,
		masterDataRepo,
		tracerInstance,
		logger,
	)

	// Attach workflow runner to cron runner when USE_WORKFLOW_ENGINE is enabled.
	actionLogWorkflowRepo := repository.NewActionLogWorkflowRepo(db, queryTimeout, tracerInstance, logger)
	workdayProvider := workday.NewProvider(cfg.GoogleCalendarAPIKey)
	conditionEvaluator := conditiondsl.NewEvaluator(workdayProvider)
	stageHandler := cron.NewStageHandler(masterDataRepo, actionLogWorkflowRepo, logger)
	// Wire manual-flow enqueuer so manual-flow trigger_ids go to the queue
	// instead of being bot-sent. Adapter shim converts cron's DTO into the
	// manual_action usecase input without introducing a cross-package import.
	manualEnq := &cronManualActionAdapter{uc: manualActionUsecase}
	actionDispatcher := cron.NewChannelDispatcherWith(stageHandler, cron.DispatcherOptions{
		ManualEnqueuer: manualEnq,
		WASender:       waSenderForCron,
	}, logger)
	workflowRunner := cron.NewWorkflowRunner(
		workflowUsecase,
		automationRuleUsecase,
		conditionEvaluator,
		actionLogWorkflowRepo,
		masterDataRepo,
		actionDispatcher,
		stageHandler,
		workdayProvider,
		cfg.UseWorkflowEngine,
		logger,
	)
	maintenanceRunner := cron.NewMaintenanceRunner(masterDataRepo, logger)
	_ = maintenanceRunner // wired to HTTP endpoints below
	if cfg.UseWorkflowEngine {
		logger.Info().Msg("Workflow engine enabled (USE_WORKFLOW_ENGINE=true)")
	}
	_ = workflowRunner

	validate := pkgValidator.New()

	exceptionHandler := response.NewHTTPExceptionHandler(logger, cfg.EnableStackTrace)

	handler := deliveryHttp.SetupHandler(deliveryHttpDeps.Deps{
		Cfg:              cfg,
		Logger:           logger,
		Validator:        validate,
		Tracer:           tracerInstance,
		ExceptionHandler: exceptionHandler,
		CronRunner:       cronRunner,
		ReplyHandler:     replyHandler,
		CheckinHandler:   checkinHandler,
		HandoffHandler:   handoffHandler,
		PaymentVerifier:  paymentVerifier,
		DashboardUsecase: dashboardUsecase,
		LogRepo:          logRepo,
		TriggerRuleRepo:  triggerRuleRepo,
		SystemConfigRepo: systemConfigRepo,
		RuleEngine:       ruleEngine,
		AuthUsecase:      authUsecase,
		WorkspaceUC:      workspaceUsecase,
		NotificationUC:   notificationUsecase,
		MasterDataUC:     masterDataUsecase,
		CustomFieldUC:    customFieldUsecase,
		TeamUC:           teamUsecase,
		MessagingUC:      messagingUsecase,
		WorkflowUC:       workflowUsecase,
		AutomationRuleUC: automationRuleUsecase,
		PipelineViewUC:   pipelineViewUsecase,
		InvoiceUC:        invoiceUsecase,
		InvoiceCron:         invoiceCron,
		PaperIDSvc:          invoicePaperIDSvc,
		AnalyticsUC:         analyticsUsecase,
		ReportsUC:           reportsUsecase,
		RevenueTargetRepo:   revenueTargetRepo,
		RevenueSnapshotRepo: revenueSnapshotRepo,
		WorkspaceRepo:       workspaceRepo,
		CollectionUC:        collectionUsecase,
		UserPreferencesUC:   userPreferencesUsecase,
		WorkspaceIntegrationUC: workspaceIntegrationUsecase,
		ApprovalDispatcher: approvaluc.NewWithExtras(
			approvalRequestRepo,
			invoiceUsecase,
			masterDataUsecase,
			collectionUsecase,
			automationRuleUsecase,
			&stageApproverAdapter{uc: masterDataUsecase},
			&integrationApproverAdapter{uc: workspaceIntegrationUsecase},
			logger,
		),
		ManualActionUC:         manualActionUsecase,
		AuditWorkspaceAccessUC: auditWsAccessUsecase,
		FirefliesUC:            firefliesUsecase,
		ClaudeExtractionUC:     claudeExtractionUsecase,
		ReactivationUC:         reactivationUsecase,
		CoachingUC:             coachingUsecase,
		RejectionAnalysisUC:    rejectionAnalysisUsecase,

		// Mock plumbing
		MockOutbox:       mockOutbox,
		MockClaudeClient: mockClaudeForHandler,
		MockFFClient:     mockFFForHandler,
		MockWASender:     mockWAForHandler,
		MockSMTPClient:   mockSMTPForHandler,

		PDPUC: pdpUsecase,

		// Analytics cache — backed by Redis when available, nil-safe noop otherwise.
		AnalyticsCache: rediscache.New(rdb, "csagent:analytics", logger),

		WorkspaceThemeRepo:  workspaceThemeRepo,
		ActivityFeedRepo:    activityFeedRepo,
		TeamActivityRepo:    teamActivityRepo,
		RevokedSessionsRepo: revokedSessionsRepo,
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Port),
		Handler: handler,
	}

	go func() {
		logger.Info().Msgf("Server running on http://localhost:%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msgf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Gracefully shutting down server...")

	if removeLivenessErr := os.Remove(LiveFile); removeLivenessErr != nil {
		log.Fatal(removeLivenessErr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal().Err(err).Msgf("Server shutdown failed: %v", err)
	}

	closeRedis(rdb, logger)
	logger.Info().Msg("Server shutdown completed.")
}

// stageApproverAdapter wraps master_data.Usecase.ApplyApprovedStageTransition
// into the narrow `(any, error)` shape the central approval dispatcher expects.
type stageApproverAdapter struct{ uc masterdatauc.Usecase }

func (a *stageApproverAdapter) ApplyApprovedStageTransition(ctx context.Context, workspaceID, approvalID, checkerEmail string) (any, error) {
	return a.uc.ApplyApprovedStageTransition(ctx, workspaceID, approvalID, checkerEmail)
}

// integrationApproverAdapter wraps workspace_integration.Usecase similarly.
type integrationApproverAdapter struct{ uc workspaceintegrationuc.Usecase }

func (a *integrationApproverAdapter) ApplyApprovedKeyChange(ctx context.Context, workspaceID, approvalID, checkerEmail string) (any, error) {
	return a.uc.ApplyApprovedKeyChange(ctx, workspaceID, approvalID, checkerEmail)
}

// mockWASenderAdapter bridges cron.WASendRequest → haloaimock.SendRequest so
// the cron dispatcher's narrow port doesn't depend on the mock package. In
// production, a real HaloAI adapter would sit here instead.
type mockWASenderAdapter struct {
	mock haloaimock.Sender
}

func (a *mockWASenderAdapter) Send(ctx context.Context, req cron.WASendRequest) (string, error) {
	resp, err := a.mock.Send(ctx, haloaimock.SendRequest{
		WorkspaceID: req.WorkspaceID,
		To:          req.To,
		TemplateID:  req.TemplateID,
		Body:        req.Body,
		Variables:   req.Variables,
	})
	if err != nil {
		return "", err
	}
	return resp.MessageID, nil
}

// cronManualActionAdapter bridges cron.ManualActionEnqueueInput (what the
// dispatcher speaks) to manualactionuc.CreatePendingInput (what the usecase
// accepts). Same field names; kept in main.go so neither package imports
// the other.
type cronManualActionAdapter struct {
	uc manualactionuc.Usecase
}

func (a *cronManualActionAdapter) Enqueue(ctx context.Context, in cron.ManualActionEnqueueInput) error {
	if a == nil || a.uc == nil {
		return nil
	}
	_, err := a.uc.CreatePending(ctx, manualactionuc.CreatePendingInput{
		WorkspaceID:    in.WorkspaceID,
		MasterDataID:   in.MasterDataID,
		TriggerID:      in.TriggerID,
		FlowCategory:   in.FlowCategory,
		Role:           in.Role,
		AssignedToUser: in.AssignedToUser,
		Priority:       in.Priority,
		DueAt:          in.DueAt,
		SuggestedDraft: in.SuggestedDraft,
		ContextSummary: in.ContextSummary,
	})
	return err
}

func closeRedis(rdb *redis.Client, logger zerolog.Logger) {
	if rdb == nil {
		return
	}

	if err := rdb.Close(); err != nil {
		logger.Info().Msgf("Failed to close Redis connection: %v", err)
	} else {
		logger.Info().Msg("Redis connection closed.")
	}
}
