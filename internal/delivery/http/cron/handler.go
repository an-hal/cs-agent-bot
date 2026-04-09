package cron

import (
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	usecaseCron "github.com/Sejutacita/cs-agent-bot/internal/usecase/cron"
	"github.com/rs/zerolog"
)

type CronHandler struct {
	cronRunner usecaseCron.CronRunner
	logger     zerolog.Logger
}

func NewCronHandler(cronRunner usecaseCron.CronRunner, logger zerolog.Logger) *CronHandler {
	return &CronHandler{
		cronRunner: cronRunner,
		logger:     logger,
	}
}

// Run godoc
// @Summary      Trigger Cron Job
// @Description  Triggers the bot's cron job. Creates one background job per workspace and returns immediately. Requires GCP Cloud Scheduler OIDC token authentication.
// @Tags         cron
// @Security     BearerAuth
// @Produce      json
// @Success      202  {object}  response.StandardResponse  "Cron jobs accepted - processing in background"
// @Failure      401  {object}  response.StandardResponse  "Unauthorized - invalid or missing OIDC token"
// @Failure      500  {object}  response.StandardResponse  "Internal server error - failed to start cron run"
// @Router       /cron/run [get]
func (h *CronHandler) Run(w http.ResponseWriter, r *http.Request) error {
	h.logger.Info().Msg("Cron run triggered")

	jobs, err := h.cronRunner.StartRunAll(r.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to start cron run")
		return response.StandardError(w, r, http.StatusInternalServerError, "Failed to start cron run", "INTERNAL_ERROR", nil, "")
	}

	h.logger.Info().Int("total_jobs", len(jobs)).Msg("Cron jobs dispatched")
	return response.StandardSuccess(w, r, http.StatusAccepted, "Cron run accepted", jobs)
}
