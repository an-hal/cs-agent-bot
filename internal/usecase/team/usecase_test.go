package team

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
)

// ───── stubs ────────────────────────────────────────────────────────────────

type stubRoleRepo struct {
	roles        map[string]*entity.Role
	byName       map[string]*entity.Role
	memberCounts map[string]int
	createOut    *entity.Role
	updateOut    *entity.Role
	deleted      []string
}

func (s *stubRoleRepo) List(ctx context.Context) ([]entity.Role, error) {
	out := make([]entity.Role, 0, len(s.roles))
	for _, r := range s.roles {
		out = append(out, *r)
	}
	return out, nil
}
func (s *stubRoleRepo) GetByID(ctx context.Context, id string) (*entity.Role, error) {
	return s.roles[id], nil
}
func (s *stubRoleRepo) GetByName(ctx context.Context, name string) (*entity.Role, error) {
	return s.byName[name], nil
}
func (s *stubRoleRepo) Create(ctx context.Context, r *entity.Role) (*entity.Role, error) {
	if s.createOut != nil {
		return s.createOut, nil
	}
	out := *r
	out.ID = "role-new"
	return &out, nil
}
func (s *stubRoleRepo) Update(ctx context.Context, id string, patch repository.RolePatch) (*entity.Role, error) {
	if s.updateOut != nil {
		return s.updateOut, nil
	}
	existing := s.roles[id]
	if existing == nil {
		return nil, repository.ErrTeamNotFound
	}
	out := *existing
	if patch.Name != nil {
		out.Name = *patch.Name
	}
	return &out, nil
}
func (s *stubRoleRepo) Delete(ctx context.Context, id string) error {
	s.deleted = append(s.deleted, id)
	return nil
}
func (s *stubRoleRepo) CountMembers(ctx context.Context, roleID string) (int, error) {
	return s.memberCounts[roleID], nil
}

type stubPermRepo struct {
	perms     map[string]*entity.RolePermission // key = role|ws|module
	upsertErr error
	upserted  []entity.RolePermission
}

func permKey(role, ws, mod string) string { return role + "|" + ws + "|" + mod }

func (s *stubPermRepo) ListByRole(ctx context.Context, roleID string) ([]entity.RolePermission, error) {
	var out []entity.RolePermission
	for _, p := range s.perms {
		if p.RoleID == roleID {
			out = append(out, *p)
		}
	}
	return out, nil
}
func (s *stubPermRepo) ListByRoleWorkspace(ctx context.Context, roleID, workspaceID string) ([]entity.RolePermission, error) {
	var out []entity.RolePermission
	for _, p := range s.perms {
		if p.RoleID == roleID && p.WorkspaceID == workspaceID {
			out = append(out, *p)
		}
	}
	return out, nil
}
func (s *stubPermRepo) GetOne(ctx context.Context, roleID, workspaceID, moduleID string) (*entity.RolePermission, error) {
	return s.perms[permKey(roleID, workspaceID, moduleID)], nil
}
func (s *stubPermRepo) Upsert(ctx context.Context, p *entity.RolePermission) (*entity.RolePermission, error) {
	if s.upsertErr != nil {
		return nil, s.upsertErr
	}
	out := *p
	s.upserted = append(s.upserted, out)
	if s.perms == nil {
		s.perms = map[string]*entity.RolePermission{}
	}
	s.perms[permKey(p.RoleID, p.WorkspaceID, p.ModuleID)] = &out
	return &out, nil
}

type stubMemberRepo struct {
	byID        map[string]*entity.TeamMember
	byEmail     map[string]*entity.TeamMember
	byToken     map[string]*entity.TeamMember
	listOut     []entity.TeamMember
	listTotal   int64
	createOut   *entity.TeamMember
	createErr   error
	updateErr   error
	deleted     []string
	updateCalls []repository.TeamMemberPatch
	summary     repository.TeamMemberSummary
}

