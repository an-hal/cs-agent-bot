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
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/dashboard"
	"github.com/rs/zerolog"
)

type EscalationHandler struct {
	uc     dashboard.DashboardUsecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewEscalationHandler(uc dashboard.DashboardUsecase, logger zerolog.Logger, tr tracer.Tracer) *EscalationHandler {
	return &EscalationHandler{uc: uc, logger: logger, tracer: tr}
}

// List godoc
// @Summary      List escalations
// @Description  Returns paginated escalations for the workspace specified in the X-Workspace-ID header.
// @Tags         Dashboard
// @Param        X-Workspace-ID  header    string  true   "Workspace ID"
// @Param        company_id      query     string  false  "Filter by company ID"
// @Param        status          query     string  false  "Filter by status (Open/Resolved)"
// @Param        priority        query     string  false  "Filter by priority"
// @Param        search          query     string  false  "Search across company_id, reason, esc_id, notes"
// @Param        offset          query     int     false  "Pagination offset (default 0)"
// @Param        limit           query     int     false  "Limit per page (default 10, max 100)"
// @Success      200  {object}  response.StandardResponseWithMeta{data=[]entity.Escalation}
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/data-master/escalations [get]
func (h *EscalationHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.EscalationList")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	params := pagination.FromRequest(r)
	q := r.URL.Query()

	wsID := ctxutil.GetWorkspaceID(ctx)
	filter := entity.EscalationFilter{
		WorkspaceIDs: []string{wsID},
		CompanyID:    q.Get("company_id"),
		Status:       q.Get("status"),
		Priority:     q.Get("priority"),
		Search:       q.Get("search"),
	}

	logger.Info().Str("workspace_id", wsID).Str("status", filter.Status).Msg("Incoming list escalations request")

	escalations, total, err := h.uc.GetEscalations(ctx, filter, params)
	if err != nil {
		return err
	}
	if escalations == nil {
		escalations = []entity.Escalation{}
	}

	logger.Info().Int("count", len(escalations)).Int64("total", total).Msg("Successfully fetched escalations")

	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Escalations", pagination.NewMeta(params, total), escalations)
}

// Get godoc
// @Summary      Get escalation by ID
// @Description  Returns a single escalation.
// @Tags         Dashboard
// @Param        id  path  string  true  "Escalation ID"
// @Success      200  {object}  response.StandardResponse{data=entity.Escalation}
// @Failure      404  {object}  response.StandardResponse
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/data-master/escalations/{id} [get]
func (h *EscalationHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.EscalationGet")
	defer span.End()

	id := router.GetParam(r, "id")
	if id == "" {
		return apperror.BadRequest("id is required")
	}

	esc, err := h.uc.GetEscalation(ctx, id)
	if err != nil {
		return err
	}

	return response.StandardSuccess(w, r, http.StatusOK, "Escalation", esc)
}

// resolveEscalationRequest is the request body for PUT /escalations/{id}/resolve.
type resolveEscalationRequest struct {
	ResolutionNote string `json:"resolution_note"`
}

// Resolve godoc
// @Summary      Resolve escalation
// @Description  Sets escalation status to Resolved with an optional resolution note.
// @Tags         Dashboard
// @Param        id    path  string                     true  "Escalation ID"
// @Param        body  body  resolveEscalationRequest   true  "Resolution payload"
// @Success      200  {object}  response.StandardResponse
// @Failure      400  {object}  response.StandardResponse
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/data-master/escalations/{id}/resolve [put]
func (h *EscalationHandler) Resolve(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.EscalationResolve")
	defer span.End()

	id := router.GetParam(r, "id")
	if id == "" {
		return apperror.BadRequest("id is required")
	}

	var req resolveEscalationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationError("Invalid request body")
	}

	resolvedBy := "unknown"
	if u, ok := middleware.GetJWTUser(r.Context()); ok {
		resolvedBy = u.Email
	}

	if err := h.uc.ResolveEscalation(ctx, id, resolvedBy, req.ResolutionNote); err != nil {
		return err
	}

	return response.StandardSuccess(w, r, http.StatusOK, "Escalation resolved", nil)
}

// ListByCompany godoc
// @Summary      List escalations for a company
// @Description  Returns paginated escalations for a specific company.
// @Tags         Dashboard
// @Param        X-Workspace-ID  header  string  true   "Workspace ID"
// @Param        company_id      path    string  true   "Company ID"
// @Param        offset          query   int     false  "Pagination offset"
// @Param        limit           query   int     false  "Limit per page"
// @Success      200  {object}  response.StandardResponseWithMeta{data=[]entity.Escalation}
// @Failure      400  {object}  response.StandardResponse
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/clients/{company_id}/escalations [get]
func (h *EscalationHandler) ListByCompany(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.EscalationListByCompany")
	defer span.End()

	companyID := router.GetParam(r, "company_id")
	if companyID == "" {
		return apperror.BadRequest("company_id is required")
	}

	wsID := ctxutil.GetWorkspaceID(ctx)
	params := pagination.FromRequest(r)

	escalations, total, err := h.uc.GetEscalationsByCompany(ctx, wsID, companyID, params)
	if err != nil {
		return err
	}
	if escalations == nil {
		escalations = []entity.Escalation{}
	}

	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Company escalations", pagination.NewMeta(params, total), escalations)
}
