package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/config"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
)

type PostgresClient struct {
	dbHost     string
	dbPort     string
	dbUser     string
	dbPassword string
	dbName     string
	dbSSLMode  string
	logger     zerolog.Logger
	enabled    bool

	// Pool settings
	maxOpenConns    int
	maxIdleConns    int
	connMaxLifetime time.Duration
	connMaxIdleTime time.Duration

	// Query timeout
	QueryTimeout time.Duration

	// Stats logging
	statsLoggingEnabled bool
}

func NewPostgresClient(cfg *config.AppConfig, logger zerolog.Logger) *PostgresClient {
	enabled := strings.EqualFold(cfg.DBEnabled, "true") || cfg.DBEnabled == "1"

	return &PostgresClient{
		dbHost:     cfg.DBHost,
		dbPort:     cfg.DBPort,
		dbUser:     cfg.DBUser,
		dbPassword: cfg.DBPassword,
		dbName:     cfg.DBName,
		dbSSLMode:  cfg.DBSSLMode,
		logger:     logger,
		enabled:    enabled,

		// Pool settings
		maxOpenConns:    cfg.DBMaxOpenConns,
		maxIdleConns:    cfg.DBMaxIdleConns,
		connMaxLifetime: cfg.DBConnMaxLifetime,
		connMaxIdleTime: cfg.DBConnMaxIdleTime,

		// Query timeout
		QueryTimeout: cfg.DBQueryTimeout,

		// Stats logging
		statsLoggingEnabled: cfg.DBStatsLoggingEnabled,
	}
}

func (p *PostgresClient) InitPostgresDB() *sql.DB {
	if !p.enabled {
		p.logger.Warn().Msg("PostgreSQL is disabled via DB_ENABLED env, skipping initialization")
		return nil
	}

	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		p.dbHost, p.dbPort, p.dbUser, p.dbPassword, p.dbName, p.dbSSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		p.logger.Fatal().Err(err).Msg("Failed to open database connection")
	}

	// Configure connection pool
	db.SetMaxOpenConns(p.maxOpenConns)
	db.SetMaxIdleConns(p.maxIdleConns)
	db.SetConnMaxLifetime(p.connMaxLifetime)
	db.SetConnMaxIdleTime(p.connMaxIdleTime)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		p.logger.Fatal().Err(err).Msg("Failed to ping database")
	}

	p.logger.Info().Msg("Connected to PostgreSQL successfully")
	p.logger.Info().
		Int("max_open_conns", p.maxOpenConns).
		Int("max_idle_conns", p.maxIdleConns).
		Dur("conn_max_lifetime", p.connMaxLifetime).
		Dur("conn_max_idle_time", p.connMaxIdleTime).
		Dur("query_timeout", p.QueryTimeout).
		Msg("PostgreSQL connection pool configured")

	return db
}

// StartStatsLogger starts a goroutine that periodically logs connection pool statistics.
// The interval is fixed at 5 minutes.
// This method should only be called if stats logging is enabled.
func (p *PostgresClient) StartStatsLogger(ctx context.Context, db *sql.DB) {
	if !p.statsLoggingEnabled {
		return
	}

	if db == nil {
		p.logger.Warn().Msg("Cannot start stats logger: database connection is nil")
		return
	}

	interval := 5 * time.Minute
	p.logger.Info().Dur("interval", interval).Msg("PostgreSQL stats logging enabled")

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				p.logger.Info().Msg("PostgreSQL stats logger stopped")
				return
			case <-ticker.C:
				stats := db.Stats()
				p.logger.Info().
					Int("open_connections", stats.OpenConnections).
					Int("in_use", stats.InUse).
					Int("idle", stats.Idle).
					Int("max_open_connections", stats.MaxOpenConnections).
					Int64("wait_count", stats.WaitCount).
					Dur("wait_duration", stats.WaitDuration).
					Int64("max_idle_closed", stats.MaxIdleClosed).
					Int64("max_idle_time_closed", stats.MaxIdleTimeClosed).
					Int64("max_lifetime_closed", stats.MaxLifetimeClosed).
					Msg("PostgreSQL pool stats")
			}
		}
	}()
}

// IsStatsLoggingEnabled returns whether stats logging is enabled
func (p *PostgresClient) IsStatsLoggingEnabled() bool {
	return p.statsLoggingEnabled
}

// PingWithRetry attempts to ping the database with exponential backoff retry.
func (p *PostgresClient) PingWithRetry(ctx context.Context, db *sql.DB, maxRetries int) error {
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	baseDelay := 1 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := db.PingContext(ctx); err == nil {
			if attempt > 0 {
				p.logger.Info().
					Int("attempts", attempt+1).
					Msg("Database ping successful")
			}
			return nil
		} else {
			if attempt < maxRetries-1 {
				delay := baseDelay * time.Duration(1<<attempt) // Exponential backoff: 1s, 2s, 4s, 8s...
				p.logger.Warn().Err(err).
					Int("attempt", attempt+1).
					Int("max_retries", maxRetries).
					Dur("retry_in", delay).
					Msg("Database ping failed, retrying")
				time.Sleep(delay)
			} else {
				p.logger.Error().Err(err).
					Int("attempts", maxRetries).
					Msg("Database ping failed after all retries")
				return fmt.Errorf("failed to ping database after %d attempts: %w", maxRetries, err)
			}
		}
	}

	return fmt.Errorf("failed to ping database after %d attempts", maxRetries)
}