func (s *stubMemberRepo) List(ctx context.Context, filter repository.TeamMemberFilter) ([]entity.TeamMember, int64, error) {
	return s.listOut, s.listTotal, nil
}
func (s *stubMemberRepo) GetByID(ctx context.Context, id string) (*entity.TeamMember, error) {
	return s.byID[id], nil
}
func (s *stubMemberRepo) GetByEmail(ctx context.Context, email string) (*entity.TeamMember, error) {
	return s.byEmail[email], nil
}
func (s *stubMemberRepo) GetByInviteToken(ctx context.Context, token string) (*entity.TeamMember, error) {
	return s.byToken[token], nil
}
func (s *stubMemberRepo) Create(ctx context.Context, m *entity.TeamMember) (*entity.TeamMember, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	out := *m
	if out.ID == "" {
		out.ID = "m-new"
	}
	if s.createOut != nil {
		return s.createOut, nil
	}
	return &out, nil
}
func (s *stubMemberRepo) Update(ctx context.Context, id string, patch repository.TeamMemberPatch) (*entity.TeamMember, error) {
	s.updateCalls = append(s.updateCalls, patch)
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	existing := s.byID[id]
	if existing == nil {
		return nil, repository.ErrTeamNotFound
	}
	out := *existing
	if patch.Name != nil {
		out.Name = *patch.Name
	}
	if patch.Status != nil {
		out.Status = *patch.Status
	}
	if patch.RoleID != nil {
		out.RoleID = *patch.RoleID
	}
	if patch.UserID != nil {
		out.UserID = *patch.UserID
	}
	if patch.InviteToken != nil {
		out.InviteToken = *patch.InviteToken
	}
	if patch.JoinedAt != nil {
		out.JoinedAt = patch.JoinedAt
	}
	return &out, nil
}
func (s *stubMemberRepo) Delete(ctx context.Context, id string) error {
	if s.byID[id] == nil {
		return repository.ErrTeamNotFound
	}
	s.deleted = append(s.deleted, id)
	return nil
}
func (s *stubMemberRepo) Summary(ctx context.Context, workspaceID string) (repository.TeamMemberSummary, error) {
	return s.summary, nil
}

type stubAssignRepo struct {
	byMember   map[string][]string
	assigned   []struct{ MemberID, WorkspaceID string }
	replaced   map[string][]string
	assignErr  error
	replaceErr error
}

func (s *stubAssignRepo) ListByMember(ctx context.Context, memberID string) ([]entity.MemberWorkspaceAssignment, error) {
	return nil, nil
}
func (s *stubAssignRepo) ListWorkspaceIDsByMember(ctx context.Context, memberID string) ([]string, error) {
	return s.byMember[memberID], nil
}
func (s *stubAssignRepo) Assign(ctx context.Context, memberID, workspaceID, assignedBy string) error {
	if s.assignErr != nil {
		return s.assignErr
	}
	s.assigned = append(s.assigned, struct{ MemberID, WorkspaceID string }{memberID, workspaceID})
	return nil
}
func (s *stubAssignRepo) Unassign(ctx context.Context, memberID, workspaceID string) error {
	return nil
}
func (s *stubAssignRepo) ReplaceForMember(ctx context.Context, memberID string, workspaceIDs []string, assignedBy string) error {
	if s.replaceErr != nil {
		return s.replaceErr
	}
	if s.replaced == nil {
		s.replaced = map[string][]string{}
	}
	s.replaced[memberID] = workspaceIDs
	return nil
}
func (s *stubAssignRepo) Has(ctx context.Context, memberID, workspaceID string) (bool, error) {
	return false, nil
}

type stubApprovalRepo struct{}

func (s *stubApprovalRepo) Create(ctx context.Context, a *entity.ApprovalRequest) (*entity.ApprovalRequest, error) {
	return a, nil
}
func (s *stubApprovalRepo) GetByID(ctx context.Context, workspaceID, id string) (*entity.ApprovalRequest, error) {
	return nil, nil
}
func (s *stubApprovalRepo) UpdateStatus(ctx context.Context, workspaceID, id, newStatus, checkerEmail, reason string) error {
	return nil
}

type stubWhitelist struct {
	createCalls int
	err         error
}

func (s *stubWhitelist) Create(ctx context.Context, email, addedBy, notes string) (*entity.WhitelistEntry, error) {
	s.createCalls++
	if s.err != nil {
		return nil, s.err
	}
	return &entity.WhitelistEntry{Email: email}, nil
}

// ───── fixtures ─────────────────────────────────────────────────────────────

func fixedNow() time.Time { return time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC) }

