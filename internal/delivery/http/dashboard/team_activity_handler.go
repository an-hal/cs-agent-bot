package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

type TeamActivityHandler struct {
	repo   repository.TeamActivityLogRepository
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewTeamActivityHandler(repo repository.TeamActivityLogRepository, logger zerolog.Logger, tr tracer.Tracer) *TeamActivityHandler {
	return &TeamActivityHandler{repo: repo, logger: logger, tracer: tr}
}

// List godoc
// @Summary  List team activity (invites, role changes, etc.)
// @Tags     Team
// @Param    X-Workspace-ID header string true "Workspace ID"
// @Param    limit query int false "Max items"
// @Router   /api/team/activity [get]
func (h *TeamActivityHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.TeamActivityList")
	defer span.End()
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.repo.List(ctx, ctxutil.GetWorkspaceID(ctx), limit)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Team activity", out)
}

type recordTeamActivityBody struct {
	Action      string         `json:"action"`
	TargetEmail string         `json:"target_email"`
	RoleID      string         `json:"role_id"`
	Detail      map[string]any `json:"detail"`
}

// Record godoc
// @Summary  Append a team activity entry (used by team handlers / tests)
// @Tags     Team
// @Accept   json
// @Param    X-Workspace-ID header string true "Workspace ID"
// @Param    body body recordTeamActivityBody true "Entry"
// @Router   /api/team/activity [post]
func (h *TeamActivityHandler) Record(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.TeamActivityRecord")
	defer span.End()
	var b recordTeamActivityBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	if b.Action == "" {
		return apperror.ValidationError("action required")
	}
	err := h.repo.Append(ctx, &repository.TeamActivityLog{
		WorkspaceID: ctxutil.GetWorkspaceID(ctx),
		ActorEmail:  callerEmail(r),
		Action:      b.Action,
		TargetEmail: b.TargetEmail,
		RoleID:      b.RoleID,
		Detail:      b.Detail,
	})
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusCreated, "Team activity recorded", nil)
}
