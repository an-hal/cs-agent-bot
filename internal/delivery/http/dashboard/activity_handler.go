package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/middleware"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
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

// activityMeta is the pagination metadata returned with list responses.
type activityMeta struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
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
//	Filter by workspace_id, category, limit, offset, and since.
//
// @Tags         Dashboard
// @Param        workspace_id  query     string  false  "Workspace ID"
// @Param        category      query     string  false  "Category filter: bot | data | team (default: all)"
// @Param        limit         query     int     false  "Max entries to return (default 50, max 200)"
// @Param        offset        query     int     false  "Pagination offset (default 0)"
// @Param        since         query     string  false  "ISO 8601 timestamp — return entries after this time"
// @Success      200  {object}  response.StandardResponseWithMeta{data=[]entity.ActivityLog}
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/activity-logs [get]
func (h *ActivityHandler) List(w http.ResponseWriter, r *http.Request) error {
	sp := r.URL.Query()

	workspaceID := sp.Get("workspace_id")
	category := sp.Get("category")

	limit := 50
	if v := sp.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 200 {
		limit = 200
	}

	offset := 0
	if v := sp.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

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
		Limit:       limit,
		Offset:      offset,
	}

	logs, total, err := h.uc.GetActivityLogs(r.Context(), filter)
	if err != nil {
		return err
	}

	// Return empty slice instead of null
	if logs == nil {
		logs = []entity.ActivityLog{}
	}

	meta := activityMeta{Total: total, Limit: limit, Offset: offset}
	response.StandardSuccessWithMeta(w, r, http.StatusOK, "Activity logs", meta, logs)
	return nil
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

	// Bot activities are written automatically — reject manual bot writes
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

	// Actor is always taken from the authenticated JWT — never trusted from the body
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

	response.StandardSuccess(w, r, http.StatusCreated, "Activity recorded", entry)
	return nil
}