func newFixture() (*stubRoleRepo, *stubPermRepo, *stubMemberRepo, *stubAssignRepo, *stubApprovalRepo, *stubWhitelist, Usecase) {
	superAdmin := &entity.Role{ID: "role-sa", Name: entity.RoleSuperAdmin, IsSystem: true}
	admin := &entity.Role{ID: "role-admin", Name: "Admin", IsSystem: true}
	custom := &entity.Role{ID: "role-custom", Name: "Custom", IsSystem: false}

	rr := &stubRoleRepo{
		roles: map[string]*entity.Role{
			"role-sa":     superAdmin,
			"role-admin":  admin,
			"role-custom": custom,
		},
		byName: map[string]*entity.Role{
			entity.RoleSuperAdmin: superAdmin,
			"Admin":               admin,
			"Custom":              custom,
		},
		memberCounts: map[string]int{},
	}

	pr := &stubPermRepo{perms: map[string]*entity.RolePermission{}}

	sa := &entity.TeamMember{ID: "m-sa", Name: "Alice", Email: "alice@x.io", RoleID: "role-sa", Status: entity.MemberStatusActive}
	adminMember := &entity.TeamMember{ID: "m-admin", Name: "Bob", Email: "bob@x.io", RoleID: "role-admin", Status: entity.MemberStatusActive}
	other := &entity.TeamMember{ID: "m-other", Name: "Carol", Email: "carol@x.io", RoleID: "role-custom", Status: entity.MemberStatusActive}

	mr := &stubMemberRepo{
		byID: map[string]*entity.TeamMember{
			"m-sa":    sa,
			"m-admin": adminMember,
			"m-other": other,
		},
		byEmail: map[string]*entity.TeamMember{
			"alice@x.io": sa,
			"bob@x.io":   adminMember,
			"carol@x.io": other,
		},
		byToken: map[string]*entity.TeamMember{},
	}

	ar := &stubAssignRepo{byMember: map[string][]string{}}
	aprv := &stubApprovalRepo{}
	wl := &stubWhitelist{}

	uc := New(rr, pr, mr, ar, aprv, wl, Options{
		Now:      fixedNow,
		TokenGen: func() (string, error) { return "tok_fixed", nil },
	})
	return rr, pr, mr, ar, aprv, wl, uc
}

// ───── tests ────────────────────────────────────────────────────────────────

func TestInviteMember_CreatesPendingAndAssignsWorkspaces(t *testing.T) {
	_, _, mr, ar, _, _, uc := newFixture()
	out, err := uc.InviteMember(context.Background(), "alice@x.io", InviteRequest{
		Name: "Dan", Email: "DAN@x.io", RoleID: "role-custom", WorkspaceIDs: []string{"ws-1", "ws-2"},
	})
	if err != nil {
		t.Fatalf("invite err: %v", err)
	}
	if out.Status != entity.MemberStatusPending {
		t.Errorf("status = %q, want pending", out.Status)
	}
	if out.Email != "dan@x.io" {
		t.Errorf("email not lowered: %q", out.Email)
	}
	if out.InviteToken != "tok_fixed" {
		t.Errorf("token = %q", out.InviteToken)
	}
	if out.InviteExpires == nil || !out.InviteExpires.Equal(fixedNow().Add(DefaultInviteTTL)) {
		t.Errorf("expires wrong: %v", out.InviteExpires)
	}
	if len(ar.assigned) != 2 {
		t.Errorf("assigned = %d, want 2", len(ar.assigned))
	}
	_ = mr
}

func TestInviteMember_DuplicateEmailConflict(t *testing.T) {
	_, _, _, _, _, _, uc := newFixture()
	_, err := uc.InviteMember(context.Background(), "alice@x.io", InviteRequest{
		Name: "Bobby", Email: "bob@x.io", RoleID: "role-custom", WorkspaceIDs: []string{"ws-1"},
	})
	var ae *apperror.AppError
	if !errors.As(err, &ae) || ae.Code != apperror.CodeConflict {
		t.Errorf("want conflict, got %v", err)
	}
}

func TestInviteMember_RequiresWorkspaces(t *testing.T) {
	_, _, _, _, _, _, uc := newFixture()
	_, err := uc.InviteMember(context.Background(), "alice@x.io", InviteRequest{
		Name: "Dan", Email: "dan@x.io", RoleID: "role-custom",
	})
	var ae *apperror.AppError
	if !errors.As(err, &ae) || ae.Code != apperror.CodeBadRequest {
		t.Errorf("want bad_request, got %v", err)
	}
}

func TestChangeRole_NonSuperAdminCannotModifySuperAdmin(t *testing.T) {
	_, _, _, _, _, _, uc := newFixture()
	_, err := uc.ChangeRole(context.Background(), "bob@x.io", "m-sa", "role-custom")
	var ae *apperror.AppError
	if !errors.As(err, &ae) || ae.Code != apperror.CodeForbidden {
		t.Errorf("want forbidden, got %v", err)
	}
}

