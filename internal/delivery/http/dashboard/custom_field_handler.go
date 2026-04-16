package dashboard

import (
	"encoding/json"
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/custom_field"
	"github.com/rs/zerolog"
)

// CustomFieldHandler implements /master-data/field-definitions endpoints.
type CustomFieldHandler struct {
	uc     custom_field.Usecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

// NewCustomFieldHandler constructs a CustomFieldHandler.
func NewCustomFieldHandler(uc custom_field.Usecase, logger zerolog.Logger, tr tracer.Tracer) *CustomFieldHandler {
	return &CustomFieldHandler{uc: uc, logger: logger, tracer: tr}
}

// List godoc
// @Summary      List custom field definitions
// @Tags         CustomField
// @Security     BearerAuth
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Success      200 {object} response.StandardResponse{data=[]entity.CustomFieldDefinition}
// @Router       /api/master-data/field-definitions [get]
func (h *CustomFieldHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.CustomFieldList")
	defer span.End()
	out, err := h.uc.List(ctx, ctxutil.GetWorkspaceID(ctx))
	if err != nil {
		return err
	}
	if out == nil {
		out = []entity.CustomFieldDefinition{}
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Custom field definitions", out)
}

// Create godoc
// @Summary      Create a custom field definition
// @Tags         CustomField
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Param        body body custom_field.CreateRequest true "Create payload"
// @Success      201 {object} response.StandardResponse{data=entity.CustomFieldDefinition}
// @Router       /api/master-data/field-definitions [post]
func (h *CustomFieldHandler) Create(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.CustomFieldCreate")
	defer span.End()
	var req custom_field.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.Create(ctx, ctxutil.GetWorkspaceID(ctx), req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusCreated, "Custom field created", out)
}

// Update godoc
// @Summary      Update a custom field definition (field_key immutable)
// @Tags         CustomField
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Param        id   path string true "Field definition UUID"
// @Param        body body custom_field.UpdateRequest true "Update payload"
// @Success      200 {object} response.StandardResponse{data=entity.CustomFieldDefinition}
// @Router       /api/master-data/field-definitions/{id} [put]
func (h *CustomFieldHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.CustomFieldUpdate")
	defer span.End()
	id := router.GetParam(r, "id")
	var req custom_field.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.Update(ctx, ctxutil.GetWorkspaceID(ctx), id, req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Custom field updated", out)
}

// Delete godoc
// @Summary      Delete a custom field definition
// @Tags         CustomField
// @Security     BearerAuth
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Param        id path string true "Field definition UUID"
// @Success      200 {object} response.StandardResponse
// @Router       /api/master-data/field-definitions/{id} [delete]
func (h *CustomFieldHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.CustomFieldDelete")
	defer span.End()
	id := router.GetParam(r, "id")
	if err := h.uc.Delete(ctx, ctxutil.GetWorkspaceID(ctx), id); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Custom field deleted", nil)
}

// ReorderRequest is the payload for /field-definitions/reorder.
type ReorderRequest struct {
	Order []repository.ReorderItem `json:"order"`
}

// Reorder godoc
// @Summary      Reorder custom field definitions in a single transaction
// @Tags         CustomField
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Param        body body ReorderRequest true "Reorder payload"
// @Success      200 {object} response.StandardResponse
// @Router       /api/master-data/field-definitions/reorder [put]
func (h *CustomFieldHandler) Reorder(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.CustomFieldReorder")
	defer span.End()
	var req ReorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	if err := h.uc.Reorder(ctx, ctxutil.GetWorkspaceID(ctx), req.Order); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Reordered", nil)
}
