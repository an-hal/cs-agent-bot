package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// BackgroundJobRepository defines data access for background_jobs.
type BackgroundJobRepository interface {
	Create(ctx context.Context, job *entity.BackgroundJob) error
	GetByID(ctx context.Context, jobID string) (*entity.BackgroundJob, error)
	ListByWorkspace(ctx context.Context, workspaceID, jobType, entityType string, p pagination.Params) ([]entity.BackgroundJob, int64, error)
	UpdateProgress(ctx context.Context, jobID string, totalRows, processed, success, failed, skipped int, errs []entity.JobRowError) error
	UpdateStatus(ctx context.Context, jobID, status string) error
	UpdateStoragePath(ctx context.Context, jobID, storagePath string) error
	MarkOrphansFailed(ctx context.Context) error
}

type backgroundJobRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

// NewBackgroundJobRepo creates a BackgroundJobRepository backed by PostgreSQL.
func NewBackgroundJobRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) BackgroundJobRepository {
	return &backgroundJobRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *backgroundJobRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

func (r *backgroundJobRepo) Create(ctx context.Context, job *entity.BackgroundJob) error {
	ctx, span := r.tracer.Start(ctx, "background_job.repository.Create")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	errJSON, _ := json.Marshal(job.Errors)
	metaJSON, _ := json.Marshal(job.Metadata)

	query, args, err := psql.Insert("background_jobs").
		Columns("id", "workspace_id", "job_type", "status", "entity_type", "filename", "total_rows", "errors", "metadata", "created_by").
		Values(job.ID, job.WorkspaceID, job.JobType, job.Status, job.EntityType, job.Filename, job.TotalRows, errJSON, metaJSON, job.CreatedBy).
		ToSql()
	if err != nil {
		return fmt.Errorf("background_job.Create: build query: %w", err)
	}

	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("background_job.Create: exec: %w", err)
	}
	return nil
}

func (r *backgroundJobRepo) GetByID(ctx context.Context, jobID string) (*entity.BackgroundJob, error) {
	ctx, span := r.tracer.Start(ctx, "background_job.repository.GetByID")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := psql.Select(jobColumns).
		From("background_jobs").
		Where(sq.Eq{"id": jobID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("background_job.GetByID: build query: %w", err)
	}

	row := r.db.QueryRowContext(ctx, query, args...)
	job, err := scanJob(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("background_job.GetByID: scan: %w", err)
	}
	return job, nil
}

func (r *backgroundJobRepo) ListByWorkspace(ctx context.Context, workspaceID, jobType, entityType string, p pagination.Params) ([]entity.BackgroundJob, int64, error) {
	ctx, span := r.tracer.Start(ctx, "background_job.repository.ListByWorkspace")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	cond := sq.Eq{"workspace_id": workspaceID}
	if jobType != "" {
		cond["job_type"] = jobType
	}
	if entityType != "" {
		cond["entity_type"] = entityType
	}

	countQ, countArgs, _ := psql.Select("COUNT(*)").From("background_jobs").Where(cond).ToSql()
	var total int64
	if err := r.db.QueryRowContext(ctx, countQ, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("background_job.ListByWorkspace: count: %w", err)
	}

	query, args, err := psql.Select(jobColumns).
		From("background_jobs").
		Where(cond).
		OrderBy("created_at DESC").
		Limit(uint64(p.Limit)).
		Offset(uint64(p.Offset)).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("background_job.ListByWorkspace: build query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("background_job.ListByWorkspace: query: %w", err)
	}
	defer rows.Close()

	var jobs []entity.BackgroundJob
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("background_job.ListByWorkspace: scan: %w", err)
		}
		jobs = append(jobs, *job)
	}
	return jobs, total, rows.Err()
}

func (r *backgroundJobRepo) UpdateProgress(ctx context.Context, jobID string, totalRows, processed, success, failed, skipped int, errs []entity.JobRowError) error {
	ctx, span := r.tracer.Start(ctx, "background_job.repository.UpdateProgress")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	errJSON, _ := json.Marshal(errs)

	query, args, err := psql.Update("background_jobs").
		Set("total_rows", totalRows).
		Set("processed", processed).
		Set("success", success).
		Set("failed", failed).
		Set("skipped", skipped).
		Set("errors", errJSON).
		Set("updated_at", time.Now()).
		Where(sq.Eq{"id": jobID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("background_job.UpdateProgress: build query: %w", err)
	}

	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("background_job.UpdateProgress: exec: %w", err)
	}
	return nil
}

func (r *backgroundJobRepo) UpdateStatus(ctx context.Context, jobID, status string) error {
	ctx, span := r.tracer.Start(ctx, "background_job.repository.UpdateStatus")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := psql.Update("background_jobs").
		Set("status", status).
		Set("updated_at", time.Now()).
		Where(sq.Eq{"id": jobID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("background_job.UpdateStatus: build query: %w", err)
	}

	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("background_job.UpdateStatus: exec: %w", err)
	}
	return nil
}

func (r *backgroundJobRepo) UpdateStoragePath(ctx context.Context, jobID, storagePath string) error {
	ctx, span := r.tracer.Start(ctx, "background_job.repository.UpdateStoragePath")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := psql.Update("background_jobs").
		Set("storage_path", storagePath).
		Set("updated_at", time.Now()).
		Where(sq.Eq{"id": jobID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("background_job.UpdateStoragePath: build query: %w", err)
	}

	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("background_job.UpdateStoragePath: exec: %w", err)
	}
	return nil
}

// MarkOrphansFailed marks any jobs stuck in 'processing' as 'failed'.
// Call at application startup to clean up jobs orphaned by a previous crash.
func (r *backgroundJobRepo) MarkOrphansFailed(ctx context.Context) error {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := psql.Update("background_jobs").
		Set("status", entity.JobStatusFailed).
		Set("updated_at", time.Now()).
		Where(sq.Eq{"status": entity.JobStatusProcessing}).
		ToSql()
	if err != nil {
		return fmt.Errorf("background_job.MarkOrphansFailed: build query: %w", err)
	}

	res, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("background_job.MarkOrphansFailed: exec: %w", err)
	}

	if n, _ := res.RowsAffected(); n > 0 {
		r.logger.Warn().Int64("count", n).Msg("Marked orphaned processing jobs as failed")
	}
	return nil
}

const jobColumns = "id::text, workspace_id::text, job_type, status, entity_type, COALESCE(filename,'') as filename, COALESCE(storage_path,'') as storage_path, total_rows, processed, success, failed, skipped, COALESCE(errors,'[]'::jsonb) as errors, COALESCE(metadata,'{}'::jsonb) as metadata, created_by, created_at, updated_at"

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanJob(s scanner) (*entity.BackgroundJob, error) {
	var j entity.BackgroundJob
	var errJSON, metaJSON []byte

	err := s.Scan(
		&j.ID,
		&j.WorkspaceID,
		&j.JobType,
		&j.Status,
		&j.EntityType,
		&j.Filename,
		&j.StoragePath,
		&j.TotalRows,
		&j.Processed,
		&j.Success,
		&j.Failed,
		&j.Skipped,
		&errJSON,
		&metaJSON,
		&j.CreatedBy,
		&j.CreatedAt,
		&j.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if len(errJSON) > 0 {
		_ = json.Unmarshal(errJSON, &j.Errors)
	}
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &j.Metadata)
	}
	return &j, nil
}

