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
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	manualaction "github.com/Sejutacita/cs-agent-bot/internal/usecase/manual_action"
	"github.com/rs/zerolog"
)

type ManualActionHandler struct {
	uc     manualaction.Usecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewManualActionHandler(uc manualaction.Usecase, logger zerolog.Logger, tr tracer.Tracer) *ManualActionHandler {
	return &ManualActionHandler{uc: uc, logger: logger, tracer: tr}
}

// List godoc
// @Summary      List manual action queue entries (filterable)
// @Tags         ManualActions
// @Security     BearerAuth
// @Param        X-Workspace-ID    header  string  true   "Workspace ID"
// @Param        status            query   string  false  "pending|in_progress|sent|skipped|expired"
// @Param        assigned_to_user  query   string  false  "Assignee email (default: caller)"
// @Param        mine              query   bool    false  "Shortcut: filter to caller's assignments"
// @Param        role              query   string  false  "sdr|bd|ae|admin"
// @Param        priority          query   string  false  "P0|P1|P2"
// @Param        flow_category     query   string  false  "flow category"
// @Param        limit             query   int     false  "Max items (default 50, max 200)"
// @Param        offset            query   int     false  "Offset"
// @Success      200  {object}  response.StandardResponseWithMeta{data=[]entity.ManualAction}
// @Router       /api/manual-actions [get]
func (h *ManualActionHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ManualActionList")
	defer span.End()

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	assignee := r.URL.Query().Get("assigned_to_user")
	if r.URL.Query().Get("mine") == "true" && assignee == "" {
		assignee = callerEmail(r)
	}

	filter := entity.ManualActionFilter{
		WorkspaceID:    ctxutil.GetWorkspaceID(ctx),
		Status:         r.URL.Query().Get("status"),
		AssignedToUser: assignee,
		Role:           r.URL.Query().Get("role"),
		Priority:       r.URL.Query().Get("priority"),
		FlowCategory:   r.URL.Query().Get("flow_category"),
		Limit:          limit,
		Offset:         offset,
	}
	out, total, err := h.uc.List(ctx, filter)
	if err != nil {
		return err
	}
	if out == nil {
		out = []entity.ManualAction{}
	}
	meta := pagination.Meta{Total: total, Offset: offset, Limit: limit}
	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Manual actions", meta, out)
}

// Get godoc
// @Summary      Get a manual action by ID
// @Tags         ManualActions
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        id              path    string  true  "Manual action ID"
// @Success      200  {object}  response.StandardResponse{data=entity.ManualAction}
// @Router       /api/manual-actions/{id} [get]
func (h *ManualActionHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ManualActionGet")
	defer span.End()

	id := router.GetParam(r, "id")
	out, err := h.uc.Get(ctx, ctxutil.GetWorkspaceID(ctx), id)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Manual action", out)
}

// MarkSent godoc
// @Summary      Confirm the human sent this manual action
// @Tags         ManualActions
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID  header  string                          true  "Workspace ID"
// @Param        id              path    string                          true  "Manual action ID"
// @Param        body            body    manualaction.MarkSentRequest    true  "Send confirmation"
// @Success      200  {object}  response.StandardResponse{data=entity.ManualAction}
// @Router       /api/manual-actions/{id}/mark-sent [patch]
func (h *ManualActionHandler) MarkSent(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ManualActionMarkSent")
	defer span.End()

	id := router.GetParam(r, "id")
	var req manualaction.MarkSentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.MarkSent(ctx, ctxutil.GetWorkspaceID(ctx), id, req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Manual action marked sent", out)
}

type skipManualActionBody struct {
	Reason string `json:"reason"`
}

// Skip godoc
// @Summary      Skip a manual action (min 5-char reason)
// @Tags         ManualActions
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID  header  string                 true  "Workspace ID"
// @Param        id              path    string                 true  "Manual action ID"
// @Param        body            body    skipManualActionBody   true  "Skip reason"
// @Success      200  {object}  response.StandardResponse{data=entity.ManualAction}
// @Router       /api/manual-actions/{id}/skip [patch]
func (h *ManualActionHandler) Skip(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ManualActionSkip")
	defer span.End()

	id := router.GetParam(r, "id")
	var body skipManualActionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.Skip(ctx, ctxutil.GetWorkspaceID(ctx), id, body.Reason)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Manual action skipped", out)
}
