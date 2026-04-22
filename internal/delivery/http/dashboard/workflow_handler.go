package dashboard

import (
	"encoding/json"
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/middleware"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	workflowuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/workflow"
	"github.com/rs/zerolog"
)

// workflowActorFromCtx extracts the actor email from JWT context.
func workflowActorFromCtx(r *http.Request) string {
	if u, ok := middleware.GetJWTUser(r.Context()); ok {
		return u.Email
	}
	return "unknown"
}

// WorkflowHandler exposes the /workflows/* endpoints for the workflow engine.
type WorkflowHandler struct {
	uc     workflowuc.Usecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewWorkflowHandler(uc workflowuc.Usecase, logger zerolog.Logger, tr tracer.Tracer) *WorkflowHandler {
	return &WorkflowHandler{uc: uc, logger: logger, tracer: tr}
}

// ─── List ─────────────────────────────────────────────────────────────────────

// ListWorkflows godoc
// @Summary      List workflows
// @Description  List all workflows for the current workspace.
// @Tags         Workflows
// @Param        status  query  string  false  "Filter: active | draft | disabled"
// @Success      200  {object}  response.StandardResponseWithMeta{data=[]entity.WorkflowListItem}
// @Router       /api/workflows [get]
func (h *WorkflowHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "workflow.handler.List")
	defer span.End()

	wsID := ctxutil.GetWorkspaceID(ctx)
	var statusPtr *string
	if s := r.URL.Query().Get("status"); s != "" {
		statusPtr = &s
	}

	items, err := h.uc.List(ctx, wsID, statusPtr)
	if err != nil {
		return err
	}

	type meta struct {
		Total int `json:"total"`
	}
	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Workflows", meta{Total: len(items)}, items)
}

// ─── Get ──────────────────────────────────────────────────────────────────────

// GetWorkflow godoc
// @Summary      Get workflow by ID
// @Description  Returns full workflow with nodes, edges, steps, tabs, stats, columns.
// @Tags         Workflows
// @Param        id   path  string  true  "Workflow ID"
// @Success      200  {object}  response.StandardResponse{data=entity.WorkflowFull}
// @Failure      404  {object}  response.StandardResponse
// @Router       /api/workflows/{id} [get]
func (h *WorkflowHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "workflow.handler.Get")
	defer span.End()

	id := router.GetParam(r, "id")
	wsID := ctxutil.GetWorkspaceID(ctx)

	full, err := h.uc.GetByID(ctx, wsID, id)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Workflow", full)
}

// ─── GetBySlug ────────────────────────────────────────────────────────────────

// GetWorkflowBySlug godoc
// @Summary      Get workflow by slug
// @Description  Returns full workflow by slug (used by /pipeline/{slug} frontend route).
// @Tags         Workflows
// @Param        slug  path  string  true  "Workflow slug"
// @Success      200   {object}  response.StandardResponse{data=entity.WorkflowFull}
// @Failure      404   {object}  response.StandardResponse
// @Router       /api/workflows/by-slug/{slug} [get]
func (h *WorkflowHandler) GetBySlug(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "workflow.handler.GetBySlug")
	defer span.End()

	slug := router.GetParam(r, "slug")
	wsID := ctxutil.GetWorkspaceID(ctx)

	full, err := h.uc.GetBySlug(ctx, wsID, slug)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Workflow", full)
}

// ─── Create ───────────────────────────────────────────────────────────────────

// CreateWorkflow godoc
// @Summary      Create workflow
// @Tags         Workflows
// @Param        body  body  entity.Workflow  true  "Workflow data"
// @Success      201   {object}  response.StandardResponse{data=entity.Workflow}
// @Failure      400   {object}  response.StandardResponse
// @Router       /api/workflows [post]
func (h *WorkflowHandler) Create(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "workflow.handler.Create")
	defer span.End()

	var req entity.Workflow
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}
	req.WorkspaceID = ctxutil.GetWorkspaceID(ctx)
	actor := workflowActorFromCtx(r)

	created, err := h.uc.Create(ctx, actor, &req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusCreated, "Workflow created", created)
}

// ─── Update ───────────────────────────────────────────────────────────────────

