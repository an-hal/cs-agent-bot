package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/dto"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
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
		MessageID:   payload.Trigger.MessageID,
		PhoneNumber: payload.Customer.Phone,
		MessageType: payload.Message.Body,
		Text:        payload.Message.Body,
	}

	// Extract request-scoped values before responding — r.Context() is canceled once the handler returns.
	requestID := ctxutil.GetRequestID(r.Context())
	traceID := ctxutil.GetTraceID(r.Context())

	// Respond BEFORE any processing (must return 200 within 5 seconds)
	w.WriteHeader(http.StatusOK)

	// Process in background goroutine using a detached context so the DB/HTTP calls
	// are not canceled when the HTTP handler returns.
	go func() {
		defer func() {
			if rv := recover(); rv != nil {
				h.logger.Error().
					Interface("panic", rv).
					Str("message_id", payload.Trigger.MessageID).
					Msg("Panic in webhook processing goroutine")
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		ctx = ctxutil.SetRequestID(ctx, requestID)
		ctx = ctxutil.SetTraceID(ctx, traceID)

		if err := h.replyHandler.HandleIncomingReply(ctx, ucPayload); err != nil {
			h.logger.Error().Err(err).
				Str("message_id", payload.Trigger.MessageID).
				Msg("Failed to handle incoming reply")
		}
	}()
}
