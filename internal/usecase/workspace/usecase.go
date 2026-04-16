// Package workspace implements the multi-workspace CRM core: workspace CRUD,
// member management, invitations, and notifications. Workspace ID is the
// tenant key for all other features.
package workspace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
)

// Usecase exposes workspace business operations.
type Usecase interface {
	List(ctx context.Context, callerEmail string) ([]entity.Workspace, error)
	Get(ctx context.Context, id, callerEmail string) (*WorkspaceDetail, error)
	Create(ctx context.Context, callerEmail string, req CreateRequest) (*entity.Workspace, error)
	Update(ctx context.Context, id, callerEmail string, req UpdateRequest) (*entity.Workspace, error)
	SoftDelete(ctx context.Context, id, callerEmail string) error
	Switch(ctx context.Context, id, callerEmail string) (*SwitchResponse, error)

	ListMembers(ctx context.Context, workspaceID, callerEmail string) ([]entity.WorkspaceMember, error)
	Invite(ctx context.Context, workspaceID, callerEmail string, req InviteRequest) (*entity.WorkspaceInvitation, error)
	UpdateMemberRole(ctx context.Context, workspaceID, memberID, callerEmail, newRole string) (*entity.WorkspaceMember, error)
	RemoveMember(ctx context.Context, workspaceID, memberID, callerEmail string) error
	AcceptInvitation(ctx context.Context, token, userEmail, userName string) (*entity.WorkspaceMember, error)
}

type usecase struct {
	workspaceRepo  repository.WorkspaceRepository
	memberRepo     repository.WorkspaceMemberRepository
	invitationRepo repository.WorkspaceInvitationRepository
	now            func() time.Time
	tokenGen       func() (string, error)
}

// New constructs a workspace usecase. Pass nil for nowFn / tokenFn to use defaults.
func New(
	wsRepo repository.WorkspaceRepository,
	memberRepo repository.WorkspaceMemberRepository,
	invitationRepo repository.WorkspaceInvitationRepository,
	nowFn func() time.Time,
	tokenFn func() (string, error),
) Usecase {
	if nowFn == nil {
		nowFn = time.Now
	}
	if tokenFn == nil {
		tokenFn = generateToken
	}
	return &usecase{
		workspaceRepo:  wsRepo,
		memberRepo:     memberRepo,
		invitationRepo: invitationRepo,
		now:            nowFn,
		tokenGen:       tokenFn,
	}
}

// CreateRequest is the payload for POST /workspaces.
type CreateRequest struct {
	Name     string                 `json:"name"`
	Slug     string                 `json:"slug"`
	Logo     string                 `json:"logo"`
	Color    string                 `json:"color"`
	Plan     string                 `json:"plan"`
	Settings map[string]interface{} `json:"settings"`
}

// UpdateRequest is the payload for PUT /workspaces/{id}. Slug is intentionally
// absent because slugs are immutable after creation (would break bookmarks).
type UpdateRequest struct {
	Name     *string                `json:"name,omitempty"`
	Logo     *string                `json:"logo,omitempty"`
	Color    *string                `json:"color,omitempty"`
	Plan     *string                `json:"plan,omitempty"`
	Settings map[string]interface{} `json:"settings,omitempty"`
}

// InviteRequest is the payload for POST /workspaces/{id}/members/invite.
type InviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// WorkspaceDetail enriches a workspace with members and lightweight stats.
type WorkspaceDetail struct {
	*entity.Workspace
	Members []entity.WorkspaceMember `json:"members"`
}

// SwitchResponse is returned from POST /workspaces/{id}/switch.
type SwitchResponse struct {
	Workspace    *entity.Workspace `json:"workspace"`
	UserRole     string            `json:"user_role"`
	LastActiveAt time.Time         `json:"last_active_at"`
}

var slugRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func validateSlug(slug string) error {
	if slug == "" || len(slug) > 50 {
		return apperror.ValidationError("slug must be 1–50 chars")
	}
	if !slugRegex.MatchString(slug) {
		return apperror.ValidationError("slug must be lowercase alphanumeric with optional hyphens")
	}
	return nil
}

func validRole(role string) bool {
	switch role {
	case entity.WorkspaceRoleOwner, entity.WorkspaceRoleAdmin,
		entity.WorkspaceRoleMember, entity.WorkspaceRoleViewer:
		return true
	}
	return false
}

func defaultLogo(name string) string {
	cleaned := strings.TrimSpace(name)
	if cleaned == "" {
		return "WS"
	}
	if len(cleaned) >= 2 {
		return strings.ToUpper(cleaned[:2])
	}
	return strings.ToUpper(cleaned)
}

// List returns workspaces visible to the caller.
func (u *usecase) List(ctx context.Context, callerEmail string) ([]entity.Workspace, error) {
	if callerEmail == "" {
		return nil, apperror.Unauthorized("caller email missing")
	}
	return u.workspaceRepo.ListForUser(ctx, callerEmail)
}

// Get returns a workspace plus members, after verifying caller membership.
func (u *usecase) Get(ctx context.Context, id, callerEmail string) (*WorkspaceDetail, error) {
	if _, err := u.assertMembership(ctx, id, callerEmail); err != nil {
		return nil, err
	}
	w, err := u.workspaceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return nil, apperror.NotFound("workspace", "Workspace tidak ditemukan")
	}
	members, err := u.memberRepo.List(ctx, id)
	if err != nil {
		return nil, err
	}
	return &WorkspaceDetail{Workspace: w, Members: members}, nil
}

// Create inserts a workspace and adds the caller as the initial owner.
func (u *usecase) Create(ctx context.Context, callerEmail string, req CreateRequest) (*entity.Workspace, error) {
	if callerEmail == "" {
		return nil, apperror.Unauthorized("caller email missing")
	}
	if strings.TrimSpace(req.Name) == "" || len(req.Name) > 100 {
		return nil, apperror.ValidationError("name must be 1–100 chars")
	}
	req.Slug = strings.ToLower(strings.TrimSpace(req.Slug))
	if err := validateSlug(req.Slug); err != nil {
		return nil, err
	}

	if existing, err := u.workspaceRepo.GetBySlug(ctx, req.Slug); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, apperror.Conflict("Slug '" + req.Slug + "' sudah digunakan")
	}

	logo := req.Logo
	if logo == "" {
		logo = defaultLogo(req.Name)
	}
	color := req.Color
	if color == "" {
		color = "#534AB7"
	}
	plan := req.Plan
	if plan == "" {
		plan = "Basic"
	}

	created, err := u.workspaceRepo.Create(ctx, &entity.Workspace{
		Slug:     req.Slug,
		Name:     req.Name,
		Logo:     logo,
		Color:    color,
		Plan:     plan,
		Settings: req.Settings,
	})
	if err != nil {
		return nil, err
	}
	if _, err := u.memberRepo.Add(ctx, &entity.WorkspaceMember{
		WorkspaceID: created.ID,
		UserEmail:   callerEmail,
		UserName:    callerEmail,
		Role:        entity.WorkspaceRoleOwner,
		InvitedBy:   callerEmail,
	}); err != nil {
		return nil, err
	}
	return created, nil
}

// Update applies a partial update. Settings are deep-merged.
func (u *usecase) Update(ctx context.Context, id, callerEmail string, req UpdateRequest) (*entity.Workspace, error) {
	role, err := u.assertMembership(ctx, id, callerEmail)
	if err != nil {
		return nil, err
	}
	if !entity.CanManageWorkspace(role) {
		return nil, apperror.Forbidden("Hanya owner atau admin yang bisa mengubah workspace")
	}

	current, err := u.workspaceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, apperror.NotFound("workspace", "Workspace tidak ditemukan")
	}
	if current.IsHolding && req.Settings == nil && req.Name == nil && req.Color == nil && req.Logo == nil && req.Plan == nil {
		return current, nil
	}

	patch := repository.WorkspacePatch{
		Name:  req.Name,
		Logo:  req.Logo,
		Color: req.Color,
		Plan:  req.Plan,
	}
	if req.Settings != nil {
		patch.Settings = mergeSettings(current.Settings, req.Settings)
	}
	return u.workspaceRepo.Update(ctx, id, patch)
}

