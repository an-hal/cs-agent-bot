package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
)

// TemplateRepository defines the interface for template operations.
type TemplateRepository interface {
	GetTemplate(ctx context.Context, templateID string) (*entity.Template, error)
	GetTemplatesByCategory(ctx context.Context, category string) ([]entity.Template, error)
	GetActiveTemplate(ctx context.Context, templateID string) (*entity.Template, error)
	GetAllPaginated(ctx context.Context, filter entity.TemplateFilter, p pagination.Params) ([]entity.Template, int64, error)
	UpdateFields(ctx context.Context, templateID string, fields map[string]interface{}) error
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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate template rows: %w", err)
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

// GetAllPaginated returns paginated templates with optional filters.
func (r *templateRepo) GetAllPaginated(ctx context.Context, filter entity.TemplateFilter, p pagination.Params) ([]entity.Template, int64, error) {
	ctx, span := r.tracer.Start(ctx, "template.repository.GetAllPaginated")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	where := sq.And{}
	if filter.Category != "" {
		where = append(where, sq.Eq{"template_category": filter.Category})
	}
	if filter.Language != "" {
		where = append(where, sq.Eq{"language": filter.Language})
	}
	if filter.Active != nil {
		where = append(where, sq.Eq{"active": *filter.Active})
	}

	// Count
	countBuilder := database.PSQL.Select("COUNT(*)").From("templates")
	if len(where) > 0 {
		countBuilder = countBuilder.Where(where)
	}
	countQ, countArgs, err := countBuilder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build count query: %w", err)
	}
	var total int64
	if scanErr := r.DB.QueryRowContext(ctx, countQ, countArgs...).Scan(&total); scanErr != nil {
		return nil, 0, fmt.Errorf("count templates: %w", scanErr)
	}

	// Data
	dataBuilder := database.PSQL.
		Select("template_id", "template_name", "template_content", "template_category", "language", "active", "created_at", "updated_at").
		From("templates").
		OrderBy("template_category, template_id").
		Limit(uint64(p.Limit)).
		Offset(uint64(p.Offset))
	if len(where) > 0 {
		dataBuilder = dataBuilder.Where(where)
	}
	dataQ, dataArgs, err := dataBuilder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build data query: %w", err)
	}

	rows, err := r.DB.QueryContext(ctx, dataQ, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query templates: %w", err)
	}
	defer rows.Close()

	var templates []entity.Template
	for rows.Next() {
		var t entity.Template
		if err := rows.Scan(
			&t.TemplateID, &t.TemplateName, &t.TemplateContent, &t.TemplateCategory, &t.Language, &t.Active, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan template: %w", err)
		}
		templates = append(templates, t)
	}
	return templates, total, rows.Err()
}

// UpdateFields updates specific fields on a template.
func (r *templateRepo) UpdateFields(ctx context.Context, templateID string, fields map[string]interface{}) error {
	ctx, span := r.tracer.Start(ctx, "template.repository.UpdateFields")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	fields["updated_at"] = time.Now()

	query, args, err := database.PSQL.
		Update("templates").
		SetMap(fields).
		Where(sq.Eq{"template_id": templateID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	if _, err = r.DB.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("update template: %w", err)
	}
	return nil
}
