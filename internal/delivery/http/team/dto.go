// Package team exposes HTTP handlers for the team management / RBAC feature.
// Wraps internal/usecase/team.Usecase. Handlers run behind JWT + workspace
// middleware; each route is gated by RequirePermission(...) at registration.
package team

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/middleware"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
)

// callerEmail extracts the JWT-authenticated email (empty string if missing).
func callerEmail(r *http.Request) string {
	if u, ok := middleware.GetJWTUser(r.Context()); ok {
		return u.Email
	}
	return ""
}

// callerUserID extracts the JWT-authenticated user ID (empty string if missing).
func callerUserID(r *http.Request) string {
	if u, ok := middleware.GetJWTUser(r.Context()); ok {
		return u.ID
	}
	return ""
}

// listMemberFilterFromQuery parses query params into a TeamMemberFilter.
// workspaceID is read from middleware context, not the query string.
func listMemberFilterFromQuery(r *http.Request, workspaceID string) repository.TeamMemberFilter {
	q := r.URL.Query()
	limit := 50
	offset := 0
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return repository.TeamMemberFilter{
		WorkspaceID: workspaceID,
		Status:      strings.TrimSpace(q.Get("status")),
		RoleID:      strings.TrimSpace(q.Get("role_id")),
		Search:      strings.TrimSpace(q.Get("search")),
		Offset:      offset,
		Limit:       limit,
		SortBy:      strings.TrimSpace(q.Get("sort_by")),
		SortDir:     strings.TrimSpace(q.Get("sort_dir")),
	}
}

// ChangeRoleRequest is the body for PUT /team/members/{id}/role.
type ChangeRoleRequest struct {
	RoleID string `json:"role_id"`
}

// ChangeStatusRequest is the body for PUT /team/members/{id}/status.
type ChangeStatusRequest struct {
	Status string `json:"status"`
}

// UpdateWorkspacesRequest is the body for PUT /team/members/{id}/workspaces.
type UpdateWorkspacesRequest struct {
	WorkspaceIDs []string `json:"workspace_ids"`
}

// AcceptInvitationRequest is the body for POST /team/invitations/{token}/accept.
type AcceptInvitationRequest struct {
	UserID string `json:"user_id"`
}
