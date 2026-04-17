// Package team implements team management, RBAC roles, role permissions, and
// the invitation flow. Member rows are canonical; on invite-accept we also
// append to the auth whitelist so feat/01 auth continues to work.
package team

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
)

// WhitelistAppender is the small slice of WhitelistRepository that the team
// usecase needs in order to honour feat/01's auth gate (Option A dual-write).
// TODO(feat/04+): bridge to ms-auth-proxy once an HTTP client exists.
type WhitelistAppender interface {
	Create(ctx context.Context, email, addedBy, notes string) (*entity.WhitelistEntry, error)
}

// Usecase exposes team business operations.
type Usecase interface {
	// Members
	ListMembers(ctx context.Context, filter repository.TeamMemberFilter) (*ListMembersResult, error)
	GetMember(ctx context.Context, id string) (*MemberDetail, error)
	InviteMember(ctx context.Context, callerEmail string, req InviteRequest) (*entity.TeamMember, error)
	UpdateMember(ctx context.Context, id string, req UpdateMemberRequest) (*entity.TeamMember, error)
	ChangeRole(ctx context.Context, callerEmail, id, newRoleID string) (*entity.TeamMember, error)
	ChangeStatus(ctx context.Context, id, newStatus string) (*entity.TeamMember, error)
	UpdateMemberWorkspaces(ctx context.Context, callerEmail, id string, workspaceIDs []string) (*entity.TeamMember, error)
	RemoveMember(ctx context.Context, callerEmail, id string) error
	AcceptInvitation(ctx context.Context, token, userID string) (*entity.TeamMember, error)

	// Roles
	ListRoles(ctx context.Context, workspaceID string) ([]RoleSummary, error)
	GetRole(ctx context.Context, id, workspaceID string) (*RoleDetail, error)
	CreateRole(ctx context.Context, req CreateRoleRequest) (*entity.Role, error)
	UpdateRole(ctx context.Context, id string, req UpdateRoleRequest) (*entity.Role, error)
	UpdateRolePermissions(ctx context.Context, callerEmail, id string, req UpdateRolePermissionRequest) (*PermissionChange, error)
	DeleteRole(ctx context.Context, id string) error

	// Permissions
	CheckPermission(ctx context.Context, callerEmail, workspaceID, module, action string) (bool, *entity.ResolvedPermission, error)
	GetMyPermissions(ctx context.Context, callerEmail, workspaceID string) (*MyPermissions, error)
}

// ─── DTOs ────────────────────────────────────────────────────────────────────

// InviteRequest is the body for POST /team/members/invite.
type InviteRequest struct {
	Name         string   `json:"name"`
	Email        string   `json:"email"`
	RoleID       string   `json:"role_id"`
	WorkspaceIDs []string `json:"workspace_ids"`
	Department   string   `json:"department"`
}

// UpdateMemberRequest is the body for PUT /team/members/{id}.
type UpdateMemberRequest struct {
	Name        *string `json:"name,omitempty"`
	Department  *string `json:"department,omitempty"`
	AvatarColor *string `json:"avatar_color,omitempty"`
}

// CreateRoleRequest is the body for POST /team/roles.
type CreateRoleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
	BgColor     string `json:"bg_color"`
}

// UpdateRoleRequest is the body for PUT /team/roles/{id}.
type UpdateRoleRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Color       *string `json:"color,omitempty"`
	BgColor     *string `json:"bg_color,omitempty"`
}

// PermissionFlags is the wire format for a single permission set.
type PermissionFlags struct {
	ViewList   string `json:"view_list"`
	ViewDetail bool   `json:"view_detail"`
	Create     bool   `json:"create"`
	Edit       bool   `json:"edit"`
	Delete     bool   `json:"delete"`
	Export     bool   `json:"export"`
	Import     bool   `json:"import"`
}

// UpdateRolePermissionRequest is the body for PUT /team/roles/{id}/permissions.
type UpdateRolePermissionRequest struct {
	WorkspaceID string          `json:"workspace_id"`
	ModuleID    string          `json:"module_id"`
	Permissions PermissionFlags `json:"permissions"`
}

