package entity

import "time"

// Role is a named set of permissions. System roles (IsSystem=true) cannot be
// renamed or deleted.
type Role struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Color       string    `json:"color"`
	BgColor     string    `json:"bg_color"`
	IsSystem    bool      `json:"is_system"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// RolePermission is one (role, workspace, module) permission row.
// ViewList has three possible values: "false", "true", "all".
type RolePermission struct {
	ID          string    `json:"id"`
	RoleID      string    `json:"role_id"`
	WorkspaceID string    `json:"workspace_id"`
	ModuleID    string    `json:"module_id"`
	ViewList    string    `json:"view_list"`
	ViewDetail  bool      `json:"view_detail"`
	CanCreate   bool      `json:"create"`
	CanEdit     bool      `json:"edit"`
	CanDelete   bool      `json:"delete"`
	CanExport   bool      `json:"export"`
	CanImport   bool      `json:"import"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TeamMember is a user of the dashboard. Email is authoritative; UserID is
// nullable and only populated once external auth has linked the account.
type TeamMember struct {
	ID            string     `json:"id"`
	UserID        string     `json:"user_id,omitempty"`
	Name          string     `json:"name"`
	Email         string     `json:"email"`
	Initials      string     `json:"initials"`
	RoleID        string     `json:"role_id"`
	Status        string     `json:"status"`
	Department    string     `json:"department"`
	AvatarColor   string     `json:"avatar_color"`
	InviteToken   string     `json:"invite_token,omitempty"`
	InviteExpires *time.Time `json:"invite_expires,omitempty"`
	InvitedBy     string     `json:"invited_by,omitempty"`
	JoinedAt      *time.Time `json:"joined_at,omitempty"`
	LastActiveAt  *time.Time `json:"last_active_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// MemberWorkspaceAssignment links a team member to a workspace.
type MemberWorkspaceAssignment struct {
	ID          string    `json:"id"`
	MemberID    string    `json:"member_id"`
	WorkspaceID string    `json:"workspace_id"`
	AssignedAt  time.Time `json:"assigned_at"`
	AssignedBy  string    `json:"assigned_by,omitempty"`
}

// Member statuses.
const (
	MemberStatusActive   = "active"
	MemberStatusPending  = "pending"
	MemberStatusInactive = "inactive"
)

// Permission modules.
const (
	ModuleDashboard  = "dashboard"
	ModuleAnalytics  = "analytics"
	ModuleReports    = "reports"
	ModuleAE         = "ae"
	ModuleSDR        = "sdr"
	ModuleBD         = "bd"
	ModuleCS         = "cs"
	ModuleDataMaster = "data_master"
	ModuleTeam       = "team"
)

// AllModules is the canonical module list used by the seed and the
// permission resolver.
var AllModules = []string{
	ModuleDashboard, ModuleAnalytics, ModuleReports,
	ModuleAE, ModuleSDR, ModuleBD, ModuleCS,
	ModuleDataMaster, ModuleTeam,
}

// Permission actions.
const (
	ActionViewList   = "view_list"
	ActionViewDetail = "view_detail"
	ActionCreate     = "create"
	ActionEdit       = "edit"
	ActionDelete     = "delete"
	ActionExport     = "export"
	ActionImport     = "import"
)

// ViewList scope tokens.
const (
	ViewScopeFalse = "false"
	ViewScopeTrue  = "true"
	ViewScopeAll   = "all"
)

// RoleSuperAdmin is the canonical name of the Super Admin role — guarded
// against self-delete and non-super-admin modification.
const RoleSuperAdmin = "Super Admin"

// Team approval request types (checker-maker).
const (
	ApprovalTypeInviteMember     = "invite_member"
	ApprovalTypeRemoveMember     = "remove_member"
	ApprovalTypeChangePermission = "change_permission"
)

// ResolvedPermission is the outcome of a permission check against a single
// (role, workspace, module) tuple. It collapses the boolean actions plus the
// ternary ViewList scope into something handlers can read directly.
type ResolvedPermission struct {
	RoleID      string `json:"role_id"`
	WorkspaceID string `json:"workspace_id"`
	ModuleID    string `json:"module_id"`
	ViewList    string `json:"view_list"`
	ViewDetail  bool   `json:"view_detail"`
	CanCreate   bool   `json:"create"`
	CanEdit     bool   `json:"edit"`
	CanDelete   bool   `json:"delete"`
	CanExport   bool   `json:"export"`
	CanImport   bool   `json:"import"`
}

// Allowed reports whether the given action is permitted. For view_list, both
// 'true' and 'all' mean allowed; for everything else the boolean flag wins.
func (p ResolvedPermission) Allowed(action string) bool {
	switch action {
	case ActionViewList:
		return p.ViewList == ViewScopeTrue || p.ViewList == ViewScopeAll
	case ActionViewDetail:
		return p.ViewDetail
	case ActionCreate:
		return p.CanCreate
	case ActionEdit:
		return p.CanEdit
	case ActionDelete:
		return p.CanDelete
	case ActionExport:
		return p.CanExport
	case ActionImport:
		return p.CanImport
	}
	return false
}
