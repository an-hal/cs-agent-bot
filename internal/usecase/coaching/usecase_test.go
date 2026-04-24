package coaching

import (
	"context"
	"testing"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

type stubRepo struct {
	row      *entity.CoachingSession
	updated  *entity.CoachingSession
	inserted *entity.CoachingSession
}

func (s *stubRepo) Insert(ctx context.Context, c *entity.CoachingSession) (*entity.CoachingSession, error) {
	s.inserted = c
	out := *c
	out.ID = "cs-1"
	out.Status = entity.CoachingStatusDraft
	out.CreatedAt = time.Now()
	out.UpdatedAt = out.CreatedAt
	return &out, nil
}
func (s *stubRepo) GetByID(ctx context.Context, workspaceID, id string) (*entity.CoachingSession, error) {
	return s.row, nil
}
func (s *stubRepo) List(ctx context.Context, f entity.CoachingSessionFilter) ([]entity.CoachingSession, int64, error) {
	return nil, 0, nil
}
func (s *stubRepo) Update(ctx context.Context, c *entity.CoachingSession) (*entity.CoachingSession, error) {
	s.updated = c
	return c, nil
}
func (s *stubRepo) Delete(ctx context.Context, workspaceID, id string) error { return nil }

func intPtr(v int) *int { return &v }

func TestCreate_LowercasesEmails(t *testing.T) {
	s := &stubRepo{}
	uc := New(s)
	_, err := uc.Create(context.Background(), CreateRequest{
		WorkspaceID: "ws-1",
		BDEmail:     "BD@Example.com",
		CoachEmail:  "Coach@Example.com",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if s.inserted.BDEmail != "bd@example.com" || s.inserted.CoachEmail != "coach@example.com" {
		t.Errorf("emails not lowercased")
	}
}

func TestUpdate_ScoresValidationAndAverage(t *testing.T) {
	s := &stubRepo{row: &entity.CoachingSession{
		ID: "cs-1", WorkspaceID: "ws-1", Status: entity.CoachingStatusDraft,
	}}
	uc := New(s)
	_, err := uc.Update(context.Background(), UpdateRequest{
		WorkspaceID: "ws-1", ID: "cs-1",
		BANTSClarityScore:   intPtr(4),
		DiscoveryDepthScore: intPtr(5),
		ToneFitScore:        intPtr(3),
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if s.updated.OverallScore == nil {
		t.Fatal("expected overall score computed")
	}
	if avg := *s.updated.OverallScore; avg != 4.0 {
		t.Errorf("expected overall=4.0 (avg of 4,5,3), got %v", avg)
	}
}

func TestUpdate_OutOfRangeRejected(t *testing.T) {
	s := &stubRepo{row: &entity.CoachingSession{ID: "cs-1", WorkspaceID: "ws-1", Status: entity.CoachingStatusDraft}}
	uc := New(s)
	_, err := uc.Update(context.Background(), UpdateRequest{
		WorkspaceID: "ws-1", ID: "cs-1",
		BANTSClarityScore: intPtr(10),
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestSubmit_RequiresScoredRow(t *testing.T) {
	s := &stubRepo{row: &entity.CoachingSession{ID: "cs-1", WorkspaceID: "ws-1", Status: entity.CoachingStatusDraft}}
	uc := New(s)
	_, err := uc.Submit(context.Background(), "ws-1", "cs-1", "coach@example.com")
	if err == nil {
		t.Fatal("expected error for unscored row")
	}
}
