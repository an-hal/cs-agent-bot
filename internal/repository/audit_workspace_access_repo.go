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

type AuditWorkspaceAccessRepository interface {
	Insert(ctx context.Context, a *entity.AuditWorkspaceAccess) (*entity.AuditWorkspaceAccess, error)
	List(ctx context.Context, filter entity.AuditWorkspaceAccessFilter) ([]entity.AuditWorkspaceAccess, int64, error)
}

type auditWorkspaceAccessRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewAuditWorkspaceAccessRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) AuditWorkspaceAccessRepository {
	return &auditWorkspaceAccessRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

const awaColumns = `id::text, workspace_id::text, actor_email, access_kind,
    resource, resource_id, ip_address, user_agent, reason, created_at`

func scanAWA(s interface{ Scan(dest ...interface{}) error }) (*entity.AuditWorkspaceAccess, error) {
	var a entity.AuditWorkspaceAccess
	err := s.Scan(&a.ID, &a.WorkspaceID, &a.ActorEmail, &a.Kind,
		&a.Resource, &a.ResourceID, &a.IPAddress, &a.UserAgent, &a.Reason, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *auditWorkspaceAccessRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

func (r *auditWorkspaceAccessRepo) Insert(ctx context.Context, a *entity.AuditWorkspaceAccess) (*entity.AuditWorkspaceAccess, error) {
	ctx, span := r.tracer.Start(ctx, "audit_workspace_access.repository.Insert")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Insert("audit_logs_workspace_access").
		Columns("workspace_id", "actor_email", "access_kind", "resource", "resource_id", "ip_address", "user_agent", "reason").
		Values(a.WorkspaceID, a.ActorEmail, a.Kind, a.Resource, a.ResourceID, a.IPAddress, a.UserAgent, a.Reason).
		Suffix("RETURNING " + awaColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert: %w", err)
	}
	return scanAWA(r.db.QueryRowContext(ctx, query, args...))
}

func (r *auditWorkspaceAccessRepo) List(ctx context.Context, f entity.AuditWorkspaceAccessFilter) ([]entity.AuditWorkspaceAccess, int64, error) {
	ctx, span := r.tracer.Start(ctx, "audit_workspace_access.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	conds := sq.And{sq.Expr("workspace_id::text = ?", f.WorkspaceID)}
	if f.ActorEmail != "" {
		conds = append(conds, sq.Eq{"actor_email": f.ActorEmail})
	}
	if f.Kind != "" {
		conds = append(conds, sq.Eq{"access_kind": f.Kind})
	}
	if f.Resource != "" {
		conds = append(conds, sq.Eq{"resource": f.Resource})
	}
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query, args, err := database.PSQL.
		Select(awaColumns).From("audit_logs_workspace_access").
		Where(conds).OrderBy("created_at DESC").
		Limit(uint64(limit)).Offset(uint64(f.Offset)).ToSql()
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []entity.AuditWorkspaceAccess
	for rows.Next() {
		a, err := scanAWA(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	cQ, cArgs, err := database.PSQL.Select("COUNT(*)").From("audit_logs_workspace_access").Where(conds).ToSql()
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, cQ, cArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}
