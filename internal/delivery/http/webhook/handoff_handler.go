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

type HandoffHTTPHandler struct {
	handler usecaseWebhook.HandoffHandler
	logger  zerolog.Logger
}

func NewHandoffHTTPHandler(handler usecaseWebhook.HandoffHandler, logger zerolog.Logger) *HandoffHTTPHandler {
	return &HandoffHTTPHandler{
		handler: handler,
		logger:  logger,
	}
}

// Handle godoc
// @Summary      Onboard New Client from BD Handoff
// @Description  Creates a new client record when a Business Development team member hands off a new prospect. Requires HMAC authentication using X-Handoff-Secret header.
// @Tags         api
// @Security     X-Handoff-Secret
// @Param        request body dto.HandoffCreateRequest true "New client handoff payload"
// @Success      201  {object}  response.StandardResponse  "Client onboarded successfully"
// @Failure      400  {object}  response.StandardResponse  "Invalid request body or validation error"
// @Failure      401  {object}  response.StandardResponse  "Unauthorized - invalid HMAC signature"
// @Failure      500  {object}  response.StandardResponse  "Internal server error"
// @Router       /api/handoff/new-client [post]
func (h *HandoffHTTPHandler) Handle(w http.ResponseWriter, r *http.Request) error {
	var payload dto.HandoffCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return apperror.BadRequest("Invalid request body")
	}

	if payload.CompanyID == "" || payload.CompanyName == "" || payload.PICWA == "" {
		return apperror.ValidationError("company_id, company_name, and pic_wa are required")
	}

	// Convert DTO to usecase type
	ucPayload := usecaseWebhook.NewClientPayload{
		CompanyID:       payload.CompanyID,
		CompanyName:     payload.CompanyName,
		PICName:         payload.PICName,
		PICWA:           payload.PICWA,
		OwnerName:       payload.OwnerName,
		OwnerWA:         payload.OwnerWA,
		Segment:         payload.Segment,
		ContractMonths:  payload.ContractMonths,
		ContractStart:   payload.ContractStart,
		ContractEnd:     payload.ContractEnd,
		ActivationDate:  payload.ActivationDate,
		OwnerTelegramID: payload.OwnerTelegramID,
	}

	if err := h.handler.HandleNewClient(r.Context(), ucPayload); err != nil {
		h.logger.Error().Err(err).Str("company_id", payload.CompanyID).Msg("Handoff failed")
		return err
	}

	return response.StandardSuccess(w, r, http.StatusCreated, "Client onboarded", nil)
}
