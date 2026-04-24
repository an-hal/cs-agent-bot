package workspaceintegration

import (
	"context"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
)

type stubRepo struct {
	getRes    *entity.WorkspaceIntegration
	listRes   []entity.WorkspaceIntegration
	upsertArg *entity.WorkspaceIntegration
}

func (s *stubRepo) GetByProvider(ctx context.Context, workspaceID, provider string) (*entity.WorkspaceIntegration, error) {
	return s.getRes, nil
}
func (s *stubRepo) List(ctx context.Context, workspaceID string) ([]entity.WorkspaceIntegration, error) {
	return s.listRes, nil
}
func (s *stubRepo) Upsert(ctx context.Context, w *entity.WorkspaceIntegration) (*entity.WorkspaceIntegration, error) {
	s.upsertArg = w
	out := *w
	out.ID = "int-1"
	return &out, nil
}
func (s *stubRepo) Delete(ctx context.Context, workspaceID, provider string) error { return nil }

func TestGet_RedactsSecrets(t *testing.T) {
	s := &stubRepo{
		getRes: &entity.WorkspaceIntegration{
			ID:          "int-1",
			WorkspaceID: "ws-1",
			Provider:    entity.IntegrationProviderHaloAI,
			Config: map[string]any{
				"api_url":      "https://halo.example",
				"wa_api_token": "super-secret",
				"api_key":      "kkk",
				"Business_ID":  "biz-123",
			},
		},
	}
	uc := New(s)
	out, err := uc.Get(context.Background(), "ws-1", entity.IntegrationProviderHaloAI)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if out.Config["wa_api_token"] != RedactedMarker {
		t.Errorf("wa_api_token not redacted: %v", out.Config["wa_api_token"])
	}
	if out.Config["api_key"] != RedactedMarker {
		t.Errorf("api_key not redacted: %v", out.Config["api_key"])
	}
	if out.Config["api_url"] != "https://halo.example" {
		t.Errorf("api_url should not be redacted")
	}
	if out.Config["Business_ID"] != "biz-123" {
		t.Errorf("non-secret key should not be redacted")
	}
}

func TestUpsert_RejectsRedactedPlaceholder(t *testing.T) {
	uc := New(&stubRepo{})
	_, err := uc.Upsert(context.Background(), UpsertRequest{
		WorkspaceID: "ws-1",
		Provider:    entity.IntegrationProviderHaloAI,
		Config:      map[string]any{"wa_api_token": RedactedMarker},
	})
	if err == nil {
		t.Fatal("expected error for redacted placeholder")
	}
	if ae := apperror.GetAppError(err); ae == nil {
		t.Fatalf("expected AppError, got %v", err)
	}
}

func TestUpsert_RejectsUnknownProvider(t *testing.T) {
	uc := New(&stubRepo{})
	_, err := uc.Upsert(context.Background(), UpsertRequest{
		WorkspaceID: "ws-1",
		Provider:    "unknown",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpsert_DefaultsActive(t *testing.T) {
	s := &stubRepo{}
	uc := New(s)
	_, err := uc.Upsert(context.Background(), UpsertRequest{
		WorkspaceID: "ws-1",
		Provider:    entity.IntegrationProviderTelegram,
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !s.upsertArg.IsActive {
		t.Errorf("expected IsActive=true by default")
	}
}
