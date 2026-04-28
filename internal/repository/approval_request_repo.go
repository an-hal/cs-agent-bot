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
	List(ctx context.Context, workspaceID string, filter ApprovalFilter) ([]entity.ApprovalRequest, int64, error)
}

// ApprovalFilter is the workspace-scoped query for listing approval requests.
// Empty values are treated as "no filter".
type ApprovalFilter struct {
	Status      string // pending | approved | rejected | expired
	RequestType string // bulk_import_master_data | delete_client_record | ...
	MakerEmail  string
	Limit       int
	Offset      int
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

func (r *approvalRequestRepo) List(ctx context.Context, workspaceID string, f ApprovalFilter) ([]entity.ApprovalRequest, int64, error) {
	ctx, span := r.tracer.Start(ctx, "approval_request.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	conds := sq.And{sq.Expr("workspace_id::text = ?", workspaceID)}
	if f.Status != "" {
		conds = append(conds, sq.Eq{"status": f.Status})
	}
	if f.RequestType != "" {
		conds = append(conds, sq.Eq{"request_type": f.RequestType})
	}
	if f.MakerEmail != "" {
		conds = append(conds, sq.Eq{"maker_email": f.MakerEmail})
	}

	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	q := database.PSQL.Select(arColumns).From("approval_requests").
		Where(conds).
		OrderBy("created_at DESC").
		Limit(uint64(f.Limit)).
		Offset(uint64(f.Offset))
	query, args, err := q.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build list: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query list: %w", err)
	}
	defer rows.Close()

	var out []entity.ApprovalRequest
	for rows.Next() {
		ar, err := scanAR(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan: %w", err)
		}
		out = append(out, *ar)
	}

	cq, cArgs, _ := database.PSQL.Select("COUNT(*)").From("approval_requests").Where(conds).ToSql()
	var total int64
	_ = r.db.QueryRowContext(ctx, cq, cArgs...).Scan(&total)
	return out, total, rows.Err()
}

func (r *approvalRequestRepo) UpdateStatus(ctx context.Context, workspaceID, id, newStatus, checkerEmail, reason string) error {
	ctx, span := r.tracer.Start(ctx, "approval_request.repository.UpdateStatus")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	res, err := r.db.ExecContext(ctx,
		`UPDATE approval_requests
            SET status = $1::text,
                checker_email = NULLIF($2,''),
                checker_at = NOW(),
                rejection_reason = NULLIF($3,''),
                applied_at = CASE WHEN $1::text = 'approved' THEN NOW() ELSE applied_at END,
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
