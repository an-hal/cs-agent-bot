package main

import (
	"context"

	"github.com/Sejutacita/cs-agent-bot/internal/usecase/cron"
	haloaimock "github.com/Sejutacita/cs-agent-bot/internal/usecase/haloai_mock"
	manualactionuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/manual_action"
	masterdatauc "github.com/Sejutacita/cs-agent-bot/internal/usecase/master_data"
	workspaceintegrationuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/workspace_integration"
)

// stageApproverAdapter wraps master_data.Usecase.ApplyApprovedStageTransition
// into the narrow `(any, error)` shape the central approval dispatcher expects.
type stageApproverAdapter struct{ uc masterdatauc.Usecase }

func (a *stageApproverAdapter) ApplyApprovedStageTransition(ctx context.Context, workspaceID, approvalID, checkerEmail string) (any, error) {
	return a.uc.ApplyApprovedStageTransition(ctx, workspaceID, approvalID, checkerEmail)
}

// integrationApproverAdapter wraps workspace_integration.Usecase similarly.
type integrationApproverAdapter struct{ uc workspaceintegrationuc.Usecase }

func (a *integrationApproverAdapter) ApplyApprovedKeyChange(ctx context.Context, workspaceID, approvalID, checkerEmail string) (any, error) {
	return a.uc.ApplyApprovedKeyChange(ctx, workspaceID, approvalID, checkerEmail)
}

// mockWASenderAdapter bridges cron.WASendRequest → haloaimock.SendRequest so
// the cron dispatcher's narrow port doesn't depend on the mock package. In
// production, a real HaloAI adapter would sit here instead.
type mockWASenderAdapter struct {
	mock haloaimock.Sender
}

func (a *mockWASenderAdapter) Send(ctx context.Context, req cron.WASendRequest) (string, error) {
	resp, err := a.mock.Send(ctx, haloaimock.SendRequest{
		WorkspaceID: req.WorkspaceID,
		To:          req.To,
		TemplateID:  req.TemplateID,
		Body:        req.Body,
		Variables:   req.Variables,
	})
	if err != nil {
		return "", err
	}
	return resp.MessageID, nil
}

// cronManualActionAdapter bridges cron.ManualActionEnqueueInput (what the
// dispatcher speaks) to manualactionuc.CreatePendingInput (what the usecase
// accepts). Same field names; kept here so neither package imports the
// other.
type cronManualActionAdapter struct {
	uc manualactionuc.Usecase
}

func (a *cronManualActionAdapter) Enqueue(ctx context.Context, in cron.ManualActionEnqueueInput) error {
	if a == nil || a.uc == nil {
		return nil
	}
	_, err := a.uc.CreatePending(ctx, manualactionuc.CreatePendingInput{
		WorkspaceID:    in.WorkspaceID,
		MasterDataID:   in.MasterDataID,
		TriggerID:      in.TriggerID,
		FlowCategory:   in.FlowCategory,
		Role:           in.Role,
		AssignedToUser: in.AssignedToUser,
		Priority:       in.Priority,
		DueAt:          in.DueAt,
		SuggestedDraft: in.SuggestedDraft,
		ContextSummary: in.ContextSummary,
	})
	return err
}
