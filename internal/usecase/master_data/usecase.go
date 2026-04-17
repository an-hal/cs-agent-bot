// Package master_data implements the hybrid-schema CRM Master Data feature:
// CRUD over the existing `clients` table (exposed as the `master_data` view),
// per-workspace JSONB custom fields, stage transitions, flexible workflow
// queries, attention/stats summaries, and dashboard mutation logging.
//
// The bot's P0-P5 cron logic continues to operate on `clients` directly via
// the legacy ClientRepository. This package never touches that path.
package master_data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
)

// WriteContext describes who is invoking a write. The bot is forbidden from
// writing certain financial/lifecycle fields; the dashboard user is allowed.
type WriteContext string

const (
	// WriteContextDashboardUser is the AE / dashboard caller — full write access.
	WriteContextDashboardUser WriteContext = "dashboard_user"
	// WriteContextBot is the cron / webhook actor — denied for protected fields.
	WriteContextBot WriteContext = "bot_actor"
)

// Fields the bot may NEVER write. CLAUDE.md business rule #3.
var protectedFields = map[string]struct{}{
	"payment_status": {},
	"renewed":        {},
	"rejected":       {},
}

// Usecase exposes Master Data business operations.
type Usecase interface {
	List(ctx context.Context, workspaceID string, filter entity.MasterDataFilter) ([]entity.MasterData, int64, error)
	Get(ctx context.Context, workspaceID, id string) (*entity.MasterData, error)
	Create(ctx context.Context, workspaceID, actorEmail string, req CreateRequest) (*entity.MasterData, error)
	Patch(ctx context.Context, workspaceID, id, actorEmail string, ctxKind WriteContext, req PatchRequest) (*entity.MasterData, []string, error)
	RequestDelete(ctx context.Context, workspaceID, id, actorEmail string) (*entity.ApprovalRequest, error)
	ApplyApprovedDelete(ctx context.Context, workspaceID, approvalID, checkerEmail string) error
	Stats(ctx context.Context, workspaceID string) (*repository.MasterDataStats, error)
	Attention(ctx context.Context, workspaceID, search string, offset, limit int) ([]entity.MasterData, int64, *AttentionSummary, error)
	Query(ctx context.Context, workspaceID string, conds []repository.QueryCondition, limit int) ([]entity.MasterData, int64, error)
	Transition(ctx context.Context, workspaceID, id, actorEmail string, ctxKind WriteContext, req TransitionRequest) (*TransitionResult, error)
	ListMutations(ctx context.Context, workspaceID string, since *time.Time, limit int) ([]entity.MasterDataMutation, error)

	RequestImport(ctx context.Context, workspaceID, actorEmail, fileName string, mode ImportMode, rowCount int, preview []map[string]any, fileRef string) (*entity.ApprovalRequest, error)
	Export(ctx context.Context, workspaceID string, w io.Writer) error
	Template(ctx context.Context, workspaceID string, w io.Writer) error
}

// CreateRequest is the payload for POST /master-data/clients.
type CreateRequest struct {
	CompanyID       string         `json:"company_id"`
	CompanyName     string         `json:"company_name"`
	Stage           string         `json:"stage"`
	PICName         string         `json:"pic_name"`
	PICNickname     string         `json:"pic_nickname"`
	PICRole         string         `json:"pic_role"`
	PICWA           string         `json:"pic_wa"`
	PICEmail        string         `json:"pic_email"`
	OwnerName       string         `json:"owner_name"`
	OwnerWA         string         `json:"owner_wa"`
	OwnerTelegramID string         `json:"owner_telegram_id"`
	BotActive       *bool          `json:"bot_active"`
	Blacklisted     *bool          `json:"blacklisted"`
	SequenceStatus  string         `json:"sequence_status"`
	RiskFlag        string         `json:"risk_flag"`
	ContractStart   *time.Time     `json:"contract_start"`
	ContractEnd     *time.Time     `json:"contract_end"`
	ContractMonths  int            `json:"contract_months"`
	PaymentStatus   string         `json:"payment_status"`
	PaymentTerms    string         `json:"payment_terms"`
	FinalPrice      int64          `json:"final_price"`
	Notes           string         `json:"notes"`
	CustomFields    map[string]any `json:"custom_fields"`
}