// UpdateWorkflow godoc
// @Summary      Update workflow metadata
// @Tags         Workflows
// @Param        id    path  string          true  "Workflow ID"
// @Param        body  body  map[string]any  true  "Fields to update"
// @Success      200   {object}  response.StandardResponse
// @Failure      404   {object}  response.StandardResponse
// @Router       /api/workflows/{id} [put]
func (h *WorkflowHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "workflow.handler.Update")
	defer span.End()

	id := router.GetParam(r, "id")
	wsID := ctxutil.GetWorkspaceID(ctx)
	actor := workflowActorFromCtx(r)

	var fields map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
		return err
	}

	if err := h.uc.Update(ctx, wsID, id, actor, fields); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Updated", nil)
}

// ─── Delete ───────────────────────────────────────────────────────────────────

// DeleteWorkflow godoc
// @Summary      Delete workflow
// @Tags         Workflows
// @Param        id  path  string  true  "Workflow ID"
// @Success      200  {object}  response.StandardResponse
// @Failure      404  {object}  response.StandardResponse
// @Router       /api/workflows/{id} [delete]
func (h *WorkflowHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "workflow.handler.Delete")
	defer span.End()

	id := router.GetParam(r, "id")
	wsID := ctxutil.GetWorkspaceID(ctx)

	if err := h.uc.Delete(ctx, wsID, id); err != nil {
		return err
	}
	type deleted struct {
		Message string `json:"message"`
		ID      string `json:"id"`
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Deleted", deleted{Message: "Deleted", ID: id})
}

// ─── Canvas ───────────────────────────────────────────────────────────────────

type saveCanvasRequest struct {
	Nodes []entity.WorkflowNode `json:"nodes"`
	Edges []entity.WorkflowEdge `json:"edges"`
}

// SaveCanvas godoc
// @Summary      Save canvas (bulk replace nodes + edges)
// @Description  Replaces all nodes and edges for a workflow in a single transaction.
// @Tags         Workflows
// @Param        id    path  string              true  "Workflow ID"
// @Param        body  body  saveCanvasRequest   true  "Canvas data"
// @Success      200   {object}  response.StandardResponse{data=workflow.CanvasSaveResult}
// @Failure      404   {object}  response.StandardResponse
// @Router       /api/workflows/{id}/canvas [put]
func (h *WorkflowHandler) SaveCanvas(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "workflow.handler.SaveCanvas")
	defer span.End()

	id := router.GetParam(r, "id")
	wsID := ctxutil.GetWorkspaceID(ctx)

	var req saveCanvasRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}

	result, err := h.uc.SaveCanvas(ctx, wsID, id, req.Nodes, req.Edges)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Canvas saved", result)
}

// ─── Steps ────────────────────────────────────────────────────────────────────

// GetSteps godoc
// @Summary      List pipeline steps for a workflow
// @Tags         Workflows
// @Param        id  path  string  true  "Workflow ID"
// @Success      200  {object}  response.StandardResponse{data=[]entity.WorkflowStep}
// @Router       /api/workflows/{id}/steps [get]
func (h *WorkflowHandler) GetSteps(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "workflow.handler.GetSteps")
	defer span.End()

	id := router.GetParam(r, "id")
	wsID := ctxutil.GetWorkspaceID(ctx)

	full, err := h.uc.GetByID(ctx, wsID, id)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Steps", full.Steps)
}

type saveStepsRequest struct {
	Steps []entity.WorkflowStep `json:"steps"`
}

// SaveSteps godoc
// @Summary      Bulk save pipeline steps
// @Tags         Workflows
// @Param        id    path  string            true  "Workflow ID"
// @Param        body  body  saveStepsRequest  true  "Steps"
// @Success      200   {object}  response.StandardResponse
// @Router       /api/workflows/{id}/steps [put]
func (h *WorkflowHandler) SaveSteps(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "workflow.handler.SaveSteps")
	defer span.End()

	id := router.GetParam(r, "id")
	wsID := ctxutil.GetWorkspaceID(ctx)

	var req saveStepsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}

	if err := h.uc.SaveSteps(ctx, wsID, id, req.Steps); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Steps saved", nil)
}

// ─── Step by key ──────────────────────────────────────────────────────────────