// PermissionChange describes what actually changed after a permission update.
type PermissionChange struct {
	Role           *entity.Role    `json:"role"`
	ModuleID       string          `json:"module_id"`
	WorkspaceID    string          `json:"workspace_id"`
	ChangedActions []string        `json:"changed_actions"`
	Summary        string          `json:"summary"`
	After          PermissionFlags `json:"permissions"`
}

// ListMembersResult bundles the paginated list + summary.
type ListMembersResult struct {
	Data    []MemberDetail          `json:"data"`
	Meta    ListMeta                `json:"meta"`
	Summary repository.TeamMemberSummary `json:"summary"`
}

// ListMeta carries pagination metadata.
type ListMeta struct {
	Offset int   `json:"offset"`
	Limit  int   `json:"limit"`
	Total  int64 `json:"total"`
}

// MemberDetail is a member joined with its role and assigned workspace IDs.
type MemberDetail struct {
	entity.TeamMember
	Role         *entity.Role `json:"role,omitempty"`
	WorkspaceIDs []string     `json:"workspace_ids"`
}

// RoleSummary is the list-view role payload.
type RoleSummary struct {
	entity.Role
	MemberCount int `json:"member_count"`
}

// RoleDetail is the full permission matrix for a role in one workspace.
type RoleDetail struct {
	entity.Role
	MemberCount int                        `json:"member_count"`
	Permissions map[string]PermissionFlags `json:"permissions"`
}

// MyPermissions carries the current user's full matrix for one workspace.
type MyPermissions struct {
	Role        *entity.Role               `json:"role"`
	WorkspaceID string                     `json:"workspace_id"`
	Permissions map[string]PermissionFlags `json:"permissions"`
}

// ─── implementation ──────────────────────────────────────────────────────────

// DefaultInviteTTL is how long invite tokens live. Kept as a constant rather
// than a magic number so tests can override it via the usecase constructor.
const DefaultInviteTTL = 72 * time.Hour

type usecase struct {
	roleRepo        repository.RoleRepository
	permRepo        repository.RolePermissionRepository
	memberRepo      repository.TeamMemberRepository
	assignRepo      repository.MemberWorkspaceAssignmentRepository
	approvalRepo    repository.ApprovalRequestRepository
	whitelist       WhitelistAppender
	now             func() time.Time
	tokenGen        func() (string, error)
	inviteTTL       time.Duration
}

// Options configures the team usecase.
type Options struct {
	Now       func() time.Time
	TokenGen  func() (string, error)
	InviteTTL time.Duration
}

// New constructs a team usecase. Nil whitelist is allowed (tests); nil opts
// use sensible defaults.
func New(
	roleRepo repository.RoleRepository,
	permRepo repository.RolePermissionRepository,
	memberRepo repository.TeamMemberRepository,
	assignRepo repository.MemberWorkspaceAssignmentRepository,
	approvalRepo repository.ApprovalRequestRepository,
	whitelist WhitelistAppender,
	opts Options,
) Usecase {
	uc := &usecase{
		roleRepo:     roleRepo,
		permRepo:     permRepo,
		memberRepo:   memberRepo,
		assignRepo:   assignRepo,
		approvalRepo: approvalRepo,
		whitelist:    whitelist,
		now:          opts.Now,
		tokenGen:     opts.TokenGen,
		inviteTTL:    opts.InviteTTL,
	}
	if uc.now == nil {
		uc.now = time.Now
	}
	if uc.tokenGen == nil {
		uc.tokenGen = generateToken
	}
	if uc.inviteTTL <= 0 {
		uc.inviteTTL = DefaultInviteTTL
	}
	return uc
}

func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("random: %w", err)
	}
	return "tok_" + hex.EncodeToString(buf), nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func initialsOf(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		runes := []rune(parts[0])
		if len(runes) >= 2 {
			return strings.ToUpper(string(runes[:2]))
		}
		return strings.ToUpper(string(runes))
	}
	return strings.ToUpper(string([]rune(parts[0])[:1]) + string([]rune(parts[len(parts)-1])[:1]))
}

