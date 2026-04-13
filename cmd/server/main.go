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
	pkgValidator "github.com/Sejutacita/cs-agent-bot/internal/pkg/validator"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	appTracer "github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/auth"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/classifier"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/cron"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/dashboard"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/escalation"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/haloai"
	notificationuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/notification"
	usecasePayment "github.com/Sejutacita/cs-agent-bot/internal/usecase/payment"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/telegram"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/template"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/trigger"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/webhook"
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
