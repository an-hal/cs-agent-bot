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
	"github.com/lib/pq"
	"github.com/rs/zerolog"
)

// MessageTemplateRepository exposes CRUD for message_templates.
type MessageTemplateRepository interface {
	List(ctx context.Context, workspaceID string, filter entity.MessageTemplateFilter) ([]entity.MessageTemplate, error)
	Get(ctx context.Context, workspaceID, id string) (*entity.MessageTemplate, error)
	Create(ctx context.Context, t *entity.MessageTemplate) (*entity.MessageTemplate, error)
	Update(ctx context.Context, t *entity.MessageTemplate) (*entity.MessageTemplate, error)
	Delete(ctx context.Context, workspaceID, id string) error
}

type messageTemplateRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewMessageTemplateRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) MessageTemplateRepository {
	return &messageTemplateRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *messageTemplateRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const messageTemplateColumns = "id, workspace_id::text, trigger_id, phase, phase_label, channel, role, category, action, timing, condition, message, variables, stop_if, sent_flag, priority, updated_at, updated_by, created_at"

func scanMessageTemplate(scanner interface {
	Scan(dest ...interface{}) error
}) (*entity.MessageTemplate, error) {
	var t entity.MessageTemplate
	if err := scanner.Scan(
		&t.ID, &t.WorkspaceID, &t.TriggerID, &t.Phase, &t.PhaseLabel,
		&t.Channel, &t.Role, &t.Category, &t.Action, &t.Timing,
		&t.Condition, &t.Message, pq.Array(&t.Variables), &t.StopIf,
		&t.SentFlag, &t.Priority, &t.UpdatedAt, &t.UpdatedBy, &t.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *messageTemplateRepo) List(ctx context.Context, workspaceID string, filter entity.MessageTemplateFilter) ([]entity.MessageTemplate, error) {
	ctx, span := r.tracer.Start(ctx, "message_template.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	where := sq.And{sq.Eq{"workspace_id": workspaceID}}
	if filter.Role != "" {
		where = append(where, sq.Eq{"role": filter.Role})
	}
	if filter.Channel != "" {
		where = append(where, sq.Eq{"channel": filter.Channel})
	}
	if filter.Category != "" {
		where = append(where, sq.Eq{"category": filter.Category})
	}
	if len(filter.Phases) > 0 {
		where = append(where, sq.Eq{"phase": filter.Phases})
	}
	if filter.Search != "" {
		pattern := "%" + filter.Search + "%"
		where = append(where, sq.Or{
			sq.ILike{"id": pattern},
			sq.ILike{"action": pattern},
			sq.ILike{"message": pattern},
		})
	}

	query, args, err := database.PSQL.
		Select(messageTemplateColumns).
		From("message_templates").
		Where(where).
		OrderBy("phase ASC", "id ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query message_templates: %w", err)
	}
	defer rows.Close()

	var out []entity.MessageTemplate
	for rows.Next() {
		t, err := scanMessageTemplate(rows)
		if err != nil {
			return nil, fmt.Errorf("scan message_template: %w", err)
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

func (r *messageTemplateRepo) Get(ctx context.Context, workspaceID, id string) (*entity.MessageTemplate, error) {
	ctx, span := r.tracer.Start(ctx, "message_template.repository.Get")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(messageTemplateColumns).
		From("message_templates").
		Where(sq.Eq{"id": id, "workspace_id": workspaceID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	t, err := scanMessageTemplate(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan message_template: %w", err)
	}
	return t, nil
}

func (r *messageTemplateRepo) Create(ctx context.Context, t *entity.MessageTemplate) (*entity.MessageTemplate, error) {
	ctx, span := r.tracer.Start(ctx, "message_template.repository.Create")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Insert("message_templates").
		Columns("id", "workspace_id", "trigger_id", "phase", "phase_label", "channel", "role", "category",
			"action", "timing", "condition", "message", "variables", "stop_if", "sent_flag", "priority",
			"updated_at", "updated_by").
		Values(t.ID, t.WorkspaceID, t.TriggerID, t.Phase, t.PhaseLabel, t.Channel, t.Role, t.Category,
			t.Action, t.Timing, t.Condition, t.Message, pq.Array(t.Variables), t.StopIf, t.SentFlag, t.Priority,
			t.UpdatedAt, t.UpdatedBy).
		Suffix("RETURNING " + messageTemplateColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert: %w", err)
	}
	out, err := scanMessageTemplate(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		return nil, fmt.Errorf("insert message_template: %w", err)
	}
	return out, nil
}

func (r *messageTemplateRepo) Update(ctx context.Context, t *entity.MessageTemplate) (*entity.MessageTemplate, error) {
	ctx, span := r.tracer.Start(ctx, "message_template.repository.Update")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	now := time.Now().UTC()
	t.UpdatedAt = &now

	query, args, err := database.PSQL.
		Update("message_templates").
		SetMap(map[string]interface{}{
			"trigger_id":  t.TriggerID,
			"phase":       t.Phase,
			"phase_label": t.PhaseLabel,
			"channel":     t.Channel,
			"role":        t.Role,
			"category":    t.Category,
			"action":      t.Action,
			"timing":      t.Timing,
			"condition":   t.Condition,
			"message":     t.Message,
			"variables":   pq.Array(t.Variables),
			"stop_if":     t.StopIf,
			"sent_flag":   t.SentFlag,
			"priority":    t.Priority,
			"updated_at":  t.UpdatedAt,
			"updated_by":  t.UpdatedBy,
		}).
		Where(sq.Eq{"id": t.ID, "workspace_id": t.WorkspaceID}).
		Suffix("RETURNING " + messageTemplateColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build update: %w", err)
	}
	out, err := scanMessageTemplate(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("update message_template: %w", err)
	}
	return out, nil
}

func (r *messageTemplateRepo) Delete(ctx context.Context, workspaceID, id string) error {
	ctx, span := r.tracer.Start(ctx, "message_template.repository.Delete")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Delete("message_templates").
		Where(sq.Eq{"id": id, "workspace_id": workspaceID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build delete: %w", err)
	}
	res, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete message_template: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