func TestChangeRole_SuperAdminCanModifySuperAdmin(t *testing.T) {
	_, _, _, _, _, _, uc := newFixture()
	_, err := uc.ChangeRole(context.Background(), "alice@x.io", "m-sa", "role-custom")
	if err != nil {
		t.Errorf("super admin should be allowed: %v", err)
	}
}

func TestRemoveMember_SelfDeleteBlocked(t *testing.T) {
	_, _, _, _, _, _, uc := newFixture()
	err := uc.RemoveMember(context.Background(), "alice@x.io", "m-sa")
	var ae *apperror.AppError
	if !errors.As(err, &ae) || ae.Code != apperror.CodeForbidden {
		t.Errorf("want forbidden self-delete, got %v", err)
	}
}

func TestRemoveMember_NonSuperAdminCannotDeleteSuperAdmin(t *testing.T) {
	_, _, _, _, _, _, uc := newFixture()
	err := uc.RemoveMember(context.Background(), "bob@x.io", "m-sa")
	var ae *apperror.AppError
	if !errors.As(err, &ae) || ae.Code != apperror.CodeForbidden {
		t.Errorf("want forbidden, got %v", err)
	}
}

func TestRemoveMember_HappyPath(t *testing.T) {
	_, _, mr, _, _, _, uc := newFixture()
	if err := uc.RemoveMember(context.Background(), "alice@x.io", "m-other"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(mr.deleted) != 1 || mr.deleted[0] != "m-other" {
		t.Errorf("deleted = %v", mr.deleted)
	}
}

func TestAcceptInvitation_ExpiredToken(t *testing.T) {
	_, _, mr, _, _, _, uc := newFixture()
	past := fixedNow().Add(-time.Hour)
	mr.byToken["tok-old"] = &entity.TeamMember{
		ID: "m-pending", Email: "pending@x.io", Status: entity.MemberStatusPending,
		InviteToken: "tok-old", InviteExpires: &past,
	}
	_, err := uc.AcceptInvitation(context.Background(), "tok-old", "user-123")
	var ae *apperror.AppError
	if !errors.As(err, &ae) || ae.Code != apperror.CodeBadRequest {
		t.Errorf("want bad_request expired, got %v", err)
	}
}

func TestAcceptInvitation_HappyPathDualWritesWhitelist(t *testing.T) {
	_, _, mr, _, _, wl, uc := newFixture()
	future := fixedNow().Add(time.Hour)
	pending := &entity.TeamMember{
		ID: "m-pending", Email: "pending@x.io", RoleID: "role-custom",
		Status: entity.MemberStatusPending, InviteToken: "tok-good", InviteExpires: &future,
	}
	mr.byID["m-pending"] = pending
	mr.byToken["tok-good"] = pending

	out, err := uc.AcceptInvitation(context.Background(), "tok-good", "user-123")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out.Status != entity.MemberStatusActive {
		t.Errorf("status = %q", out.Status)
	}
	if wl.createCalls != 1 {
		t.Errorf("whitelist Create calls = %d, want 1", wl.createCalls)
	}
}

func TestDeleteRole_BlockedWhenMembersAttached(t *testing.T) {
	rr, _, _, _, _, _, uc := newFixture()
	rr.memberCounts["role-custom"] = 3
	err := uc.DeleteRole(context.Background(), "role-custom")
	var ae *apperror.AppError
	if !errors.As(err, &ae) || ae.Code != apperror.CodeConflict {
		t.Errorf("want conflict, got %v", err)
	}
}

func TestDeleteRole_BlockedForSystemRole(t *testing.T) {
	_, _, _, _, _, _, uc := newFixture()
	err := uc.DeleteRole(context.Background(), "role-sa")
	var ae *apperror.AppError
	if !errors.As(err, &ae) || ae.Code != apperror.CodeForbidden {
		t.Errorf("want forbidden, got %v", err)
	}
}

func TestUpdateRole_BlockSystemRename(t *testing.T) {
	_, _, _, _, _, _, uc := newFixture()
	newName := "Renamed"
	_, err := uc.UpdateRole(context.Background(), "role-sa", UpdateRoleRequest{Name: &newName})
	var ae *apperror.AppError
	if !errors.As(err, &ae) || ae.Code != apperror.CodeForbidden {
		t.Errorf("want forbidden, got %v", err)
	}
}

func TestUpdateRolePermissions_NonSuperAdminCannotEditSuperAdmin(t *testing.T) {
	_, _, _, _, _, _, uc := newFixture()
	_, err := uc.UpdateRolePermissions(context.Background(), "bob@x.io", "role-sa", UpdateRolePermissionRequest{
		WorkspaceID: "ws-1", ModuleID: entity.ModuleTeam,
		Permissions: PermissionFlags{ViewList: entity.ViewScopeAll, Edit: true},
	})
	var ae *apperror.AppError
	if !errors.As(err, &ae) || ae.Code != apperror.CodeForbidden {
		t.Errorf("want forbidden, got %v", err)
	}
}

func TestUpdateRolePermissions_DiffActions(t *testing.T) {
	_, pr, _, _, _, _, uc := newFixture()
	pr.perms[permKey("role-custom", "ws-1", entity.ModuleDataMaster)] = &entity.RolePermission{
		RoleID: "role-custom", WorkspaceID: "ws-1", ModuleID: entity.ModuleDataMaster,
		ViewList: entity.ViewScopeFalse, CanEdit: false,
	}
	change, err := uc.UpdateRolePermissions(context.Background(), "alice@x.io", "role-custom", UpdateRolePermissionRequest{
		WorkspaceID: "ws-1", ModuleID: entity.ModuleDataMaster,
		Permissions: PermissionFlags{ViewList: entity.ViewScopeAll, Edit: true},
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	wantChanged := map[string]bool{entity.ActionViewList: true, entity.ActionEdit: true}
	if len(change.ChangedActions) != len(wantChanged) {
		t.Errorf("changed = %v", change.ChangedActions)
	}
	for _, a := range change.ChangedActions {
		if !wantChanged[a] {
			t.Errorf("unexpected changed action %q", a)
		}
	}
}

func TestCheckPermission_ViewListScopes(t *testing.T) {
	_, pr, _, _, _, _, uc := newFixture()
	pr.perms[permKey("role-custom", "ws-1", entity.ModuleAE)] = &entity.RolePermission{
		RoleID: "role-custom", WorkspaceID: "ws-1", ModuleID: entity.ModuleAE,
		ViewList: entity.ViewScopeAll, CanEdit: true,
	}
	cases := []struct {
		action string
		want   bool
	}{
		{entity.ActionViewList, true},
		{entity.ActionEdit, true},
		{entity.ActionDelete, false},
	}
	for _, c := range cases {
		allowed, _, err := uc.CheckPermission(context.Background(), "carol@x.io", "ws-1", entity.ModuleAE, c.action)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if allowed != c.want {
			t.Errorf("action %q allowed = %v, want %v", c.action, allowed, c.want)
		}
	}
}

func TestCheckPermission_NoMemberRecord(t *testing.T) {
	_, _, _, _, _, _, uc := newFixture()
	allowed, resolved, err := uc.CheckPermission(context.Background(), "ghost@x.io", "ws-1", entity.ModuleAE, entity.ActionViewList)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if allowed || resolved != nil {
		t.Errorf("ghost caller should be denied")
	}
}

func TestCheckPermission_NoRowReturnsFalseScope(t *testing.T) {
	_, _, _, _, _, _, uc := newFixture()
	allowed, resolved, err := uc.CheckPermission(context.Background(), "carol@x.io", "ws-1", entity.ModuleTeam, entity.ActionViewList)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if allowed {
		t.Errorf("want deny when no perm row")
	}
	if resolved == nil || resolved.ViewList != entity.ViewScopeFalse {
		t.Errorf("resolved = %+v", resolved)
	}
}

func TestGetMyPermissions_FillsMissingModules(t *testing.T) {
	_, pr, _, _, _, _, uc := newFixture()
	pr.perms[permKey("role-custom", "ws-1", entity.ModuleAE)] = &entity.RolePermission{
		RoleID: "role-custom", WorkspaceID: "ws-1", ModuleID: entity.ModuleAE,
		ViewList: entity.ViewScopeTrue,
	}
	my, err := uc.GetMyPermissions(context.Background(), "carol@x.io", "ws-1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(my.Permissions) != len(entity.AllModules) {
		t.Errorf("got %d modules, want %d", len(my.Permissions), len(entity.AllModules))
	}
	if my.Permissions[entity.ModuleAE].ViewList != entity.ViewScopeTrue {
		t.Errorf("AE view_list wrong")
	}
	if my.Permissions[entity.ModuleTeam].ViewList != entity.ViewScopeFalse {
		t.Errorf("Team view_list should default to false")
	}
}
