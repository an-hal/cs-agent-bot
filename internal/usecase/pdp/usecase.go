// Package pdp implements the compliance surface: subject-erasure (SAR /
// right-to-be-forgotten) requests + per-data-class retention policies.
//
// Erasure request lifecycle: pending → approved → executed (or → rejected).
// Retention policies are configured per workspace and applied by a cron task.
package pdp

import (
	"context"
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
)

type Usecase interface {
	// Erasure
	CreateErasure(ctx context.Context, req CreateErasureRequest) (*entity.PDPErasureRequest, error)
	GetErasure(ctx context.Context, workspaceID, id string) (*entity.PDPErasureRequest, error)
	ListErasure(ctx context.Context, f entity.PDPErasureRequestFilter) ([]entity.PDPErasureRequest, int64, error)
	ApproveErasure(ctx context.Context, workspaceID, id, reviewer string) (*entity.PDPErasureRequest, error)
	RejectErasure(ctx context.Context, workspaceID, id, reviewer, reason string) (*entity.PDPErasureRequest, error)
	// ExecuteErasure performs the actual scrubbing across the in-scope tables.
	// Must be called AFTER approval; records the execution summary.
	ExecuteErasure(ctx context.Context, workspaceID, id string) (*entity.PDPErasureRequest, error)

	// Retention
	UpsertPolicy(ctx context.Context, req UpsertPolicyRequest) (*entity.PDPRetentionPolicy, error)
	ListPolicies(ctx context.Context, workspaceID string, activeOnly bool) ([]entity.PDPRetentionPolicy, error)
	DeletePolicy(ctx context.Context, workspaceID, id string) error
	RunRetention(ctx context.Context, workspaceID string) (RetentionRunSummary, error)
}

type CreateErasureRequest struct {
	WorkspaceID  string   `json:"workspace_id"`
	SubjectEmail string   `json:"subject_email"`
	SubjectKind  string   `json:"subject_kind"`
	Reason       string   `json:"reason"`
	Scope        []string `json:"scope"`
	Requester    string   `json:"-"`
}

type UpsertPolicyRequest struct {
	WorkspaceID   string `json:"workspace_id"`
	DataClass     string `json:"data_class"`
	RetentionDays int    `json:"retention_days"`
	Action        string `json:"action"`
	IsActive      *bool  `json:"is_active,omitempty"`
	ActorEmail    string `json:"-"`
}

// RetentionRunSummary summarizes one nightly cron pass.
type RetentionRunSummary struct {
	WorkspaceID string         `json:"workspace_id"`
	TotalRows   int            `json:"total_rows"`
	Details     map[string]int `json:"details"` // data_class -> rows affected
}

// PolicyEnforcer runs the actual DELETE/UPDATE against the target table. Real
// impls wire to each domain repo; a noop is used in tests and when the cron
// is disabled.
type PolicyEnforcer interface {
	Enforce(ctx context.Context, policy entity.PDPRetentionPolicy) (rowsAffected int, err error)
}

// ErasureExecutor scrubs a subject's PII across the in-scope tables.
type ErasureExecutor interface {
	Execute(ctx context.Context, req entity.PDPErasureRequest) (summary map[string]any, err error)
}

type usecase struct {
	repo      repository.PDPRepository
	enforcer  PolicyEnforcer
	executor  ErasureExecutor
}

func New(repo repository.PDPRepository, enforcer PolicyEnforcer, executor ErasureExecutor) Usecase {
	return &usecase{repo: repo, enforcer: enforcer, executor: executor}
}

// ─── Erasure ────────────────────────────────────────────────────────────────

func (u *usecase) CreateErasure(ctx context.Context, req CreateErasureRequest) (*entity.PDPErasureRequest, error) {
	if req.WorkspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	if strings.TrimSpace(req.SubjectEmail) == "" {
		return nil, apperror.ValidationError("subject_email required")
	}
	if strings.TrimSpace(req.Requester) == "" {
		return nil, apperror.ValidationError("requester required")
	}
	if req.Scope == nil {
		req.Scope = []string{"master_data", "action_log", "master_data_mutations", "fireflies_transcripts"}
	}
	return u.repo.CreateErasure(ctx, &entity.PDPErasureRequest{
		WorkspaceID:  req.WorkspaceID,
		SubjectEmail: strings.ToLower(req.SubjectEmail),
		SubjectKind:  req.SubjectKind,
		Requester:    strings.ToLower(req.Requester),
		Reason:       req.Reason,
		Scope:        req.Scope,
	})
}

func (u *usecase) GetErasure(ctx context.Context, workspaceID, id string) (*entity.PDPErasureRequest, error) {
	if workspaceID == "" || id == "" {
		return nil, apperror.ValidationError("workspace_id and id required")
	}
	e, err := u.repo.GetErasure(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, apperror.NotFound("pdp_erasure_request", id)
	}
	return e, nil
}

