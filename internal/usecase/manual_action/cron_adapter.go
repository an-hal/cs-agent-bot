package manualaction

import (
	"context"
	"time"
)

// CronEnqueueInput mirrors cron.ManualActionEnqueueInput without importing
// the cron package (keeps dep graph acyclic — cron → manualaction, not both ways).
// The fields match one-for-one; adapter in main.go converts the cron DTO into
// our internal CreatePendingInput.
type CronEnqueueInput struct {
	WorkspaceID    string
	MasterDataID   string
	TriggerID      string
	FlowCategory   string
	Role           string
	AssignedToUser string
	Priority       string
	DueAt          time.Time
	SuggestedDraft string
	ContextSummary map[string]any
}

// CronEnqueuer is the tiny interface the cron dispatcher calls. We implement
// it here so the dep flows cron → manualaction.
type CronEnqueuer struct {
	uc Usecase
}

// NewCronEnqueuer returns an enqueuer backed by the given usecase.
func NewCronEnqueuer(uc Usecase) *CronEnqueuer {
	return &CronEnqueuer{uc: uc}
}

// Enqueue converts the DTO into a CreatePendingInput and calls the usecase.
func (e *CronEnqueuer) Enqueue(ctx context.Context, in CronEnqueueInput) error {
	if e == nil || e.uc == nil {
		return nil
	}
	_, err := e.uc.CreatePending(ctx, CreatePendingInput{
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
