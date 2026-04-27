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
	"github.com/lib/pq"
	"github.com/rs/zerolog"
)

// WorkspaceRepository describes data access for workspaces.
type WorkspaceRepository interface {
	GetAll(ctx context.Context) ([]entity.Workspace, error)
	GetByID(ctx context.Context, id string) (*entity.Workspace, error)
	GetBySlug(ctx context.Context, slug string) (*entity.Workspace, error)
	ListForUser(ctx context.Context, userEmail string) ([]entity.Workspace, error)
	ListForMember(ctx context.Context, userEmail string) ([]entity.Workspace, error)
	Create(ctx context.Context, w *entity.Workspace) (*entity.Workspace, error)
	Update(ctx context.Context, id string, patch WorkspacePatch) (*entity.Workspace, error)
	SoftDelete(ctx context.Context, id string) error
}

// WorkspacePatch carries optional fields for partial workspace updates.
// A nil pointer means "leave unchanged"; settings are deep-merged at the usecase layer.
type WorkspacePatch struct {
	Name     *string
	Logo     *string
	Color    *string
	Plan     *string
	Settings map[string]interface{} // already-merged map; pass nil to skip
	IsActive *bool
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

const workspaceColumns = "id::text, slug, name, logo, color, plan, is_holding, member_ids::text[], settings, is_active, created_at, updated_at"

const workspaceColumnsQualified = "w.id::text, w.slug, w.name, w.logo, w.color, w.plan, w.is_holding, w.member_ids::text[], w.settings, w.is_active, w.created_at, w.updated_at"

func scanWorkspace(scanner interface {
	Scan(dest ...interface{}) error
}) (*entity.Workspace, error) {
	var (
		w           entity.Workspace
		settingsRaw []byte
	)
	err := scanner.Scan(
		&w.ID, &w.Slug, &w.Name, &w.Logo, &w.Color, &w.Plan,
		&w.IsHolding, pq.Array(&w.MemberIDs), &settingsRaw,
		&w.IsActive, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(settingsRaw) > 0 {
		if err := json.Unmarshal(settingsRaw, &w.Settings); err != nil {
			return nil, fmt.Errorf("unmarshal settings: %w", err)
		}
	}
	if w.Settings == nil {
		w.Settings = map[string]interface{}{}
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
		Where(sq.Eq{"is_active": true}).
		OrderBy("is_holding ASC", "name ASC").
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
		if errors.Is(err, sql.ErrNoRows) {
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query workspace: %w", err)
	}
	return w, nil
}

// ListForMember returns workspaces the given email has access to, *strictly*
// via membership — no holding-bypass. Two ACL paths are unioned:
//   - workspace_members.user_email (active row), or
//   - team_members.email + member_workspace_assignments (team-management ACL).
//
// Use this when the caller actually needs to gate access (FE allowlist, dashboard
// switcher). For the legacy permissive list, see ListForUser.
func (r *workspaceRepo) ListForMember(ctx context.Context, userEmail string) ([]entity.Workspace, error) {
	ctx, span := r.tracer.Start(ctx, "workspace.repository.ListForMember")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query := `
		SELECT DISTINCT ` + workspaceColumnsQualified + `
		FROM workspaces w
		WHERE w.is_active = TRUE
		  AND (
		    EXISTS (
		      SELECT 1 FROM workspace_members wm
		      WHERE wm.workspace_id = w.id
		        AND LOWER(wm.user_email) = LOWER($1)
		        AND wm.is_active = TRUE
		    )
		    OR EXISTS (
		      SELECT 1 FROM team_members tm
		      JOIN member_workspace_assignments mwa ON mwa.member_id = tm.id
		      WHERE LOWER(tm.email) = LOWER($1)
		        AND tm.status = 'active'
		        AND mwa.workspace_id = w.id
		    )
		  )
		ORDER BY w.is_holding ASC, w.name ASC
	`

	rows, err := r.db.QueryContext(ctx, query, userEmail)
	if err != nil {
		return nil, fmt.Errorf("query workspaces for member: %w", err)
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

// ListForUser returns workspaces the given user_email has active membership in,
// plus any holding workspace whose member_ids overlap.
func (r *workspaceRepo) ListForUser(ctx context.Context, userEmail string) ([]entity.Workspace, error) {
	ctx, span := r.tracer.Start(ctx, "workspace.repository.ListForUser")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select("DISTINCT " + workspaceColumnsQualified).
		From("workspaces w").
		LeftJoin("workspace_members wm ON wm.workspace_id = w.id").
		Where(sq.And{
			sq.Eq{"w.is_active": true},
			sq.Or{
				sq.And{
					sq.Eq{"wm.user_email": userEmail},
					sq.Eq{"wm.is_active": true},
				},
				sq.Expr("w.is_holding = TRUE"),
			},
		}).
		OrderBy("w.is_holding ASC", "w.name ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query workspaces for user: %w", err)
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

func (r *workspaceRepo) Create(ctx context.Context, w *entity.Workspace) (*entity.Workspace, error) {
	ctx, span := r.tracer.Start(ctx, "workspace.repository.Create")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	settingsJSON, err := json.Marshal(coalesceSettings(w.Settings))
	if err != nil {
		return nil, fmt.Errorf("marshal settings: %w", err)
	}

	query, args, err := database.PSQL.
		Insert("workspaces").
		Columns("slug", "name", "logo", "color", "plan", "is_holding", "member_ids", "settings", "is_active").
		Values(w.Slug, w.Name, w.Logo, w.Color, w.Plan, w.IsHolding, pq.Array(w.MemberIDs), settingsJSON, true).
		Suffix("RETURNING " + workspaceColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert: %w", err)
	}

	out, err := scanWorkspace(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		return nil, fmt.Errorf("insert workspace: %w", err)
	}
	return out, nil
}

func (r *workspaceRepo) Update(ctx context.Context, id string, patch WorkspacePatch) (*entity.Workspace, error) {
	ctx, span := r.tracer.Start(ctx, "workspace.repository.Update")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	upd := database.PSQL.Update("workspaces").Where(sq.Expr("id::text = ?", id))
	dirty := false
	if patch.Name != nil {
		upd = upd.Set("name", *patch.Name)
		dirty = true
	}
	if patch.Logo != nil {
		upd = upd.Set("logo", *patch.Logo)
		dirty = true
	}
	if patch.Color != nil {
		upd = upd.Set("color", *patch.Color)
		dirty = true
	}
	if patch.Plan != nil {
		upd = upd.Set("plan", *patch.Plan)
		dirty = true
	}
	if patch.IsActive != nil {
		upd = upd.Set("is_active", *patch.IsActive)
		dirty = true
	}
	if patch.Settings != nil {
		raw, err := json.Marshal(patch.Settings)
		if err != nil {
			return nil, fmt.Errorf("marshal settings patch: %w", err)
		}
		upd = upd.Set("settings", raw)
		dirty = true
	}
	if !dirty {
		return r.GetByID(ctx, id)
	}

	query, args, err := upd.Suffix("RETURNING " + workspaceColumns).ToSql()
	if err != nil {
		return nil, fmt.Errorf("build update: %w", err)
	}

	out, err := scanWorkspace(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("update workspace: %w", err)
	}
	return out, nil
}

func (r *workspaceRepo) SoftDelete(ctx context.Context, id string) error {
	ctx, span := r.tracer.Start(ctx, "workspace.repository.SoftDelete")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "UPDATE workspaces SET is_active = FALSE WHERE id::text = $1", id); err != nil {
		return fmt.Errorf("soft delete workspace: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "UPDATE workspace_members SET is_active = FALSE WHERE workspace_id::text = $1", id); err != nil {
		return fmt.Errorf("cascade members: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func coalesceSettings(s map[string]interface{}) map[string]interface{} {
	if s == nil {
		return map[string]interface{}{}
	}
	return s
}