func rpToFlags(p entity.RolePermission) PermissionFlags {
	return PermissionFlags{
		ViewList:   p.ViewList,
		ViewDetail: p.ViewDetail,
		Create:     p.CanCreate,
		Edit:       p.CanEdit,
		Delete:     p.CanDelete,
		Export:     p.CanExport,
		Import:     p.CanImport,
	}
}

func flagsToEntity(p PermissionFlags) entity.RolePermission {
	return entity.RolePermission{
		ViewList:   p.ViewList,
		ViewDetail: p.ViewDetail,
		CanCreate:  p.Create,
		CanEdit:    p.Edit,
		CanDelete:  p.Delete,
		CanExport:  p.Export,
		CanImport:  p.Import,
	}
}

func diffActions(before, after PermissionFlags) []string {
	var changed []string
	if before.ViewList != after.ViewList {
		changed = append(changed, entity.ActionViewList)
	}
	if before.ViewDetail != after.ViewDetail {
		changed = append(changed, entity.ActionViewDetail)
	}
	if before.Create != after.Create {
		changed = append(changed, entity.ActionCreate)
	}
	if before.Edit != after.Edit {
		changed = append(changed, entity.ActionEdit)
	}
	if before.Delete != after.Delete {
		changed = append(changed, entity.ActionDelete)
	}
	if before.Export != after.Export {
		changed = append(changed, entity.ActionExport)
	}
	if before.Import != after.Import {
		changed = append(changed, entity.ActionImport)
	}
	return changed
}

func (u *usecase) currentMember(ctx context.Context, email string) (*entity.TeamMember, *entity.Role, error) {
	if email == "" {
		return nil, nil, apperror.Unauthorized("missing caller email")
	}
	m, err := u.memberRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, nil, apperror.InternalError(err)
	}
	if m == nil {
		return nil, nil, apperror.Forbidden("caller is not a team member")
	}
	role, err := u.roleRepo.GetByID(ctx, m.RoleID)
	if err != nil {
		return nil, nil, apperror.InternalError(err)
	}
	return m, role, nil
}

// ─── Members ─────────────────────────────────────────────────────────────────

func (u *usecase) ListMembers(ctx context.Context, filter repository.TeamMemberFilter) (*ListMembersResult, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	members, total, err := u.memberRepo.List(ctx, filter)
	if err != nil {
		return nil, apperror.InternalError(err)
	}

	details := make([]MemberDetail, 0, len(members))
	for i := range members {
		d, derr := u.enrichMember(ctx, &members[i])
		if derr != nil {
			return nil, derr
		}
		details = append(details, *d)
	}

	summary, err := u.memberRepo.Summary(ctx, filter.WorkspaceID)
	if err != nil {
		return nil, apperror.InternalError(err)
	}

	return &ListMembersResult{
		Data:    details,
		Meta:    ListMeta{Offset: filter.Offset, Limit: filter.Limit, Total: total},
		Summary: summary,
	}, nil
}

func (u *usecase) enrichMember(ctx context.Context, m *entity.TeamMember) (*MemberDetail, error) {
	role, err := u.roleRepo.GetByID(ctx, m.RoleID)
	if err != nil {
		return nil, apperror.InternalError(err)
	}
	wsIDs, err := u.assignRepo.ListWorkspaceIDsByMember(ctx, m.ID)
	if err != nil {
		return nil, apperror.InternalError(err)
	}
	return &MemberDetail{TeamMember: *m, Role: role, WorkspaceIDs: wsIDs}, nil
}

func (u *usecase) GetMember(ctx context.Context, id string) (*MemberDetail, error) {
	m, err := u.memberRepo.GetByID(ctx, id)
	if err != nil {
		return nil, apperror.InternalError(err)
	}
	if m == nil {
		return nil, apperror.NotFound("team_member", "")
	}
	return u.enrichMember(ctx, m)
}

