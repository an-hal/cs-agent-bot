package team

import (
	"encoding/json"
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	teamuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/team"
	"github.com/rs/zerolog"
)

// Handler exposes the team usecase over HTTP.
type Handler struct {
	uc     teamuc.Usecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

// NewHandler constructs the team HTTP handler.
func NewHandler(uc teamuc.Usecase, logger zerolog.Logger, tr tracer.Tracer) *Handler {
	return &Handler{uc: uc, logger: logger, tracer: tr}
}

// ListMembers godoc
// @Summary      List team members in the current workspace
// @Tags         Team
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        status          query   string  false "Filter by status"
// @Param        role_id         query   string  false "Filter by role"
// @Param        search          query   string  false "Search name/email/dept"
// @Param        offset          query   int     false "Pagination offset"
// @Param        limit           query   int     false "Pagination limit"
// @Success      200  {object}  response.StandardResponse{data=team.ListMembersResult}
// @Failure      401  {object}  response.StandardResponse
// @Failure      403  {object}  response.StandardResponse
// @Router       /api/team/members [get]
func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "team.handler.ListMembers")
	defer span.End()

	workspaceID := ctxutil.GetWorkspaceID(ctx)
	filter := listMemberFilterFromQuery(r, workspaceID)
	out, err := h.uc.ListMembers(ctx, filter)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Members", out)
}

// GetMember godoc
// @Summary      Get a single team member
// @Tags         Team
// @Security     BearerAuth
// @Param        id              path    string  true  "Member ID"
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse{data=team.MemberDetail}
// @Failure      404  {object}  response.StandardResponse
// @Router       /api/team/members/{id} [get]
func (h *Handler) GetMember(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "team.handler.GetMember")
	defer span.End()

	id := router.GetParam(r, "id")
	out, err := h.uc.GetMember(ctx, id)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Member", out)
}

// InviteMember godoc
// @Summary      Invite a new member to the team
// @Tags         Team
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID  header  string                  true  "Workspace ID"
// @Param        body            body    team.InviteRequest      true  "Invite payload"
// @Success      201  {object}  response.StandardResponse{data=entity.TeamMember}
// @Failure      400  {object}  response.StandardResponse
// @Failure      409  {object}  response.StandardResponse
// @Router       /api/team/members/invite [post]
func (h *Handler) InviteMember(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "team.handler.InviteMember")
	defer span.End()

	var req teamuc.InviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.InviteMember(ctx, callerEmail(r), req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusCreated, "Invitation sent", out)
}

// UpdateMember godoc
// @Summary      Update a member (partial)
// @Tags         Team
// @Security     BearerAuth
// @Accept       json
// @Param        id              path    string                      true  "Member ID"
// @Param        X-Workspace-ID  header  string                      true  "Workspace ID"
// @Param        body            body    team.UpdateMemberRequest    true  "Patch payload"
// @Success      200  {object}  response.StandardResponse{data=entity.TeamMember}
// @Failure      404  {object}  response.StandardResponse
// @Router       /api/team/members/{id} [put]
func (h *Handler) UpdateMember(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "team.handler.UpdateMember")
	defer span.End()

	id := router.GetParam(r, "id")
	var req teamuc.UpdateMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.UpdateMember(ctx, id, req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Member updated", out)
}

// ChangeRole godoc
// @Summary      Change a member's role
// @Tags         Team
// @Security     BearerAuth
// @Accept       json
// @Param        id              path    string             true  "Member ID"
// @Param        X-Workspace-ID  header  string             true  "Workspace ID"
// @Param        body            body    ChangeRoleRequest  true  "Role payload"
// @Success      200  {object}  response.StandardResponse{data=entity.TeamMember}
// @Failure      403  {object}  response.StandardResponse
// @Router       /api/team/members/{id}/role [put]
func (h *Handler) ChangeRole(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "team.handler.ChangeRole")
	defer span.End()

	id := router.GetParam(r, "id")
	var req ChangeRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	if req.RoleID == "" {
		return apperror.BadRequest("role_id is required")
	}
	out, err := h.uc.ChangeRole(ctx, callerEmail(r), id, req.RoleID)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Member role updated", out)
}

// ChangeStatus godoc
// @Summary      Change a member's status
// @Tags         Team
// @Security     BearerAuth
// @Accept       json
// @Param        id              path    string               true  "Member ID"
// @Param        X-Workspace-ID  header  string               true  "Workspace ID"
// @Param        body            body    ChangeStatusRequest  true  "Status payload"
// @Success      200  {object}  response.StandardResponse{data=entity.TeamMember}
// @Failure      400  {object}  response.StandardResponse
// @Router       /api/team/members/{id}/status [put]
func (h *Handler) ChangeStatus(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "team.handler.ChangeStatus")
	defer span.End()

	id := router.GetParam(r, "id")
	var req ChangeStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.ChangeStatus(ctx, id, req.Status)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Member status updated", out)
}

// UpdateMemberWorkspaces godoc
// @Summary      Replace the set of workspaces a member is assigned to
// @Tags         Team
// @Security     BearerAuth
// @Accept       json
// @Param        id              path    string                   true  "Member ID"
// @Param        X-Workspace-ID  header  string                   true  "Workspace ID"
// @Param        body            body    UpdateWorkspacesRequest  true  "Workspace IDs"
// @Success      200  {object}  response.StandardResponse{data=entity.TeamMember}
// @Router       /api/team/members/{id}/workspaces [put]
func (h *Handler) UpdateMemberWorkspaces(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "team.handler.UpdateMemberWorkspaces")
	defer span.End()

	id := router.GetParam(r, "id")
	var req UpdateWorkspacesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.UpdateMemberWorkspaces(ctx, callerEmail(r), id, req.WorkspaceIDs)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Workspaces updated", out)
}

// RemoveMember godoc
// @Summary      Remove a team member
// @Tags         Team
// @Security     BearerAuth
// @Param        id              path    string  true  "Member ID"
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse
// @Failure      403  {object}  response.StandardResponse
// @Router       /api/team/members/{id} [delete]
func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "team.handler.RemoveMember")
	defer span.End()

	id := router.GetParam(r, "id")
	if err := h.uc.RemoveMember(ctx, callerEmail(r), id); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Member removed", map[string]string{"id": id})
}

// AcceptInvitation godoc
// @Summary      Accept a team invitation
// @Tags         Team
// @Security     BearerAuth
// @Param        token  path  string  true  "Invite token"
// @Success      200  {object}  response.StandardResponse{data=entity.TeamMember}
// @Failure      400  {object}  response.StandardResponse
// @Failure      404  {object}  response.StandardResponse
// @Router       /api/team/invitations/{token}/accept [post]
func (h *Handler) AcceptInvitation(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "team.handler.AcceptInvitation")
	defer span.End()

	token := router.GetParam(r, "token")
	userID := callerUserID(r)
	out, err := h.uc.AcceptInvitation(ctx, token, userID)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Invitation accepted", out)
}
