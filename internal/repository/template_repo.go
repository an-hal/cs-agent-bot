package repository

import (
	"context"
	"database/sql"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
)

// TemplateRepository defines the interface for template operations.
type TemplateRepository interface {
	GetTemplate(ctx context.Context, templateID string) (*entity.Template, error)
	GetTemplatesByCategory(ctx context.Context, category string) ([]entity.Template, error)
	GetActiveTemplate(ctx context.Context, templateID string) (*entity.Template, error)
}

type templateRepo struct {
	DB           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
}

// NewTemplateRepo creates a new instance of TemplateRepository.
func NewTemplateRepo(db *sql.DB, queryTimeout time.Duration, tracer tracer.Tracer) TemplateRepository {
	return &templateRepo{
		DB:           db,
		queryTimeout: queryTimeout,
		tracer:       tracer,
	}
}

func (r *templateRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

// GetTemplate retrieves a template by ID.
func (r *templateRepo) GetTemplate(ctx context.Context, templateID string) (*entity.Template, error) {
	ctx, span := r.tracer.Start(ctx, "template.repository.GetTemplate")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select("template_id", "template_name", "template_content", "template_category", "language", "active", "created_at", "updated_at").
		From("templates").
		Where(sq.Eq{"template_id": templateID}).
		ToSql()
	if err != nil {
		return nil, err
	}

	var t entity.Template
	err = r.DB.QueryRowContext(ctx, query, args...).Scan(
		&t.TemplateID, &t.TemplateName, &t.TemplateContent, &t.TemplateCategory, &t.Language, &t.Active, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &t, nil
}

// GetTemplatesByCategory retrieves all templates in a category.
func (r *templateRepo) GetTemplatesByCategory(ctx context.Context, category string) ([]entity.Template, error) {
	ctx, span := r.tracer.Start(ctx, "template.repository.GetTemplatesByCategory")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select("template_id", "template_name", "template_content", "template_category", "language", "active", "created_at", "updated_at").
		From("templates").
		Where(sq.And{
			sq.Eq{"template_category": category},
			sq.Eq{"active": true},
		}).
		OrderBy("template_id").
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []entity.Template
	for rows.Next() {
		var t entity.Template
		err := rows.Scan(
			&t.TemplateID, &t.TemplateName, &t.TemplateContent, &t.TemplateCategory, &t.Language, &t.Active, &t.CreatedAt, &t.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		templates = append(templates, t)
	}

	return templates, nil
}

// GetActiveTemplate retrieves an active template by ID.
func (r *templateRepo) GetActiveTemplate(ctx context.Context, templateID string) (*entity.Template, error) {
	ctx, span := r.tracer.Start(ctx, "template.repository.GetActiveTemplate")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select("template_id", "template_name", "template_content", "template_category", "language", "active", "created_at", "updated_at").
		From("templates").
		Where(sq.And{
			sq.Eq{"template_id": templateID},
			sq.Eq{"active": true},
		}).
		ToSql()
	if err != nil {
		return nil, err
	}

	var t entity.Template
	err = r.DB.QueryRowContext(ctx, query, args...).Scan(
		&t.TemplateID, &t.TemplateName, &t.TemplateContent, &t.TemplateCategory, &t.Language, &t.Active, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &t, nil
}