func (u *usecase) InviteMember(ctx context.Context, callerEmail string, req InviteRequest) (*entity.TeamMember, error) {
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Name == "" || req.Email == "" || req.RoleID == "" {
		return nil, apperror.BadRequest("name, email, role_id are required")
	}
	if len(req.WorkspaceIDs) == 0 {
		return nil, apperror.BadRequest("workspace_ids required")
	}

	existing, err := u.memberRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, apperror.InternalError(err)
	}
	if existing != nil {
		return nil, apperror.Conflict(fmt.Sprintf("Email %s is already a member", req.Email))
	}

	role, err := u.roleRepo.GetByID(ctx, req.RoleID)
	if err != nil {
		return nil, apperror.InternalError(err)
	}
	if role == nil {
		return nil, apperror.NotFound("role", "")
	}

	token, err := u.tokenGen()
	if err != nil {
		return nil, apperror.InternalError(err)
	}
	expires := u.now().Add(u.inviteTTL)

	m := &entity.TeamMember{
		Name:          req.Name,
		Email:         req.Email,
		Initials:      initialsOf(req.Name),
		RoleID:        req.RoleID,
		Status:        entity.MemberStatusPending,
		Department:    req.Department,
		InviteToken:   token,
		InviteExpires: &expires,
	}
	created, err := u.memberRepo.Create(ctx, m)
	if err != nil {
		return nil, apperror.InternalError(err)
	}
	for _, wsID := range req.WorkspaceIDs {
		if err := u.assignRepo.Assign(ctx, created.ID, wsID, ""); err != nil {
			return nil, apperror.InternalError(err)
		}
	}
	return created, nil
}

func (u *usecase) UpdateMember(ctx context.Context, id string, req UpdateMemberRequest) (*entity.TeamMember, error) {
	patch := repository.TeamMemberPatch{
		Name:        req.Name,
		Department:  req.Department,
		AvatarColor: req.AvatarColor,
	}
	out, err := u.memberRepo.Update(ctx, id, patch)
	if err != nil {
		if errors.Is(err, repository.ErrTeamNotFound) {
			return nil, apperror.NotFound("team_member", "")
		}
		return nil, apperror.InternalError(err)
	}
	return out, nil
}

func (u *usecase) ChangeRole(ctx context.Context, callerEmail, id, newRoleID string) (*entity.TeamMember, error) {
	_, callerRole, err := u.currentMember(ctx, callerEmail)
	if err != nil {
		return nil, err
	}

	target, err := u.memberRepo.GetByID(ctx, id)
	if err != nil {
		return nil, apperror.InternalError(err)
	}
	if target == nil {
		return nil, apperror.NotFound("team_member", "")
	}
	targetRole, err := u.roleRepo.GetByID(ctx, target.RoleID)
	if err != nil {
		return nil, apperror.InternalError(err)
	}

	// Only Super Admin can change another Super Admin.
	if targetRole != nil && targetRole.Name == entity.RoleSuperAdmin {
		if callerRole == nil || callerRole.Name != entity.RoleSuperAdmin {
			return nil, apperror.Forbidden("only a Super Admin can change another Super Admin's role")
		}
	}

	patch := repository.TeamMemberPatch{RoleID: &newRoleID}
	out, err := u.memberRepo.Update(ctx, id, patch)
	if err != nil {
		return nil, apperror.InternalError(err)
	}
	return out, nil
}

func (u *usecase) ChangeStatus(ctx context.Context, id, newStatus string) (*entity.TeamMember, error) {
	if newStatus != entity.MemberStatusActive && newStatus != entity.MemberStatusInactive && newStatus != entity.MemberStatusPending {
		return nil, apperror.BadRequest("invalid status")
	}
	patch := repository.TeamMemberPatch{Status: &newStatus}
	out, err := u.memberRepo.Update(ctx, id, patch)
	if err != nil {
		return nil, apperror.InternalError(err)
	}
	return out, nil
}

