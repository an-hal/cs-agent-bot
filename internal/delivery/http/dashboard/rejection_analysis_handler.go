package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	rejectionanalysis "github.com/Sejutacita/cs-agent-bot/internal/usecase/rejection_analysis"
	"github.com/rs/zerolog"
)

type RejectionAnalysisHandler struct {
	uc     rejectionanalysis.Usecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewRejectionAnalysisHandler(uc rejectionanalysis.Usecase, logger zerolog.Logger, tr tracer.Tracer) *RejectionAnalysisHandler {
	return &RejectionAnalysisHandler{uc: uc, logger: logger, tracer: tr}
}

// List godoc
// @Tags  RejectionAnalysis
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param master_data_id query string false "Filter to a client"
// @Param category query string false "price|authority|timing|feature|tone|other"
// @Param limit query int false "Max items"
// @Param offset query int false "Offset"
// @Router /api/rejection-analysis [get]
func (h *RejectionAnalysisHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.RejectionAnalysisList")
	defer span.End()
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	out, total, err := h.uc.List(ctx, entity.RejectionAnalysisFilter{
		WorkspaceID:       ctxutil.GetWorkspaceID(ctx),
		MasterDataID:      r.URL.Query().Get("master_data_id"),
		RejectionCategory: r.URL.Query().Get("category"),
		Limit:             limit,
		Offset:            offset,
	})
	if err != nil {
		return err
	}
	if out == nil {
		out = []entity.RejectionAnalysis{}
	}
	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Rejection analysis",
		pagination.Meta{Total: total, Offset: offset, Limit: limit}, out)
}

// Analyze godoc
// @Summary  Run the rule classifier over a reply and store the result
// @Tags     RejectionAnalysis
// @Accept   json
// @Param    X-Workspace-ID header string true "Workspace ID"
// @Param    body body rejectionanalysis.AnalyzeRequest true "Reply"
// @Router   /api/rejection-analysis/analyze [post]
func (h *RejectionAnalysisHandler) Analyze(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.RejectionAnalysisAnalyze")
	defer span.End()
	var req rejectionanalysis.AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	req.WorkspaceID = ctxutil.GetWorkspaceID(ctx)
	out, err := h.uc.Analyze(ctx, req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusCreated, "Rejection analyzed", out)
}

// Record godoc
// @Summary  Store a pre-classified rejection analysis
// @Tags     RejectionAnalysis
// @Accept   json
// @Param    X-Workspace-ID header string true "Workspace ID"
// @Param    body body rejectionanalysis.RecordRequest true "Analysis"
// @Router   /api/rejection-analysis [post]
func (h *RejectionAnalysisHandler) Record(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.RejectionAnalysisRecord")
	defer span.End()
	var req rejectionanalysis.RecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	req.WorkspaceID = ctxutil.GetWorkspaceID(ctx)
	out, err := h.uc.Record(ctx, req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusCreated, "Rejection recorded", out)
}

// Stats godoc
// @Summary  Aggregate rejection categories over a time window
// @Tags     RejectionAnalysis
// @Param    X-Workspace-ID header string true "Workspace ID"
// @Param    days query int false "Lookback days (default 30)"
// @Router   /api/rejection-analysis/stats [get]
func (h *RejectionAnalysisHandler) Stats(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.RejectionAnalysisStats")
	defer span.End()
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	out, err := h.uc.CategoryStats(ctx, ctxutil.GetWorkspaceID(ctx), days)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Category stats", out)
}
