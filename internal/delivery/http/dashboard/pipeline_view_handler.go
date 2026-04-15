package dashboard

import (
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	pipelineview "github.com/Sejutacita/cs-agent-bot/internal/usecase/pipeline_view"
	workflowuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/workflow"
	"github.com/rs/zerolog"
)

// PipelineViewHandler handles GET /workflows/{id}/data — returns master_data
// records filtered by workflow stage_filter, tab DSL, search, and pagination.
type PipelineViewHandler struct {
	workflowUC workflowuc.Usecase
	pipelineUC pipelineview.Usecase
	logger     zerolog.Logger
	tracer     tracer.Tracer
}

func NewPipelineViewHandler(
	workflowUC workflowuc.Usecase,
	pipelineUC pipelineview.Usecase,
	logger zerolog.Logger,
	tr tracer.Tracer,
) *PipelineViewHandler {
	return &PipelineViewHandler{
		workflowUC: workflowUC,
		pipelineUC: pipelineUC,
		logger:     logger,
		tracer:     tr,
	}
}

// GetData godoc
// @Summary      Get pipeline data
// @Description  Returns master_data records filtered by workflow stage_filter and tab DSL.
// @Tags         Workflows
// @Param        id       path   string  true   "Workflow ID"
// @Param        tab      query  string  false  "Tab key (from workflow tabs config)"
// @Param        search   query  string  false  "Search company_name, pic_name, company_id"
// @Param        offset   query  int     false  "Pagination offset"
// @Param        limit    query  int     false  "Page size (default 20, max 200)"
// @Param        sort_by  query  string  false  "Sort column (updated_at, contract_end, final_price, ...)"
// @Param        sort_dir query  string  false  "asc | desc (default desc)"
// @Success      200  {object}  response.StandardResponseWithMeta
// @Failure      404  {object}  response.StandardResponse
// @Router       /api/workflows/{id}/data [get]
func (h *PipelineViewHandler) GetData(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "pipelineView.handler.GetData")
	defer span.End()

	id := router.GetParam(r, "id")
	wsID := ctxutil.GetWorkspaceID(ctx)
	q := r.URL.Query()

	// Load the workflow to get stage_filter, tabs, stats.
	wf, err := h.workflowUC.GetByID(ctx, wsID, id)
	if err != nil {
		return err
	}

	params := pagination.FromRequest(r)
	resp, err := h.pipelineUC.GetData(
		ctx,
		wsID,
		wf,
		q.Get("tab"),
		q.Get("search"),
		params,
		q.Get("sort_by"),
		q.Get("sort_dir"),
	)
	if err != nil {
		return err
	}

	type meta struct {
		Offset int   `json:"offset"`
		Limit  int   `json:"limit"`
		Total  int64 `json:"total"`
	}
	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Pipeline data",
		meta{Offset: params.Offset, Limit: params.Limit, Total: resp.Total},
		resp,
	)
}
