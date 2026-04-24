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

type PDPRepository interface {
	// Erasure requests
	CreateErasure(ctx context.Context, req *entity.PDPErasureRequest) (*entity.PDPErasureRequest, error)
	GetErasure(ctx context.Context, workspaceID, id string) (*entity.PDPErasureRequest, error)
	ListErasure(ctx context.Context, f entity.PDPErasureRequestFilter) ([]entity.PDPErasureRequest, int64, error)
	ReviewErasure(ctx context.Context, workspaceID, id, status, reviewedBy, rejectionReason string) error
	MarkExecuted(ctx context.Context, workspaceID, id string, summary map[string]any) error

	// Retention policies
	UpsertPolicy(ctx context.Context, p *entity.PDPRetentionPolicy) (*entity.PDPRetentionPolicy, error)
	ListPolicies(ctx context.Context, workspaceID string, activeOnly bool) ([]entity.PDPRetentionPolicy, error)
	DeletePolicy(ctx context.Context, workspaceID, id string) error
	RecordPolicyRun(ctx context.Context, id string, rowsAffected int) error
}

type pdpRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewPDPRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) PDPRepository {
	return &pdpRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *pdpRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const pdpErasureColumns = `id::text, workspace_id::text, subject_email, subject_kind,
    requester, reason, scope, status, rejection_reason,
    COALESCE(reviewed_by,''), reviewed_at, executed_at, execution_summary,
    expires_at, created_at, updated_at`

func scanErasure(s interface{ Scan(dest ...interface{}) error }) (*entity.PDPErasureRequest, error) {
	var e entity.PDPErasureRequest
	var scopeRaw, summaryRaw []byte
	err := s.Scan(&e.ID, &e.WorkspaceID, &e.SubjectEmail, &e.SubjectKind,
		&e.Requester, &e.Reason, &scopeRaw, &e.Status, &e.RejectionReason,
		&e.ReviewedBy, &e.ReviewedAt, &e.ExecutedAt, &summaryRaw,
		&e.ExpiresAt, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if len(scopeRaw) > 0 {
		_ = json.Unmarshal(scopeRaw, &e.Scope)
	}
	if len(summaryRaw) > 0 {
		_ = json.Unmarshal(summaryRaw, &e.ExecutionSummary)
	}
	if e.Scope == nil {
		e.Scope = []string{}
	}
	if e.ExecutionSummary == nil {
		e.ExecutionSummary = map[string]any{}
	}
	return &e, nil
}

func (r *pdpRepo) CreateErasure(ctx context.Context, req *entity.PDPErasureRequest) (*entity.PDPErasureRequest, error) {
	ctx, span := r.tracer.Start(ctx, "pdp.repository.CreateErasure")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	scopeRaw, _ := json.Marshal(req.Scope)
	if len(scopeRaw) == 0 {
		scopeRaw = []byte("[]")
	}
	q, args, err := database.PSQL.Insert("pdp_erasure_requests").
		Columns("workspace_id", "subject_email", "subject_kind", "requester", "reason", "scope").
		Values(req.WorkspaceID, req.SubjectEmail, defaultIfBlank(req.SubjectKind, "contact"),
			req.Requester, req.Reason, string(scopeRaw)).
		Suffix("RETURNING " + pdpErasureColumns).ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert: %w", err)
	}
	return scanErasure(r.db.QueryRowContext(ctx, q, args...))
}

func (r *pdpRepo) GetErasure(ctx context.Context, workspaceID, id string) (*entity.PDPErasureRequest, error) {
	ctx, span := r.tracer.Start(ctx, "pdp.repository.GetErasure")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q, args, err := database.PSQL.Select(pdpErasureColumns).From("pdp_erasure_requests").
		Where(sq.And{sq.Expr("workspace_id::text = ?", workspaceID), sq.Expr("id::text = ?", id)}).ToSql()
	if err != nil {
		return nil, err
	}
	out, err := scanErasure(r.db.QueryRowContext(ctx, q, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return out, nil
}

func (r *pdpRepo) ListErasure(ctx context.Context, f entity.PDPErasureRequestFilter) ([]entity.PDPErasureRequest, int64, error) {
	ctx, span := r.tracer.Start(ctx, "pdp.repository.ListErasure")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	conds := sq.And{sq.Expr("workspace_id::text = ?", f.WorkspaceID)}
	if f.Status != "" {
		conds = append(conds, sq.Eq{"status": f.Status})
	}
	if f.SubjectEmail != "" {
		conds = append(conds, sq.Eq{"subject_email": f.SubjectEmail})
	}
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q, args, err := database.PSQL.Select(pdpErasureColumns).From("pdp_erasure_requests").
		Where(conds).OrderBy("created_at DESC").
		Limit(uint64(limit)).Offset(uint64(f.Offset)).ToSql()
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []entity.PDPErasureRequest
	for rows.Next() {
		e, err := scanErasure(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	cQ, cArgs, err := database.PSQL.Select("COUNT(*)").From("pdp_erasure_requests").Where(conds).ToSql()
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, cQ, cArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *pdpRepo) ReviewErasure(ctx context.Context, workspaceID, id, status, reviewedBy, rejectionReason string) error {
	ctx, span := r.tracer.Start(ctx, "pdp.repository.ReviewErasure")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	res, err := r.db.ExecContext(ctx, `
        UPDATE pdp_erasure_requests
           SET status = $1,
               reviewed_by = $2,
               reviewed_at = NOW(),
               rejection_reason = $3,
               updated_at = NOW()
         WHERE workspace_id::text = $4 AND id::text = $5`,
		status, reviewedBy, rejectionReason, workspaceID, id,
	)
	if err != nil {
		return fmt.Errorf("review erasure: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("erasure request not found")
	}
	return nil
}

func (r *pdpRepo) MarkExecuted(ctx context.Context, workspaceID, id string, summary map[string]any) error {
	ctx, span := r.tracer.Start(ctx, "pdp.repository.MarkExecuted")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	raw, _ := json.Marshal(coalesceMap(summary))
	res, err := r.db.ExecContext(ctx, `
        UPDATE pdp_erasure_requests
           SET status = 'executed',
               executed_at = NOW(),
               execution_summary = $1::jsonb,
               updated_at = NOW()
         WHERE workspace_id::text = $2 AND id::text = $3`,
		string(raw), workspaceID, id,
	)
	if err != nil {
		return fmt.Errorf("mark executed: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("erasure request not found")
	}
	return nil
}

// ─── Retention policies ─────────────────────────────────────────────────────

const pdpPolicyColumns = `id::text, workspace_id::text, data_class, retention_days,
    action, is_active, last_run_at, last_run_rows, created_by, created_at, updated_at`

func scanPolicy(s interface{ Scan(dest ...interface{}) error }) (*entity.PDPRetentionPolicy, error) {
	var p entity.PDPRetentionPolicy
	err := s.Scan(&p.ID, &p.WorkspaceID, &p.DataClass, &p.RetentionDays,
		&p.Action, &p.IsActive, &p.LastRunAt, &p.LastRunRows,
		&p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *pdpRepo) UpsertPolicy(ctx context.Context, p *entity.PDPRetentionPolicy) (*entity.PDPRetentionPolicy, error) {
	ctx, span := r.tracer.Start(ctx, "pdp.repository.UpsertPolicy")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q := `
        INSERT INTO pdp_retention_policies
            (workspace_id, data_class, retention_days, action, is_active, created_by)
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (workspace_id, data_class) DO UPDATE SET
            retention_days = EXCLUDED.retention_days,
            action         = EXCLUDED.action,
            is_active      = EXCLUDED.is_active,
            updated_at     = NOW()
        RETURNING ` + pdpPolicyColumns
	return scanPolicy(r.db.QueryRowContext(ctx, q,
		p.WorkspaceID, p.DataClass, p.RetentionDays, defaultIfBlank(p.Action, entity.PDPRetentionActionDelete),
		p.IsActive, p.CreatedBy,
	))
}

func (r *pdpRepo) ListPolicies(ctx context.Context, workspaceID string, activeOnly bool) ([]entity.PDPRetentionPolicy, error) {
	ctx, span := r.tracer.Start(ctx, "pdp.repository.ListPolicies")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	conds := sq.And{sq.Expr("workspace_id::text = ?", workspaceID)}
	if activeOnly {
		conds = append(conds, sq.Eq{"is_active": true})
	}
	q, args, err := database.PSQL.Select(pdpPolicyColumns).From("pdp_retention_policies").
		Where(conds).OrderBy("data_class ASC").ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []entity.PDPRetentionPolicy
	for rows.Next() {
		p, err := scanPolicy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (r *pdpRepo) DeletePolicy(ctx context.Context, workspaceID, id string) error {
	ctx, span := r.tracer.Start(ctx, "pdp.repository.DeletePolicy")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	res, err := r.db.ExecContext(ctx,
		"DELETE FROM pdp_retention_policies WHERE workspace_id::text = $1 AND id::text = $2",
		workspaceID, id)
	if err != nil {
		return fmt.Errorf("delete policy: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("policy not found")
	}
	return nil
}

func (r *pdpRepo) RecordPolicyRun(ctx context.Context, id string, rowsAffected int) error {
	ctx, span := r.tracer.Start(ctx, "pdp.repository.RecordPolicyRun")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	_, err := r.db.ExecContext(ctx, `
        UPDATE pdp_retention_policies
           SET last_run_at = NOW(), last_run_rows = $1, updated_at = NOW()
         WHERE id::text = $2`,
		rowsAffected, id,
	)
	if err != nil {
		return fmt.Errorf("record run: %w", err)
	}
	return nil
}

// Silence unused-import warning from pq pulled in via master_data_repo.
var _ = pq.Array