func (u *usecase) UpdateMemberWorkspaces(ctx context.Context, callerEmail, id string, workspaceIDs []string) (*entity.TeamMember, error) {
	m, err := u.memberRepo.GetByID(ctx, id)
	if err != nil {
		return nil, apperror.InternalError(err)
	}
	if m == nil {
		return nil, apperror.NotFound("team_member", "")
	}
	if err := u.assignRepo.ReplaceForMember(ctx, id, workspaceIDs, ""); err != nil {
		return nil, apperror.InternalError(err)
	}
	return m, nil
}

func (u *usecase) RemoveMember(ctx context.Context, callerEmail, id string) error {
	caller, callerRole, err := u.currentMember(ctx, callerEmail)
	if err != nil {
		return err
	}
	if caller.ID == id {
		return apperror.Forbidden("cannot delete yourself")
	}

	target, err := u.memberRepo.GetByID(ctx, id)
	if err != nil {
		return apperror.InternalError(err)
	}
	if target == nil {
		return apperror.NotFound("team_member", "")
	}
	targetRole, err := u.roleRepo.GetByID(ctx, target.RoleID)
	if err != nil {
		return apperror.InternalError(err)
	}
	if targetRole != nil && targetRole.Name == entity.RoleSuperAdmin {
		if callerRole == nil || callerRole.Name != entity.RoleSuperAdmin {
			return apperror.Forbidden("only a Super Admin can delete a Super Admin")
		}
	}

	if err := u.memberRepo.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrTeamNotFound) {
			return apperror.NotFound("team_member", "")
		}
		return apperror.InternalError(err)
	}
	return nil
}

// AcceptInvitation validates the token, flips status to active, links the
// caller's user ID, and appends the member to the auth whitelist so feat/01's
// auth gate keeps working (Option A: dual-write).
//
// TODO(feat/04+): bridge to ms-auth-proxy here so the auth service also
// knows about the newly active user without relying on the local whitelist.
func (u *usecase) AcceptInvitation(ctx context.Context, token, userID string) (*entity.TeamMember, error) {
	if token == "" {
		return nil, apperror.BadRequest("token required")
	}
	m, err := u.memberRepo.GetByInviteToken(ctx, token)
	if err != nil {
		return nil, apperror.InternalError(err)
	}
	if m == nil {
		return nil, apperror.NotFound("invitation", "invalid or expired token")
	}
	if m.InviteExpires != nil && m.InviteExpires.Before(u.now()) {
		return nil, apperror.BadRequest("invite token expired")
	}

	now := u.now()
	empty := ""
	active := entity.MemberStatusActive
	patch := repository.TeamMemberPatch{
		Status:      &active,
		InviteToken: &empty,
		JoinedAt:    &now,
	}
	if userID != "" {
		patch.UserID = &userID
	}
	out, err := u.memberRepo.Update(ctx, m.ID, patch)
	if err != nil {
		return nil, apperror.InternalError(err)
	}

	// Dual-write to the auth whitelist so feat/01 auth keeps passing. Nil
	// whitelist is allowed (unit tests), errors are non-fatal for idempotence.
	if u.whitelist != nil {
		if _, werr := u.whitelist.Create(ctx, out.Email, "team-invite", "auto via team invite accept"); werr != nil {
			if !errors.Is(werr, repository.ErrWhitelistDuplicate) {
				return out, apperror.InternalError(werr)
			}
		}
	}
	return out, nil
}

// ─── Roles ───────────────────────────────────────────────────────────────────

func (u *usecase) ListRoles(ctx context.Context, workspaceID string) ([]RoleSummary, error) {
	roles, err := u.roleRepo.List(ctx)
	if err != nil {
		return nil, apperror.InternalError(err)
	}
	out := make([]RoleSummary, 0, len(roles))
	for _, r := range roles {
		count, cerr := u.roleRepo.CountMembers(ctx, r.ID)
		if cerr != nil {
			return nil, apperror.InternalError(cerr)
		}
		out = append(out, RoleSummary{Role: r, MemberCount: count})
	}
	return out, nil
}

