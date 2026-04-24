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
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/coaching"
	"github.com/rs/zerolog"
)

type CoachingHandler struct {
	uc     coaching.Usecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewCoachingHandler(uc coaching.Usecase, logger zerolog.Logger, tr tracer.Tracer) *CoachingHandler {
	return &CoachingHandler{uc: uc, logger: logger, tracer: tr}
}

// List godoc
// @Summary  List coaching sessions (filter by bd/coach/status)
// @Tags     Coaching
// @Param    X-Workspace-ID header string true  "Workspace ID"
// @Param    bd             query  string false "bd_email"
// @Param    coach          query  string false "coach_email"
// @Param    status         query  string false "draft|submitted|reviewed"
// @Param    limit          query  int    false "Max items (default 50)"
// @Param    offset         query  int    false "Offset"
// @Router   /api/coaching/sessions [get]
func (h *CoachingHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.CoachingList")
	defer span.End()
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	out, total, err := h.uc.List(ctx, entity.CoachingSessionFilter{
		WorkspaceID: ctxutil.GetWorkspaceID(ctx),
		BDEmail:     r.URL.Query().Get("bd"),
		CoachEmail:  r.URL.Query().Get("coach"),
		Status:      r.URL.Query().Get("status"),
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		return err
	}
	if out == nil {
		out = []entity.CoachingSession{}
	}
	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Coaching sessions",
		pagination.Meta{Total: total, Offset: offset, Limit: limit}, out)
}

// Get godoc
// @Tags     Coaching
// @Param    X-Workspace-ID header string true "Workspace ID"
// @Param    id             path  string true "Session ID"
// @Router   /api/coaching/sessions/{id} [get]
func (h *CoachingHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.CoachingGet")
	defer span.End()
	out, err := h.uc.Get(ctx, ctxutil.GetWorkspaceID(ctx), router.GetParam(r, "id"))
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Coaching session", out)
}

// Create godoc
// @Tags     Coaching
// @Accept   json
// @Param    X-Workspace-ID header string true "Workspace ID"
// @Param    body           body   coaching.CreateRequest true "Session"
// @Router   /api/coaching/sessions [post]
func (h *CoachingHandler) Create(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.CoachingCreate")
	defer span.End()
	var req coaching.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	req.WorkspaceID = ctxutil.GetWorkspaceID(ctx)
	if req.CoachEmail == "" {
		req.CoachEmail = callerEmail(r)
	}
	out, err := h.uc.Create(ctx, req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusCreated, "Coaching session created", out)
}

// Update godoc
// @Tags     Coaching
// @Accept   json
// @Param    X-Workspace-ID header string true "Workspace ID"
// @Param    id             path  string true "Session ID"
// @Param    body           body   coaching.UpdateRequest true "Patch"
// @Router   /api/coaching/sessions/{id} [patch]
func (h *CoachingHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.CoachingUpdate")
	defer span.End()
	var req coaching.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	req.WorkspaceID = ctxutil.GetWorkspaceID(ctx)
	req.ID = router.GetParam(r, "id")
	out, err := h.uc.Update(ctx, req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Coaching session updated", out)
}

// Submit godoc
// @Tags     Coaching
// @Param    X-Workspace-ID header string true "Workspace ID"
// @Param    id             path  string true "Session ID"
// @Router   /api/coaching/sessions/{id}/submit [post]
func (h *CoachingHandler) Submit(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.CoachingSubmit")
	defer span.End()
	out, err := h.uc.Submit(ctx, ctxutil.GetWorkspaceID(ctx), router.GetParam(r, "id"), callerEmail(r))
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Coaching session submitted", out)
}

// Delete godoc
// @Tags     Coaching
// @Param    X-Workspace-ID header string true "Workspace ID"
// @Param    id             path  string true "Session ID"
// @Router   /api/coaching/sessions/{id} [delete]
func (h *CoachingHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.CoachingDelete")
	defer span.End()
	if err := h.uc.Delete(ctx, ctxutil.GetWorkspaceID(ctx), router.GetParam(r, "id")); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Coaching session deleted", nil)
}
