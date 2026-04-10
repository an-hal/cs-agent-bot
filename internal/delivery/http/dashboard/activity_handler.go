package dashboard

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/middleware"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/dashboard"
	"github.com/rs/zerolog"
)

// ActivityHandler handles activity log endpoints for the dashboard.
type ActivityHandler struct {
	uc     dashboard.DashboardUsecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

// NewActivityHandler creates a new ActivityHandler.
func NewActivityHandler(uc dashboard.DashboardUsecase, logger zerolog.Logger, tr tracer.Tracer) *ActivityHandler {
	return &ActivityHandler{uc: uc, logger: logger, tracer: tr}
}

// recordActivityRequest is the request body for the POST endpoint.
type recordActivityRequest struct {
	Category     string `json:"category"`
	Action       string `json:"action"`
	Target       string `json:"target"`
	Detail       string `json:"detail"`
	RefID        string `json:"ref_id"`
	ResourceType string `json:"resource_type"`
	Status       string `json:"status"`
}

// List godoc
// @Summary      List activity logs
// @Description  Returns paginated activity log entries across all categories (bot, data, team).
// @Tags         Dashboard
// @Param        X-Workspace-ID  header    string  true   "Workspace ID"
// @Param        category       query     string  false  "Category filter: bot | data | team (default: all)"
// @Param        resource_type  query     string  false  "Resource type filter: client | invoice | template | trigger_rule | bot | team"
// @Param        ref_id         query     string  false  "Filter by record ID (company_id, template_id, rule_id, invoice_id)"
// @Param        offset         query     int     false  "Pagination offset (default 0)"
// @Param        limit          query     int     false  "Max entries to return (default 10, max 100)"
// @Param        since          query     string  false  "ISO 8601 timestamp — return entries after this time"
// @Success      200  {object}  response.StandardResponseWithMeta{data=[]entity.ActivityLog}
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/data-master/activity-logs [get]
func (h *ActivityHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ActivityList")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)

	sp := r.URL.Query()
	params := pagination.FromRequest(r)
	workspaceID := ctxutil.GetWorkspaceID(ctx)
	category := sp.Get("category")
	resourceType := sp.Get("resource_type")
	refID := sp.Get("ref_id")

	logger.Info().Str("workspace_id", workspaceID).Str("category", category).Str("resource_type", resourceType).Str("ref_id", refID).Int("offset", params.Offset).Int("limit", params.Limit).Msg("Incoming list activity logs request")

	var since *time.Time
	if v := sp.Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			since = &t
		}
	}

	filter := entity.ActivityFilter{
		WorkspaceIDs: []string{workspaceID},
		Category:     category,
		ResourceType: resourceType,
		RefID:        refID,
		Since:        since,
		Limit:        params.Limit,
		Offset:       params.Offset,
	}

	logs, total, err := h.uc.GetActivityLogs(ctx, filter)
	if err != nil {
		return err
	}

	if logs == nil {
		logs = []entity.ActivityLog{}
	}

	logger.Info().Int("count", len(logs)).Int("total", total).Msg("Successfully fetched activity logs")

	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Activity logs", pagination.NewMeta(params, int64(total)), logs)
}

// Record godoc
// @Summary      Record an activity
// @Description  Records a data or team activity pushed by the dashboard.
// @Tags         Dashboard
// @Param        X-Workspace-ID  header    string                 true   "Workspace ID"
// @Param        body             body      recordActivityRequest  true   "Activity payload"
// @Success      201  {object}  response.StandardResponse{data=entity.ActivityLog}
// @Failure      400  {object}  response.StandardResponse
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/data-master/activity-logs [post]
func (h *ActivityHandler) Record(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ActivityRecord")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	logger.Info().Msg("Incoming record activity request")

	var req recordActivityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationError("Invalid request body")
	}

	if req.Category == entity.ActivityCategoryBot {
		return apperror.BadRequest("Category 'bot' cannot be written manually")
	}

	if req.Category != entity.ActivityCategoryData && req.Category != entity.ActivityCategoryTeam {
		return apperror.BadRequest("Category must be 'data' or 'team'")
	}

	if req.Action == "" {
		return apperror.BadRequest("Field 'action' is required")
	}

	actor := "unknown"
	if u, ok := middleware.GetJWTUser(r.Context()); ok {
		actor = u.Email
	}

	entry := entity.ActivityLog{
		WorkspaceID:  ctxutil.GetWorkspaceID(ctx),
		Category:     req.Category,
		ActorType:    entity.ActivityActorHuman,
		Actor:        actor,
		Action:       req.Action,
		Target:       req.Target,
		Detail:       req.Detail,
		RefID:        req.RefID,
		ResourceType: req.ResourceType,
		Status:       req.Status,
	}

	if err := h.uc.RecordActivity(ctx, entry); err != nil {
		return err
	}

	logger.Info().Str("category", req.Category).Str("action", req.Action).Msg("Successfully recorded activity")

	return response.StandardSuccess(w, r, http.StatusCreated, "Activity recorded", entry)
}