func (u *usecase) GetRole(ctx context.Context, id, workspaceID string) (*RoleDetail, error) {
	role, err := u.roleRepo.GetByID(ctx, id)
	if err != nil {
		return nil, apperror.InternalError(err)
	}
	if role == nil {
		return nil, apperror.NotFound("role", "")
	}
	count, err := u.roleRepo.CountMembers(ctx, id)
	if err != nil {
		return nil, apperror.InternalError(err)
	}

	var perms []entity.RolePermission
	if workspaceID != "" {
		perms, err = u.permRepo.ListByRoleWorkspace(ctx, id, workspaceID)
	} else {
		perms, err = u.permRepo.ListByRole(ctx, id)
	}
	if err != nil {
		return nil, apperror.InternalError(err)
	}

	byModule := map[string]PermissionFlags{}
	for _, p := range perms {
		byModule[p.ModuleID] = rpToFlags(p)
	}
	return &RoleDetail{Role: *role, MemberCount: count, Permissions: byModule}, nil
}

func (u *usecase) CreateRole(ctx context.Context, req CreateRoleRequest) (*entity.Role, error) {
	if req.Name == "" {
		return nil, apperror.BadRequest("name is required")
	}
	role := &entity.Role{
		Name:        req.Name,
		Description: req.Description,
		Color:       req.Color,
		BgColor:     req.BgColor,
		IsSystem:    false,
	}
	return u.roleRepo.Create(ctx, role)
}

func (u *usecase) UpdateRole(ctx context.Context, id string, req UpdateRoleRequest) (*entity.Role, error) {
	existing, err := u.roleRepo.GetByID(ctx, id)
	if err != nil {
		return nil, apperror.InternalError(err)
	}
	if existing == nil {
		return nil, apperror.NotFound("role", "")
	}
	// System roles cannot be renamed.
	if existing.IsSystem && req.Name != nil && *req.Name != existing.Name {
		return nil, apperror.Forbidden("system roles cannot be renamed")
	}
	patch := repository.RolePatch{
		Name:        req.Name,
		Description: req.Description,
		Color:       req.Color,
		BgColor:     req.BgColor,
	}
	return u.roleRepo.Update(ctx, id, patch)
}

func (u *usecase) UpdateRolePermissions(ctx context.Context, callerEmail, id string, req UpdateRolePermissionRequest) (*PermissionChange, error) {
	_, callerRole, err := u.currentMember(ctx, callerEmail)
	if err != nil {
		return nil, err
	}
	role, err := u.roleRepo.GetByID(ctx, id)
	if err != nil {
		return nil, apperror.InternalError(err)
	}
	if role == nil {
		return nil, apperror.NotFound("role", "")
	}
	if role.Name == entity.RoleSuperAdmin && (callerRole == nil || callerRole.Name != entity.RoleSuperAdmin) {
		return nil, apperror.Forbidden("only a Super Admin can edit Super Admin permissions")
	}
	if req.WorkspaceID == "" || req.ModuleID == "" {
		return nil, apperror.BadRequest("workspace_id and module_id are required")
	}

	before, err := u.permRepo.GetOne(ctx, id, req.WorkspaceID, req.ModuleID)
	if err != nil {
		return nil, apperror.InternalError(err)
	}
	var beforeFlags PermissionFlags
	if before != nil {
		beforeFlags = rpToFlags(*before)
	} else {
		beforeFlags = PermissionFlags{ViewList: entity.ViewScopeFalse}
	}

	newRP := flagsToEntity(req.Permissions)
	newRP.RoleID = id
	newRP.WorkspaceID = req.WorkspaceID
	newRP.ModuleID = req.ModuleID

	after, err := u.permRepo.Upsert(ctx, &newRP)
	if err != nil {
		return nil, apperror.InternalError(err)
	}
	afterFlags := rpToFlags(*after)

	changed := diffActions(beforeFlags, afterFlags)
	summary := fmt.Sprintf("%s/%s changed: %s", req.ModuleID, req.WorkspaceID, strings.Join(changed, ","))

	return &PermissionChange{
		Role:           role,
		ModuleID:       req.ModuleID,
		WorkspaceID:    req.WorkspaceID,
		ChangedActions: changed,
		Summary:        summary,
		After:          afterFlags,
	}, nil
}

