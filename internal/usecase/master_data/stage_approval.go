package master_data

import (
	"context"
	"fmt"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
)

// RequestStageTransition creates an approval_request rather than executing the
// stage change directly. Used when the transition crosses a gated boundary
// (e.g. prospect→client which affects handoff + contract fields).
func (u *usecase) RequestStageTransition(
	ctx context.Context,
	workspaceID, clientID, actorEmail string,
	req TransitionRequest,
) (*entity.ApprovalRequest, error) {
	if workspaceID == "" || clientID == "" {
		return nil, apperror.ValidationError("workspace_id and client_id required")
	}
	if !isValidStage(req.NewStage) {
		return nil, apperror.ValidationError("invalid new_stage")
	}
	curr, err := u.repo.GetByID(ctx, workspaceID, clientID)
	if err != nil {
		return nil, err
	}
	if curr == nil {
		return nil, apperror.NotFound("master_data", clientID)
	}
	desc := fmt.Sprintf("Stage transition for %s (%s): %s → %s",
		curr.CompanyName, curr.CompanyID, curr.Stage, req.NewStage)
	payload := map[string]any{
		"client_id":      clientID,
		"from_stage":     curr.Stage,
		"to_stage":       req.NewStage,
		"reason":         req.Reason,
		"updates":        req.Updates,
		"custom_updates": req.CustomFieldUpdates,
	}
	return u.approvalRepo.Create(ctx, &entity.ApprovalRequest{
		WorkspaceID: workspaceID,
		RequestType: entity.ApprovalTypeStageTransition,
		Description: desc,
		Payload:     payload,
		MakerEmail:  actorEmail,
	})
}

// ApplyApprovedStageTransition executes a pending stage_transition approval.
// Same semantics as Transition() but gated by the checker-maker flow.
func (u *usecase) ApplyApprovedStageTransition(ctx context.Context, workspaceID, approvalID, checkerEmail string) (*TransitionResult, error) {
	if workspaceID == "" || approvalID == "" {
		return nil, apperror.ValidationError("workspace_id and approval_id required")
	}
	ar, err := u.approvalRepo.GetByID(ctx, workspaceID, approvalID)
	if err != nil {
		return nil, err
	}
	if ar == nil || ar.RequestType != entity.ApprovalTypeStageTransition {
		return nil, apperror.NotFound("stage_transition approval", approvalID)
	}
	if ar.Status != entity.ApprovalStatusPending {
		return nil, apperror.BadRequest("approval not pending (status=" + ar.Status + ")")
	}
	if ar.MakerEmail == checkerEmail {
		return nil, apperror.BadRequest("cannot approve your own request")
	}
	clientID, _ := ar.Payload["client_id"].(string)
	toStage, _ := ar.Payload["to_stage"].(string)
	reason, _ := ar.Payload["reason"].(string)
	if clientID == "" || toStage == "" {
		return nil, apperror.BadRequest("approval payload missing client_id or to_stage")
	}
	// Reconstruct custom field updates (if any).
	var customUpdates map[string]any
	if cu, ok := ar.Payload["custom_updates"].(map[string]any); ok {
		customUpdates = cu
	}
	// Core patch reconstruction is non-trivial (pointer types) — apply only
	// stage + custom_fields here; richer patches can be added in follow-up.
	out, err := u.Transition(ctx, workspaceID, clientID, checkerEmail, WriteContextDashboardUser, TransitionRequest{
		NewStage:           toStage,
		Reason:             reason,
		CustomFieldUpdates: customUpdates,
	})
	if err != nil {
		return nil, err
	}
	if err := u.approvalRepo.UpdateStatus(ctx, workspaceID, approvalID, entity.ApprovalStatusApproved, checkerEmail, ""); err != nil {
		return nil, fmt.Errorf("mark approved: %w", err)
	}
	return out, nil
}
