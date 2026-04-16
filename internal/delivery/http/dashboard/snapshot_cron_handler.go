package dashboard

import (
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// SnapshotCronHandler handles the cron endpoint for rebuilding revenue snapshots.
type SnapshotCronHandler struct {
	workspaceRepo repository.WorkspaceRepository
	snapshotRepo  repository.RevenueSnapshotRepository
	logger        zerolog.Logger
	tracer        tracer.Tracer
}

// NewSnapshotCronHandler creates a new SnapshotCronHandler.
func NewSnapshotCronHandler(
	wr repository.WorkspaceRepository,
	sr repository.RevenueSnapshotRepository,
	logger zerolog.Logger,
	tr tracer.Tracer,
) *SnapshotCronHandler {
	return &SnapshotCronHandler{workspaceRepo: wr, snapshotRepo: sr, logger: logger, tracer: tr}
}

// Rebuild godoc
// @Summary      Rebuild revenue snapshots
// @Description  Recomputes revenue_snapshots from invoices for all workspaces. Run nightly via Cloud Scheduler.
// @Tags         Cron
// @Success      200  {object}  response.StandardResponse
// @Failure      500  {object}  response.StandardResponse
// @Router       /cron/analytics/rebuild-snapshots [get]
func (h *SnapshotCronHandler) Rebuild(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "snapshot_cron.handler.Rebuild")
	defer span.End()

	workspaces, err := h.workspaceRepo.GetAll(ctx)
	if err != nil {
		return err
	}

	rebuilt := 0
	for _, ws := range workspaces {
		if ws.IsHolding {
			continue
		}
		if err := h.snapshotRepo.RebuildFromInvoices(ctx, ws.ID, 18); err != nil {
			h.logger.Warn().Err(err).Str("workspace_id", ws.ID).Msg("Failed to rebuild snapshots")
			continue
		}
		rebuilt++
	}

	h.logger.Info().Int("rebuilt", rebuilt).Msg("Revenue snapshots rebuilt")
	return response.StandardSuccess(w, r, http.StatusOK, "Snapshots rebuilt", map[string]int{"workspaces_rebuilt": rebuilt})
}
