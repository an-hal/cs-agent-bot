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

type RejectionAnalysisRepository interface {
	Insert(ctx context.Context, a *entity.RejectionAnalysis) (*entity.RejectionAnalysis, error)
	List(ctx context.Context, filter entity.RejectionAnalysisFilter) ([]entity.RejectionAnalysis, int64, error)
	CountByCategory(ctx context.Context, workspaceID string, since time.Time) (map[string]int, error)
}

type rejectionAnalysisRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewRejectionAnalysisRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) RejectionAnalysisRepository {
	return &rejectionAnalysisRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *rejectionAnalysisRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const raColumns = `id::text, workspace_id::text, master_data_id::text,
    source_channel, source_message, rejection_category, severity,
    analysis_summary, suggested_response, analyst, analyst_version,
    detected_at, created_at`

func scanRA(s interface{ Scan(dest ...interface{}) error }) (*entity.RejectionAnalysis, error) {
	var a entity.RejectionAnalysis
	err := s.Scan(&a.ID, &a.WorkspaceID, &a.MasterDataID,
		&a.SourceChannel, &a.SourceMessage, &a.RejectionCategory, &a.Severity,
		&a.AnalysisSummary, &a.SuggestedResponse, &a.Analyst, &a.AnalystVersion,
		&a.DetectedAt, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *rejectionAnalysisRepo) Insert(ctx context.Context, a *entity.RejectionAnalysis) (*entity.RejectionAnalysis, error) {
	ctx, span := r.tracer.Start(ctx, "rejection_analysis.repository.Insert")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q, args, err := database.PSQL.Insert("rejection_analysis_log").
		Columns("workspace_id", "master_data_id", "source_channel", "source_message",
			"rejection_category", "severity", "analysis_summary", "suggested_response",
			"analyst", "analyst_version").
		Values(a.WorkspaceID, a.MasterDataID, defaultIfBlank(a.SourceChannel, "wa"),
			a.SourceMessage, a.RejectionCategory, defaultIfBlank(a.Severity, entity.RejectionSeverityMid),
			a.AnalysisSummary, a.SuggestedResponse,
			defaultIfBlank(a.Analyst, entity.RejectionAnalystRule), a.AnalystVersion).
		Suffix("RETURNING " + raColumns).ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert: %w", err)
	}
	return scanRA(r.db.QueryRowContext(ctx, q, args...))
}

func (r *rejectionAnalysisRepo) List(ctx context.Context, f entity.RejectionAnalysisFilter) ([]entity.RejectionAnalysis, int64, error) {
	ctx, span := r.tracer.Start(ctx, "rejection_analysis.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	conds := sq.And{sq.Expr("workspace_id::text = ?", f.WorkspaceID)}
	if f.MasterDataID != "" {
		conds = append(conds, sq.Expr("master_data_id::text = ?", f.MasterDataID))
	}
	if f.RejectionCategory != "" {
		conds = append(conds, sq.Eq{"rejection_category": f.RejectionCategory})
	}
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q, args, err := database.PSQL.Select(raColumns).From("rejection_analysis_log").
		Where(conds).OrderBy("detected_at DESC").
		Limit(uint64(limit)).Offset(uint64(f.Offset)).ToSql()
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []entity.RejectionAnalysis
	for rows.Next() {
		a, err := scanRA(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	cQ, cArgs, err := database.PSQL.Select("COUNT(*)").From("rejection_analysis_log").Where(conds).ToSql()
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, cQ, cArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *rejectionAnalysisRepo) CountByCategory(ctx context.Context, workspaceID string, since time.Time) (map[string]int, error) {
	ctx, span := r.tracer.Start(ctx, "rejection_analysis.repository.CountByCategory")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, `
        SELECT rejection_category, COUNT(*)
          FROM rejection_analysis_log
         WHERE workspace_id::text = $1
           AND detected_at >= $2
         GROUP BY rejection_category
         ORDER BY COUNT(*) DESC`,
		workspaceID, since,
	)
	if err != nil {
		return nil, fmt.Errorf("aggregate: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var cat string
		var n int
		if err := rows.Scan(&cat, &n); err != nil {
			return nil, err
		}
		out[cat] = n
	}
	return out, rows.Err()
}
