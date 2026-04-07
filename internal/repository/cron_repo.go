package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
)

// CronRepository defines the interface for cron log operations.
type CronRepository interface {
	MarkPending(ctx context.Context, runDate time.Time, companyID string) error
	MarkDone(ctx context.Context, id int, status string) error
	MarkError(ctx context.Context, id int, errMsg string) error
	GetPendingClients(ctx context.Context, runDate time.Time) ([]string, error)
	GetPendingCount(ctx context.Context, runDate time.Time) (int, error)
}

type cronRepo struct {
	DB           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
}

// NewCronRepo creates a new instance of CronRepository.
func NewCronRepo(db *sql.DB, queryTimeout time.Duration, tracer tracer.Tracer) CronRepository {
	return &cronRepo{
		DB:           db,
		queryTimeout: queryTimeout,
		tracer:       tracer,
	}
}

func (r *cronRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

// MarkPending creates a pending cron log entry for a client on a given date.
func (r *cronRepo) MarkPending(ctx context.Context, runDate time.Time, companyID string) error {
	ctx, span := r.tracer.Start(ctx, "cron.repository.MarkPending")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	// Use INSERT ON CONFLICT DO NOTHING to handle duplicate entries
	query, args, err := database.PSQL.
		Insert("cron_log").
		Columns("run_date", "company_id", "status").
		Values(runDate, companyID, "pending").
		Suffix("ON CONFLICT (run_date, company_id) DO NOTHING").
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.DB.ExecContext(ctx, query, args...)
	return err
}

// MarkDone updates a cron log entry as completed.
func (r *cronRepo) MarkDone(ctx context.Context, id int, status string) error {
	ctx, span := r.tracer.Start(ctx, "cron.repository.MarkDone")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Update("cron_log").
		SetMap(map[string]interface{}{
			"status":       status,
			"processed_at": sq.Expr("NOW()"),
		}).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.DB.ExecContext(ctx, query, args...)
	return err
}

// MarkError updates a cron log entry with an error message.
func (r *cronRepo) MarkError(ctx context.Context, id int, errMsg string) error {
	ctx, span := r.tracer.Start(ctx, "cron.repository.MarkError")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Update("cron_log").
		SetMap(map[string]interface{}{
			"status":        "error",
			"processed_at":  sq.Expr("NOW()"),
			"error_message": errMsg,
		}).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.DB.ExecContext(ctx, query, args...)
	return err
}

// GetPendingClients retrieves all company IDs with pending status for a given date.
func (r *cronRepo) GetPendingClients(ctx context.Context, runDate time.Time) ([]string, error) {
	ctx, span := r.tracer.Start(ctx, "cron.repository.GetPendingClients")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select("id", "company_id").
		From("cron_log").
		Where(sq.And{
			sq.Eq{"run_date": runDate},
			sq.Eq{"status": "pending"},
		}).
		OrderBy("company_id").
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var companyIDs []string
	for rows.Next() {
		var id int
		var companyID string
		if err := rows.Scan(&id, &companyID); err != nil {
			return nil, err
		}
		companyIDs = append(companyIDs, companyID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cron rows: %w", err)
	}

	return companyIDs, nil
}

// GetPendingCount returns the count of pending entries for a given date.
func (r *cronRepo) GetPendingCount(ctx context.Context, runDate time.Time) (int, error) {
	ctx, span := r.tracer.Start(ctx, "cron.repository.GetPendingCount")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select("COUNT(*)").
		From("cron_log").
		Where(sq.And{
			sq.Eq{"run_date": runDate},
			sq.Eq{"status": "pending"},
		}).
		ToSql()
	if err != nil {
		return 0, err
	}

	var count int
	err = r.DB.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}
