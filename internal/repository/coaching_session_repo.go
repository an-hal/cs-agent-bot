package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

type CoachingSessionRepository interface {
	Insert(ctx context.Context, c *entity.CoachingSession) (*entity.CoachingSession, error)
	GetByID(ctx context.Context, workspaceID, id string) (*entity.CoachingSession, error)
	List(ctx context.Context, filter entity.CoachingSessionFilter) ([]entity.CoachingSession, int64, error)
	Update(ctx context.Context, c *entity.CoachingSession) (*entity.CoachingSession, error)
	Delete(ctx context.Context, workspaceID, id string) error
}

type coachingSessionRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewCoachingSessionRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) CoachingSessionRepository {
	return &coachingSessionRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *coachingSessionRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const coachColumns = `id::text, workspace_id::text, bd_email, coach_email,
    COALESCE(master_data_id::text,''), COALESCE(claude_extraction_id::text,''),
    session_type, session_date, bants_clarity_score, discovery_depth_score,
    tone_fit_score, next_step_clarity_score, overall_score,
    strengths, improvements, action_items, status, created_at, updated_at`

func scanCS(s interface{ Scan(dest ...interface{}) error }) (*entity.CoachingSession, error) {
	var c entity.CoachingSession
	err := s.Scan(&c.ID, &c.WorkspaceID, &c.BDEmail, &c.CoachEmail,
		&c.MasterDataID, &c.ClaudeExtractionID,
		&c.SessionType, &c.SessionDate,
		&c.BANTSClarityScore, &c.DiscoveryDepthScore,
		&c.ToneFitScore, &c.NextStepClarityScore, &c.OverallScore,
		&c.Strengths, &c.Improvements, &c.ActionItems,
		&c.Status, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *coachingSessionRepo) Insert(ctx context.Context, c *entity.CoachingSession) (*entity.CoachingSession, error) {
	ctx, span := r.tracer.Start(ctx, "coaching_session.repository.Insert")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	var mdParam, ceParam any
	if c.MasterDataID != "" {
		mdParam = c.MasterDataID
	}
	if c.ClaudeExtractionID != "" {
		ceParam = c.ClaudeExtractionID
	}

	q := `
        INSERT INTO coaching_sessions
            (workspace_id, bd_email, coach_email, master_data_id, claude_extraction_id,
             session_type, bants_clarity_score, discovery_depth_score,
             tone_fit_score, next_step_clarity_score, overall_score,
             strengths, improvements, action_items, status)
        VALUES ($1, $2, $3, $4::uuid, $5::uuid, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
        RETURNING ` + coachColumns
	return scanCS(r.db.QueryRowContext(ctx, q,
		c.WorkspaceID, c.BDEmail, c.CoachEmail, mdParam, ceParam,
		defaultIfBlank(c.SessionType, entity.CoachingTypePeerReview),
		c.BANTSClarityScore, c.DiscoveryDepthScore,
		c.ToneFitScore, c.NextStepClarityScore, c.OverallScore,
		c.Strengths, c.Improvements, c.ActionItems,
		defaultIfBlank(c.Status, entity.CoachingStatusDraft),
	))
}

func (r *coachingSessionRepo) GetByID(ctx context.Context, workspaceID, id string) (*entity.CoachingSession, error) {
	ctx, span := r.tracer.Start(ctx, "coaching_session.repository.GetByID")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q, args, err := database.PSQL.Select(coachColumns).From("coaching_sessions").
		Where(sq.And{sq.Expr("workspace_id::text = ?", workspaceID), sq.Expr("id::text = ?", id)}).ToSql()
	if err != nil {
		return nil, err
	}
	out, err := scanCS(r.db.QueryRowContext(ctx, q, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return out, nil
}

func (r *coachingSessionRepo) List(ctx context.Context, f entity.CoachingSessionFilter) ([]entity.CoachingSession, int64, error) {
	ctx, span := r.tracer.Start(ctx, "coaching_session.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	conds := sq.And{sq.Expr("workspace_id::text = ?", f.WorkspaceID)}
	if f.BDEmail != "" {
		conds = append(conds, sq.Eq{"bd_email": f.BDEmail})
	}
	if f.CoachEmail != "" {
		conds = append(conds, sq.Eq{"coach_email": f.CoachEmail})
	}
	if f.Status != "" {
		conds = append(conds, sq.Eq{"status": f.Status})
	}
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q, args, err := database.PSQL.Select(coachColumns).From("coaching_sessions").
		Where(conds).OrderBy("session_date DESC").
		Limit(uint64(limit)).Offset(uint64(f.Offset)).ToSql()
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []entity.CoachingSession
	for rows.Next() {
		c, err := scanCS(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	cQ, cArgs, err := database.PSQL.Select("COUNT(*)").From("coaching_sessions").Where(conds).ToSql()
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, cQ, cArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *coachingSessionRepo) Update(ctx context.Context, c *entity.CoachingSession) (*entity.CoachingSession, error) {
	ctx, span := r.tracer.Start(ctx, "coaching_session.repository.Update")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q := `
        UPDATE coaching_sessions
           SET bants_clarity_score      = $1,
               discovery_depth_score    = $2,
               tone_fit_score           = $3,
               next_step_clarity_score  = $4,
               overall_score            = $5,
               strengths                = $6,
               improvements             = $7,
               action_items             = $8,
               status                   = $9,
               updated_at               = NOW()
         WHERE workspace_id::text = $10 AND id::text = $11
         RETURNING ` + coachColumns
	return scanCS(r.db.QueryRowContext(ctx, q,
		c.BANTSClarityScore, c.DiscoveryDepthScore,
		c.ToneFitScore, c.NextStepClarityScore, c.OverallScore,
		c.Strengths, c.Improvements, c.ActionItems, c.Status,
		c.WorkspaceID, c.ID,
	))
}

func (r *coachingSessionRepo) Delete(ctx context.Context, workspaceID, id string) error {
	ctx, span := r.tracer.Start(ctx, "coaching_session.repository.Delete")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	res, err := r.db.ExecContext(ctx,
		"DELETE FROM coaching_sessions WHERE workspace_id::text = $1 AND id::text = $2",
		workspaceID, id,
	)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("session not found")
	}
	return nil
}
