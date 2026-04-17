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

// TemplateEditLogRepository is an INSERT-only log of template changes.
type TemplateEditLogRepository interface {
	Append(ctx context.Context, log *entity.TemplateEditLog) (*entity.TemplateEditLog, error)
	List(ctx context.Context, workspaceID string, filter entity.TemplateEditLogFilter) ([]entity.TemplateEditLog, error)
}

type templateEditLogRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewTemplateEditLogRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) TemplateEditLogRepository {
	return &templateEditLogRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *templateEditLogRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const templateEditLogColumns = "id::text, workspace_id::text, template_id, template_type, field, old_value, new_value, edited_by, edited_at"

func (r *templateEditLogRepo) Append(ctx context.Context, log *entity.TemplateEditLog) (*entity.TemplateEditLog, error) {
	ctx, span := r.tracer.Start(ctx, "template_edit_log.repository.Append")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Insert("template_edit_logs").
		Columns("workspace_id", "template_id", "template_type", "field", "old_value", "new_value", "edited_by").
		Values(log.WorkspaceID, log.TemplateID, log.TemplateType, log.Field, log.OldValue, log.NewValue, log.EditedBy).
		Suffix("RETURNING " + templateEditLogColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert: %w", err)
	}
	var out entity.TemplateEditLog
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&out.ID, &out.WorkspaceID, &out.TemplateID, &out.TemplateType,
		&out.Field, &out.OldValue, &out.NewValue, &out.EditedBy, &out.EditedAt,
	); err != nil {
		return nil, fmt.Errorf("insert template_edit_log: %w", err)
	}
	return &out, nil
}

func (r *templateEditLogRepo) List(ctx context.Context, workspaceID string, filter entity.TemplateEditLogFilter) ([]entity.TemplateEditLog, error) {
	ctx, span := r.tracer.Start(ctx, "template_edit_log.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	where := sq.And{sq.Eq{"workspace_id": workspaceID}}
	if filter.TemplateID != "" {
		where = append(where, sq.Eq{"template_id": filter.TemplateID})
	}
	if filter.TemplateType != "" {
		where = append(where, sq.Eq{"template_type": filter.TemplateType})
	}
	if filter.Since != nil {
		where = append(where, sq.GtOrEq{"edited_at": *filter.Since})
	}

	limit := uint64(filter.Limit)
	if limit == 0 {
		limit = 50
	}

	query, args, err := database.PSQL.
		Select(templateEditLogColumns).
		From("template_edit_logs").
		Where(where).
		OrderBy("edited_at DESC").
		Limit(limit).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query template_edit_logs: %w", err)
	}
	defer rows.Close()

	var out []entity.TemplateEditLog
	for rows.Next() {
		var e entity.TemplateEditLog
		if err := rows.Scan(
			&e.ID, &e.WorkspaceID, &e.TemplateID, &e.TemplateType,
			&e.Field, &e.OldValue, &e.NewValue, &e.EditedBy, &e.EditedAt,
		); err != nil {
			return nil, fmt.Errorf("scan template_edit_log: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