// SoftDelete marks the workspace and its members inactive.
func (u *usecase) SoftDelete(ctx context.Context, id, callerEmail string) error {
	role, err := u.assertMembership(ctx, id, callerEmail)
	if err != nil {
		return err
	}
	if !entity.CanDeleteWorkspace(role) {
		return apperror.Forbidden("Hanya owner yang bisa menghapus workspace")
	}
	return u.workspaceRepo.SoftDelete(ctx, id)
}

// Switch returns workspace + role for the caller. Audit-only.
func (u *usecase) Switch(ctx context.Context, id, callerEmail string) (*SwitchResponse, error) {
	role, err := u.assertMembership(ctx, id, callerEmail)
	if err != nil {
		return nil, err
	}
	w, err := u.workspaceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return nil, apperror.NotFound("workspace", "Workspace tidak ditemukan")
	}
	return &SwitchResponse{Workspace: w, UserRole: role, LastActiveAt: u.now()}, nil
}

// ListMembers returns active workspace members after caller membership check.
func (u *usecase) ListMembers(ctx context.Context, workspaceID, callerEmail string) ([]entity.WorkspaceMember, error) {
	if _, err := u.assertMembership(ctx, workspaceID, callerEmail); err != nil {
		return nil, err
	}
	return u.memberRepo.List(ctx, workspaceID)
}

// Invite creates an invitation row. The actual email send is handled outside
// the transactional path by a notification dispatcher.
func (u *usecase) Invite(ctx context.Context, workspaceID, callerEmail string, req InviteRequest) (*entity.WorkspaceInvitation, error) {
	role, err := u.assertMembership(ctx, workspaceID, callerEmail)
	if err != nil {
		return nil, err
	}
	if !entity.CanInviteMembers(role) {
		return nil, apperror.Forbidden("Anda tidak memiliki izin untuk mengundang member")
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		return nil, apperror.ValidationError("email tidak valid")
	}
	if req.Role == "" {
		req.Role = entity.WorkspaceRoleMember
	}
	if !validRole(req.Role) {
		return nil, apperror.ValidationError("role tidak valid")
	}
	if existing, err := u.memberRepo.GetByWorkspaceAndEmail(ctx, workspaceID, req.Email); err != nil {
		return nil, err
	} else if existing != nil && existing.IsActive {
		return nil, apperror.Conflict("User sudah menjadi member workspace ini")
	}

	token, err := u.tokenGen()
	if err != nil {
		return nil, apperror.InternalError(err)
	}
	return u.invitationRepo.Create(ctx, &entity.WorkspaceInvitation{
		WorkspaceID: workspaceID,
		Email:       req.Email,
		Role:        req.Role,
		InviteToken: token,
		Status:      entity.InvitationStatusPending,
		InvitedBy:   callerEmail,
		ExpiresAt:   u.now().Add(7 * 24 * time.Hour),
	})
}

// UpdateMemberRole changes a member role. Owner-only.
func (u *usecase) UpdateMemberRole(ctx context.Context, workspaceID, memberID, callerEmail, newRole string) (*entity.WorkspaceMember, error) {
	role, err := u.assertMembership(ctx, workspaceID, callerEmail)
	if err != nil {
		return nil, err
	}
	if !entity.CanRemoveMembers(role) {
		return nil, apperror.Forbidden("Hanya owner yang bisa mengubah role")
	}
	if !validRole(newRole) {
		return nil, apperror.ValidationError("role tidak valid")
	}
	target, err := u.memberRepo.Get(ctx, memberID)
	if err != nil {
		return nil, err
	}
	if target == nil || target.WorkspaceID != workspaceID {
		return nil, apperror.NotFound("member", "Member tidak ditemukan")
	}
	if target.UserEmail == callerEmail && target.Role == entity.WorkspaceRoleOwner && newRole != entity.WorkspaceRoleOwner {
		return nil, apperror.Forbidden("Owner tidak bisa menurunkan role dirinya sendiri")
	}
	return u.memberRepo.UpdateRole(ctx, memberID, newRole)
}

