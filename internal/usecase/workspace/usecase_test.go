package workspace

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
)

// ---------- stub repositories ----------

type stubWorkspaceRepo struct {
	listForUser   []entity.Workspace
	listForMember []entity.Workspace
	getByID       *entity.Workspace
	getBySlug     *entity.Workspace
	createOut     *entity.Workspace
	updateOut     *entity.Workspace
	createErr     error
	getErr        error
	deleted       []string

	createCalls int
	updateCalls int
}

func (s *stubWorkspaceRepo) GetAll(ctx context.Context) ([]entity.Workspace, error) {
	return nil, nil
}
func (s *stubWorkspaceRepo) GetByID(ctx context.Context, id string) (*entity.Workspace, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.getByID, nil
}
func (s *stubWorkspaceRepo) GetBySlug(ctx context.Context, slug string) (*entity.Workspace, error) {
	return s.getBySlug, nil
}
func (s *stubWorkspaceRepo) ListForUser(ctx context.Context, email string) ([]entity.Workspace, error) {
	return s.listForUser, nil
}
func (s *stubWorkspaceRepo) ListForMember(ctx context.Context, email string) ([]entity.Workspace, error) {
	return s.listForMember, nil
}
func (s *stubWorkspaceRepo) Create(ctx context.Context, w *entity.Workspace) (*entity.Workspace, error) {
	s.createCalls++
	if s.createErr != nil {
		return nil, s.createErr
	}
	out := *w
	out.ID = "ws-new"
	out.IsActive = true
	out.CreatedAt = time.Now()
	out.UpdatedAt = out.CreatedAt
	if s.createOut != nil {
		return s.createOut, nil
	}
	return &out, nil
}
func (s *stubWorkspaceRepo) Update(ctx context.Context, id string, patch repository.WorkspacePatch) (*entity.Workspace, error) {
	s.updateCalls++
	if s.updateOut != nil {
		return s.updateOut, nil
	}
	return s.getByID, nil
}
func (s *stubWorkspaceRepo) SoftDelete(ctx context.Context, id string) error {
	s.deleted = append(s.deleted, id)
	return nil
}

type stubMemberRepo struct {
	byEmail   map[string]*entity.WorkspaceMember
	listOut   []entity.WorkspaceMember
	getByID   *entity.WorkspaceMember
	addCalls  int
	addReturn *entity.WorkspaceMember
	addErr    error
	updateOut *entity.WorkspaceMember
	removed   []string
}

func (s *stubMemberRepo) List(ctx context.Context, workspaceID string) ([]entity.WorkspaceMember, error) {
	return s.listOut, nil
}
func (s *stubMemberRepo) Get(ctx context.Context, id string) (*entity.WorkspaceMember, error) {
	return s.getByID, nil
}
func (s *stubMemberRepo) GetByWorkspaceAndEmail(ctx context.Context, workspaceID, email string) (*entity.WorkspaceMember, error) {
	if s.byEmail == nil {
		return nil, nil
	}
	return s.byEmail[strings.ToLower(email)], nil
}
func (s *stubMemberRepo) Add(ctx context.Context, m *entity.WorkspaceMember) (*entity.WorkspaceMember, error) {
	s.addCalls++
	if s.addErr != nil {
		return nil, s.addErr
	}
	if s.addReturn != nil {
		return s.addReturn, nil
	}
	out := *m
	out.ID = "mem-new"
	out.IsActive = true
	return &out, nil
}
func (s *stubMemberRepo) UpdateRole(ctx context.Context, id, role string) (*entity.WorkspaceMember, error) {
	if s.updateOut != nil {
		return s.updateOut, nil
	}
	if s.getByID != nil {
		out := *s.getByID
		out.Role = role
		return &out, nil
	}
	return nil, nil
}
func (s *stubMemberRepo) Remove(ctx context.Context, id string) error {
	s.removed = append(s.removed, id)
	return nil
}

