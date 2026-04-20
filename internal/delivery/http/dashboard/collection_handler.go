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
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/collection"
	"github.com/rs/zerolog"
)

// CollectionHandler implements schema-level endpoints (collections + fields).
type CollectionHandler struct {
	uc     collection.Usecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

// NewCollectionHandler constructs a CollectionHandler.
func NewCollectionHandler(uc collection.Usecase, logger zerolog.Logger, tr tracer.Tracer) *CollectionHandler {
	return &CollectionHandler{uc: uc, logger: logger, tracer: tr}
}

func actorEmail(r *http.Request) string {
	if u, ok := middleware.GetJWTUser(r.Context()); ok {
		return u.Email
	}
	return ""
}

// List godoc
// @Summary      List collections
// @Tags         Collections
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse{data=[]entity.Collection}
// @Router       /api/collections [get]
func (h *CollectionHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "collection.handler.List")
	defer span.End()
	items, err := h.uc.ListCollections(ctx, ctxutil.GetWorkspaceID(ctx))
	if err != nil {
		return err
	}
	if items == nil {
		items = []entity.Collection{}
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Collections retrieved", items)
}

// Get godoc
// @Summary      Get collection by id
// @Tags         Collections
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        id   path  string  true  "Collection UUID"
// @Success      200  {object}  response.StandardResponse{data=entity.Collection}
// @Router       /api/collections/{id} [get]
func (h *CollectionHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "collection.handler.Get")
	defer span.End()
	id := router.GetParam(r, "id")
	c, err := h.uc.GetCollection(ctx, ctxutil.GetWorkspaceID(ctx), id)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Collection retrieved", c)
}

// Create godoc
// @Summary      Create collection (requires approval)
// @Tags         Collections
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        body  body  collection.CreateCollectionRequest  true  "Create payload"
// @Success      202  {object}  response.StandardResponse{data=entity.ApprovalRequest}
// @Router       /api/collections [post]
func (h *CollectionHandler) Create(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "collection.handler.Create")
	defer span.End()
	var req collection.CreateCollectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	ar, err := h.uc.RequestCreateCollection(ctx, ctxutil.GetWorkspaceID(ctx), actorEmail(r), req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusAccepted, "Collection create approval requested", ar)
}

// Update godoc
// @Summary      Update collection meta (no schema change)
// @Tags         Collections
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        id    path  string  true  "Collection UUID"
// @Param        body  body  collection.UpdateCollectionMetaRequest  true  "Meta"
// @Success      200  {object}  response.StandardResponse{data=entity.Collection}
// @Router       /api/collections/{id} [patch]
func (h *CollectionHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "collection.handler.Update")
	defer span.End()
	id := router.GetParam(r, "id")
	var req collection.UpdateCollectionMetaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.UpdateCollectionMeta(ctx, ctxutil.GetWorkspaceID(ctx), actorEmail(r), id, req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Collection updated", out)
}

// Delete godoc
// @Summary      Delete collection (requires approval)
// @Tags         Collections
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        id   path  string  true  "Collection UUID"
// @Success      202  {object}  response.StandardResponse{data=entity.ApprovalRequest}
// @Router       /api/collections/{id} [delete]
func (h *CollectionHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "collection.handler.Delete")
	defer span.End()
	id := router.GetParam(r, "id")
	ar, err := h.uc.RequestDeleteCollection(ctx, ctxutil.GetWorkspaceID(ctx), actorEmail(r), id)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusAccepted, "Collection delete approval requested", ar)
}

// AddField godoc
// @Summary      Add field to collection (requires approval)
// @Tags         Collections
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        id    path  string  true  "Collection UUID"
// @Param        body  body  collection.FieldInput  true  "Field"
// @Success      202  {object}  response.StandardResponse{data=entity.ApprovalRequest}
// @Router       /api/collections/{id}/fields [post]
func (h *CollectionHandler) AddField(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "collection.handler.AddField")
	defer span.End()
	id := router.GetParam(r, "id")
	var req collection.FieldInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	ar, err := h.uc.RequestAddField(ctx, ctxutil.GetWorkspaceID(ctx), actorEmail(r), id, req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusAccepted, "Add-field approval requested", ar)
}

// UpdateField godoc
// @Summary      Update field meta (label, required, order, options — no type change)
// @Tags         Collections
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        id        path  string  true  "Collection UUID"
// @Param        field_id  path  string  true  "Field UUID"
// @Param        body  body  collection.UpdateFieldMetaRequest  true  "Field meta"
// @Success      200  {object}  response.StandardResponse{data=entity.CollectionField}
// @Router       /api/collections/{id}/fields/{field_id} [patch]
func (h *CollectionHandler) UpdateField(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "collection.handler.UpdateField")
	defer span.End()
	id := router.GetParam(r, "id")
	fieldID := router.GetParam(r, "field_id")
	var req collection.UpdateFieldMetaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.UpdateFieldMeta(ctx, ctxutil.GetWorkspaceID(ctx), actorEmail(r), id, fieldID, req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Field updated", out)
}

// DeleteField godoc
// @Summary      Delete field (requires approval)
// @Tags         Collections
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        id        path  string  true  "Collection UUID"
// @Param        field_id  path  string  true  "Field UUID"
// @Success      202  {object}  response.StandardResponse{data=entity.ApprovalRequest}
// @Router       /api/collections/{id}/fields/{field_id} [delete]
func (h *CollectionHandler) DeleteField(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "collection.handler.DeleteField")
	defer span.End()
	id := router.GetParam(r, "id")
	fieldID := router.GetParam(r, "field_id")
	ar, err := h.uc.RequestDeleteField(ctx, ctxutil.GetWorkspaceID(ctx), actorEmail(r), id, fieldID)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusAccepted, "Delete-field approval requested", ar)
}

// ApplyApproval godoc
// @Summary      Apply a pending collection_schema_change approval
// @Description  Checker-maker: executes the pending schema change atomically
// @Tags         Collections
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        approval_id  path  string  true  "Approval request UUID"
// @Success      200  {object}  response.StandardResponse{data=entity.ApprovalRequest}
// @Router       /api/collections/approvals/{approval_id}/approve [post]
func (h *CollectionHandler) ApplyApproval(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "collection.handler.ApplyApproval")
	defer span.End()
	approvalID := router.GetParam(r, "approval_id")
	ar, err := h.uc.ApplyCollectionSchemaChange(ctx, ctxutil.GetWorkspaceID(ctx), approvalID, actorEmail(r))
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Approval applied", ar)
}
