package claudeextraction

import (
	"context"
	"errors"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/rs/zerolog"
)

type stubRepo struct {
	row      *entity.ClaudeExtraction
	inserted *entity.ClaudeExtraction
	updated  *entity.ClaudeExtraction
}

func (s *stubRepo) Insert(ctx context.Context, e *entity.ClaudeExtraction) (*entity.ClaudeExtraction, error) {
	s.inserted = e
	out := *e
	out.ID = "ce-1"
	out.Status = entity.ClaudeExtractionStatusPending
	return &out, nil
}
func (s *stubRepo) GetByID(ctx context.Context, workspaceID, id string) (*entity.ClaudeExtraction, error) {
	return s.row, nil
}
func (s *stubRepo) ListBySource(ctx context.Context, workspaceID, sourceType, sourceID string) ([]entity.ClaudeExtraction, error) {
	return nil, nil
}
func (s *stubRepo) ListForMasterData(ctx context.Context, workspaceID, masterDataID string, limit int) ([]entity.ClaudeExtraction, error) {
	return nil, nil
}
func (s *stubRepo) UpdateResult(ctx context.Context, e *entity.ClaudeExtraction) error {
	s.updated = e
	return nil
}
func (s *stubRepo) MarkSuperseded(ctx context.Context, workspaceID, sourceType, sourceID, keepID string) error {
	return nil
}

type stubClient struct {
	res *Result
	err error
}

func (c *stubClient) Extract(ctx context.Context, transcriptText string, hints map[string]any) (*Result, error) {
	return c.res, c.err
}

func intPtr(v int) *int             { return &v }
func floatPtr(v float64) *float64   { return &v }

func TestStart_InsertsPendingRow(t *testing.T) {
	s := &stubRepo{}
	uc := New(s, &stubClient{}, zerolog.Nop())
	out, err := uc.Start(context.Background(), StartRequest{
		WorkspaceID: "ws-1",
		SourceType:  entity.ClaudeSourceFireflies,
		SourceID:    "ff-1",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if out.Status != entity.ClaudeExtractionStatusPending {
		t.Errorf("expected pending, got %q", out.Status)
	}
}

func TestRun_NilClientMarksFailed(t *testing.T) {
	s := &stubRepo{row: &entity.ClaudeExtraction{
		ID: "ce-1", WorkspaceID: "ws-1",
		Status: entity.ClaudeExtractionStatusPending,
	}}
	uc := New(s, nil, zerolog.Nop())
	out, err := uc.Run(context.Background(), "ws-1", "ce-1", "transcript", nil)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if out.Status != entity.ClaudeExtractionStatusFailed {
		t.Errorf("expected failed, got %q", out.Status)
	}
	if out.ErrorMessage == "" {
		t.Error("expected error message")
	}
}

func TestRun_Success_WritesResult(t *testing.T) {
	s := &stubRepo{row: &entity.ClaudeExtraction{
		ID: "ce-1", WorkspaceID: "ws-1",
		Status: entity.ClaudeExtractionStatusPending,
	}}
	c := &stubClient{
		res: &Result{
			Fields:              map[string]any{"hc_size": 250},
			Model:               "claude-sonnet-4-6",
			BANTSBudget:         intPtr(4),
			BANTSAuthority:      intPtr(3),
			BANTSNeed:           intPtr(5),
			BANTSTiming:         intPtr(4),
			BANTSSentiment:      intPtr(4),
			BANTSScore:          floatPtr(80),
			BANTSClassification: "Hot",
			BuyingIntent:        "HIGH",
			PromptTokens:        1234,
			CompletionTokens:    567,
		},
	}
	uc := New(s, c, zerolog.Nop())
	out, err := uc.Run(context.Background(), "ws-1", "ce-1", "transcript", nil)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if out.Status != entity.ClaudeExtractionStatusSucceeded {
		t.Errorf("expected succeeded, got %q", out.Status)
	}
	if out.BANTSClassification != "hot" {
		t.Errorf("classification not lowercased: %q", out.BANTSClassification)
	}
	if out.BuyingIntent != "high" {
		t.Errorf("buying intent not lowercased: %q", out.BuyingIntent)
	}
	if s.updated == nil {
		t.Error("expected repo.UpdateResult to be called")
	}
}

func TestRun_ClientErrorPersistsFailure(t *testing.T) {
	s := &stubRepo{row: &entity.ClaudeExtraction{
		ID: "ce-1", WorkspaceID: "ws-1",
		Status: entity.ClaudeExtractionStatusPending,
	}}
	c := &stubClient{err: errors.New("rate limited")}
	uc := New(s, c, zerolog.Nop())
	out, err := uc.Run(context.Background(), "ws-1", "ce-1", "transcript", nil)
	if err == nil {
		t.Fatal("expected error from client")
	}
	if out.Status != entity.ClaudeExtractionStatusFailed {
		t.Errorf("expected failed status, got %q", out.Status)
	}
}
