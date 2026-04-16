package dashboard

import (
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/reports"
	"github.com/rs/zerolog"
)

// ReportsHandler handles reports endpoints.
type ReportsHandler struct {
	uc     reports.Usecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

// NewReportsHandler creates a new ReportsHandler.
func NewReportsHandler(uc reports.Usecase, logger zerolog.Logger, tr tracer.Tracer) *ReportsHandler {
	return &ReportsHandler{uc: uc, logger: logger, tracer: tr}
}

// ExecutiveSummary godoc
// @Summary      Executive summary report
// @Description  KPI summary + auto-generated highlights.
// @Tags         Reports
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse{data=entity.ExecutiveSummaryResponse}
// @Failure      500  {object}  response.StandardResponse
// @Router       /reports/executive-summary [get]
func (h *ReportsHandler) ExecutiveSummary(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "reports.handler.ExecutiveSummary")
	defer span.End()

	wsID := ctxutil.GetWorkspaceID(ctx)
	data, err := h.uc.ExecutiveSummary(ctx, wsID)
	if err != nil {
		return err
	}

	return response.StandardSuccess(w, r, http.StatusOK, "Executive summary retrieved", data)
}

// RevenueContracts godoc
// @Summary      Revenue & contracts report
// @Description  Total contract value, plan type revenue, top clients, contract expiry, payment breakdown.
// @Tags         Reports
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse{data=entity.RevenueReportResponse}
// @Failure      500  {object}  response.StandardResponse
// @Router       /reports/revenue-contracts [get]
func (h *ReportsHandler) RevenueContracts(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "reports.handler.RevenueContracts")
	defer span.End()

	wsID := ctxutil.GetWorkspaceID(ctx)
	data, err := h.uc.RevenueContracts(ctx, wsID)
	if err != nil {
		return err
	}

	return response.StandardSuccess(w, r, http.StatusOK, "Revenue contracts retrieved", data)
}

// ClientHealth godoc
// @Summary      Client health report
// @Description  Risk, payment, NPS, usage distributions + payment issue clients.
// @Tags         Reports
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse{data=entity.HealthReportResponse}
// @Failure      500  {object}  response.StandardResponse
// @Router       /reports/client-health [get]
func (h *ReportsHandler) ClientHealth(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "reports.handler.ClientHealth")
	defer span.End()

	wsID := ctxutil.GetWorkspaceID(ctx)
	data, err := h.uc.ClientHealth(ctx, wsID)
	if err != nil {
		return err
	}

	return response.StandardSuccess(w, r, http.StatusOK, "Client health retrieved", data)
}

// EngagementRetention godoc
// @Summary      Engagement & retention report
// @Description  Bot activity, NPS reply, checkin reply, cross-sell, renewal stats.
// @Tags         Reports
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse{data=entity.EngagementReportResponse}
// @Failure      500  {object}  response.StandardResponse
// @Router       /reports/engagement-retention [get]
func (h *ReportsHandler) EngagementRetention(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "reports.handler.EngagementRetention")
	defer span.End()

	wsID := ctxutil.GetWorkspaceID(ctx)
	data, err := h.uc.EngagementRetention(ctx, wsID)
	if err != nil {
		return err
	}

	return response.StandardSuccess(w, r, http.StatusOK, "Engagement retention retrieved", data)
}

// WorkspaceComparison godoc
// @Summary      Workspace comparison report (holding only)
// @Description  Per-workspace stats + combined totals. Only available for holding workspace.
// @Tags         Reports
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse{data=entity.WorkspaceComparisonResponse}
// @Failure      500  {object}  response.StandardResponse
// @Router       /reports/workspace-comparison [get]
func (h *ReportsHandler) WorkspaceComparison(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "reports.handler.WorkspaceComparison")
	defer span.End()

	wsID := ctxutil.GetWorkspaceID(ctx)
	data, err := h.uc.WorkspaceComparison(ctx, wsID)
	if err != nil {
		return err
	}

	return response.StandardSuccess(w, r, http.StatusOK, "Workspace comparison retrieved", data)
}

// Export godoc
// @Summary      Export report
// @Description  Export report as XLSX. Creates a background job.
// @Tags         Reports
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        tab             query   string  true   "Report tab: executive | revenue | health | engagement | comparison"
// @Param        format          query   string  false  "Export format: xlsx (default)"
// @Success      200  {object}  response.StandardResponse
// @Failure      400  {object}  response.StandardResponse
// @Router       /reports/export [post]
func (h *ReportsHandler) Export(w http.ResponseWriter, r *http.Request) error {
	_, span := h.tracer.Start(r.Context(), "reports.handler.Export")
	defer span.End()

	return response.StandardSuccess(w, r, http.StatusOK, "Report export will be available in a future release", map[string]string{"status": "not_implemented"})
}
