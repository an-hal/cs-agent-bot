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

// ApprovalRequestRepository is a minimal scaffold for the checker-maker queue.
// Feature 04 will own the full schema.
type ApprovalRequestRepository interface {
	Create(ctx context.Context, a *entity.ApprovalRequest) (*entity.ApprovalRequest, error)
	GetByID(ctx context.Context, workspaceID, id string) (*entity.ApprovalRequest, error)
	UpdateStatus(ctx context.Context, workspaceID, id, newStatus, checkerEmail, reason string) error
}

type approvalRequestRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

// NewApprovalRequestRepo constructs a minimal approval request repository.
func NewApprovalRequestRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) ApprovalRequestRepository {
	return &approvalRequestRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *approvalRequestRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const arColumns = `id::text, workspace_id::text, request_type, description, payload,
    status, maker_email, maker_at, COALESCE(checker_email,''), checker_at,
    COALESCE(rejection_reason,''), expires_at, applied_at, created_at, updated_at`

func scanAR(scanner interface {
	Scan(dest ...interface{}) error
}) (*entity.ApprovalRequest, error) {
	var a entity.ApprovalRequest
	var payloadRaw []byte
	err := scanner.Scan(
		&a.ID, &a.WorkspaceID, &a.RequestType, &a.Description, &payloadRaw,
		&a.Status, &a.MakerEmail, &a.MakerAt, &a.CheckerEmail, &a.CheckerAt,
		&a.RejectionReason, &a.ExpiresAt, &a.AppliedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(payloadRaw) > 0 {
		_ = json.Unmarshal(payloadRaw, &a.Payload)
	}
	if a.Payload == nil {
		a.Payload = map[string]any{}
	}
	return &a, nil
}

func (r *approvalRequestRepo) Create(ctx context.Context, a *entity.ApprovalRequest) (*entity.ApprovalRequest, error) {
	ctx, span := r.tracer.Start(ctx, "approval_request.repository.Create")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	payloadRaw, err := json.Marshal(coalesceMap(a.Payload))
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	query, args, err := database.PSQL.
		Insert("approval_requests").
		Columns("workspace_id", "request_type", "description", "payload", "maker_email").
		Values(a.WorkspaceID, a.RequestType, a.Description, string(payloadRaw), a.MakerEmail).
		Suffix("RETURNING " + arColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert: %w", err)
	}
	out, err := scanAR(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		return nil, fmt.Errorf("insert approval: %w", err)
	}
	return out, nil
}

func (r *approvalRequestRepo) GetByID(ctx context.Context, workspaceID, id string) (*entity.ApprovalRequest, error) {
	ctx, span := r.tracer.Start(ctx, "approval_request.repository.GetByID")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(arColumns).From("approval_requests").
		Where(sq.And{
			sq.Expr("workspace_id::text = ?", workspaceID),
			sq.Expr("id::text = ?", id),
		}).ToSql()
	if err != nil {
		return nil, fmt.Errorf("build: %w", err)
	}
	out, err := scanAR(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query: %w", err)
	}
	return out, nil
}

func (r *approvalRequestRepo) UpdateStatus(ctx context.Context, workspaceID, id, newStatus, checkerEmail, reason string) error {
	ctx, span := r.tracer.Start(ctx, "approval_request.repository.UpdateStatus")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	res, err := r.db.ExecContext(ctx,
		`UPDATE approval_requests
            SET status = $1,
                checker_email = NULLIF($2,''),
                checker_at = NOW(),
                rejection_reason = NULLIF($3,''),
                applied_at = CASE WHEN $1 = 'approved' THEN NOW() ELSE applied_at END,
                updated_at = NOW()
            WHERE workspace_id::text = $4 AND id::text = $5`,
		newStatus, checkerEmail, reason, workspaceID, id,
	)
	if err != nil {
		return fmt.Errorf("update approval: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("not found")
	}
	return nil
}
