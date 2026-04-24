package reactivation

import (
	"context"
	"testing"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
)

type stubRepo struct {
	triggers   []entity.ReactivationTrigger
	upserted   *entity.ReactivationTrigger
	event      *entity.ReactivationEvent
	recentCnt  int
}

func (s *stubRepo) UpsertTrigger(ctx context.Context, t *entity.ReactivationTrigger) (*entity.ReactivationTrigger, error) {
	s.upserted = t
	out := *t
	out.ID = "rt-new"
	return &out, nil
}
func (s *stubRepo) GetTrigger(ctx context.Context, workspaceID, id string) (*entity.ReactivationTrigger, error) {
	for _, t := range s.triggers {
		if t.ID == id {
			return &t, nil
		}
	}
	return nil, nil
}
func (s *stubRepo) ListTriggers(ctx context.Context, workspaceID string, activeOnly bool) ([]entity.ReactivationTrigger, error) {
	return s.triggers, nil
}
func (s *stubRepo) DeleteTrigger(ctx context.Context, workspaceID, id string) error { return nil }
func (s *stubRepo) RecordEvent(ctx context.Context, e *entity.ReactivationEvent) (*entity.ReactivationEvent, error) {
	s.event = e
	out := *e
	out.ID = "re-1"
	return &out, nil
}
func (s *stubRepo) ListEventsForMasterData(ctx context.Context, workspaceID, masterDataID string, limit int) ([]entity.ReactivationEvent, error) {
	return nil, nil
}
func (s *stubRepo) CountRecentForClient(ctx context.Context, workspaceID, masterDataID, triggerCode string, within time.Duration) (int, error) {
	return s.recentCnt, nil
}

func TestReactivate_ManualBypassesRateLimit(t *testing.T) {
	s := &stubRepo{
		triggers:  []entity.ReactivationTrigger{{ID: "rt-manual", Code: entity.ReactivationCodeManual}},
		recentCnt: 5,
	}
	uc := New(s)
	out, err := uc.Reactivate(context.Background(), ReactivateRequest{
		WorkspaceID:  "ws-1",
		MasterDataID: "md-1",
		Note:         "ad-hoc outreach",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if out.TriggerID != "rt-manual" {
		t.Errorf("expected manual trigger, got %q", out.TriggerID)
	}
}

func TestReactivate_NonManualBlockedByRateLimit(t *testing.T) {
	s := &stubRepo{
		triggers:  []entity.ReactivationTrigger{{ID: "rt-price", Code: entity.ReactivationCodePriceChange}},
		recentCnt: 1,
	}
	uc := New(s)
	_, err := uc.Reactivate(context.Background(), ReactivateRequest{
		WorkspaceID:  "ws-1",
		MasterDataID: "md-1",
		TriggerCode:  entity.ReactivationCodePriceChange,
	})
	if err == nil {
		t.Fatal("expected rate-limit error")
	}
	if ae := apperror.GetAppError(err); ae == nil || ae.HTTPStatus != 409 {
		t.Fatalf("expected 409 Conflict, got %v", err)
	}
}

func TestReactivate_AutoCreatesManualTrigger(t *testing.T) {
	s := &stubRepo{triggers: nil}
	uc := New(s)
	_, err := uc.Reactivate(context.Background(), ReactivateRequest{
		WorkspaceID:  "ws-1",
		MasterDataID: "md-1",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if s.upserted == nil || s.upserted.Code != entity.ReactivationCodeManual {
		t.Errorf("expected manual trigger auto-created")
	}
}
