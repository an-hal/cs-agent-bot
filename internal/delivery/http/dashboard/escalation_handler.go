package dashboard

import (
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
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