// PatchRequest is the payload for PUT /master-data/clients/{id}.
type PatchRequest struct {
	CompanyName     *string        `json:"company_name,omitempty"`
	Stage           *string        `json:"stage,omitempty"`
	PICName         *string        `json:"pic_name,omitempty"`
	PICNickname     *string        `json:"pic_nickname,omitempty"`
	PICRole         *string        `json:"pic_role,omitempty"`
	PICWA           *string        `json:"pic_wa,omitempty"`
	PICEmail        *string        `json:"pic_email,omitempty"`
	OwnerName       *string        `json:"owner_name,omitempty"`
	OwnerWA         *string        `json:"owner_wa,omitempty"`
	OwnerTelegramID *string        `json:"owner_telegram_id,omitempty"`
	BotActive       *bool          `json:"bot_active,omitempty"`
	Blacklisted     *bool          `json:"blacklisted,omitempty"`
	SequenceStatus  *string        `json:"sequence_status,omitempty"`
	RiskFlag        *string        `json:"risk_flag,omitempty"`
	ContractStart   *time.Time     `json:"contract_start,omitempty"`
	ContractEnd     *time.Time     `json:"contract_end,omitempty"`
	ContractMonths  *int           `json:"contract_months,omitempty"`
	PaymentStatus   *string        `json:"payment_status,omitempty"`
	PaymentTerms    *string        `json:"payment_terms,omitempty"`
	FinalPrice      *int64         `json:"final_price,omitempty"`
	LastPaymentDate *time.Time     `json:"last_payment_date,omitempty"`
	Renewed         *bool          `json:"renewed,omitempty"`
	Notes           *string        `json:"notes,omitempty"`
	CustomFields    map[string]any `json:"custom_fields,omitempty"`
}

// TransitionRequest is the payload for POST /master-data/clients/{id}/transition.
type TransitionRequest struct {
	NewStage           string         `json:"new_stage"`
	Updates            PatchRequest   `json:"updates,omitempty"`
	CustomFieldUpdates map[string]any `json:"custom_field_updates,omitempty"`
	TriggerID          string         `json:"trigger_id"`
	Reason             string         `json:"reason"`
}

// TransitionResult is the response shape for a transition.
type TransitionResult struct {
	Data          *entity.MasterData `json:"data"`
	PreviousStage string             `json:"previous_stage"`
}

// AttentionSummary aggregates the counts shown on the attention tab.
type AttentionSummary struct {
	HighRisk int64 `json:"high_risk"`
	Overdue  int64 `json:"overdue"`
	Expiring int64 `json:"expiring"`
}

type usecase struct {
	repo         repository.MasterDataRepository
	cfdRepo      repository.CustomFieldDefinitionRepository
	mutationRepo repository.MasterDataMutationRepository
	approvalRepo repository.ApprovalRequestRepository
}

// New constructs a master_data usecase.
func New(
	repo repository.MasterDataRepository,
	cfdRepo repository.CustomFieldDefinitionRepository,
	mutationRepo repository.MasterDataMutationRepository,
	approvalRepo repository.ApprovalRequestRepository,
) Usecase {
	return &usecase{
		repo:         repo,
		cfdRepo:      cfdRepo,
		mutationRepo: mutationRepo,
		approvalRepo: approvalRepo,
	}
}

// ───────────────────────────── List / Get ─────────────────────────────

func (u *usecase) List(ctx context.Context, workspaceID string, filter entity.MasterDataFilter) ([]entity.MasterData, int64, error) {
	if workspaceID == "" {
		return nil, 0, apperror.ValidationError("workspace_id required")
	}
	if len(filter.WorkspaceIDs) == 0 {
		filter.WorkspaceIDs = []string{workspaceID}
	}
	return u.repo.List(ctx, filter)
}

func (u *usecase) Get(ctx context.Context, workspaceID, id string) (*entity.MasterData, error) {
	if workspaceID == "" || id == "" {
		return nil, apperror.ValidationError("workspace_id and id required")
	}
	out, err := u.repo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, apperror.NotFound("master_data", "")
	}
	return out, nil
}

// ───────────────────────────── Create ─────────────────────────────

