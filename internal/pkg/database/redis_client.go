package database

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/config"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type RedisClient struct {
	host     string
	port     string
	password string
	db       int
	logger   zerolog.Logger
	enabled  bool
}

func NewRedisClient(cfg *config.AppConfig, logger zerolog.Logger) RedisClient {
	enabled := strings.EqualFold(cfg.RedisEnabled, "true") || cfg.RedisEnabled == "1"

	dbIndex := 0
	if cfg.RedisDB != "" && cfg.RedisDB != "not_set" {
		if v, err := strconv.Atoi(cfg.RedisDB); err == nil {
			dbIndex = v
		} else {
			logger.Warn().
				Err(err).
				Msgf("⚠️ Invalid RedisDB value '%s', fallback to 0", cfg.RedisDB)
		}
	}

	return RedisClient{
		host:     cfg.RedisHost,
		port:     cfg.RedisPort,
		password: cfg.RedisPassword,
		db:       dbIndex,
		logger:   logger,
		enabled:  enabled,
	}
}

func (r *RedisClient) InitRedis() *redis.Client {
	if !r.enabled {
		r.logger.Warn().Msg("🟡 Redis is disabled via REDIS_ENABLED env, skipping Redis initialization")
		return nil
	}

	addr := fmt.Sprintf("%s:%s", r.host, r.port)

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: r.password,
		DB:       r.db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		r.logger.Fatal().Err(err).Msgf("❌ Failed to ping Redis: %v", err)
	}

	r.logger.Info().Msg("✅ Connected to Redis successfully")
	return client
}
