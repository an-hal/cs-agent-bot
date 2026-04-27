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

// RolePermissionRepository is CRUD for the role/workspace/module permission matrix.
type RolePermissionRepository interface {
	ListByRole(ctx context.Context, roleID string) ([]entity.RolePermission, error)
	ListByRoleWorkspace(ctx context.Context, roleID, workspaceID string) ([]entity.RolePermission, error)
	GetOne(ctx context.Context, roleID, workspaceID, moduleID string) (*entity.RolePermission, error)
	Upsert(ctx context.Context, p *entity.RolePermission) (*entity.RolePermission, error)
	SeedDefaultsForWorkspace(ctx context.Context, workspaceID string) error
}

type rolePermissionRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

// NewRolePermissionRepo constructs a postgres-backed role permission repository.
func NewRolePermissionRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) RolePermissionRepository {
	return &rolePermissionRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *rolePermissionRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const rpColumns = `id::text, role_id::text, workspace_id::text, module_id, view_list,
    view_detail, can_create, can_edit, can_delete, can_export, can_import, created_at, updated_at`

func scanRolePermission(s interface{ Scan(dest ...interface{}) error }) (*entity.RolePermission, error) {
	var p entity.RolePermission
	if err := s.Scan(
		&p.ID, &p.RoleID, &p.WorkspaceID, &p.ModuleID, &p.ViewList,
		&p.ViewDetail, &p.CanCreate, &p.CanEdit, &p.CanDelete, &p.CanExport, &p.CanImport,
		&p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *rolePermissionRepo) ListByRole(ctx context.Context, roleID string) ([]entity.RolePermission, error) {
	ctx, span := r.tracer.Start(ctx, "role_permission.repository.ListByRole")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.Select(rpColumns).From("role_permissions").
		Where(sq.Expr("role_id::text = ?", roleID)).
		OrderBy("workspace_id", "module_id").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var out []entity.RolePermission
	for rows.Next() {
		p, err := scanRolePermission(rows)
		if err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (r *rolePermissionRepo) ListByRoleWorkspace(ctx context.Context, roleID, workspaceID string) ([]entity.RolePermission, error) {
	ctx, span := r.tracer.Start(ctx, "role_permission.repository.ListByRoleWorkspace")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.Select(rpColumns).From("role_permissions").
		Where(sq.And{
			sq.Expr("role_id::text = ?", roleID),
			sq.Expr("workspace_id::text = ?", workspaceID),
		}).
		OrderBy("module_id").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var out []entity.RolePermission
	for rows.Next() {
		p, err := scanRolePermission(rows)
		if err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (r *rolePermissionRepo) GetOne(ctx context.Context, roleID, workspaceID, moduleID string) (*entity.RolePermission, error) {
	ctx, span := r.tracer.Start(ctx, "role_permission.repository.GetOne")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.Select(rpColumns).From("role_permissions").
		Where(sq.And{
			sq.Expr("role_id::text = ?", roleID),
			sq.Expr("workspace_id::text = ?", workspaceID),
			sq.Eq{"module_id": moduleID},
		}).ToSql()
	if err != nil {
		return nil, fmt.Errorf("build: %w", err)
	}
	p, err := scanRolePermission(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query: %w", err)
	}
	return p, nil
}

func (r *rolePermissionRepo) Upsert(ctx context.Context, p *entity.RolePermission) (*entity.RolePermission, error) {
	ctx, span := r.tracer.Start(ctx, "role_permission.repository.Upsert")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query := `
        INSERT INTO role_permissions
            (role_id, workspace_id, module_id, view_list, view_detail,
             can_create, can_edit, can_delete, can_export, can_import)
        VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10)
        ON CONFLICT (role_id, workspace_id, module_id) DO UPDATE SET
            view_list   = EXCLUDED.view_list,
            view_detail = EXCLUDED.view_detail,
            can_create  = EXCLUDED.can_create,
            can_edit    = EXCLUDED.can_edit,
            can_delete  = EXCLUDED.can_delete,
            can_export  = EXCLUDED.can_export,
            can_import  = EXCLUDED.can_import,
            updated_at  = NOW()
        RETURNING ` + rpColumns

	out, err := scanRolePermission(r.db.QueryRowContext(ctx, query,
		p.RoleID, p.WorkspaceID, p.ModuleID, p.ViewList, p.ViewDetail,
		p.CanCreate, p.CanEdit, p.CanDelete, p.CanExport, p.CanImport,
	))
	if err != nil {
		return nil, fmt.Errorf("upsert role permission: %w", err)
	}
	return out, nil
}
