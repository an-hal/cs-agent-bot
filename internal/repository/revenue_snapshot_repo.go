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

// RevenueSnapshotRepository describes data access for revenue_snapshots.
type RevenueSnapshotRepository interface {
	List(ctx context.Context, workspaceID string, months int) ([]entity.RevenueSnapshot, error)
	Upsert(ctx context.Context, snap entity.RevenueSnapshot) error
	RebuildFromInvoices(ctx context.Context, workspaceID string, monthsBack int) error
}

type revenueSnapshotRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

// NewRevenueSnapshotRepo creates a RevenueSnapshotRepository backed by PostgreSQL.
func NewRevenueSnapshotRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) RevenueSnapshotRepository {
	return &revenueSnapshotRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *revenueSnapshotRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

func (r *revenueSnapshotRepo) List(ctx context.Context, workspaceID string, months int) ([]entity.RevenueSnapshot, error) {
	ctx, span := r.tracer.Start(ctx, "revenue_snapshot.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q := `SELECT id, workspace_id, year, month, revenue_actual, deals_won, deals_lost, computed_at
		FROM revenue_snapshots
		WHERE workspace_id = $1
		ORDER BY year ASC, month ASC
		LIMIT $2`

	rows, err := r.db.QueryContext(ctx, q, workspaceID, months)
	if err != nil {
		return nil, fmt.Errorf("revenue snapshot list: %w", err)
	}
	defer rows.Close()

	var snapshots []entity.RevenueSnapshot
	for rows.Next() {
		var s entity.RevenueSnapshot
		if err := rows.Scan(&s.ID, &s.WorkspaceID, &s.Year, &s.Month, &s.RevenueActual, &s.DealsWon, &s.DealsLost, &s.ComputedAt); err != nil {
			return nil, fmt.Errorf("revenue snapshot scan: %w", err)
		}
		snapshots = append(snapshots, s)
	}
	if snapshots == nil {
		snapshots = []entity.RevenueSnapshot{}
	}
	return snapshots, rows.Err()
}

func (r *revenueSnapshotRepo) Upsert(ctx context.Context, snap entity.RevenueSnapshot) error {
	ctx, span := r.tracer.Start(ctx, "revenue_snapshot.repository.Upsert")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q := `INSERT INTO revenue_snapshots (workspace_id, year, month, revenue_actual, deals_won, deals_lost, computed_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (workspace_id, year, month)
		DO UPDATE SET revenue_actual = EXCLUDED.revenue_actual, deals_won = EXCLUDED.deals_won,
			deals_lost = EXCLUDED.deals_lost, computed_at = NOW()`

	_, err := r.db.ExecContext(ctx, q, snap.WorkspaceID, snap.Year, snap.Month, snap.RevenueActual, snap.DealsWon, snap.DealsLost)
	if err != nil {
		return fmt.Errorf("revenue snapshot upsert: %w", err)
	}
	return nil
}

func (r *revenueSnapshotRepo) RebuildFromInvoices(ctx context.Context, workspaceID string, monthsBack int) error {
	ctx, span := r.tracer.Start(ctx, "revenue_snapshot.repository.RebuildFromInvoices")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q := `
		INSERT INTO revenue_snapshots (workspace_id, year, month, revenue_actual, deals_won, deals_lost, computed_at)
		SELECT
			$1 AS workspace_id,
			EXTRACT(YEAR FROM issue_date)::int   AS year,
			EXTRACT(MONTH FROM issue_date)::int   AS month,
			COALESCE(SUM(CASE WHEN payment_status = 'Lunas' THEN amount ELSE 0 END)::bigint, 0) AS revenue_actual,
			COUNT(*) FILTER (WHERE payment_status = 'Lunas')::int AS deals_won,
			COUNT(*) FILTER (WHERE payment_status IN ('Terlambat','Belum bayar'))::int AS deals_lost,
			NOW() AS computed_at
		FROM invoices
		WHERE workspace_id = $1
			AND issue_date >= NOW() - ($2 || ' months')::interval
		GROUP BY 1, 2, 3
		ON CONFLICT (workspace_id, year, month)
		DO UPDATE SET
			revenue_actual = EXCLUDED.revenue_actual,
			deals_won = EXCLUDED.deals_won,
			deals_lost = EXCLUDED.deals_lost,
			computed_at = NOW()
	`
	_, err := r.db.ExecContext(ctx, q, workspaceID, fmt.Sprintf("%d", monthsBack))
	if err != nil {
		return fmt.Errorf("revenue snapshot rebuild: %w", err)
	}
	return nil
}