type stubInvitationRepo struct {
	created    *entity.WorkspaceInvitation
	getByToken *entity.WorkspaceInvitation
	listOut    []entity.WorkspaceInvitation
	accepted   []string
	revoked    []string
	createErr  error
}

func (s *stubInvitationRepo) Create(ctx context.Context, inv *entity.WorkspaceInvitation) (*entity.WorkspaceInvitation, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	out := *inv
	out.ID = "inv-new"
	s.created = &out
	return &out, nil
}
func (s *stubInvitationRepo) GetByToken(ctx context.Context, token string) (*entity.WorkspaceInvitation, error) {
	return s.getByToken, nil
}
func (s *stubInvitationRepo) List(ctx context.Context, workspaceID string) ([]entity.WorkspaceInvitation, error) {
	return s.listOut, nil
}
func (s *stubInvitationRepo) MarkAccepted(ctx context.Context, id string) error {
	s.accepted = append(s.accepted, id)
	return nil
}
func (s *stubInvitationRepo) Revoke(ctx context.Context, id string) error {
	s.revoked = append(s.revoked, id)
	return nil
}

// ---------- helpers ----------

func fixedNow() time.Time {
	return time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)
}

func fixedToken() (string, error) { return "fixed-token-abc", nil }

func newWithStubs(ws *stubWorkspaceRepo, m *stubMemberRepo, inv *stubInvitationRepo) Usecase {
	return New(ws, m, inv, nil, fixedNow, fixedToken)
}

func ownerMember() *entity.WorkspaceMember {
	return &entity.WorkspaceMember{
		ID:          "mem-owner",
		WorkspaceID: "ws-1",
		UserEmail:   "owner@x.com",
		Role:        entity.WorkspaceRoleOwner,
		IsActive:    true,
	}
}

// ---------- tests ----------

func TestCreate(t *testing.T) {
	tests := []struct {
		name    string
		caller  string
		req     CreateRequest
		setup   func(*stubWorkspaceRepo, *stubMemberRepo)
		wantErr bool
		errCode int
	}{
		{
			name:   "ok",
			caller: "owner@x.com",
			req:    CreateRequest{Name: "Acme", Slug: "acme"},
		},
		{
			name:    "no caller",
			caller:  "",
			req:     CreateRequest{Name: "Acme", Slug: "acme"},
			wantErr: true,
			errCode: 401,
		},
		{
			name:    "empty name",
			caller:  "x@x.com",
			req:     CreateRequest{Name: "", Slug: "acme"},
			wantErr: true,
			errCode: 422,
		},
		{
			name:    "bad slug",
			caller:  "x@x.com",
			req:     CreateRequest{Name: "X", Slug: "Bad Slug"},
			wantErr: true,
			errCode: 422,
		},
		{
			name:   "slug conflict",
			caller: "x@x.com",
			req:    CreateRequest{Name: "X", Slug: "taken"},
			setup: func(ws *stubWorkspaceRepo, _ *stubMemberRepo) {
				ws.getBySlug = &entity.Workspace{ID: "ws-existing", Slug: "taken"}
			},
			wantErr: true,
			errCode: 409,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ws := &stubWorkspaceRepo{}
			mr := &stubMemberRepo{}
			ir := &stubInvitationRepo{}
			if tc.setup != nil {
				tc.setup(ws, mr)
			}
			uc := newWithStubs(ws, mr, ir)
			_, err := uc.Create(context.Background(), tc.caller, tc.req)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				var ae *apperror.AppError
				if errors.As(err, &ae) && ae.HTTPStatus != tc.errCode {
					t.Fatalf("want code %d, got %d", tc.errCode, ae.HTTPStatus)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mr.addCalls != 1 {
				t.Fatalf("want 1 member add call, got %d", mr.addCalls)
			}
		})
	}
}

