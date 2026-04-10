package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// SystemConfigRepository provides data access for the system_config table.
type SystemConfigRepository interface {
	GetAll(ctx context.Context) (map[string]string, error)
	GetByKey(ctx context.Context, key string) (string, error)
	ListAll(ctx context.Context) ([]entity.SystemConfig, error)
	Upsert(ctx context.Context, key, value, updatedBy string) error
}

type systemConfigRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewSystemConfigRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) SystemConfigRepository {
	return &systemConfigRepo{
		db:           db,
		queryTimeout: queryTimeout,
		tracer:       tr,
		logger:       logger,
	}
}

func (r *systemConfigRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

// GetAll returns all system config entries as a key→value map.
func (r *systemConfigRepo) GetAll(ctx context.Context) (map[string]string, error) {
	ctx, span := r.tracer.Start(ctx, "system_config.repository.GetAll")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select("key", "value").
		From("system_config").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query GetAll system_config: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query GetAll system_config: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scan system_config row: %w", err)
		}
		result[key] = value
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate system_config rows: %w", err)
	}

	return result, nil
}

// GetByKey returns the value for a single config key. Returns empty string if not found.
func (r *systemConfigRepo) GetByKey(ctx context.Context, key string) (string, error) {
	ctx, span := r.tracer.Start(ctx, "system_config.repository.GetByKey")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select("value").
		From("system_config").
		Where(sq.Eq{"key": key}).
		ToSql()
	if err != nil {
		return "", fmt.Errorf("build query GetByKey system_config: %w", err)
	}

	var value string
	err = r.db.QueryRowContext(ctx, query, args...).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("query GetByKey system_config: %w", err)
	}

	return value, nil
}

// ListAll returns all system config entries with full metadata for the dashboard.
func (r *systemConfigRepo) ListAll(ctx context.Context) ([]entity.SystemConfig, error) {
	ctx, span := r.tracer.Start(ctx, "system_config.repository.ListAll")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select("key", "value", "description", "updated_at", "updated_by").
		From("system_config").
		OrderBy("key").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query ListAll system_config: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query ListAll system_config: %w", err)
	}
	defer rows.Close()

	var configs []entity.SystemConfig
	for rows.Next() {
		var c entity.SystemConfig
		var updatedBy sql.NullString
		var updatedAt sql.NullTime
		if err := rows.Scan(&c.Key, &c.Value, &c.Description, &updatedAt, &updatedBy); err != nil {
			return nil, fmt.Errorf("scan system_config row: %w", err)
		}
		if updatedBy.Valid {
			c.UpdatedBy = updatedBy.String
		}
		if updatedAt.Valid {
			c.UpdatedAt = &updatedAt.Time
		}
		configs = append(configs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate system_config rows: %w", err)
	}

	return configs, nil
}

// Upsert inserts or updates a single config entry.
func (r *systemConfigRepo) Upsert(ctx context.Context, key, value, updatedBy string) error {
	ctx, span := r.tracer.Start(ctx, "system_config.repository.Upsert")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query := `
		INSERT INTO system_config (key, value, updated_at, updated_by)
		VALUES ($1, $2, NOW(), $3)
		ON CONFLICT (key) DO UPDATE
		  SET value = EXCLUDED.value,
		      updated_at = NOW(),
		      updated_by = EXCLUDED.updated_by`

	if _, err := r.db.ExecContext(ctx, query, key, value, updatedBy); err != nil {
		return fmt.Errorf("upsert system_config key=%s: %w", key, err)
	}

	return nil
}
