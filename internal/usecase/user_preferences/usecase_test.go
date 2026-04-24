package userpreferences

import (
	"context"
	"strings"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
)

type stubRepo struct {
	getRes    *entity.UserPreference
	listRes   []entity.UserPreference
	upsertRes *entity.UserPreference
	upsertArg *entity.UserPreference
	deleteErr error
	getErr    error
}

func (s *stubRepo) Get(ctx context.Context, workspaceID, userEmail, namespace string) (*entity.UserPreference, error) {
	return s.getRes, s.getErr
}
func (s *stubRepo) List(ctx context.Context, workspaceID, userEmail string) ([]entity.UserPreference, error) {
	return s.listRes, nil
}
func (s *stubRepo) Upsert(ctx context.Context, p *entity.UserPreference) (*entity.UserPreference, error) {
	s.upsertArg = p
	if s.upsertRes != nil {
		return s.upsertRes, nil
	}
	out := *p
	out.ID = "pref-1"
	return &out, nil
}
func (s *stubRepo) Delete(ctx context.Context, workspaceID, userEmail, namespace string) error {
	return s.deleteErr
}

func TestUpsert_NormalizesEmailAndDefaultsValue(t *testing.T) {
	s := &stubRepo{}
	uc := New(s)

	out, err := uc.Upsert(context.Background(), UpsertRequest{
		WorkspaceID: "ws-1",
		UserEmail:   "Foo@Bar.com",
		Namespace:   "theme",
		Value:       nil,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Namespace != "theme" {
		t.Errorf("namespace not set: %v", out.Namespace)
	}
	if s.upsertArg.UserEmail != "foo@bar.com" {
		t.Errorf("expected lowercased email, got %q", s.upsertArg.UserEmail)
	}
	if s.upsertArg.Value == nil {
		t.Errorf("expected non-nil value")
	}
}

func TestUpsert_ValidationErrors(t *testing.T) {
	uc := New(&stubRepo{})

	cases := []struct {
		name    string
		req     UpsertRequest
		wantMsg string
	}{
		{"no workspace", UpsertRequest{UserEmail: "x@y.z", Namespace: "ns"}, "workspace_id"},
		{"no email", UpsertRequest{WorkspaceID: "ws", Namespace: "ns"}, "user_email"},
		{"no namespace", UpsertRequest{WorkspaceID: "ws", UserEmail: "x@y.z"}, "namespace"},
		{"namespace too long", UpsertRequest{WorkspaceID: "ws", UserEmail: "x@y.z", Namespace: strings.Repeat("a", 129)}, "<= 128"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := uc.Upsert(context.Background(), tc.req)
			if err == nil {
				t.Fatalf("expected error")
			}
			if ae := apperror.GetAppError(err); ae == nil {
				t.Fatalf("expected AppError, got %v", err)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantMsg)
			}
		})
	}
}

func TestGet_NotFoundWhenRepoReturnsNil(t *testing.T) {
	uc := New(&stubRepo{getRes: nil})
	_, err := uc.Get(context.Background(), "ws-1", "a@b.c", "theme")
	if err == nil {
		t.Fatal("expected not found error")
	}
	if !apperror.IsNotFound(err) {
		t.Errorf("expected NotFound, got %v", err)
	}
}
