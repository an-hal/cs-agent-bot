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
	sheetsClient "github.com/Sejutacita/cs-agent-bot/internal/pkg/database/sheets"
	pkgLogger "github.com/Sejutacita/cs-agent-bot/internal/pkg/logger"
	pkgValidator "github.com/Sejutacita/cs-agent-bot/internal/pkg/validator"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	pkgCache "github.com/Sejutacita/cs-agent-bot/internal/service/cache"
	appTracer "github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/classifier"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/cron"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/escalation"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/haloai"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/telegram"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/template"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/trigger"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/webhook"
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

	// Initialize tracer
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

	// Initialize Redis
	redisClient := pkgDatabase.NewRedisClient(cfg, logger)
	rdb := redisClient.InitRedis()

	// Initialize Google Sheets client
	sheetsService, err := sheetsClient.NewSheetsClient(cfg, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize Google Sheets client")
	}

	// Initialize SheetCache
	cacheService := pkgCache.NewSheetCache(rdb, sheetsService, logger)

	// Start background cache refresher (runs until app shutdown)
	cacheCtx, cacheCancel := context.WithCancel(context.Background())
	defer cacheCancel()
	cacheService.StartRefresher(cacheCtx)

	// Initialize Repositories
	clientRepo := repository.NewClientRepo(sheetsService, cacheService, logger)
	invoiceRepo := repository.NewInvoiceRepo(sheetsService, cacheService, logger)
	flagsRepo := repository.NewFlagsRepo(sheetsService, cacheService, logger)
	logRepo := repository.NewLogRepo(sheetsService, logger)
	escalationRepo := repository.NewEscalationRepo(sheetsService, cacheService, logger)
	configRepo := repository.NewConfigRepo(sheetsService, cacheService, logger)

	// Initialize External Services
	haloaiClient := haloai.NewHaloAIClient(cfg.HaloAIAPIURL, cfg.WAAPIToken, cfg.HaloAIBusinessID, cfg.HaloAIChannelID, logger)
	telegramNotifier := telegram.NewTelegramNotifier(cfg.TelegramBotToken, cfg.TelegramAELeadID, logger)

	// Initialize Template Resolver (depends on configRepo)
	templateResolver := template.NewTemplateResolver(configRepo, logger)

	// Initialize Reply Classifier
	replyClassifier := classifier.NewReplyClassifier()

	// Initialize Escalation Handler
	escalationHandler := escalation.NewEscalationHandler(
		flagsRepo,
		logRepo,
		escalationRepo,
		telegramNotifier,
		logger,
	)

	// Initialize Trigger Service (all evaluators)
	triggerService := trigger.NewTriggerService(
		clientRepo,
		invoiceRepo,
		flagsRepo,
		logRepo,
		configRepo,
		escalationRepo,
		templateResolver,
		haloaiClient,
		telegramNotifier,
		escalationHandler,
		cacheService,
		cfg,
		logger,
	)

	// Initialize Cron Runner
	cronRunner := cron.NewCronRunner(
		clientRepo,
		flagsRepo,
		invoiceRepo,
		logRepo,
		triggerService,
		logger,
	)

	// Initialize Webhook Handlers
	replyHandler := webhook.NewReplyHandler(
		clientRepo,
		flagsRepo,
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

	// Validator
	validate := pkgValidator.New()

	// Exception Handler
	exceptionHandler := response.NewHTTPExceptionHandler(logger, cfg.EnableStackTrace)

	// HTTP Handler Setup
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
