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
	"github.com/lib/pq"
	"github.com/rs/zerolog"
)

type WorkspaceRepository interface {
	GetAll(ctx context.Context) ([]entity.Workspace, error)
	GetByID(ctx context.Context, id string) (*entity.Workspace, error)
	GetBySlug(ctx context.Context, slug string) (*entity.Workspace, error)
}

type workspaceRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewWorkspaceRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) WorkspaceRepository {
	return &workspaceRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *workspaceRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const workspaceColumns = "id::text, slug, name, logo, color, plan, is_holding, member_ids::text[], created_at"

func scanWorkspace(scanner interface {
	Scan(dest ...interface{}) error
}) (*entity.Workspace, error) {
	var w entity.Workspace
	err := scanner.Scan(&w.ID, &w.Slug, &w.Name, &w.Logo, &w.Color, &w.Plan, &w.IsHolding, pq.Array(&w.MemberIDs), &w.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *workspaceRepo) GetAll(ctx context.Context) ([]entity.Workspace, error) {
	ctx, span := r.tracer.Start(ctx, "workspace.repository.GetAll")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(workspaceColumns).
		From("workspaces").
		OrderBy("created_at").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query workspaces: %w", err)
	}
	defer rows.Close()

	var workspaces []entity.Workspace
	for rows.Next() {
		w, err := scanWorkspace(rows)
		if err != nil {
			return nil, fmt.Errorf("scan workspace: %w", err)
		}
		workspaces = append(workspaces, *w)
	}
	return workspaces, rows.Err()
}

func (r *workspaceRepo) GetByID(ctx context.Context, id string) (*entity.Workspace, error) {
	ctx, span := r.tracer.Start(ctx, "workspace.repository.GetByID")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(workspaceColumns).
		From("workspaces").
		Where(sq.Expr("id::text = ?", id)).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	w, err := scanWorkspace(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query workspace: %w", err)
	}
	return w, nil
}

func (r *workspaceRepo) GetBySlug(ctx context.Context, slug string) (*entity.Workspace, error) {
	ctx, span := r.tracer.Start(ctx, "workspace.repository.GetBySlug")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(workspaceColumns).
		From("workspaces").
		Where(sq.Eq{"slug": slug}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	w, err := scanWorkspace(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query workspace: %w", err)
	}
	return w, nil
}