// RemoveMember deletes a member from a workspace. Owner-only. Cannot remove self.
func (u *usecase) RemoveMember(ctx context.Context, workspaceID, memberID, callerEmail string) error {
	role, err := u.assertMembership(ctx, workspaceID, callerEmail)
	if err != nil {
		return err
	}
	if !entity.CanRemoveMembers(role) {
		return apperror.Forbidden("Hanya owner yang bisa menghapus member")
	}
	target, err := u.memberRepo.Get(ctx, memberID)
	if err != nil {
		return err
	}
	if target == nil || target.WorkspaceID != workspaceID {
		return apperror.NotFound("member", "Member tidak ditemukan")
	}
	if target.UserEmail == callerEmail {
		return apperror.Forbidden("Owner tidak bisa menghapus dirinya sendiri dari workspace")
	}
	return u.memberRepo.Remove(ctx, memberID)
}

// AcceptInvitation consumes an invite token and adds the user as a member.
func (u *usecase) AcceptInvitation(ctx context.Context, token, userEmail, userName string) (*entity.WorkspaceMember, error) {
	if token == "" {
		return nil, apperror.ValidationError("token kosong")
	}
	inv, err := u.invitationRepo.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, apperror.NotFound("invitation", "Invitation tidak ditemukan")
	}
	if inv.Status != entity.InvitationStatusPending {
		return nil, apperror.BadRequest("Invitation sudah tidak berlaku")
	}
	if u.now().After(inv.ExpiresAt) {
		return nil, apperror.BadRequest("Invitation kedaluwarsa")
	}
	if !strings.EqualFold(inv.Email, userEmail) {
		return nil, apperror.Forbidden("Email tidak cocok dengan invitation")
	}

	member, err := u.memberRepo.Add(ctx, &entity.WorkspaceMember{
		WorkspaceID: inv.WorkspaceID,
		UserEmail:   strings.ToLower(userEmail),
		UserName:    userName,
		Role:        inv.Role,
		InvitedBy:   inv.InvitedBy,
	})
	if err != nil {
		return nil, err
	}
	if err := u.invitationRepo.MarkAccepted(ctx, inv.ID); err != nil {
		return nil, err
	}
	return member, nil
}

// assertMembership verifies the caller has an active membership in the workspace
// and returns their role. Returns Forbidden otherwise.
func (u *usecase) assertMembership(ctx context.Context, workspaceID, callerEmail string) (string, error) {
	if callerEmail == "" {
		return "", apperror.Unauthorized("caller email missing")
	}
	m, err := u.memberRepo.GetByWorkspaceAndEmail(ctx, workspaceID, callerEmail)
	if err != nil {
		return "", err
	}
	if m == nil || !m.IsActive {
		return "", apperror.Forbidden("Anda tidak memiliki akses ke workspace ini")
	}
	return m.Role, nil
}

// mergeSettings performs a shallow deep-merge of settings (top-level keys are merged,
// nested values are replaced wholesale unless both sides are maps).
func mergeSettings(base, patch map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(base)+len(patch))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range patch {
		if existing, ok := out[k]; ok {
			if em, emOK := existing.(map[string]interface{}); emOK {
				if pm, pmOK := v.(map[string]interface{}); pmOK {
					out[k] = mergeSettings(em, pm)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", errors.New("failed to generate token")
	}
	return hex.EncodeToString(b), nil
}
