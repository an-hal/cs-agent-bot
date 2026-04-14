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

// ErrTeamNotFound is returned when a team row is missing.
var ErrTeamNotFound = errors.New("team: not found")

// RoleRepository is CRUD for roles.
type RoleRepository interface {
	List(ctx context.Context) ([]entity.Role, error)
	GetByID(ctx context.Context, id string) (*entity.Role, error)
	GetByName(ctx context.Context, name string) (*entity.Role, error)
	Create(ctx context.Context, r *entity.Role) (*entity.Role, error)
	Update(ctx context.Context, id string, patch RolePatch) (*entity.Role, error)
	Delete(ctx context.Context, id string) error
	CountMembers(ctx context.Context, roleID string) (int, error)
}

// RolePatch is a partial role update. Nil pointer = leave unchanged.
type RolePatch struct {
	Name        *string
	Description *string
	Color       *string
	BgColor     *string
}

type roleRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

// NewRoleRepo constructs a postgres-backed role repository.
func NewRoleRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) RoleRepository {
	return &roleRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *roleRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const roleColumns = "id::text, name, description, color, bg_color, is_system, created_at, updated_at"

func scanRole(s interface{ Scan(dest ...interface{}) error }) (*entity.Role, error) {
	var role entity.Role
	if err := s.Scan(
		&role.ID, &role.Name, &role.Description, &role.Color, &role.BgColor,
		&role.IsSystem, &role.CreatedAt, &role.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *roleRepo) List(ctx context.Context) ([]entity.Role, error) {
	ctx, span := r.tracer.Start(ctx, "role.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.Select(roleColumns).From("roles").OrderBy("is_system DESC", "name ASC").ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query roles: %w", err)
	}
	defer rows.Close()

	var out []entity.Role
	for rows.Next() {
		role, err := scanRole(rows)
		if err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		out = append(out, *role)
	}
	return out, rows.Err()
}

func (r *roleRepo) GetByID(ctx context.Context, id string) (*entity.Role, error) {
	ctx, span := r.tracer.Start(ctx, "role.repository.GetByID")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.Select(roleColumns).From("roles").
		Where(sq.Expr("id::text = ?", id)).ToSql()
	if err != nil {
		return nil, fmt.Errorf("build: %w", err)
	}
	role, err := scanRole(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query: %w", err)
	}
	return role, nil
}

func (r *roleRepo) GetByName(ctx context.Context, name string) (*entity.Role, error) {
	ctx, span := r.tracer.Start(ctx, "role.repository.GetByName")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.Select(roleColumns).From("roles").
		Where(sq.Eq{"name": name}).ToSql()
	if err != nil {
		return nil, fmt.Errorf("build: %w", err)
	}
	role, err := scanRole(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query: %w", err)
	}
	return role, nil
}

func (r *roleRepo) Create(ctx context.Context, role *entity.Role) (*entity.Role, error) {
	ctx, span := r.tracer.Start(ctx, "role.repository.Create")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.Insert("roles").
		Columns("name", "description", "color", "bg_color", "is_system").
		Values(role.Name, role.Description, role.Color, role.BgColor, role.IsSystem).
		Suffix("RETURNING " + roleColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert: %w", err)
	}
	out, err := scanRole(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		return nil, fmt.Errorf("insert role: %w", err)
	}
	return out, nil
}

func (r *roleRepo) Update(ctx context.Context, id string, patch RolePatch) (*entity.Role, error) {
	ctx, span := r.tracer.Start(ctx, "role.repository.Update")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	upd := database.PSQL.Update("roles").Where(sq.Expr("id::text = ?", id))
	dirty := false
	if patch.Name != nil {
		upd = upd.Set("name", *patch.Name)
		dirty = true
	}
	if patch.Description != nil {
		upd = upd.Set("description", *patch.Description)
		dirty = true
	}
	if patch.Color != nil {
		upd = upd.Set("color", *patch.Color)
		dirty = true
	}
	if patch.BgColor != nil {
		upd = upd.Set("bg_color", *patch.BgColor)
		dirty = true
	}
	if !dirty {
		return r.GetByID(ctx, id)
	}
	upd = upd.Set("updated_at", sq.Expr("NOW()")).Suffix("RETURNING " + roleColumns)

	query, args, err := upd.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build update: %w", err)
	}
	out, err := scanRole(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTeamNotFound
		}
		return nil, fmt.Errorf("update role: %w", err)
	}
	return out, nil
}

func (r *roleRepo) Delete(ctx context.Context, id string) error {
	ctx, span := r.tracer.Start(ctx, "role.repository.Delete")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	res, err := r.db.ExecContext(ctx, `DELETE FROM roles WHERE id::text = $1`, id)
	if err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrTeamNotFound
	}
	return nil
}

func (r *roleRepo) CountMembers(ctx context.Context, roleID string) (int, error) {
	ctx, span := r.tracer.Start(ctx, "role.repository.CountMembers")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM team_members WHERE role_id::text = $1`, roleID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count members: %w", err)
	}
	return count, nil
}
