package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// RevenueTargetRepository describes data access for revenue_targets.
type RevenueTargetRepository interface {
	List(ctx context.Context, workspaceID string) ([]entity.RevenueTarget, error)
	Upsert(ctx context.Context, target entity.RevenueTarget) error
}

type revenueTargetRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

// NewRevenueTargetRepo creates a RevenueTargetRepository backed by PostgreSQL.
func NewRevenueTargetRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) RevenueTargetRepository {
	return &revenueTargetRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *revenueTargetRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

func (r *revenueTargetRepo) List(ctx context.Context, workspaceID string) ([]entity.RevenueTarget, error) {
	ctx, span := r.tracer.Start(ctx, "revenue_target.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q := `SELECT id, workspace_id, year, month, target_amount, created_by, created_at, updated_at
		FROM revenue_targets WHERE workspace_id = $1 ORDER BY year DESC, month DESC`

	rows, err := r.db.QueryContext(ctx, q, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("revenue target list: %w", err)
	}
	defer rows.Close()

	var targets []entity.RevenueTarget
	for rows.Next() {
		var t entity.RevenueTarget
		if err := rows.Scan(&t.ID, &t.WorkspaceID, &t.Year, &t.Month, &t.TargetAmount, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("revenue target scan: %w", err)
		}
		targets = append(targets, t)
	}
	if targets == nil {
		targets = []entity.RevenueTarget{}
	}
	return targets, rows.Err()
}

func (r *revenueTargetRepo) Upsert(ctx context.Context, target entity.RevenueTarget) error {
	ctx, span := r.tracer.Start(ctx, "revenue_target.repository.Upsert")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q := `INSERT INTO revenue_targets (workspace_id, year, month, target_amount, created_by, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (workspace_id, year, month)
		DO UPDATE SET target_amount = EXCLUDED.target_amount, updated_at = NOW()`

	_, err := r.db.ExecContext(ctx, q, target.WorkspaceID, target.Year, target.Month, target.TargetAmount, target.CreatedBy)
	if err != nil {
		return fmt.Errorf("revenue target upsert: %w", err)
	}
	return nil
}
