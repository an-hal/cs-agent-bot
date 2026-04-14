package team

import (
	"encoding/json"
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	teamuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/team"
)

// ListRoles godoc
// @Summary      List roles
// @Tags         Team
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse{data=[]team.RoleSummary}
// @Router       /api/team/roles [get]
func (h *Handler) ListRoles(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "team.handler.ListRoles")
	defer span.End()

	workspaceID := ctxutil.GetWorkspaceID(ctx)
	out, err := h.uc.ListRoles(ctx, workspaceID)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Roles", out)
}

// GetRole godoc
// @Summary      Get role with full permission matrix for current workspace
// @Tags         Team
// @Security     BearerAuth
// @Param        id              path    string  true  "Role ID"
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse{data=team.RoleDetail}
// @Failure      404  {object}  response.StandardResponse
// @Router       /api/team/roles/{id} [get]
func (h *Handler) GetRole(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "team.handler.GetRole")
	defer span.End()

	id := router.GetParam(r, "id")
	workspaceID := ctxutil.GetWorkspaceID(ctx)
	out, err := h.uc.GetRole(ctx, id, workspaceID)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Role", out)
}

// CreateRole godoc
// @Summary      Create a new role
// @Tags         Team
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID  header  string                  true  "Workspace ID"
// @Param        body            body    team.CreateRoleRequest  true  "Role payload"
// @Success      201  {object}  response.StandardResponse{data=entity.Role}
// @Router       /api/team/roles [post]
func (h *Handler) CreateRole(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "team.handler.CreateRole")
	defer span.End()

	var req teamuc.CreateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.CreateRole(ctx, req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusCreated, "Role created", out)
}

// UpdateRole godoc
// @Summary      Update a role (partial)
// @Tags         Team
// @Security     BearerAuth
// @Accept       json
// @Param        id              path    string                  true  "Role ID"
// @Param        X-Workspace-ID  header  string                  true  "Workspace ID"
// @Param        body            body    team.UpdateRoleRequest  true  "Patch payload"
// @Success      200  {object}  response.StandardResponse{data=entity.Role}
// @Failure      403  {object}  response.StandardResponse
// @Router       /api/team/roles/{id} [put]
func (h *Handler) UpdateRole(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "team.handler.UpdateRole")
	defer span.End()

	id := router.GetParam(r, "id")
	var req teamuc.UpdateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.UpdateRole(ctx, id, req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Role updated", out)
}

// UpdateRolePermissions godoc
// @Summary      Update one (workspace, module) permission cell for a role
// @Tags         Team
// @Security     BearerAuth
// @Accept       json
// @Param        id              path    string                            true  "Role ID"
// @Param        X-Workspace-ID  header  string                            true  "Workspace ID"
// @Param        body            body    team.UpdateRolePermissionRequest  true  "Permission payload"
// @Success      200  {object}  response.StandardResponse{data=team.PermissionChange}
// @Failure      400  {object}  response.StandardResponse
// @Failure      403  {object}  response.StandardResponse
// @Router       /api/team/roles/{id}/permissions [put]
func (h *Handler) UpdateRolePermissions(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "team.handler.UpdateRolePermissions")
	defer span.End()

	id := router.GetParam(r, "id")
	var req teamuc.UpdateRolePermissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = ctxutil.GetWorkspaceID(ctx)
	}
	out, err := h.uc.UpdateRolePermissions(ctx, callerEmail(r), id, req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Permissions updated", out)
}

// DeleteRole godoc
// @Summary      Delete a role
// @Tags         Team
// @Security     BearerAuth
// @Param        id              path    string  true  "Role ID"
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse
// @Failure      403  {object}  response.StandardResponse
// @Failure      409  {object}  response.StandardResponse
// @Router       /api/team/roles/{id} [delete]
func (h *Handler) DeleteRole(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "team.handler.DeleteRole")
	defer span.End()

	id := router.GetParam(r, "id")
	if err := h.uc.DeleteRole(ctx, id); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Role deleted", map[string]string{"id": id})
}

// GetMyPermissions godoc
// @Summary      Get the caller's full permission matrix for the current workspace
// @Tags         Team
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse{data=team.MyPermissions}
// @Router       /api/team/permissions/me [get]
func (h *Handler) GetMyPermissions(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "team.handler.GetMyPermissions")
	defer span.End()

	workspaceID := ctxutil.GetWorkspaceID(ctx)
	out, err := h.uc.GetMyPermissions(ctx, callerEmail(r), workspaceID)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Permissions", out)
}
