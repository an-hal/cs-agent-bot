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

// EmailTemplateRepository exposes CRUD for email_templates.
type EmailTemplateRepository interface {
	List(ctx context.Context, workspaceID string, filter entity.EmailTemplateFilter) ([]entity.EmailTemplate, error)
	Get(ctx context.Context, workspaceID, id string) (*entity.EmailTemplate, error)
	Create(ctx context.Context, t *entity.EmailTemplate) (*entity.EmailTemplate, error)
	Update(ctx context.Context, t *entity.EmailTemplate) (*entity.EmailTemplate, error)
	Delete(ctx context.Context, workspaceID, id string) error
}

type emailTemplateRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewEmailTemplateRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) EmailTemplateRepository {
	return &emailTemplateRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *emailTemplateRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const emailTemplateColumns = "id, workspace_id::text, name, role, category, status, subject, body_html, variables, updated_at, updated_by, created_at"

func scanEmailTemplate(scanner interface {
	Scan(dest ...interface{}) error
}) (*entity.EmailTemplate, error) {
	var t entity.EmailTemplate
	if err := scanner.Scan(
		&t.ID, &t.WorkspaceID, &t.Name, &t.Role, &t.Category, &t.Status,
		&t.Subject, &t.BodyHTML, pq.Array(&t.Variables),
		&t.UpdatedAt, &t.UpdatedBy, &t.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *emailTemplateRepo) List(ctx context.Context, workspaceID string, filter entity.EmailTemplateFilter) ([]entity.EmailTemplate, error) {
	ctx, span := r.tracer.Start(ctx, "email_template.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	where := sq.And{sq.Eq{"workspace_id": workspaceID}}
	if filter.Role != "" {
		where = append(where, sq.Eq{"role": filter.Role})
	}
	if filter.Category != "" {
		where = append(where, sq.Eq{"category": filter.Category})
	}
	if filter.Status != "" {
		where = append(where, sq.Eq{"status": filter.Status})
	}
	if filter.Search != "" {
		pattern := "%" + filter.Search + "%"
		where = append(where, sq.Or{
			sq.ILike{"id": pattern},
			sq.ILike{"name": pattern},
			sq.ILike{"subject": pattern},
		})
	}

	query, args, err := database.PSQL.
		Select(emailTemplateColumns).
		From("email_templates").
		Where(where).
		OrderBy("name ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query email_templates: %w", err)
	}
	defer rows.Close()

	var out []entity.EmailTemplate
	for rows.Next() {
		t, err := scanEmailTemplate(rows)
		if err != nil {
			return nil, fmt.Errorf("scan email_template: %w", err)
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

func (r *emailTemplateRepo) Get(ctx context.Context, workspaceID, id string) (*entity.EmailTemplate, error) {
	ctx, span := r.tracer.Start(ctx, "email_template.repository.Get")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(emailTemplateColumns).
		From("email_templates").
		Where(sq.Eq{"id": id, "workspace_id": workspaceID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	t, err := scanEmailTemplate(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan email_template: %w", err)
	}
	return t, nil
}

func (r *emailTemplateRepo) Create(ctx context.Context, t *entity.EmailTemplate) (*entity.EmailTemplate, error) {
	ctx, span := r.tracer.Start(ctx, "email_template.repository.Create")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Insert("email_templates").
		Columns("id", "workspace_id", "name", "role", "category", "status",
			"subject", "body_html", "variables", "updated_at", "updated_by").
		Values(t.ID, t.WorkspaceID, t.Name, t.Role, t.Category, t.Status,
			t.Subject, t.BodyHTML, pq.Array(t.Variables), t.UpdatedAt, t.UpdatedBy).
		Suffix("RETURNING " + emailTemplateColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert: %w", err)
	}
	out, err := scanEmailTemplate(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		return nil, fmt.Errorf("insert email_template: %w", err)
	}
	return out, nil
}

func (r *emailTemplateRepo) Update(ctx context.Context, t *entity.EmailTemplate) (*entity.EmailTemplate, error) {
	ctx, span := r.tracer.Start(ctx, "email_template.repository.Update")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	now := time.Now().UTC()
	t.UpdatedAt = &now

	query, args, err := database.PSQL.
		Update("email_templates").
		SetMap(map[string]interface{}{
			"name":       t.Name,
			"role":       t.Role,
			"category":   t.Category,
			"status":     t.Status,
			"subject":    t.Subject,
			"body_html":  t.BodyHTML,
			"variables":  pq.Array(t.Variables),
			"updated_at": t.UpdatedAt,
			"updated_by": t.UpdatedBy,
		}).
		Where(sq.Eq{"id": t.ID, "workspace_id": t.WorkspaceID}).
		Suffix("RETURNING " + emailTemplateColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build update: %w", err)
	}
	out, err := scanEmailTemplate(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("update email_template: %w", err)
	}
	return out, nil
}

func (r *emailTemplateRepo) Delete(ctx context.Context, workspaceID, id string) error {
	ctx, span := r.tracer.Start(ctx, "email_template.repository.Delete")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Delete("email_templates").
		Where(sq.Eq{"id": id, "workspace_id": workspaceID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build delete: %w", err)
	}
	res, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete email_template: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
