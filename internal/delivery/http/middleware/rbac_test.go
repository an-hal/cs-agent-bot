package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/team"
)

// stubTeamUC implements team.Usecase with only CheckPermission exercised.
type stubTeamUC struct {
	allowed  bool
	resolved *entity.ResolvedPermission
	err      error
	calls    int
}

func (s *stubTeamUC) CheckPermission(ctx context.Context, callerEmail, workspaceID, module, action string) (bool, *entity.ResolvedPermission, error) {
	s.calls++
	return s.allowed, s.resolved, s.err
}

// unused methods — required to satisfy team.Usecase
func (s *stubTeamUC) ListMembers(ctx context.Context, f repository.TeamMemberFilter) (*team.ListMembersResult, error) {
	return nil, nil
}
func (s *stubTeamUC) GetMember(ctx context.Context, id string) (*team.MemberDetail, error) {
	return nil, nil
}
func (s *stubTeamUC) InviteMember(ctx context.Context, caller string, req team.InviteRequest) (*entity.TeamMember, error) {
	return nil, nil
}
func (s *stubTeamUC) UpdateMember(ctx context.Context, id string, req team.UpdateMemberRequest) (*entity.TeamMember, error) {
	return nil, nil
}
func (s *stubTeamUC) ChangeRole(ctx context.Context, caller, id, roleID string) (*entity.TeamMember, error) {
	return nil, nil
}
func (s *stubTeamUC) ChangeStatus(ctx context.Context, id, status string) (*entity.TeamMember, error) {
	return nil, nil
}
func (s *stubTeamUC) UpdateMemberWorkspaces(ctx context.Context, caller, id string, ws []string) (*entity.TeamMember, error) {
	return nil, nil
}
func (s *stubTeamUC) RemoveMember(ctx context.Context, caller, id string) error { return nil }
func (s *stubTeamUC) AcceptInvitation(ctx context.Context, token, userID string) (*entity.TeamMember, error) {
	return nil, nil
}
func (s *stubTeamUC) ListRoles(ctx context.Context, ws string) ([]team.RoleSummary, error) {
	return nil, nil
}
func (s *stubTeamUC) GetRole(ctx context.Context, id, ws string) (*team.RoleDetail, error) {
	return nil, nil
}
func (s *stubTeamUC) CreateRole(ctx context.Context, req team.CreateRoleRequest) (*entity.Role, error) {
	return nil, nil
}
func (s *stubTeamUC) UpdateRole(ctx context.Context, id string, req team.UpdateRoleRequest) (*entity.Role, error) {
	return nil, nil
}
func (s *stubTeamUC) UpdateRolePermissions(ctx context.Context, caller, id string, req team.UpdateRolePermissionRequest) (*team.PermissionChange, error) {
	return nil, nil
}
func (s *stubTeamUC) DeleteRole(ctx context.Context, id string) error { return nil }
func (s *stubTeamUC) GetMyPermissions(ctx context.Context, caller, ws string) (*team.MyPermissions, error) {
	return nil, nil
}

// ───── helpers ──────────────────────────────────────────────────────────────

func newRequestWithCtx(email, workspaceID string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	ctx := r.Context()
	if email != "" {
		ctx = WithJWTUser(ctx, JWTUser{Email: email, ID: "u-1"})
	}
	if workspaceID != "" {
		ctx = ctxutil.SetWorkspaceID(ctx, workspaceID)
	}
	return r.WithContext(ctx)
}

// ───── tests ────────────────────────────────────────────────────────────────

func TestRequirePermission_MissingUser(t *testing.T) {
	uc := &stubTeamUC{allowed: true}
	mw := RequirePermission(entity.ModuleAE, entity.ActionViewList, uc)
	handler := mw(func(w http.ResponseWriter, r *http.Request) error { return nil })

	r := newRequestWithCtx("", "ws-1")
	err := handler(httptest.NewRecorder(), r)

	var ae *apperror.AppError
	if !errors.As(err, &ae) || ae.Code != apperror.CodeUnauthorized {
		t.Errorf("want unauthorized, got %v", err)
	}
	if uc.calls != 0 {
		t.Errorf("CheckPermission should not be called when user missing")
	}
}

func TestRequirePermission_MissingWorkspace(t *testing.T) {
	uc := &stubTeamUC{allowed: true}
	mw := RequirePermission(entity.ModuleAE, entity.ActionViewList, uc)
	handler := mw(func(w http.ResponseWriter, r *http.Request) error { return nil })

	r := newRequestWithCtx("alice@x.io", "")
	err := handler(httptest.NewRecorder(), r)

	var ae *apperror.AppError
	if !errors.As(err, &ae) || ae.Code != apperror.CodeBadRequest {
		t.Errorf("want bad_request, got %v", err)
	}
}

func TestRequirePermission_Denied(t *testing.T) {
	uc := &stubTeamUC{allowed: false, resolved: &entity.ResolvedPermission{ViewList: entity.ViewScopeFalse}}
	mw := RequirePermission(entity.ModuleTeam, entity.ActionDelete, uc)
	called := false
	handler := mw(func(w http.ResponseWriter, r *http.Request) error {
		called = true
		return nil
	})

	r := newRequestWithCtx("alice@x.io", "ws-1")
	err := handler(httptest.NewRecorder(), r)

	var ae *apperror.AppError
	if !errors.As(err, &ae) || ae.Code != apperror.CodeForbidden {
		t.Errorf("want forbidden, got %v", err)
	}
	if called {
		t.Errorf("downstream handler should not run on deny")
	}
}

func TestRequirePermission_AllowedInjectsScope(t *testing.T) {
	uc := &stubTeamUC{
		allowed:  true,
		resolved: &entity.ResolvedPermission{ViewList: entity.ViewScopeAll},
	}
	mw := RequirePermission(entity.ModuleAE, entity.ActionViewList, uc)

	var gotScope string
	handler := mw(func(w http.ResponseWriter, r *http.Request) error {
		gotScope = GetScope(r.Context())
		return nil
	})

	r := newRequestWithCtx("alice@x.io", "ws-1")
	if err := handler(httptest.NewRecorder(), r); err != nil {
		t.Fatalf("err: %v", err)
	}
	if gotScope != entity.ViewScopeAll {
		t.Errorf("scope = %q, want %q", gotScope, entity.ViewScopeAll)
	}
}

func TestRequirePermission_UsecaseErrorPropagates(t *testing.T) {
	boom := errors.New("db down")
	uc := &stubTeamUC{err: boom}
	mw := RequirePermission(entity.ModuleAE, entity.ActionViewList, uc)
	handler := mw(func(w http.ResponseWriter, r *http.Request) error { return nil })

	r := newRequestWithCtx("alice@x.io", "ws-1")
	err := handler(httptest.NewRecorder(), r)
	if !errors.Is(err, boom) {
		t.Errorf("want usecase error propagated, got %v", err)
	}
}
