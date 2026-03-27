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
// @Description  Manually triggers the bot's cron job to process all clients and send scheduled WhatsApp messages. Requires GCP Cloud Scheduler OIDC token authentication.
// @Tags         cron
// @Security     BearerAuth
// @Success      200  {object}  response.StandardResponse  "Cron run completed successfully"
// @Failure      401  {object}  response.StandardResponse  "Unauthorized - invalid or missing OIDC token"
// @Failure      500  {object}  response.StandardResponse  "Internal server error - cron run failed"
// @Router       /cron/run [get]
func (h *CronHandler) Run(w http.ResponseWriter, r *http.Request) error {
	h.logger.Info().Msg("Cron run triggered")

	if err := h.cronRunner.RunAll(r.Context()); err != nil {
		h.logger.Error().Err(err).Msg("Cron run failed")
		return response.StandardError(w, r, http.StatusInternalServerError, "Cron run failed", "INTERNAL_ERROR", nil, "")
	}

	return response.StandardSuccess(w, r, http.StatusOK, "Cron run completed", nil)
}
