package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

type ManualActionRepository interface {
	Insert(ctx context.Context, m *entity.ManualAction) (*entity.ManualAction, error)
	GetByID(ctx context.Context, workspaceID, id string) (*entity.ManualAction, error)
	List(ctx context.Context, filter entity.ManualActionFilter) ([]entity.ManualAction, int64, error)
	Update(ctx context.Context, m *entity.ManualAction) (*entity.ManualAction, error)
	ListPastDue(ctx context.Context, cutoff time.Time, limit int) ([]entity.ManualAction, error)
}

type manualActionRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewManualActionRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) ManualActionRepository {
	return &manualActionRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *manualActionRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const maqColumns = `id::text, workspace_id::text, master_data_id::text,
    trigger_id, flow_category, role, assigned_to_user,
    suggested_draft, context_summary, status, priority,
    due_at, sent_at, COALESCE(sent_channel,''), COALESCE(actual_message,''),
    COALESCE(skipped_reason,''), created_at, updated_at`

func scanMAQ(scanner interface {
	Scan(dest ...interface{}) error
}) (*entity.ManualAction, error) {
	var m entity.ManualAction
	var raw []byte
	err := scanner.Scan(
		&m.ID, &m.WorkspaceID, &m.MasterDataID,
		&m.TriggerID, &m.FlowCategory, &m.Role, &m.AssignedToUser,
		&m.SuggestedDraft, &raw, &m.Status, &m.Priority,
		&m.DueAt, &m.SentAt, &m.SentChannel, &m.ActualMessage,
		&m.SkippedReason, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m.ContextSummary)
	}
	if m.ContextSummary == nil {
		m.ContextSummary = map[string]any{}
	}
	return &m, nil
}

func (r *manualActionRepo) Insert(ctx context.Context, m *entity.ManualAction) (*entity.ManualAction, error) {
	ctx, span := r.tracer.Start(ctx, "manual_action.repository.Insert")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	raw, err := json.Marshal(coalesceMap(m.ContextSummary))
	if err != nil {
		return nil, fmt.Errorf("marshal context_summary: %w", err)
	}

	query, args, err := database.PSQL.
		Insert("manual_action_queue").
		Columns(
			"workspace_id", "master_data_id",
			"trigger_id", "flow_category", "role", "assigned_to_user",
			"suggested_draft", "context_summary", "status", "priority", "due_at",
		).
		Values(
			m.WorkspaceID, m.MasterDataID,
			m.TriggerID, m.FlowCategory, m.Role, m.AssignedToUser,
			m.SuggestedDraft, string(raw), defaultIfBlank(m.Status, entity.ManualActionStatusPending),
			defaultIfBlank(m.Priority, entity.ManualActionPriorityP2), m.DueAt,
		).
		Suffix("RETURNING " + maqColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert: %w", err)
	}
	out, err := scanMAQ(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		return nil, fmt.Errorf("insert manual action: %w", err)
	}
	return out, nil
}

func (r *manualActionRepo) GetByID(ctx context.Context, workspaceID, id string) (*entity.ManualAction, error) {
	ctx, span := r.tracer.Start(ctx, "manual_action.repository.GetByID")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(maqColumns).From("manual_action_queue").
		Where(sq.And{
			sq.Expr("workspace_id::text = ?", workspaceID),
			sq.Expr("id::text = ?", id),
		}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select: %w", err)
	}
	out, err := scanMAQ(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query manual action: %w", err)
	}
	return out, nil
}

func (r *manualActionRepo) List(ctx context.Context, filter entity.ManualActionFilter) ([]entity.ManualAction, int64, error) {
	ctx, span := r.tracer.Start(ctx, "manual_action.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	conds := sq.And{sq.Expr("workspace_id::text = ?", filter.WorkspaceID)}
	if filter.Status != "" {
		conds = append(conds, sq.Eq{"status": filter.Status})
	}
	if filter.AssignedToUser != "" {
		conds = append(conds, sq.Eq{"assigned_to_user": filter.AssignedToUser})
	}
	if filter.Role != "" {
		conds = append(conds, sq.Eq{"role": filter.Role})
	}
	if filter.Priority != "" {
		conds = append(conds, sq.Eq{"priority": filter.Priority})
	}
	if filter.FlowCategory != "" {
		conds = append(conds, sq.Eq{"flow_category": filter.FlowCategory})
	}

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	query, args, err := database.PSQL.
		Select(maqColumns).From("manual_action_queue").
		Where(conds).
		OrderBy("priority ASC", "due_at ASC").
		Limit(uint64(limit)).
		Offset(uint64(filter.Offset)).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build list: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query manual actions: %w", err)
	}
	defer rows.Close()

	var out []entity.ManualAction
	for rows.Next() {
		m, err := scanMAQ(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan manual action: %w", err)
		}
		out = append(out, *m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	countQuery, countArgs, err := database.PSQL.
		Select("COUNT(*)").From("manual_action_queue").Where(conds).ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build count: %w", err)
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count manual actions: %w", err)
	}
	return out, total, nil
}

func (r *manualActionRepo) Update(ctx context.Context, m *entity.ManualAction) (*entity.ManualAction, error) {
	ctx, span := r.tracer.Start(ctx, "manual_action.repository.Update")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q := database.PSQL.
		Update("manual_action_queue").
		Set("status", m.Status).
		Set("updated_at", sq.Expr("NOW()"))

	// Optional fields — set only if non-empty to avoid wiping prior values.
	if m.SentAt != nil {
		q = q.Set("sent_at", *m.SentAt)
	}
	if m.SentChannel != "" {
		q = q.Set("sent_channel", m.SentChannel)
	}
	if m.ActualMessage != "" {
		q = q.Set("actual_message", m.ActualMessage)
	}
	if m.SkippedReason != "" {
		q = q.Set("skipped_reason", m.SkippedReason)
	}

	query, args, err := q.Where(sq.And{
		sq.Expr("workspace_id::text = ?", m.WorkspaceID),
		sq.Expr("id::text = ?", m.ID),
	}).Suffix("RETURNING " + maqColumns).ToSql()
	if err != nil {
		return nil, fmt.Errorf("build update: %w", err)
	}
	out, err := scanMAQ(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("manual action not found")
		}
		return nil, fmt.Errorf("update manual action: %w", err)
	}
	return out, nil
}

func (r *manualActionRepo) ListPastDue(ctx context.Context, cutoff time.Time, limit int) ([]entity.ManualAction, error) {
	ctx, span := r.tracer.Start(ctx, "manual_action.repository.ListPastDue")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if limit <= 0 {
		limit = 200
	}
	query, args, err := database.PSQL.
		Select(maqColumns).From("manual_action_queue").
		Where(sq.And{
			sq.Eq{"status": entity.ManualActionStatusPending},
			sq.Lt{"due_at": cutoff},
		}).
		OrderBy("due_at ASC").
		Limit(uint64(limit)).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build past-due: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query past-due: %w", err)
	}
	defer rows.Close()

	var out []entity.ManualAction
	for rows.Next() {
		m, err := scanMAQ(rows)
		if err != nil {
			return nil, fmt.Errorf("scan past-due: %w", err)
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

func defaultIfBlank(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
