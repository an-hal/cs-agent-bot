package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/reactivation"
	"github.com/rs/zerolog"
)

type ReactivationHandler struct {
	uc     reactivation.Usecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewReactivationHandler(uc reactivation.Usecase, logger zerolog.Logger, tr tracer.Tracer) *ReactivationHandler {
	return &ReactivationHandler{uc: uc, logger: logger, tracer: tr}
}

// ListTriggers godoc
// @Summary      List reactivation triggers for this workspace
// @Tags         Reactivation
// @Param        X-Workspace-ID  header  string  true   "Workspace ID"
// @Param        active          query   bool    false  "Filter to is_active=true only"
// @Router       /api/reactivation/triggers [get]
func (h *ReactivationHandler) ListTriggers(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ReactivationListTriggers")
	defer span.End()
	active, _ := strconv.ParseBool(r.URL.Query().Get("active"))
	out, err := h.uc.ListTriggers(ctx, ctxutil.GetWorkspaceID(ctx), active)
	if err != nil {
		return err
	}
	if out == nil {
		out = []entity.ReactivationTrigger{}
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Reactivation triggers", out)
}

// GetTrigger godoc
// @Tags  Reactivation
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param id path string true "Trigger ID"
// @Router /api/reactivation/triggers/{id} [get]
func (h *ReactivationHandler) GetTrigger(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ReactivationGetTrigger")
	defer span.End()
	out, err := h.uc.GetTrigger(ctx, ctxutil.GetWorkspaceID(ctx), router.GetParam(r, "id"))
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Reactivation trigger", out)
}

// UpsertTrigger godoc
// @Summary  Create or update a reactivation trigger (by code)
// @Tags     Reactivation
// @Accept   json
// @Param    X-Workspace-ID header string true "Workspace ID"
// @Param    body body reactivation.UpsertTriggerRequest true "Trigger"
// @Router   /api/reactivation/triggers [post]
func (h *ReactivationHandler) UpsertTrigger(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ReactivationUpsertTrigger")
	defer span.End()

	var req reactivation.UpsertTriggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	req.WorkspaceID = ctxutil.GetWorkspaceID(ctx)
	req.ActorEmail = callerEmail(r)
	out, err := h.uc.UpsertTrigger(ctx, req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Reactivation trigger saved", out)
}

// DeleteTrigger godoc
// @Tags   Reactivation
// @Param  X-Workspace-ID header string true "Workspace ID"
// @Param  id path string true "Trigger ID"
// @Router /api/reactivation/triggers/{id} [delete]
func (h *ReactivationHandler) DeleteTrigger(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ReactivationDeleteTrigger")
	defer span.End()
	if err := h.uc.DeleteTrigger(ctx, ctxutil.GetWorkspaceID(ctx), router.GetParam(r, "id")); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Reactivation trigger deleted", nil)
}

type reactivateClientBody struct {
	TriggerCode string `json:"trigger_code"`
	Note        string `json:"note"`
}

// Reactivate godoc
// @Summary  Manually fire a reactivation for a client
// @Tags     Reactivation
// @Accept   json
// @Param    X-Workspace-ID header string true "Workspace ID"
// @Param    id             path  string true "master_data.id"
// @Param    body           body  reactivateClientBody true "Options"
// @Router   /api/master-data/clients/{id}/reactivate [post]
func (h *ReactivationHandler) Reactivate(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.Reactivate")
	defer span.End()

	var body reactivateClientBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.Reactivate(ctx, reactivation.ReactivateRequest{
		WorkspaceID:  ctxutil.GetWorkspaceID(ctx),
		MasterDataID: router.GetParam(r, "id"),
		TriggerCode:  body.TriggerCode,
		Note:         body.Note,
		ActorEmail:   callerEmail(r),
	})
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Reactivation event recorded", out)
}

// History godoc
// @Summary  List reactivation events for a client
// @Tags     Reactivation
// @Param    X-Workspace-ID header string true "Workspace ID"
// @Param    id             path  string true "master_data.id"
// @Router   /api/master-data/clients/{id}/reactivation-history [get]
func (h *ReactivationHandler) History(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ReactivationHistory")
	defer span.End()
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.uc.History(ctx, ctxutil.GetWorkspaceID(ctx), router.GetParam(r, "id"), limit)
	if err != nil {
		return err
	}
	if out == nil {
		out = []entity.ReactivationEvent{}
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Reactivation history", out)
}
