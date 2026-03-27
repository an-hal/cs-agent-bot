package webhook

import (
	"encoding/json"
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/dto"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	usecaseWebhook "github.com/Sejutacita/cs-agent-bot/internal/usecase/webhook"
	"github.com/rs/zerolog"
)

type CheckinFormHTTPHandler struct {
	handler usecaseWebhook.CheckinFormHandler
	logger  zerolog.Logger
}

func NewCheckinFormHTTPHandler(handler usecaseWebhook.CheckinFormHandler, logger zerolog.Logger) *CheckinFormHTTPHandler {
	return &CheckinFormHTTPHandler{
		handler: handler,
		logger:  logger,
	}
}

// Handle godoc
// @Summary      Process Check-in Form Submission
// @Description  Processes check-in form submissions from clients. Marks the client as having replied to check-in and notifies the Account Executive via Telegram.
// @Tags         webhook
// @Param        request body dto.CheckinFormRequest true "Check-in form payload"
// @Success      200  {object}  response.StandardResponse  "Checkin form processed successfully"
// @Failure      400  {object}  response.StandardResponse  "Invalid request body"
// @Failure      404  {object}  response.StandardResponse  "Company not found"
// @Failure      500  {object}  response.StandardResponse  "Internal server error"
// @Router       /webhook/checkin-form [post]
func (h *CheckinFormHTTPHandler) Handle(w http.ResponseWriter, r *http.Request) error {
	var payload dto.CheckinFormRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return apperror.BadRequest("Invalid request body")
	}

	if payload.CompanyID == "" {
		return apperror.ValidationError("company_id is required")
	}

	if err := h.handler.HandleCheckinForm(r.Context(), payload.CompanyID); err != nil {
		h.logger.Error().Err(err).Str("company_id", payload.CompanyID).Msg("Checkin form handling failed")
		return err
	}

	return response.StandardSuccess(w, r, http.StatusOK, "Checkin form processed", nil)
}
