package dashboard

import (
	"encoding/json"
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/middleware"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/workspace"
	"github.com/rs/zerolog"
)

type WorkspaceHandler struct {
	uc     workspace.Usecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewWorkspaceHandler(uc workspace.Usecase, logger zerolog.Logger, tr tracer.Tracer) *WorkspaceHandler {
	return &WorkspaceHandler{uc: uc, logger: logger, tracer: tr}
}

func callerEmail(r *http.Request) string {
	if u, ok := middleware.GetJWTUser(r.Context()); ok {
		return u.Email
	}
	return ""
}

// List godoc
// @Summary      List workspaces visible to the caller
// @Tags         Workspace
// @Security     BearerAuth
// @Success      200  {object}  response.StandardResponse{data=[]entity.Workspace}
// @Failure      401  {object}  response.StandardResponse
// @Router       /api/workspaces [get]
func (h *WorkspaceHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.WorkspaceList")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	out, err := h.uc.List(ctx, callerEmail(r))
	if err != nil {
		return err
	}
	if out == nil {
		out = []entity.Workspace{}
	}
	logger.Info().Int("count", len(out)).Msg("workspaces listed")
	return response.StandardSuccess(w, r, http.StatusOK, "Workspaces", out)
}

// Mine godoc
// @Summary      List workspaces the caller has actual membership in
// @Description  Strict membership listing — checks workspace_members and team_members ACL paths. No holding-bypass.
// @Tags         Workspace
// @Security     BearerAuth
// @Success      200  {object}  response.StandardResponse{data=[]entity.Workspace}
// @Failure      401  {object}  response.StandardResponse
// @Router       /api/workspaces/mine [get]
func (h *WorkspaceHandler) Mine(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.WorkspaceMine")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	out, err := h.uc.ListMine(ctx, callerEmail(r))
	if err != nil {
		return err
	}
	if out == nil {
		out = []entity.Workspace{}
	}
	logger.Info().Int("count", len(out)).Msg("workspaces (mine) listed")
	return response.StandardSuccess(w, r, http.StatusOK, "Workspaces", out)
}

// Get godoc
// @Summary      Get workspace details
// @Tags         Workspace
// @Security     BearerAuth
// @Param        id   path      string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse{data=workspace.WorkspaceDetail}
// @Failure      403  {object}  response.StandardResponse
// @Failure      404  {object}  response.StandardResponse
// @Router       /api/workspaces/{id} [get]
func (h *WorkspaceHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.WorkspaceGet")
	defer span.End()

	id := router.GetParam(r, "id")
	out, err := h.uc.Get(ctx, id, callerEmail(r))
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Workspace", out)
}

// Create godoc
// @Summary      Create a new workspace
// @Tags         Workspace
// @Security     BearerAuth
// @Accept       json
// @Param        body  body      workspace.CreateRequest  true  "Workspace payload"
// @Success      201   {object}  response.StandardResponse{data=entity.Workspace}
// @Failure      400   {object}  response.StandardResponse
// @Failure      409   {object}  response.StandardResponse
// @Router       /api/workspaces [post]
func (h *WorkspaceHandler) Create(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.WorkspaceCreate")
	defer span.End()

	var req workspace.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.Create(ctx, callerEmail(r), req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusCreated, "Workspace created", out)
}

// Update godoc
// @Summary      Update a workspace (partial)
// @Tags         Workspace
// @Security     BearerAuth
// @Accept       json
// @Param        id    path      string                   true  "Workspace ID"
// @Param        body  body      workspace.UpdateRequest  true  "Partial update"
// @Success      200   {object}  response.StandardResponse{data=entity.Workspace}
// @Failure      403   {object}  response.StandardResponse
// @Router       /api/workspaces/{id} [put]
func (h *WorkspaceHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.WorkspaceUpdate")
	defer span.End()

	id := router.GetParam(r, "id")
	var req workspace.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.Update(ctx, id, callerEmail(r), req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Workspace updated", out)
}

// SoftDelete godoc
// @Summary      Soft-delete a workspace
// @Tags         Workspace
// @Security     BearerAuth
// @Param        id   path      string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse
// @Failure      403  {object}  response.StandardResponse
// @Router       /api/workspaces/{id} [delete]
func (h *WorkspaceHandler) SoftDelete(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.WorkspaceDelete")
	defer span.End()

	id := router.GetParam(r, "id")
	if err := h.uc.SoftDelete(ctx, id, callerEmail(r)); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Workspace berhasil di-nonaktifkan", map[string]string{"id": id})
}

// Switch godoc
// @Summary      Switch active workspace (audit)
// @Tags         Workspace
// @Security     BearerAuth
// @Param        id   path      string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse{data=workspace.SwitchResponse}
// @Router       /api/workspaces/{id}/switch [post]
func (h *WorkspaceHandler) Switch(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.WorkspaceSwitch")
	defer span.End()

	id := router.GetParam(r, "id")
	out, err := h.uc.Switch(ctx, id, callerEmail(r))
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Workspace switched", out)
}

// ListMembers godoc
// @Summary      List members of a workspace
// @Tags         Workspace
// @Security     BearerAuth
// @Param        id   path      string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse{data=[]entity.WorkspaceMember}
// @Router       /api/workspaces/{id}/members [get]
func (h *WorkspaceHandler) ListMembers(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.WorkspaceListMembers")
	defer span.End()

	id := router.GetParam(r, "id")
	out, err := h.uc.ListMembers(ctx, id, callerEmail(r))
	if err != nil {
		return err
	}
	if out == nil {
		out = []entity.WorkspaceMember{}
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Members", out)
}

// Invite godoc
// @Summary      Invite user to workspace
// @Tags         Workspace
// @Security     BearerAuth
// @Accept       json
// @Param        id    path      string                   true  "Workspace ID"
// @Param        body  body      workspace.InviteRequest  true  "Invitation payload"
// @Success      201   {object}  response.StandardResponse{data=entity.WorkspaceInvitation}
// @Router       /api/workspaces/{id}/members/invite [post]
func (h *WorkspaceHandler) Invite(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.WorkspaceInvite")
	defer span.End()

	id := router.GetParam(r, "id")
	var req workspace.InviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.Invite(ctx, id, callerEmail(r), req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusCreated, "Invitation sent", out)
}

// UpdateMemberRole godoc
// @Summary      Update workspace member role
// @Tags         Workspace
// @Security     BearerAuth
// @Accept       json
// @Param        id         path      string                       true  "Workspace ID"
// @Param        member_id  path      string                       true  "Member ID"
// @Param        body       body      dashboard.UpdateRoleRequest  true  "Role payload"
// @Success      200        {object}  response.StandardResponse{data=entity.WorkspaceMember}
// @Router       /api/workspaces/{id}/members/{member_id} [put]
func (h *WorkspaceHandler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.WorkspaceUpdateMemberRole")
	defer span.End()

	id := router.GetParam(r, "id")
	memberID := router.GetParam(r, "member_id")
	var req UpdateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.UpdateMemberRole(ctx, id, memberID, callerEmail(r), req.Role)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Member role updated", out)
}

// RemoveMember godoc
// @Summary      Remove member from workspace
// @Tags         Workspace
// @Security     BearerAuth
// @Param        id         path      string  true  "Workspace ID"
// @Param        member_id  path      string  true  "Member ID"
// @Success      200        {object}  response.StandardResponse
// @Router       /api/workspaces/{id}/members/{member_id} [delete]
func (h *WorkspaceHandler) RemoveMember(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.WorkspaceRemoveMember")
	defer span.End()

	id := router.GetParam(r, "id")
	memberID := router.GetParam(r, "member_id")
	if err := h.uc.RemoveMember(ctx, id, memberID, callerEmail(r)); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Member dihapus dari workspace", map[string]string{"member_id": memberID})
}

// AcceptInvitation godoc
// @Summary      Accept a workspace invitation
// @Tags         Workspace
// @Security     BearerAuth
// @Param        token  path      string  true  "Invitation token"
// @Success      200    {object}  response.StandardResponse{data=entity.WorkspaceMember}
// @Router       /api/workspaces/invitations/{token}/accept [post]
func (h *WorkspaceHandler) AcceptInvitation(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.WorkspaceAcceptInvitation")
	defer span.End()

	token := router.GetParam(r, "token")
	caller := callerEmail(r)
	user, _ := middleware.GetJWTUser(r.Context())
	name := caller
	if user != nil && user.ID != "" {
		name = user.ID
	}
	out, err := h.uc.AcceptInvitation(ctx, token, caller, name)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Invitation accepted", out)
}

// UpdateRoleRequest is the body for PUT /workspaces/{id}/members/{member_id}.
type UpdateRoleRequest struct {
	Role string `json:"role"`
}
