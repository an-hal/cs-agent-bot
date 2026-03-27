package webhook

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/dto"
	usecaseWebhook "github.com/Sejutacita/cs-agent-bot/internal/usecase/webhook"
	"github.com/rs/zerolog"
)

type WAWebhookHandler struct {
	replyHandler usecaseWebhook.ReplyHandler
	logger       zerolog.Logger
}

func NewWAWebhookHandler(replyHandler usecaseWebhook.ReplyHandler, logger zerolog.Logger) *WAWebhookHandler {
	return &WAWebhookHandler{
		replyHandler: replyHandler,
		logger:       logger,
	}
}

// Handle godoc
// @Summary      Receive WhatsApp Webhook
// @Description  Receives incoming WhatsApp message events from HaloAI. Returns HTTP 200 immediately, then processes the message asynchronously in a background goroutine.
// @Tags         webhook
// @Param        request body dto.WAWebhookRequest true "WhatsApp webhook payload"
// @Success      200  {string}  string  "Webhook received (async processing)"
// @Failure      400  {string}  string  "Invalid request body"
// @Router       /webhook/wa [post]
func (h *WAWebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	var payload dto.WAWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.logger.Error().Err(err).Msg("Failed to decode WA webhook payload")
		w.WriteHeader(http.StatusOK) // still return 200 per HaloAI requirement
		return
	}

	// Convert DTO to usecase type
	ucPayload := usecaseWebhook.WAWebhookPayload{
		MessageID:   payload.MessageID,
		PhoneNumber: payload.PhoneNumber,
		MessageType: payload.MessageType,
		Text:        payload.Text,
	}

	// Respond BEFORE any processing (must return 200 within 5 seconds)
	w.WriteHeader(http.StatusOK)

	// Process in background goroutine
	go func() {
		ctx := context.Background()
		if err := h.replyHandler.HandleIncomingReply(ctx, ucPayload); err != nil {
			h.logger.Error().Err(err).
				Str("message_id", payload.MessageID).
				Msg("Failed to handle incoming reply")
		}
	}()
}