func (u *usecase) ListErasure(ctx context.Context, f entity.PDPErasureRequestFilter) ([]entity.PDPErasureRequest, int64, error) {
	if f.WorkspaceID == "" {
		return nil, 0, apperror.ValidationError("workspace_id required")
	}
	return u.repo.ListErasure(ctx, f)
}

func (u *usecase) ApproveErasure(ctx context.Context, workspaceID, id, reviewer string) (*entity.PDPErasureRequest, error) {
	e, err := u.GetErasure(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if e.Status != entity.PDPErasureStatusPending {
		return nil, apperror.BadRequest("request not pending (status=" + e.Status + ")")
	}
	if e.Requester == strings.ToLower(reviewer) {
		return nil, apperror.BadRequest("requester cannot approve their own erasure request")
	}
	if err := u.repo.ReviewErasure(ctx, workspaceID, id, entity.PDPErasureStatusApproved, strings.ToLower(reviewer), ""); err != nil {
		return nil, err
	}
	return u.GetErasure(ctx, workspaceID, id)
}

func (u *usecase) RejectErasure(ctx context.Context, workspaceID, id, reviewer, reason string) (*entity.PDPErasureRequest, error) {
	if strings.TrimSpace(reason) == "" {
		return nil, apperror.ValidationError("reason required when rejecting")
	}
	e, err := u.GetErasure(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if e.Status != entity.PDPErasureStatusPending {
		return nil, apperror.BadRequest("request not pending (status=" + e.Status + ")")
	}
	if err := u.repo.ReviewErasure(ctx, workspaceID, id, entity.PDPErasureStatusRejected, strings.ToLower(reviewer), reason); err != nil {
		return nil, err
	}
	return u.GetErasure(ctx, workspaceID, id)
}

func (u *usecase) ExecuteErasure(ctx context.Context, workspaceID, id string) (*entity.PDPErasureRequest, error) {
	e, err := u.GetErasure(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if e.Status != entity.PDPErasureStatusApproved {
		return nil, apperror.BadRequest("request not approved (status=" + e.Status + ")")
	}
	summary := map[string]any{}
	if u.executor != nil {
		summary, err = u.executor.Execute(ctx, *e)
		if err != nil {
			return nil, err
		}
	} else {
		summary["note"] = "no executor wired — scope recorded but no rows were scrubbed"
		summary["scope"] = e.Scope
	}
	if err := u.repo.MarkExecuted(ctx, workspaceID, id, summary); err != nil {
		return nil, err
	}
	return u.GetErasure(ctx, workspaceID, id)
}

// ─── Retention ──────────────────────────────────────────────────────────────

func (u *usecase) UpsertPolicy(ctx context.Context, req UpsertPolicyRequest) (*entity.PDPRetentionPolicy, error) {
	if req.WorkspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	if strings.TrimSpace(req.DataClass) == "" {
		return nil, apperror.ValidationError("data_class required")
	}
	if req.RetentionDays < 0 {
		return nil, apperror.ValidationError("retention_days must be >= 0 (0 means 'keep forever')")
	}
	switch req.Action {
	case "", entity.PDPRetentionActionDelete, entity.PDPRetentionActionAnonymize, entity.PDPRetentionActionArchive:
	default:
		return nil, apperror.ValidationError("action must be delete|anonymize|archive")
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	return u.repo.UpsertPolicy(ctx, &entity.PDPRetentionPolicy{
		WorkspaceID:   req.WorkspaceID,
		DataClass:     req.DataClass,
		RetentionDays: req.RetentionDays,
		Action:        req.Action,
		IsActive:      active,
		CreatedBy:     strings.ToLower(req.ActorEmail),
	})
}

func (u *usecase) ListPolicies(ctx context.Context, workspaceID string, activeOnly bool) ([]entity.PDPRetentionPolicy, error) {
	if workspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	return u.repo.ListPolicies(ctx, workspaceID, activeOnly)
}

func (u *usecase) DeletePolicy(ctx context.Context, workspaceID, id string) error {
	if workspaceID == "" || id == "" {
		return apperror.ValidationError("workspace_id and id required")
	}
	return u.repo.DeletePolicy(ctx, workspaceID, id)
}

func (u *usecase) RunRetention(ctx context.Context, workspaceID string) (RetentionRunSummary, error) {
	policies, err := u.repo.ListPolicies(ctx, workspaceID, true)
	if err != nil {
		return RetentionRunSummary{}, err
	}
	sum := RetentionRunSummary{
		WorkspaceID: workspaceID,
		Details:     map[string]int{},
	}
	for _, p := range policies {
		if p.RetentionDays <= 0 {
			continue
		}
		var n int
		if u.enforcer != nil {
			rows, err := u.enforcer.Enforce(ctx, p)
			if err != nil {
				continue
			}
			n = rows
		}
		_ = u.repo.RecordPolicyRun(ctx, p.ID, n)
		sum.Details[p.DataClass] = n
		sum.TotalRows += n
	}
	return sum, nil
}
