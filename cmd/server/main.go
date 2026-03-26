// @title           Customer Service Agent Bot API
// @version         1.0
// @description     AI-powered customer service agent bot for automated support

// @contact.name   Dealls Engineering
// @contact.url    https://github.com/Sejutacita

// @host      api-dev.sejutacita.id
// @BasePath  /v1/cs-agent-bot

// @securityDefinitions.apikey APIKeyAuth
// @in header
// @name X-API-Key
// @description API Key for service authentication

// @securityDefinitions.apikey APISecretAuth
// @in header
// @name X-API-Secret
// @description API Secret for service authentication (required with X-API-Key)

package main

import (
	"context"
	"database/sql"
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
	pkgLogger "github.com/Sejutacita/cs-agent-bot/internal/pkg/logger"
	pkgValidator "github.com/Sejutacita/cs-agent-bot/internal/pkg/validator"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/service/session"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase"
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

	_ = godotenv.Load() // Load .env

	// Load env config
	cfg := config.LoadConfig()

	// Override tracer service version if not set
	if cfg.TracerServiceVersion == "" {
		cfg.TracerServiceVersion = version
	}

	logger := pkgLogger.InitLogger(cfg.Env, cfg.LogLevel)

	// Initialize tracer
	tracerInstance, err := tracer.New(cfg)
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

	postgresClient := pkgDatabase.NewPostgresClient(cfg, logger)
	db := postgresClient.InitPostgresDB()
	redisClient := pkgDatabase.NewRedisClient(cfg, logger)
	rdb := redisClient.InitRedis()
	sessionStore := session.NewRedisSessionStore(rdb, logger)

	// Start PostgreSQL stats logger if enabled
	// Create a cancellable context for the stats logger
	statsCtx, statsCancel := context.WithCancel(context.Background())
	defer statsCancel()
	postgresClient.StartStatsLogger(statsCtx, db)

	// Repository Initialization
	exampleRepo := repository.NewExampleRepo(db, postgresClient.QueryTimeout, tracerInstance)

	// UseCase Initialization
	exampleUC := usecase.NewExampleUseCase(exampleRepo, db, logger, tracerInstance)

	// Validator Initialization
	validate := pkgValidator.New()

	// Exception Handler Initialization
	exceptionHandler := response.NewHTTPExceptionHandler(logger, cfg.EnableStackTrace)

	// HTTP Handler Setup
	handler := deliveryHttp.SetupHandler(deliveryHttpDeps.Deps{
		Cfg:              cfg,
		Logger:           logger,
		Validator:        validate,
		Tracer:           tracerInstance,
		ExceptionHandler: exceptionHandler,
		ExampleUC:        exampleUC,
		SessionStore:     sessionStore,
	})

	// HTTP server config
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Port),
		Handler: handler,
	}

	// Run server in goroutine
	go func() {
		logger.Info().Msgf("Server running on http://localhost:%s", cfg.Port)
		logger.Info().Msgf("Swagger running on http://localhost:%s%s/swagger/index.html", cfg.Port, cfg.RoutePrefix)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msgf("Server failed: %v", err)
		}
	}()

	// Setup signal listener
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msgf("Gracefully shutting down server...")

	// Remove liveness file
	removeLivenessErr := os.Remove(LiveFile)
	if removeLivenessErr != nil {
		log.Fatal(removeLivenessErr)
	}

	// Graceful shutdown context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal().Err(err).Msgf("Server shutdown failed: %v", err)
	}

	// Close PostgreSQL DB
	closePostgres(db, logger)

	// Close Redis DB
	closeRedis(rdb, logger)

	logger.Info().Msgf("Server shutdown completed.")
}

func closePostgres(db *sql.DB, logger zerolog.Logger) {
	if err := db.Close(); err != nil {
		logger.Info().Msgf("Failed to close PostgreSQL connection: %v", err)
	} else {
		logger.Info().Msgf("PostgreSQL connection closed.")
	}
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
