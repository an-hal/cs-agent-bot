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

// TemplateVariableRepository exposes read access to template_variables.
type TemplateVariableRepository interface {
	List(ctx context.Context, workspaceID string) ([]entity.TemplateVariable, error)
}

type templateVariableRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewTemplateVariableRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) TemplateVariableRepository {
	return &templateVariableRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *templateVariableRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const templateVariableColumns = "id::text, workspace_id::text, variable_key, display_label, source_type, source_field, description, example_value, created_at"

func (r *templateVariableRepo) List(ctx context.Context, workspaceID string) ([]entity.TemplateVariable, error) {
	ctx, span := r.tracer.Start(ctx, "template_variable.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(templateVariableColumns).
		From("template_variables").
		Where(sq.Eq{"workspace_id": workspaceID}).
		OrderBy("variable_key ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query template_variables: %w", err)
	}
	defer rows.Close()

	var out []entity.TemplateVariable
	for rows.Next() {
		var v entity.TemplateVariable
		if err := rows.Scan(
			&v.ID, &v.WorkspaceID, &v.VariableKey, &v.DisplayLabel,
			&v.SourceType, &v.SourceField, &v.Description, &v.ExampleValue, &v.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan template_variable: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
