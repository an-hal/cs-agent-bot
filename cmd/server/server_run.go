package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

const shutdownTimeout = 5 * time.Second

// runHTTPServer starts the HTTP server, blocks on SIGINT/SIGTERM, then
// performs graceful shutdown plus liveness-file cleanup and Redis close.
func runHTTPServer(addr string, handler http.Handler, rdb *redis.Client, logger zerolog.Logger) {
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", addr),
		Handler: handler,
	}

	go func() {
		logger.Info().Msgf("Server running on http://localhost:%s", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal().Err(err).Msg("Server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Gracefully shutting down server...")

	if err := os.Remove(LiveFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.Warn().Err(err).Msg("Failed to remove liveness file during shutdown")
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("Server shutdown failed")
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
