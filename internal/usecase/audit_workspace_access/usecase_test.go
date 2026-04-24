package auditworkspaceaccess

import (
	"context"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
)

type stubRepo struct {
	inserted *entity.AuditWorkspaceAccess
}

func (s *stubRepo) Insert(ctx context.Context, a *entity.AuditWorkspaceAccess) (*entity.AuditWorkspaceAccess, error) {
	s.inserted = a
	out := *a
	out.ID = "aud-1"
	return &out, nil
}
func (s *stubRepo) List(ctx context.Context, f entity.AuditWorkspaceAccessFilter) ([]entity.AuditWorkspaceAccess, int64, error) {
	return nil, 0, nil
}

func TestRecord_ValidatesAndLowercasesActor(t *testing.T) {
	s := &stubRepo{}
	uc := New(s)
	out, err := uc.Record(context.Background(), RecordRequest{
		WorkspaceID: "ws-1",
		ActorEmail:  "Actor@EXAMPLE.com",
		Kind:        entity.WorkspaceAccessKindRead,
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if out.ID != "aud-1" {
		t.Errorf("unexpected id: %s", out.ID)
	}
	if s.inserted.ActorEmail != "actor@example.com" {
		t.Errorf("actor not lowercased: %q", s.inserted.ActorEmail)
	}
}

func TestRecord_RejectsInvalidKind(t *testing.T) {
	uc := New(&stubRepo{})
	_, err := uc.Record(context.Background(), RecordRequest{
		WorkspaceID: "ws-1",
		ActorEmail:  "a@b.c",
		Kind:        "sudo",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if ae := apperror.GetAppError(err); ae == nil {
		t.Fatalf("expected AppError: %v", err)
	}
}
