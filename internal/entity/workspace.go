package entity

import "time"

// Workspace represents a tenant in the multi-workspace CRM.
// A workspace with IsHolding=true aggregates data from MemberIDs and is read-only
// against tenant-scoped tables.
type Workspace struct {
	ID        string                 `json:"id"`
	Slug      string                 `json:"slug"`
	Name      string                 `json:"name"`
	Logo      string                 `json:"logo"`
	Color     string                 `json:"color"`
	Plan      string                 `json:"plan"`
	IsHolding bool                   `json:"is_holding"`
	MemberIDs []string               `json:"member_ids"`
	Settings  map[string]interface{} `json:"settings"`
	IsActive  bool                   `json:"is_active"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// WorkspaceMember binds a user (by email) to a workspace with a role.
// user_id FK is intentionally absent — there is no users table on this branch.
type WorkspaceMember struct {
	ID          string                 `json:"id"`
	WorkspaceID string                 `json:"workspace_id"`
	UserEmail   string                 `json:"user_email"`
	UserName    string                 `json:"user_name"`
	Role        string                 `json:"role"`
	Permissions map[string]interface{} `json:"permissions"`
	IsActive    bool                   `json:"is_active"`
	InvitedAt   *time.Time             `json:"invited_at,omitempty"`
	JoinedAt    *time.Time             `json:"joined_at,omitempty"`
	InvitedBy   string                 `json:"invited_by,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// WorkspaceInvitation represents a pending invite to join a workspace.
// Once accepted the corresponding WorkspaceMember row is created and Status flips to accepted.
type WorkspaceInvitation struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspace_id"`
	Email       string     `json:"email"`
	Role        string     `json:"role"`
	InviteToken string     `json:"invite_token"`
	Status      string     `json:"status"`
	InvitedBy   string     `json:"invited_by"`
	AcceptedAt  *time.Time `json:"accepted_at,omitempty"`
	ExpiresAt   time.Time  `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Workspace roles.
const (
	WorkspaceRoleOwner  = "owner"
	WorkspaceRoleAdmin  = "admin"
	WorkspaceRoleMember = "member"
	WorkspaceRoleViewer = "viewer"
)

// Workspace invitation statuses.
const (
	InvitationStatusPending  = "pending"
	InvitationStatusAccepted = "accepted"
	InvitationStatusExpired  = "expired"
	InvitationStatusRevoked  = "revoked"
)

// CanManageWorkspace reports whether the role can edit workspace settings.
func CanManageWorkspace(role string) bool {
	return role == WorkspaceRoleOwner || role == WorkspaceRoleAdmin
}

// CanInviteMembers reports whether the role can invite new members.
func CanInviteMembers(role string) bool {
	return role == WorkspaceRoleOwner || role == WorkspaceRoleAdmin
}

// CanRemoveMembers reports whether the role can remove other members.
func CanRemoveMembers(role string) bool {
	return role == WorkspaceRoleOwner
}

// CanDeleteWorkspace reports whether the role can soft-delete the workspace.
func CanDeleteWorkspace(role string) bool {
	return role == WorkspaceRoleOwner
}