func (u *usecase) Create(ctx context.Context, workspaceID, actorEmail string, req CreateRequest) (*entity.MasterData, error) {
	if workspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	if strings.TrimSpace(req.CompanyID) == "" {
		return nil, apperror.ValidationError("company_id required")
	}
	if strings.TrimSpace(req.CompanyName) == "" {
		return nil, apperror.ValidationError("company_name required")
	}
	if req.Stage != "" && !isValidStage(req.Stage) {
		return nil, apperror.ValidationError("invalid stage")
	}
	if req.RiskFlag != "" && !isValidRisk(req.RiskFlag) {
		return nil, apperror.ValidationError("invalid risk_flag")
	}

	defs, err := u.cfdRepo.List(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("load custom field defs: %w", err)
	}
	if err := ValidateCustomFields(defs, req.CustomFields, true); err != nil {
		return nil, err
	}

	m := &entity.MasterData{
		CompanyID:       strings.TrimSpace(req.CompanyID),
		CompanyName:     strings.TrimSpace(req.CompanyName),
		Stage:           defaultIfEmpty(req.Stage, entity.StageLead),
		PICName:         req.PICName,
		PICNickname:     req.PICNickname,
		PICRole:         req.PICRole,
		PICWA:           req.PICWA,
		PICEmail:        req.PICEmail,
		OwnerName:       req.OwnerName,
		OwnerWA:         req.OwnerWA,
		OwnerTelegramID: req.OwnerTelegramID,
		BotActive:       boolOr(req.BotActive, true),
		Blacklisted:     boolOr(req.Blacklisted, false),
		SequenceStatus:  defaultIfEmpty(req.SequenceStatus, entity.SeqStatusActive),
		RiskFlag:        defaultIfEmpty(req.RiskFlag, entity.RiskNone),
		ContractStart:   req.ContractStart,
		ContractEnd:     req.ContractEnd,
		ContractMonths:  req.ContractMonths,
		PaymentStatus:   defaultIfEmpty(req.PaymentStatus, "Pending"),
		PaymentTerms:    req.PaymentTerms,
		FinalPrice:      req.FinalPrice,
		Notes:           req.Notes,
		CustomFields:    req.CustomFields,
	}
	created, err := u.repo.Create(ctx, workspaceID, m)
	if err != nil {
		return nil, err
	}

	// Best-effort mutation log
	_ = u.mutationRepo.Append(ctx, &entity.MasterDataMutation{
		WorkspaceID:   workspaceID,
		MasterDataID:  created.ID,
		CompanyID:     created.CompanyID,
		CompanyName:   created.CompanyName,
		Action:        "create_client",
		ActorEmail:    actorEmail,
		ChangedFields: []string{"*"},
		NewValues:     map[string]any{"company_id": created.CompanyID, "stage": created.Stage},
	})
	return created, nil
}

// ───────────────────────────── Patch ─────────────────────────────

func (u *usecase) Patch(ctx context.Context, workspaceID, id, actorEmail string, ctxKind WriteContext, req PatchRequest) (*entity.MasterData, []string, error) {
	if workspaceID == "" || id == "" {
		return nil, nil, apperror.ValidationError("workspace_id and id required")
	}

	// Enforce protected-fields rule.
	if ctxKind != WriteContextDashboardUser {
		if req.PaymentStatus != nil {
			return nil, nil, apperror.Forbidden("bot may not write payment_status")
		}
		if req.Renewed != nil {
			return nil, nil, apperror.Forbidden("bot may not write renewed")
		}
	}

	prev, err := u.repo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, nil, err
	}
	if prev == nil {
		return nil, nil, apperror.NotFound("master_data", "")
	}

	if req.Stage != nil && !isValidStage(*req.Stage) {
		return nil, nil, apperror.ValidationError("invalid stage")
	}
	if req.RiskFlag != nil && !isValidRisk(*req.RiskFlag) {
		return nil, nil, apperror.ValidationError("invalid risk_flag")
	}
	if req.CustomFields != nil {
		defs, err := u.cfdRepo.List(ctx, workspaceID)
		if err != nil {
			return nil, nil, fmt.Errorf("load custom field defs: %w", err)
		}
		if err := ValidateCustomFields(defs, req.CustomFields, false); err != nil {
			return nil, nil, err
		}
	}

	patch := patchRequestToRepo(req)
	updated, err := u.repo.Patch(ctx, workspaceID, id, patch)
	if err != nil {
		return nil, nil, err
	}

	changedFields, prevValues, newValues := diffMutation(prev, updated, req)

	// Best-effort mutation log
	_ = u.mutationRepo.Append(ctx, &entity.MasterDataMutation{
		WorkspaceID:    workspaceID,
		MasterDataID:   updated.ID,
		CompanyID:      updated.CompanyID,
		CompanyName:    updated.CompanyName,
		Action:         "edit_client",
		ActorEmail:     actorEmail,
		ChangedFields:  changedFields,
		PreviousValues: prevValues,
		NewValues:      newValues,
	})
	return updated, changedFields, nil
}

// ───────────────────────────── Delete (checker-maker) ─────────────────────────────

