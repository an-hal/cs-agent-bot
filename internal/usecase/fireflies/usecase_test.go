package fireflies

import (
	"context"
	"testing"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/rs/zerolog"
)

type stubRepo struct {
	existing    *entity.FirefliesTranscript
	inserted    *entity.FirefliesTranscript
	insertCalls int
}

func (s *stubRepo) Insert(ctx context.Context, t *entity.FirefliesTranscript) (*entity.FirefliesTranscript, error) {
	s.insertCalls++
	s.inserted = t
	out := *t
	out.ID = "ff-1"
	out.CreatedAt = time.Now()
	out.UpdatedAt = out.CreatedAt
	out.ExtractionStatus = entity.FirefliesStatusPending
	return &out, nil
}
func (s *stubRepo) GetByID(ctx context.Context, workspaceID, id string) (*entity.FirefliesTranscript, error) {
	return s.existing, nil
}
func (s *stubRepo) GetByFirefliesID(ctx context.Context, workspaceID, firefliesID string) (*entity.FirefliesTranscript, error) {
	return s.existing, nil
}
func (s *stubRepo) List(ctx context.Context, workspaceID, status string, limit, offset int) ([]entity.FirefliesTranscript, int64, error) {
	return nil, 0, nil
}
func (s *stubRepo) UpdateExtraction(ctx context.Context, workspaceID, id, status, errMsg, masterDataID string) error {
	return nil
}

type extractorSpy struct{ calls int }

func (e *extractorSpy) ExtractFromFireflies(ctx context.Context, workspaceID, transcriptID string) error {
	e.calls++
	return nil
}

func TestIngestWebhook_IdempotentOnExistingFirefliesID(t *testing.T) {
	s := &stubRepo{
		existing: &entity.FirefliesTranscript{ID: "ff-existing", WorkspaceID: "ws-1", FirefliesID: "abc"},
	}
	uc := New(s, nil, zerolog.Nop())
	out, err := uc.IngestWebhook(context.Background(), IngestWebhookRequest{
		WorkspaceID: "ws-1",
		FirefliesID: "abc",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if out.ID != "ff-existing" {
		t.Errorf("expected existing row, got %q", out.ID)
	}
	if s.insertCalls != 0 {
		t.Errorf("expected no insert, got %d", s.insertCalls)
	}
}

func TestIngestWebhook_InsertsAndTriggersExtractor(t *testing.T) {
	s := &stubRepo{}
	e := &extractorSpy{}
	uc := New(s, e, zerolog.Nop())
	_, err := uc.IngestWebhook(context.Background(), IngestWebhookRequest{
		WorkspaceID:    "ws-1",
		FirefliesID:    "abc-123",
		MeetingTitle:   "Discovery call",
		MeetingDateISO: "2026-04-23T10:00:00Z",
		HostEmail:      "BD@kantorku.id",
		Participants:   []string{"bd@kantorku.id", "client@acme.com"},
		TranscriptText: "...",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if s.insertCalls != 1 {
		t.Errorf("expected 1 insert, got %d", s.insertCalls)
	}
	if s.inserted.MeetingDate == nil {
		t.Error("expected meeting_date to parse")
	}
	if s.inserted.HostEmail != "bd@kantorku.id" {
		t.Errorf("host email not lowercased: %q", s.inserted.HostEmail)
	}
	// extractor runs in a goroutine — give it a beat.
	time.Sleep(50 * time.Millisecond)
	if e.calls != 1 {
		t.Errorf("expected extractor called once, got %d", e.calls)
	}
}
