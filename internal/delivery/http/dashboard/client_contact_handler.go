// Multi-stage PIC HTTP layer. See context/new/multi-stage-pic-spec.md.
//
// Lives on the existing MasterDataHandler so we don't need a new constructor
// — the handlers reuse the same usecase + tracer + logger.

package dashboard

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/master_data"
)

// ListContacts godoc
// @Summary      List contacts for a client (multi-stage)
// @Description  Returns every PIC for the client across all stages and kinds. Optional filter by stage / kind / only_primary.
// @Tags         MasterData
// @Security     BearerAuth
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Param        id            path  string true  "Client master_id (UUID)"
// @Param        stage         query string false "Filter: LEAD | PROSPECT | CLIENT | DORMANT"
// @Param        kind          query string false "Filter: internal | client_side"
// @Param        only_primary  query bool   false "If true, return only primary contacts"
// @Produce      json
// @Success      200 {object} response.StandardResponse{data=[]entity.ClientContact}
// @Router       /api/master-data/clients/{id}/contacts [get]
func (h *MasterDataHandler) ListContacts(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ListContacts")
	defer span.End()
	masterID := strings.TrimSpace(router.GetParam(r, "id"))
	if masterID == "" {
		return apperror.BadRequest("id required")
	}
	out, err := h.uc.ListContacts(ctx, ctxutil.GetWorkspaceID(ctx), masterID, repository.ClientContactFilter{
		Stage:       strings.TrimSpace(r.URL.Query().Get("stage")),
		Kind:        strings.TrimSpace(r.URL.Query().Get("kind")),
		OnlyPrimary: r.URL.Query().Get("only_primary") == "true",
	})
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Client contacts", out)
}

// CreateContact godoc
// @Summary      Add a PIC for a stage on a client
// @Tags         MasterData
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Param        id   path string true "Client master_id (UUID)"
// @Param        body body master_data.ContactCreateRequest true "Contact body"
// @Produce      json
// @Success      201 {object} response.StandardResponse{data=entity.ClientContact}
// @Router       /api/master-data/clients/{id}/contacts [post]
func (h *MasterDataHandler) CreateContact(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.CreateContact")
	defer span.End()
	masterID := strings.TrimSpace(router.GetParam(r, "id"))
	if masterID == "" {
		return apperror.BadRequest("id required")
	}
	var req master_data.ContactCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid body: " + err.Error())
	}
	out, err := h.uc.CreateContact(ctx, ctxutil.GetWorkspaceID(ctx), masterID, req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusCreated, "Contact created", out)
}

// PatchContact godoc
// @Summary      Edit a contact
// @Tags         MasterData
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Param        id          path string true "Client master_id (UUID)"
// @Param        contact_id  path string true "Contact id (UUID)"
// @Param        body        body master_data.ContactPatchRequest true "Patch body"
// @Produce      json
// @Success      200 {object} response.StandardResponse{data=entity.ClientContact}
// @Router       /api/master-data/clients/{id}/contacts/{contact_id} [patch]
func (h *MasterDataHandler) PatchContact(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.PatchContact")
	defer span.End()
	contactID := strings.TrimSpace(router.GetParam(r, "contact_id"))
	if contactID == "" {
		return apperror.BadRequest("contact_id required")
	}
	var req master_data.ContactPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid body: " + err.Error())
	}
	out, err := h.uc.PatchContact(ctx, ctxutil.GetWorkspaceID(ctx), contactID, req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Contact updated", out)
}

// DeleteContact godoc
// @Summary      Remove a contact (hard delete)
// @Tags         MasterData
// @Security     BearerAuth
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Param        id          path string true "Client master_id (UUID)"
// @Param        contact_id  path string true "Contact id (UUID)"
// @Produce      json
// @Success      200 {object} response.StandardResponse
// @Router       /api/master-data/clients/{id}/contacts/{contact_id} [delete]
func (h *MasterDataHandler) DeleteContact(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.DeleteContact")
	defer span.End()
	contactID := strings.TrimSpace(router.GetParam(r, "contact_id"))
	if contactID == "" {
		return apperror.BadRequest("contact_id required")
	}
	if err := h.uc.DeleteContact(ctx, ctxutil.GetWorkspaceID(ctx), contactID); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Contact deleted", nil)
}
