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

type ReactivationRepository interface {
	// Triggers
	UpsertTrigger(ctx context.Context, t *entity.ReactivationTrigger) (*entity.ReactivationTrigger, error)
	GetTrigger(ctx context.Context, workspaceID, id string) (*entity.ReactivationTrigger, error)
	ListTriggers(ctx context.Context, workspaceID string, activeOnly bool) ([]entity.ReactivationTrigger, error)
	DeleteTrigger(ctx context.Context, workspaceID, id string) error
	// Events
	RecordEvent(ctx context.Context, e *entity.ReactivationEvent) (*entity.ReactivationEvent, error)
	ListEventsForMasterData(ctx context.Context, workspaceID, masterDataID string, limit int) ([]entity.ReactivationEvent, error)
	CountRecentForClient(ctx context.Context, workspaceID, masterDataID, triggerCode string, within time.Duration) (int, error)
}

type reactivationRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewReactivationRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) ReactivationRepository {
	return &reactivationRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *reactivationRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const rtColumns = `id::text, workspace_id::text, code, name, description, condition,
    template_code, is_active, created_by, created_at, updated_at`

func scanRT(s interface{ Scan(dest ...interface{}) error }) (*entity.ReactivationTrigger, error) {
	var t entity.ReactivationTrigger
	err := s.Scan(&t.ID, &t.WorkspaceID, &t.Code, &t.Name, &t.Description, &t.Condition,
		&t.TemplateCode, &t.IsActive, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

const reColumns = `id::text, workspace_id::text, trigger_id::text, master_data_id::text,
    fired_at, outcome, note`

func scanRE(s interface{ Scan(dest ...interface{}) error }) (*entity.ReactivationEvent, error) {
	var e entity.ReactivationEvent
	err := s.Scan(&e.ID, &e.WorkspaceID, &e.TriggerID, &e.MasterDataID,
		&e.FiredAt, &e.Outcome, &e.Note)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *reactivationRepo) UpsertTrigger(ctx context.Context, t *entity.ReactivationTrigger) (*entity.ReactivationTrigger, error) {
	ctx, span := r.tracer.Start(ctx, "reactivation.repository.UpsertTrigger")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query := `
        INSERT INTO reactivation_triggers
            (workspace_id, code, name, description, condition, template_code, is_active, created_by)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        ON CONFLICT (workspace_id, code) DO UPDATE SET
            name = EXCLUDED.name,
            description = EXCLUDED.description,
            condition = EXCLUDED.condition,
            template_code = EXCLUDED.template_code,
            is_active = EXCLUDED.is_active,
            updated_at = NOW()
        RETURNING ` + rtColumns
	return scanRT(r.db.QueryRowContext(ctx, query,
		t.WorkspaceID, t.Code, t.Name, t.Description, t.Condition, t.TemplateCode, t.IsActive, t.CreatedBy,
	))
}

func (r *reactivationRepo) GetTrigger(ctx context.Context, workspaceID, id string) (*entity.ReactivationTrigger, error) {
	ctx, span := r.tracer.Start(ctx, "reactivation.repository.GetTrigger")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q, args, err := database.PSQL.Select(rtColumns).From("reactivation_triggers").
		Where(sq.And{sq.Expr("workspace_id::text = ?", workspaceID), sq.Expr("id::text = ?", id)}).ToSql()
	if err != nil {
		return nil, err
	}
	out, err := scanRT(r.db.QueryRowContext(ctx, q, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return out, nil
}

func (r *reactivationRepo) ListTriggers(ctx context.Context, workspaceID string, activeOnly bool) ([]entity.ReactivationTrigger, error) {
	ctx, span := r.tracer.Start(ctx, "reactivation.repository.ListTriggers")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	conds := sq.And{sq.Expr("workspace_id::text = ?", workspaceID)}
	if activeOnly {
		conds = append(conds, sq.Eq{"is_active": true})
	}
	q, args, err := database.PSQL.Select(rtColumns).From("reactivation_triggers").
		Where(conds).OrderBy("code ASC").ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []entity.ReactivationTrigger
	for rows.Next() {
		t, err := scanRT(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

func (r *reactivationRepo) DeleteTrigger(ctx context.Context, workspaceID, id string) error {
	ctx, span := r.tracer.Start(ctx, "reactivation.repository.DeleteTrigger")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	res, err := r.db.ExecContext(ctx,
		"DELETE FROM reactivation_triggers WHERE workspace_id::text = $1 AND id::text = $2",
		workspaceID, id,
	)
	if err != nil {
		return fmt.Errorf("delete trigger: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("trigger not found")
	}
	return nil
}

func (r *reactivationRepo) RecordEvent(ctx context.Context, e *entity.ReactivationEvent) (*entity.ReactivationEvent, error) {
	ctx, span := r.tracer.Start(ctx, "reactivation.repository.RecordEvent")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.Insert("reactivation_events").
		Columns("workspace_id", "trigger_id", "master_data_id", "outcome", "note").
		Values(e.WorkspaceID, e.TriggerID, e.MasterDataID, defaultIfBlank(e.Outcome, entity.ReactivationOutcomeSent), e.Note).
		Suffix("RETURNING " + reColumns).ToSql()
	if err != nil {
		return nil, err
	}
	return scanRE(r.db.QueryRowContext(ctx, query, args...))
}

func (r *reactivationRepo) ListEventsForMasterData(ctx context.Context, workspaceID, masterDataID string, limit int) ([]entity.ReactivationEvent, error) {
	ctx, span := r.tracer.Start(ctx, "reactivation.repository.ListEventsForMasterData")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q, args, err := database.PSQL.Select(reColumns).From("reactivation_events").
		Where(sq.And{
			sq.Expr("workspace_id::text = ?", workspaceID),
			sq.Expr("master_data_id::text = ?", masterDataID),
		}).OrderBy("fired_at DESC").Limit(uint64(limit)).ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []entity.ReactivationEvent
	for rows.Next() {
		e, err := scanRE(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

func (r *reactivationRepo) CountRecentForClient(ctx context.Context, workspaceID, masterDataID, triggerCode string, within time.Duration) (int, error) {
	ctx, span := r.tracer.Start(ctx, "reactivation.repository.CountRecentForClient")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if within <= 0 {
		within = 30 * 24 * time.Hour
	}
	since := time.Now().UTC().Add(-within)
	var n int
	err := r.db.QueryRowContext(ctx, `
        SELECT COUNT(*)
          FROM reactivation_events e
          JOIN reactivation_triggers t ON t.id = e.trigger_id
         WHERE e.workspace_id::text = $1
           AND e.master_data_id::text = $2
           AND t.code = $3
           AND e.fired_at >= $4`,
		workspaceID, masterDataID, triggerCode, since,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count recent reactivations: %w", err)
	}
	return n, nil
}