// GetStep godoc
// @Summary      Get a single pipeline step by key
// @Tags         Workflows
// @Param        id       path  string  true  "Workflow ID"
// @Param        stepKey  path  string  true  "Step key"
// @Success      200  {object}  response.StandardResponse{data=entity.WorkflowStep}
// @Failure      404  {object}  response.StandardResponse
// @Router       /api/workflows/{id}/steps/{stepKey} [get]
func (h *WorkflowHandler) GetStep(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "workflow.handler.GetStep")
	defer span.End()

	id := router.GetParam(r, "id")
	stepKey := router.GetParam(r, "stepKey")
	wsID := ctxutil.GetWorkspaceID(ctx)

	step, err := h.uc.GetStepByKey(ctx, wsID, id, stepKey)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Step", step)
}

// UpdateStep godoc
// @Summary      Update a single pipeline step
// @Tags         Workflows
// @Param        id       path  string          true  "Workflow ID"
// @Param        stepKey  path  string          true  "Step key"
// @Param        body     body  map[string]any  true  "Fields to update"
// @Success      200  {object}  response.StandardResponse
// @Router       /api/workflows/{id}/steps/{stepKey} [put]
func (h *WorkflowHandler) UpdateStep(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "workflow.handler.UpdateStep")
	defer span.End()

	id := router.GetParam(r, "id")
	stepKey := router.GetParam(r, "stepKey")
	wsID := ctxutil.GetWorkspaceID(ctx)

	var fields map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
		return err
	}

	if err := h.uc.UpdateStep(ctx, wsID, id, stepKey, fields); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Step updated", nil)
}

// ─── Pipeline Config ──────────────────────────────────────────────────────────

// GetConfig godoc
// @Summary      Get pipeline config (tabs + stats + columns)
// @Tags         Workflows
// @Param        id  path  string  true  "Workflow ID"
// @Success      200  {object}  response.StandardResponse{data=entity.WorkflowFull}
// @Router       /api/workflows/{id}/config [get]
func (h *WorkflowHandler) GetConfig(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "workflow.handler.GetConfig")
	defer span.End()

	id := router.GetParam(r, "id")
	wsID := ctxutil.GetWorkspaceID(ctx)

	cfg, err := h.uc.GetConfig(ctx, wsID, id)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Config", cfg)
}

type saveTabsRequest struct{ Tabs []entity.PipelineTab `json:"tabs"` }
type saveStatsRequest struct{ Stats []entity.PipelineStat `json:"stats"` }
type saveColumnsRequest struct{ Columns []entity.PipelineColumn `json:"columns"` }

// SaveTabs godoc
// @Summary      Bulk save pipeline tabs
// @Tags         Workflows
// @Param        id    path  string           true  "Workflow ID"
// @Param        body  body  saveTabsRequest  true  "Tabs"
// @Success      200   {object}  response.StandardResponse
// @Router       /api/workflows/{id}/tabs [put]
func (h *WorkflowHandler) SaveTabs(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "workflow.handler.SaveTabs")
	defer span.End()

	id := router.GetParam(r, "id")
	wsID := ctxutil.GetWorkspaceID(ctx)

	var req saveTabsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}
	if err := h.uc.SaveTabs(ctx, wsID, id, req.Tabs); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Tabs saved", nil)
}

// SaveStats godoc
// @Summary      Bulk save stat cards
// @Tags         Workflows
// @Param        id    path  string            true  "Workflow ID"
// @Param        body  body  saveStatsRequest  true  "Stats"
// @Success      200   {object}  response.StandardResponse
// @Router       /api/workflows/{id}/stats [put]
func (h *WorkflowHandler) SaveStats(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "workflow.handler.SaveStats")
	defer span.End()

	id := router.GetParam(r, "id")
	wsID := ctxutil.GetWorkspaceID(ctx)

	var req saveStatsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}
	if err := h.uc.SaveStats(ctx, wsID, id, req.Stats); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Stats saved", nil)
}

// SaveColumns godoc
// @Summary      Bulk save column config
// @Tags         Workflows
// @Param        id    path  string              true  "Workflow ID"
// @Param        body  body  saveColumnsRequest  true  "Columns"
// @Success      200   {object}  response.StandardResponse
// @Router       /api/workflows/{id}/columns [put]
func (h *WorkflowHandler) SaveColumns(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "workflow.handler.SaveColumns")
	defer span.End()

	id := router.GetParam(r, "id")
	wsID := ctxutil.GetWorkspaceID(ctx)

	var req saveColumnsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}
	if err := h.uc.SaveColumns(ctx, wsID, id, req.Columns); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Columns saved", nil)
}