func (u *usecase) RequestDelete(ctx context.Context, workspaceID, id, actorEmail string) (*entity.ApprovalRequest, error) {
	if workspaceID == "" || id == "" {
		return nil, apperror.ValidationError("workspace_id and id required")
	}
	target, err := u.repo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, apperror.NotFound("master_data", "")
	}
	payload := map[string]any{
		"client_id":    target.ID,
		"company_id":   target.CompanyID,
		"company_name": target.CompanyName,
		"stage":        target.Stage,
	}
	desc := fmt.Sprintf("Delete client record %s (%s)", target.CompanyName, target.CompanyID)
	return u.approvalRepo.Create(ctx, &entity.ApprovalRequest{
		WorkspaceID: workspaceID,
		RequestType: entity.ApprovalTypeDeleteClient,
		Description: desc,
		Payload:     payload,
		MakerEmail:  actorEmail,
	})
}

func (u *usecase) ApplyApprovedDelete(ctx context.Context, workspaceID, approvalID, checkerEmail string) error {
	if workspaceID == "" || approvalID == "" {
		return apperror.ValidationError("workspace_id and approval_id required")
	}
	ar, err := u.approvalRepo.GetByID(ctx, workspaceID, approvalID)
	if err != nil {
		return err
	}
	if ar == nil {
		return apperror.NotFound("approval_request", "")
	}
	if ar.RequestType != entity.ApprovalTypeDeleteClient {
		return apperror.BadRequest("approval is not a delete_client_record request")
	}
	if ar.Status != entity.ApprovalStatusPending {
		return apperror.BadRequest("approval is not pending")
	}
	clientID, _ := ar.Payload["client_id"].(string)
	if clientID == "" {
		return apperror.BadRequest("approval payload missing client_id")
	}
	if err := u.repo.HardDelete(ctx, workspaceID, clientID); err != nil {
		return err
	}
	if err := u.approvalRepo.UpdateStatus(ctx, workspaceID, approvalID, entity.ApprovalStatusApproved, checkerEmail, ""); err != nil {
		return err
	}
	_ = u.mutationRepo.Append(ctx, &entity.MasterDataMutation{
		WorkspaceID:   workspaceID,
		MasterDataID:  clientID,
		Action:        "delete_client",
		ActorEmail:    checkerEmail,
		ChangedFields: []string{"*"},
	})
	return nil
}

// ───────────────────────────── Stats / Attention / Query ─────────────────────────────

func (u *usecase) Stats(ctx context.Context, workspaceID string) (*repository.MasterDataStats, error) {
	if workspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	return u.repo.Stats(ctx, workspaceID)
}

func (u *usecase) Attention(ctx context.Context, workspaceID, search string, offset, limit int) ([]entity.MasterData, int64, *AttentionSummary, error) {
	if workspaceID == "" {
		return nil, 0, nil, apperror.ValidationError("workspace_id required")
	}
	rows, total, err := u.repo.Attention(ctx, workspaceID, search, offset, limit)
	if err != nil {
		return nil, 0, nil, err
	}
	stats, err := u.repo.Stats(ctx, workspaceID)
	if err != nil {
		return rows, total, nil, nil
	}
	return rows, total, &AttentionSummary{
		HighRisk: stats.HighRisk,
		Overdue:  stats.OverduePayment,
		Expiring: stats.Expiring30d,
	}, nil
}

func (u *usecase) Query(ctx context.Context, workspaceID string, conds []repository.QueryCondition, limit int) ([]entity.MasterData, int64, error) {
	if workspaceID == "" {
		return nil, 0, apperror.ValidationError("workspace_id required")
	}
	for _, c := range conds {
		if !repository.AllowedQueryOps[strings.ToLower(c.Op)] {
			return nil, 0, apperror.BadRequest(fmt.Sprintf("op %q not allowed", c.Op))
		}
	}
	return u.repo.Query(ctx, workspaceID, conds, limit)
}

// ───────────────────────────── Transition ─────────────────────────────

func (u *usecase) Transition(ctx context.Context, workspaceID, id, actorEmail string, ctxKind WriteContext, req TransitionRequest) (*TransitionResult, error) {
	if workspaceID == "" || id == "" {
		return nil, apperror.ValidationError("workspace_id and id required")
	}
	if !isValidStage(req.NewStage) {
		return nil, apperror.ValidationError("invalid new_stage")
	}
	if ctxKind != WriteContextDashboardUser {
		if req.Updates.PaymentStatus != nil || req.Updates.Renewed != nil {
			return nil, apperror.Forbidden("bot may not write payment_status or renewed")
		}
	}
	corePatch := patchRequestToRepo(req.Updates)
	prev, curr, err := u.repo.Transition(ctx, workspaceID, id, req.NewStage, corePatch, req.CustomFieldUpdates)
	if err != nil {
		if errors.Is(err, errNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, apperror.NotFound("master_data", "")
		}
		return nil, err
	}
	prevValues := map[string]any{"stage": prev.Stage}
	newValues := map[string]any{"stage": curr.Stage}
	_ = u.mutationRepo.Append(ctx, &entity.MasterDataMutation{
		WorkspaceID:    workspaceID,
		MasterDataID:   curr.ID,
		CompanyID:      curr.CompanyID,
		CompanyName:    curr.CompanyName,
		Action:         "stage_transition",
		ActorEmail:     actorEmail,
		ChangedFields:  []string{"stage"},
		PreviousValues: prevValues,
		NewValues:      newValues,
		Note:           req.Reason,
	})
	return &TransitionResult{Data: curr, PreviousStage: prev.Stage}, nil
}