func TestUpdateMergesSettings(t *testing.T) {
	ws := &stubWorkspaceRepo{
		getByID: &entity.Workspace{
			ID:   "ws-1",
			Slug: "x",
			Settings: map[string]interface{}{
				"timezone": "Asia/Jakarta",
				"working_hours": map[string]interface{}{
					"start": "09:00",
					"end":   "17:00",
				},
			},
		},
	}
	mr := &stubMemberRepo{
		byEmail: map[string]*entity.WorkspaceMember{
			"owner@x.com": ownerMember(),
		},
	}
	uc := newWithStubs(ws, mr, &stubInvitationRepo{})

	name := "Renamed"
	_, err := uc.Update(context.Background(), "ws-1", "owner@x.com", UpdateRequest{
		Name: &name,
		Settings: map[string]interface{}{
			"working_hours": map[string]interface{}{
				"start": "08:00",
			},
			"currency": "IDR",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ws.updateCalls != 1 {
		t.Fatalf("want 1 update call, got %d", ws.updateCalls)
	}
}

func TestUpdateForbiddenForViewer(t *testing.T) {
	ws := &stubWorkspaceRepo{getByID: &entity.Workspace{ID: "ws-1"}}
	mr := &stubMemberRepo{
		byEmail: map[string]*entity.WorkspaceMember{
			"v@x.com": {ID: "m1", WorkspaceID: "ws-1", UserEmail: "v@x.com", Role: entity.WorkspaceRoleViewer, IsActive: true},
		},
	}
	uc := newWithStubs(ws, mr, &stubInvitationRepo{})
	name := "x"
	_, err := uc.Update(context.Background(), "ws-1", "v@x.com", UpdateRequest{Name: &name})
	var ae *apperror.AppError
	if !errors.As(err, &ae) || ae.HTTPStatus != 403 {
		t.Fatalf("want 403, got %v", err)
	}
}

func TestSoftDeleteRequiresOwner(t *testing.T) {
	cases := []struct {
		role      string
		wantError bool
	}{
		{entity.WorkspaceRoleOwner, false},
		{entity.WorkspaceRoleAdmin, true},
		{entity.WorkspaceRoleMember, true},
		{entity.WorkspaceRoleViewer, true},
	}
	for _, c := range cases {
		t.Run(c.role, func(t *testing.T) {
			ws := &stubWorkspaceRepo{}
			mr := &stubMemberRepo{
				byEmail: map[string]*entity.WorkspaceMember{
					"u@x.com": {ID: "m", WorkspaceID: "ws-1", UserEmail: "u@x.com", Role: c.role, IsActive: true},
				},
			}
			uc := newWithStubs(ws, mr, &stubInvitationRepo{})
			err := uc.SoftDelete(context.Background(), "ws-1", "u@x.com")
			if c.wantError {
				if err == nil {
					t.Fatal("expected forbidden error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if len(ws.deleted) != 1 || ws.deleted[0] != "ws-1" {
				t.Fatalf("expected soft-delete call for ws-1")
			}
		})
	}
}

func TestInviteFlow(t *testing.T) {
	ws := &stubWorkspaceRepo{}
	mr := &stubMemberRepo{
		byEmail: map[string]*entity.WorkspaceMember{
			"owner@x.com": ownerMember(),
		},
	}
	ir := &stubInvitationRepo{}
	uc := newWithStubs(ws, mr, ir)
	inv, err := uc.Invite(context.Background(), "ws-1", "owner@x.com", InviteRequest{Email: "new@x.com"})
	if err != nil {
		t.Fatalf("invite failed: %v", err)
	}
	if inv.InviteToken != "fixed-token-abc" {
		t.Fatalf("want injected token, got %s", inv.InviteToken)
	}
	if inv.Status != entity.InvitationStatusPending {
		t.Fatalf("want pending, got %s", inv.Status)
	}
}

func TestInviteDuplicate(t *testing.T) {
	mr := &stubMemberRepo{
		byEmail: map[string]*entity.WorkspaceMember{
			"owner@x.com": ownerMember(),
			"dup@x.com":   {ID: "m2", WorkspaceID: "ws-1", UserEmail: "dup@x.com", Role: entity.WorkspaceRoleMember, IsActive: true},
		},
	}
	uc := newWithStubs(&stubWorkspaceRepo{}, mr, &stubInvitationRepo{})
	_, err := uc.Invite(context.Background(), "ws-1", "owner@x.com", InviteRequest{Email: "dup@x.com"})
	var ae *apperror.AppError
	if !errors.As(err, &ae) || ae.HTTPStatus != 409 {
		t.Fatalf("want 409, got %v", err)
	}
}

func TestRemoveMemberCannotRemoveSelf(t *testing.T) {
	owner := ownerMember()
	mr := &stubMemberRepo{
		byEmail: map[string]*entity.WorkspaceMember{
			"owner@x.com": owner,
		},
		getByID: owner,
	}
	uc := newWithStubs(&stubWorkspaceRepo{}, mr, &stubInvitationRepo{})
	err := uc.RemoveMember(context.Background(), "ws-1", "mem-owner", "owner@x.com")
	var ae *apperror.AppError
	if !errors.As(err, &ae) || ae.HTTPStatus != 403 {
		t.Fatalf("want 403, got %v", err)
	}
}

func TestAcceptInvitation(t *testing.T) {
	cases := []struct {
		name    string
		inv     *entity.WorkspaceInvitation
		email   string
		wantErr bool
	}{
		{
			name: "ok",
			inv: &entity.WorkspaceInvitation{
				ID:          "inv-1",
				WorkspaceID: "ws-1",
				Email:       "new@x.com",
				Role:        entity.WorkspaceRoleMember,
				Status:      entity.InvitationStatusPending,
				ExpiresAt:   fixedNow().Add(24 * time.Hour),
			},
			email: "new@x.com",
		},
		{
			name: "expired",
			inv: &entity.WorkspaceInvitation{
				ID: "inv-1", WorkspaceID: "ws-1", Email: "n@x.com", Status: entity.InvitationStatusPending,
				ExpiresAt: fixedNow().Add(-time.Hour),
			},
			email: "n@x.com", wantErr: true,
		},
		{
			name: "revoked",
			inv: &entity.WorkspaceInvitation{
				ID: "inv-1", WorkspaceID: "ws-1", Email: "n@x.com", Status: entity.InvitationStatusRevoked,
				ExpiresAt: fixedNow().Add(time.Hour),
			},
			email: "n@x.com", wantErr: true,
		},
		{
			name: "wrong email",
			inv: &entity.WorkspaceInvitation{
				ID: "inv-1", WorkspaceID: "ws-1", Email: "real@x.com", Status: entity.InvitationStatusPending,
				ExpiresAt: fixedNow().Add(time.Hour),
			},
			email: "imposter@x.com", wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ir := &stubInvitationRepo{getByToken: tc.inv}
			uc := newWithStubs(&stubWorkspaceRepo{}, &stubMemberRepo{}, ir)
			_, err := uc.AcceptInvitation(context.Background(), "tok", tc.email, "Some Name")
			if tc.wantErr {
				if err == nil {
					t.Fatal("want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if len(ir.accepted) != 1 {
				t.Fatalf("want 1 accept call, got %d", len(ir.accepted))
			}
		})
	}
}

func TestListRequiresEmail(t *testing.T) {
	uc := newWithStubs(&stubWorkspaceRepo{}, &stubMemberRepo{}, &stubInvitationRepo{})
	if _, err := uc.List(context.Background(), ""); err == nil {
		t.Fatal("want error")
	}
}

func TestSwitchOk(t *testing.T) {
	ws := &stubWorkspaceRepo{getByID: &entity.Workspace{ID: "ws-1", Name: "X"}}
	mr := &stubMemberRepo{byEmail: map[string]*entity.WorkspaceMember{"u@x.com": ownerMember()}}
	uc := newWithStubs(ws, mr, &stubInvitationRepo{})
	out, err := uc.Switch(context.Background(), "ws-1", "u@x.com")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if out.UserRole != entity.WorkspaceRoleOwner {
		t.Fatalf("want owner role, got %s", out.UserRole)
	}
}
