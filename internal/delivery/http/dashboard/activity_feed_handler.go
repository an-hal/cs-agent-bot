package dashboard

import (
	"net/http"
	"strconv"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

type ActivityFeedHandler struct {
	repo   repository.ActivityFeedRepository
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewActivityFeedHandler(repo repository.ActivityFeedRepository, logger zerolog.Logger, tr tracer.Tracer) *ActivityFeedHandler {
	return &ActivityFeedHandler{repo: repo, logger: logger, tracer: tr}
}

// Feed godoc
// @Summary  Unified activity feed (action_log + mutations + activity_log)
// @Tags     ActivityLogs
// @Param    X-Workspace-ID header string true "Workspace ID"
// @Param    limit query int false "Max items (default 50, max 200)"
// @Router   /api/activity-log/feed [get]
func (h *ActivityFeedHandler) Feed(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ActivityFeed")
	defer span.End()
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.repo.Feed(ctx, ctxutil.GetWorkspaceID(ctx), limit)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Activity feed", out)
}
