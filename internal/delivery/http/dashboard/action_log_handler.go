package dashboard

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// ActionLogHandler serves bot-action aggregation reads for the dashboard
// sidebar. Separate from ActivityHandler (which serves the user-facing
// activity_log table) — action_log holds bot-initiated actions only.
type ActionLogHandler struct {
	repo   repository.LogRepository
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewActionLogHandler(repo repository.LogRepository, logger zerolog.Logger, tr tracer.Tracer) *ActionLogHandler {
	return &ActionLogHandler{repo: repo, logger: logger, tracer: tr}
}

// Recent godoc
// @Summary  Last N bot actions for the current workspace
// @Tags     ActionLogs
// @Param    X-Workspace-ID header string true "Workspace ID"
// @Param    limit query int false "Max items (default 50, max 200)"
// @Router   /api/action-log/recent [get]
func (h *ActionLogHandler) Recent(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ActionLogRecent")
	defer span.End()
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	wsID := ctxutil.GetWorkspaceID(ctx)
	out, err := h.repo.GetRecentActionLogs(ctx, []string{wsID}, limit)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Recent bot actions", out)
}

// Today godoc
// @Summary  Bot actions from the current UTC day
// @Tags     ActionLogs
// @Param    X-Workspace-ID header string true "Workspace ID"
// @Param    limit query int false "Max items (default 200)"
// @Router   /api/action-log/today [get]
func (h *ActionLogHandler) Today(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ActionLogToday")
	defer span.End()
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	wsID := ctxutil.GetWorkspaceID(ctx)
	out, err := h.repo.GetTodayActionLogs(ctx, []string{wsID}, limit)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Today's bot actions", out)
}

// Summary godoc
// @Summary  Aggregated bot action counts (total, messages_sent, replies, escalations)
// @Tags     ActionLogs
// @Param    X-Workspace-ID header string true "Workspace ID"
// @Param    hours query int false "Lookback hours (default 24)"
// @Router   /api/action-log/summary [get]
func (h *ActionLogHandler) Summary(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ActionLogSummary")
	defer span.End()
	hours, _ := strconv.Atoi(r.URL.Query().Get("hours"))
	if hours <= 0 {
		hours = 24
	}
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)
	wsID := ctxutil.GetWorkspaceID(ctx)
	out, err := h.repo.GetActionLogSummary(ctx, []string{wsID}, since)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Action log summary", out)
}

// TodayActivity godoc
// @Summary  Today's unified activity (action + mutations + activity_log)
// @Description  Convenience wrapper over /activity-log/feed scoped to today's UTC.
// @Tags     ActivityLogs
// @Param    X-Workspace-ID header string true "Workspace ID"
// @Router   /api/activity-log/today [get]
func (h *ActionLogHandler) TodayActivity(w http.ResponseWriter, r *http.Request) error {
	// Simple alias — FE gets the same shape as /activity-log/feed but scoped.
	// Implemented via the action_log today endpoint for bot-scoped "today view";
	// a full cross-source today needs the feed repo. Here we return the bot slice
	// since that's what FE spec explicitly requested.
	return h.Today(w, r)
}
