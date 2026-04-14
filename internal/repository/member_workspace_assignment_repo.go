package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// MemberWorkspaceAssignmentRepository manages member_workspace_assignments.
type MemberWorkspaceAssignmentRepository interface {
	ListByMember(ctx context.Context, memberID string) ([]entity.MemberWorkspaceAssignment, error)
	ListWorkspaceIDsByMember(ctx context.Context, memberID string) ([]string, error)
	Assign(ctx context.Context, memberID, workspaceID, assignedBy string) error
	Unassign(ctx context.Context, memberID, workspaceID string) error
	ReplaceForMember(ctx context.Context, memberID string, workspaceIDs []string, assignedBy string) error
	Has(ctx context.Context, memberID, workspaceID string) (bool, error)
}

type memberWorkspaceAssignmentRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

// NewMemberWorkspaceAssignmentRepo constructs a postgres-backed assignment repository.
func NewMemberWorkspaceAssignmentRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) MemberWorkspaceAssignmentRepository {
	return &memberWorkspaceAssignmentRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *memberWorkspaceAssignmentRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

func (r *memberWorkspaceAssignmentRepo) ListByMember(ctx context.Context, memberID string) ([]entity.MemberWorkspaceAssignment, error) {
	ctx, span := r.tracer.Start(ctx, "mwa.repository.ListByMember")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	rows, err := r.db.QueryContext(ctx,
		`SELECT id::text, member_id::text, workspace_id::text, assigned_at,
                COALESCE(assigned_by::text,'')
         FROM member_workspace_assignments
         WHERE member_id::text = $1
         ORDER BY assigned_at ASC`, memberID)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var out []entity.MemberWorkspaceAssignment
	for rows.Next() {
		var a entity.MemberWorkspaceAssignment
		if err := rows.Scan(&a.ID, &a.MemberID, &a.WorkspaceID, &a.AssignedAt, &a.AssignedBy); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *memberWorkspaceAssignmentRepo) ListWorkspaceIDsByMember(ctx context.Context, memberID string) ([]string, error) {
	assignments, err := r.ListByMember(ctx, memberID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(assignments))
	for _, a := range assignments {
		ids = append(ids, a.WorkspaceID)
	}
	return ids, nil
}

func (r *memberWorkspaceAssignmentRepo) Assign(ctx context.Context, memberID, workspaceID, assignedBy string) error {
	ctx, span := r.tracer.Start(ctx, "mwa.repository.Assign")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO member_workspace_assignments (member_id, workspace_id, assigned_by)
         VALUES ($1::uuid, $2::uuid, NULLIF($3,'')::uuid)
         ON CONFLICT (member_id, workspace_id) DO NOTHING`,
		memberID, workspaceID, assignedBy)
	if err != nil {
		return fmt.Errorf("assign: %w", err)
	}
	return nil
}

func (r *memberWorkspaceAssignmentRepo) Unassign(ctx context.Context, memberID, workspaceID string) error {
	ctx, span := r.tracer.Start(ctx, "mwa.repository.Unassign")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	_, err := r.db.ExecContext(ctx,
		`DELETE FROM member_workspace_assignments
         WHERE member_id::text = $1 AND workspace_id::text = $2`,
		memberID, workspaceID)
	if err != nil {
		return fmt.Errorf("unassign: %w", err)
	}
	return nil
}

func (r *memberWorkspaceAssignmentRepo) ReplaceForMember(ctx context.Context, memberID string, workspaceIDs []string, assignedBy string) error {
	ctx, span := r.tracer.Start(ctx, "mwa.repository.ReplaceForMember")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM member_workspace_assignments WHERE member_id::text = $1`, memberID); err != nil {
		return fmt.Errorf("clear: %w", err)
	}
	for _, wsID := range workspaceIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO member_workspace_assignments (member_id, workspace_id, assigned_by)
             VALUES ($1::uuid, $2::uuid, NULLIF($3,'')::uuid)`,
			memberID, wsID, assignedBy); err != nil {
			return fmt.Errorf("insert assignment: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (r *memberWorkspaceAssignmentRepo) Has(ctx context.Context, memberID, workspaceID string) (bool, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	var found bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(
            SELECT 1 FROM member_workspace_assignments
            WHERE member_id::text = $1 AND workspace_id::text = $2)`,
		memberID, workspaceID).Scan(&found)
	if err != nil {
		return false, fmt.Errorf("exists: %w", err)
	}
	return found, nil
}