func (u *usecase) DeleteRole(ctx context.Context, id string) error {
	role, err := u.roleRepo.GetByID(ctx, id)
	if err != nil {
		return apperror.InternalError(err)
	}
	if role == nil {
		return apperror.NotFound("role", "")
	}
	if role.IsSystem {
		return apperror.Forbidden("system roles cannot be deleted")
	}
	count, err := u.roleRepo.CountMembers(ctx, id)
	if err != nil {
		return apperror.InternalError(err)
	}
	if count > 0 {
		return apperror.Conflict(fmt.Sprintf("Cannot delete role with %d active members. Reassign members first.", count))
	}
	if err := u.roleRepo.Delete(ctx, id); err != nil {
		return apperror.InternalError(err)
	}
	return nil
}

// ─── Permissions ─────────────────────────────────────────────────────────────

func (u *usecase) resolvePermission(ctx context.Context, callerEmail, workspaceID, module string) (*entity.ResolvedPermission, error) {
	m, err := u.memberRepo.GetByEmail(ctx, callerEmail)
	if err != nil {
		return nil, apperror.InternalError(err)
	}
	if m == nil || m.Status != entity.MemberStatusActive {
		return nil, nil
	}
	p, err := u.permRepo.GetOne(ctx, m.RoleID, workspaceID, module)
	if err != nil {
		return nil, apperror.InternalError(err)
	}
	if p == nil {
		return &entity.ResolvedPermission{
			RoleID:      m.RoleID,
			WorkspaceID: workspaceID,
			ModuleID:    module,
			ViewList:    entity.ViewScopeFalse,
		}, nil
	}
	return &entity.ResolvedPermission{
		RoleID:      p.RoleID,
		WorkspaceID: p.WorkspaceID,
		ModuleID:    p.ModuleID,
		ViewList:    p.ViewList,
		ViewDetail:  p.ViewDetail,
		CanCreate:   p.CanCreate,
		CanEdit:     p.CanEdit,
		CanDelete:   p.CanDelete,
		CanExport:   p.CanExport,
		CanImport:   p.CanImport,
	}, nil
}

// CheckPermission returns (allowed, resolved, error). If the caller has no
// member record, allowed is false and resolved is nil.
func (u *usecase) CheckPermission(ctx context.Context, callerEmail, workspaceID, module, action string) (bool, *entity.ResolvedPermission, error) {
	resolved, err := u.resolvePermission(ctx, callerEmail, workspaceID, module)
	if err != nil {
		return false, nil, err
	}
	if resolved == nil {
		return false, nil, nil
	}
	return resolved.Allowed(action), resolved, nil
}

func (u *usecase) GetMyPermissions(ctx context.Context, callerEmail, workspaceID string) (*MyPermissions, error) {
	m, role, err := u.currentMember(ctx, callerEmail)
	if err != nil {
		return nil, err
	}
	perms, err := u.permRepo.ListByRoleWorkspace(ctx, m.RoleID, workspaceID)
	if err != nil {
		return nil, apperror.InternalError(err)
	}
	byModule := map[string]PermissionFlags{}
	for _, p := range perms {
		byModule[p.ModuleID] = rpToFlags(p)
	}
	// Ensure every known module has an entry so UI rendering is predictable.
	for _, mod := range entity.AllModules {
		if _, ok := byModule[mod]; !ok {
			byModule[mod] = PermissionFlags{ViewList: entity.ViewScopeFalse}
		}
	}
	return &MyPermissions{Role: role, WorkspaceID: workspaceID, Permissions: byModule}, nil
}
