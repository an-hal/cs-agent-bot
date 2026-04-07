package dashboard

import (
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/dashboard"
	"github.com/rs/zerolog"
)

type WorkspaceHandler struct {
	uc     dashboard.DashboardUsecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewWorkspaceHandler(uc dashboard.DashboardUsecase, logger zerolog.Logger, tr tracer.Tracer) *WorkspaceHandler {
	return &WorkspaceHandler{uc: uc, logger: logger, tracer: tr}
}

// List godoc
// @Summary      List workspaces
// @Description  Returns all workspaces. Holding workspace aggregates data from member workspaces.
// @Tags         Dashboard
// @Success      200  {object}  response.StandardResponseWithMeta{data=[]entity.Workspace}
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/workspaces [get]
func (h *WorkspaceHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.WorkspaceList")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	logger.Info().Msg("Incoming list workspaces request")

	workspaces, err := h.uc.GetWorkspaces(ctx)
	if err != nil {
		return err
	}
	if workspaces == nil {
		workspaces = []entity.Workspace{}
	}

	logger.Info().Int("count", len(workspaces)).Msg("Successfully fetched workspaces")

	meta := pagination.Meta{Total: int64(len(workspaces)), Offset: 0, Limit: len(workspaces)}
	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Workspaces", meta, workspaces)
}
