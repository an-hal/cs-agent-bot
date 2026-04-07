package dashboard

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/middleware"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/dashboard"
)

// ActivityHandler handles activity log endpoints for the dashboard.
type ActivityHandler struct {
	uc dashboard.DashboardUsecase
}

// NewActivityHandler creates a new ActivityHandler.
func NewActivityHandler(uc dashboard.DashboardUsecase) *ActivityHandler {
	return &ActivityHandler{uc: uc}
}

// recordActivityRequest is the request body for the POST endpoint.
type recordActivityRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Category    string `json:"category"`
	Action      string `json:"action"`
	Target      string `json:"target"`
	Detail      string `json:"detail"`
	RefID       string `json:"ref_id"`
	Status      string `json:"status"`
}

// List godoc
// @Summary      List activity logs
// @Description  Returns paginated activity log entries across all categories (bot, data, team).
//
//	Filter by workspace_id, category, offset, limit, and since.
//
// @Tags         Dashboard
// @Param        workspace_id  query     string  false  "Workspace ID"
// @Param        category      query     string  false  "Category filter: bot | data | team (default: all)"
// @Param        offset        query     int     false  "Pagination offset (default 0)"
// @Param        limit         query     int     false  "Max entries to return (default 10, max 100)"
// @Param        since         query     string  false  "ISO 8601 timestamp — return entries after this time"
// @Success      200  {object}  response.StandardResponseWithMeta{data=[]entity.ActivityLog}
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/activity-logs [get]
func (h *ActivityHandler) List(w http.ResponseWriter, r *http.Request) error {
	sp := r.URL.Query()
	params := pagination.FromRequest(r)

	workspaceID := sp.Get("workspace_id")
	category := sp.Get("category")

	var since *time.Time
	if v := sp.Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			since = &t
		}
	}

	filter := entity.ActivityFilter{
		WorkspaceID: workspaceID,
		Category:    category,
		Since:       since,
		Limit:       params.Limit,
		Offset:      params.Offset,
	}

	logs, total, err := h.uc.GetActivityLogs(r.Context(), filter)
	if err != nil {
		return err
	}

	if logs == nil {
		logs = []entity.ActivityLog{}
	}

	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Activity logs", pagination.NewMeta(params, int64(total)), logs)
}

// Record godoc
// @Summary      Record an activity
// @Description  Records a data or team activity pushed by the dashboard.
//
//	Bot activities are written automatically — posting category=bot is rejected.
//
// @Tags         Dashboard
// @Param        body  body      recordActivityRequest  true  "Activity payload"
// @Success      201  {object}  response.StandardResponse{data=entity.ActivityLog}
// @Failure      400  {object}  response.StandardResponse
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/activity-logs [post]
func (h *ActivityHandler) Record(w http.ResponseWriter, r *http.Request) error {
	var req recordActivityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.StandardError(w, r, http.StatusBadRequest, "Invalid JSON body", "INVALID_BODY", nil, "")
		return nil
	}

	if req.Category == entity.ActivityCategoryBot {
		response.StandardError(w, r, http.StatusBadRequest, "Category 'bot' cannot be written manually", "VALIDATION_ERROR", nil, "")
		return nil
	}

	if req.Category != entity.ActivityCategoryData && req.Category != entity.ActivityCategoryTeam {
		response.StandardError(w, r, http.StatusBadRequest, "Category must be 'data' or 'team'", "VALIDATION_ERROR", nil, "")
		return nil
	}

	if req.Action == "" {
		response.StandardError(w, r, http.StatusBadRequest, "Field 'action' is required", "VALIDATION_ERROR", nil, "")
		return nil
	}

	actor := "unknown"
	if u, ok := middleware.GetJWTUser(r.Context()); ok {
		actor = u.Email
	}

	entry := entity.ActivityLog{
		WorkspaceID: req.WorkspaceID,
		Category:    req.Category,
		ActorType:   entity.ActivityActorHuman,
		Actor:       actor,
		Action:      req.Action,
		Target:      req.Target,
		Detail:      req.Detail,
		RefID:       req.RefID,
		Status:      req.Status,
	}

	if err := h.uc.RecordActivity(r.Context(), entry); err != nil {
		return err
	}

	return response.StandardSuccess(w, r, http.StatusCreated, "Activity recorded", entry)
}
