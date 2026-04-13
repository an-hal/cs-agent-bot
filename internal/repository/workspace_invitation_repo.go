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

type WorkspaceInvitationRepository interface {
	Create(ctx context.Context, inv *entity.WorkspaceInvitation) (*entity.WorkspaceInvitation, error)
	GetByToken(ctx context.Context, token string) (*entity.WorkspaceInvitation, error)
	List(ctx context.Context, workspaceID string) ([]entity.WorkspaceInvitation, error)
	MarkAccepted(ctx context.Context, id string) error
	Revoke(ctx context.Context, id string) error
}

type workspaceInvitationRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewWorkspaceInvitationRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) WorkspaceInvitationRepository {
	return &workspaceInvitationRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *workspaceInvitationRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const invitationColumns = "id::text, workspace_id::text, email, role, invite_token, status, invited_by, accepted_at, expires_at, created_at"

func scanInvitation(scanner interface {
	Scan(dest ...interface{}) error
}) (*entity.WorkspaceInvitation, error) {
	var inv entity.WorkspaceInvitation
	err := scanner.Scan(
		&inv.ID, &inv.WorkspaceID, &inv.Email, &inv.Role,
		&inv.InviteToken, &inv.Status, &inv.InvitedBy,
		&inv.AcceptedAt, &inv.ExpiresAt, &inv.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

func (r *workspaceInvitationRepo) Create(ctx context.Context, inv *entity.WorkspaceInvitation) (*entity.WorkspaceInvitation, error) {
	ctx, span := r.tracer.Start(ctx, "workspace_invitation.repository.Create")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Insert("workspace_invitations").
		Columns("workspace_id", "email", "role", "invite_token", "status", "invited_by", "expires_at").
		Values(inv.WorkspaceID, inv.Email, inv.Role, inv.InviteToken, inv.Status, inv.InvitedBy, inv.ExpiresAt).
		Suffix("RETURNING " + invitationColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert: %w", err)
	}
	out, err := scanInvitation(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		return nil, fmt.Errorf("insert invitation: %w", err)
	}
	return out, nil
}

func (r *workspaceInvitationRepo) GetByToken(ctx context.Context, token string) (*entity.WorkspaceInvitation, error) {
	ctx, span := r.tracer.Start(ctx, "workspace_invitation.repository.GetByToken")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(invitationColumns).
		From("workspace_invitations").
		Where(sq.Eq{"invite_token": token}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	inv, err := scanInvitation(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query invitation: %w", err)
	}
	return inv, nil
}

func (r *workspaceInvitationRepo) List(ctx context.Context, workspaceID string) ([]entity.WorkspaceInvitation, error) {
	ctx, span := r.tracer.Start(ctx, "workspace_invitation.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(invitationColumns).
		From("workspace_invitations").
		Where(sq.Expr("workspace_id::text = ?", workspaceID)).
		OrderBy("created_at DESC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query invitations: %w", err)
	}
	defer rows.Close()
	var out []entity.WorkspaceInvitation
	for rows.Next() {
		inv, err := scanInvitation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan invitation: %w", err)
		}
		out = append(out, *inv)
	}
	return out, rows.Err()
}

func (r *workspaceInvitationRepo) MarkAccepted(ctx context.Context, id string) error {
	ctx, span := r.tracer.Start(ctx, "workspace_invitation.repository.MarkAccepted")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	_, err := r.db.ExecContext(ctx,
		"UPDATE workspace_invitations SET status = 'accepted', accepted_at = NOW() WHERE id::text = $1", id)
	if err != nil {
		return fmt.Errorf("mark accepted: %w", err)
	}
	return nil
}

func (r *workspaceInvitationRepo) Revoke(ctx context.Context, id string) error {
	ctx, span := r.tracer.Start(ctx, "workspace_invitation.repository.Revoke")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	_, err := r.db.ExecContext(ctx,
		"UPDATE workspace_invitations SET status = 'revoked' WHERE id::text = $1", id)
	if err != nil {
		return fmt.Errorf("revoke invitation: %w", err)
	}
	return nil
}
