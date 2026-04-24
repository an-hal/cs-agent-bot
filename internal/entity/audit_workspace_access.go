package entity

import "time"

// Access kinds for AuditWorkspaceAccess.Kind.
const (
	WorkspaceAccessKindRead  = "read"
	WorkspaceAccessKindWrite = "write"
	WorkspaceAccessKindAdmin = "admin"
)

// AuditWorkspaceAccess records cross-workspace access for compliance (PDP).
// Written every time a user touches a workspace they are not a direct member
// of (holding/admin flow).
type AuditWorkspaceAccess struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	ActorEmail  string    `json:"actor_email"`
	Kind        string    `json:"access_kind"`
	Resource    string    `json:"resource,omitempty"`
	ResourceID  string    `json:"resource_id,omitempty"`
	IPAddress   string    `json:"ip_address,omitempty"`
	UserAgent   string    `json:"user_agent,omitempty"`
	Reason      string    `json:"reason,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// AuditWorkspaceAccessFilter is used by List.
type AuditWorkspaceAccessFilter struct {
	WorkspaceID string
	ActorEmail  string
	Kind        string
	Resource    string
	Limit       int
	Offset      int
}
