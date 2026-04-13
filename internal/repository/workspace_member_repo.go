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
	"github.com/rs/zerolog"
)

type WorkspaceMemberRepository interface {
	List(ctx context.Context, workspaceID string) ([]entity.WorkspaceMember, error)
	Get(ctx context.Context, id string) (*entity.WorkspaceMember, error)
	GetByWorkspaceAndEmail(ctx context.Context, workspaceID, email string) (*entity.WorkspaceMember, error)
	Add(ctx context.Context, m *entity.WorkspaceMember) (*entity.WorkspaceMember, error)
	UpdateRole(ctx context.Context, id, role string) (*entity.WorkspaceMember, error)
	Remove(ctx context.Context, id string) error
}

type workspaceMemberRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewWorkspaceMemberRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) WorkspaceMemberRepository {
	return &workspaceMemberRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *workspaceMemberRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const memberColumns = "id::text, workspace_id::text, user_email, user_name, role, permissions, is_active, invited_at, joined_at, COALESCE(invited_by, ''), created_at, updated_at"

func scanMember(scanner interface {
	Scan(dest ...interface{}) error
}) (*entity.WorkspaceMember, error) {
	var (
		m       entity.WorkspaceMember
		permRaw []byte
	)
	err := scanner.Scan(
		&m.ID, &m.WorkspaceID, &m.UserEmail, &m.UserName, &m.Role, &permRaw,
		&m.IsActive, &m.InvitedAt, &m.JoinedAt, &m.InvitedBy, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(permRaw) > 0 {
		if err := json.Unmarshal(permRaw, &m.Permissions); err != nil {
			return nil, fmt.Errorf("unmarshal permissions: %w", err)
		}
	}
	if m.Permissions == nil {
		m.Permissions = map[string]interface{}{}
	}
	return &m, nil
}

func (r *workspaceMemberRepo) List(ctx context.Context, workspaceID string) ([]entity.WorkspaceMember, error) {
	ctx, span := r.tracer.Start(ctx, "workspace_member.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(memberColumns).
		From("workspace_members").
		Where(sq.Expr("workspace_id::text = ?", workspaceID)).
		OrderBy("created_at ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query members: %w", err)
	}
	defer rows.Close()
	var members []entity.WorkspaceMember
	for rows.Next() {
		m, err := scanMember(rows)
		if err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, *m)
	}
	return members, rows.Err()
}

func (r *workspaceMemberRepo) Get(ctx context.Context, id string) (*entity.WorkspaceMember, error) {
	ctx, span := r.tracer.Start(ctx, "workspace_member.repository.Get")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(memberColumns).
		From("workspace_members").
		Where(sq.Expr("id::text = ?", id)).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	m, err := scanMember(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query member: %w", err)
	}
	return m, nil
}

func (r *workspaceMemberRepo) GetByWorkspaceAndEmail(ctx context.Context, workspaceID, email string) (*entity.WorkspaceMember, error) {
	ctx, span := r.tracer.Start(ctx, "workspace_member.repository.GetByWorkspaceAndEmail")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(memberColumns).
		From("workspace_members").
		Where(sq.And{
			sq.Expr("workspace_id::text = ?", workspaceID),
			sq.Eq{"user_email": email},
		}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	m, err := scanMember(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query member: %w", err)
	}
	return m, nil
}

func (r *workspaceMemberRepo) Add(ctx context.Context, m *entity.WorkspaceMember) (*entity.WorkspaceMember, error) {
	ctx, span := r.tracer.Start(ctx, "workspace_member.repository.Add")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if m.Permissions == nil {
		m.Permissions = map[string]interface{}{}
	}
	permRaw, err := json.Marshal(m.Permissions)
	if err != nil {
		return nil, fmt.Errorf("marshal permissions: %w", err)
	}

	query, args, err := database.PSQL.
		Insert("workspace_members").
		Columns("workspace_id", "user_email", "user_name", "role", "permissions", "is_active", "invited_by").
		Values(m.WorkspaceID, m.UserEmail, m.UserName, m.Role, permRaw, true, nullableString(m.InvitedBy)).
		Suffix("RETURNING " + memberColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert: %w", err)
	}
	out, err := scanMember(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		return nil, fmt.Errorf("insert member: %w", err)
	}
	return out, nil
}

func (r *workspaceMemberRepo) UpdateRole(ctx context.Context, id, role string) (*entity.WorkspaceMember, error) {
	ctx, span := r.tracer.Start(ctx, "workspace_member.repository.UpdateRole")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Update("workspace_members").
		Set("role", role).
		Where(sq.Expr("id::text = ?", id)).
		Suffix("RETURNING " + memberColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build update: %w", err)
	}
	out, err := scanMember(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("update member: %w", err)
	}
	return out, nil
}

func (r *workspaceMemberRepo) Remove(ctx context.Context, id string) error {
	ctx, span := r.tracer.Start(ctx, "workspace_member.repository.Remove")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	res, err := r.db.ExecContext(ctx, "DELETE FROM workspace_members WHERE id::text = $1", id)
	if err != nil {
		return fmt.Errorf("delete member: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