// ───────────────────────────── Mutations ─────────────────────────────

func (u *usecase) ListMutations(ctx context.Context, workspaceID string, since *time.Time, limit int) ([]entity.MasterDataMutation, error) {
	if workspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	return u.mutationRepo.List(ctx, workspaceID, since, limit)
}

// ───────────────────────────── helpers ─────────────────────────────

var errNotFound = errors.New("not found")

func isValidStage(s string) bool {
	switch s {
	case entity.StageLead, entity.StageProspect, entity.StageClient, entity.StageDormant:
		return true
	}
	return false
}

func isValidRisk(s string) bool {
	switch s {
	case entity.RiskHigh, entity.RiskMid, entity.RiskLow, entity.RiskNone:
		return true
	}
	return false
}

func defaultIfEmpty(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func boolOr(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

func patchRequestToRepo(req PatchRequest) repository.MasterDataPatch {
	return repository.MasterDataPatch{
		CompanyName:     req.CompanyName,
		Stage:           req.Stage,
		PICName:         req.PICName,
		PICNickname:     req.PICNickname,
		PICRole:         req.PICRole,
		PICWA:           req.PICWA,
		PICEmail:        req.PICEmail,
		OwnerName:       req.OwnerName,
		OwnerWA:         req.OwnerWA,
		OwnerTelegramID: req.OwnerTelegramID,
		BotActive:       req.BotActive,
		Blacklisted:     req.Blacklisted,
		SequenceStatus:  req.SequenceStatus,
		RiskFlag:        req.RiskFlag,
		ContractStart:   req.ContractStart,
		ContractEnd:     req.ContractEnd,
		ContractMonths:  req.ContractMonths,
		PaymentStatus:   req.PaymentStatus,
		PaymentTerms:    req.PaymentTerms,
		FinalPrice:      req.FinalPrice,
		LastPaymentDate: req.LastPaymentDate,
		Renewed:         req.Renewed,
		Notes:           req.Notes,
		CustomFields:    req.CustomFields,
	}
}

// IsProtectedField reports whether the named core field is bot-write-protected.
func IsProtectedField(name string) bool {
	_, ok := protectedFields[name]
	return ok
}

// diffMutation produces a coarse diff for the mutation log — values of fields
// that were present in the patch.
func diffMutation(prev, curr *entity.MasterData, req PatchRequest) ([]string, map[string]any, map[string]any) {
	changed := []string{}
	prevValues := map[string]any{}
	newValues := map[string]any{}
	if req.Stage != nil && prev.Stage != curr.Stage {
		changed = append(changed, "stage")
		prevValues["stage"] = prev.Stage
		newValues["stage"] = curr.Stage
	}
	if req.BotActive != nil && prev.BotActive != curr.BotActive {
		changed = append(changed, "bot_active")
		prevValues["bot_active"] = prev.BotActive
		newValues["bot_active"] = curr.BotActive
	}
	if req.Blacklisted != nil && prev.Blacklisted != curr.Blacklisted {
		changed = append(changed, "blacklisted")
		prevValues["blacklisted"] = prev.Blacklisted
		newValues["blacklisted"] = curr.Blacklisted
	}
	if req.RiskFlag != nil && prev.RiskFlag != curr.RiskFlag {
		changed = append(changed, "risk_flag")
		prevValues["risk_flag"] = prev.RiskFlag
		newValues["risk_flag"] = curr.RiskFlag
	}
	if req.CustomFields != nil {
		for k, v := range req.CustomFields {
			changed = append(changed, "custom_fields."+k)
			if prev.CustomFields != nil {
				prevValues["custom_fields."+k] = prev.CustomFields[k]
			}
			newValues["custom_fields."+k] = v
		}
	}
	return changed, prevValues, newValues
}

// MarshalCustomFields is a small helper used by tests/handlers.
func MarshalCustomFields(m map[string]any) string {
	if m == nil {
		return "{}"
	}
	b, _ := json.Marshal(m)
	return string(b)
}
