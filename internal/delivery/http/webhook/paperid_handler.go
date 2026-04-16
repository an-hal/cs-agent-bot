package webhook

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/invoice"
	"github.com/rs/zerolog"
)

// PaperIDWebhookHandler receives payment events from Paper.id.
// HMAC signature verification happens inside the usecase layer.
type PaperIDWebhookHandler struct {
	svc    invoice.PaperIDService
	logger zerolog.Logger
}

func NewPaperIDWebhookHandler(svc invoice.PaperIDService, logger zerolog.Logger) *PaperIDWebhookHandler {
	return &PaperIDWebhookHandler{svc: svc, logger: logger}
}

// Handle godoc
// @Summary      Paper.id payment webhook
// @Description  Receives payment-status updates from Paper.id. HMAC-SHA256 verified via X-Paper-Signature header.
// @Tags         webhook
// @Param        workspace_id      path    string  true  "Workspace ID"
// @Param        X-Paper-Signature header  string  true  "Hex-encoded HMAC-SHA256 of raw body"
// @Param        payload           body    invoice.PaperIDWebhookPayload  true  "Paper.id event payload"
// @Success      200  {object}  response.StandardResponse
// @Failure      400  {object}  response.StandardResponse
// @Failure      401  {object}  response.StandardResponse  "Invalid signature"
// @Router       /webhook/paperid/{workspace_id} [post]
func (h *PaperIDWebhookHandler) Handle(w http.ResponseWriter, r *http.Request) error {
	wsID := router.GetParam(r, "workspace_id")
	if wsID == "" {
		return apperror.BadRequest("workspace_id required in path")
	}
	sig := r.Header.Get("X-Paper-Signature")
	if sig == "" {
		return apperror.Unauthorized("missing X-Paper-Signature header")
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return apperror.BadRequest("failed to read request body")
	}

	var payload invoice.PaperIDWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}

	// Preserve raw payload for signature verification and audit.
	var rawMap map[string]any
	_ = json.Unmarshal(body, &rawMap)
	payload.Raw = rawMap

	if err := h.svc.HandleWebhook(r.Context(), wsID, sig, payload); err != nil {
		h.logger.Warn().Err(err).Str("workspace_id", wsID).Msg("Paper.id webhook processing failed")
		return err
	}

	return response.StandardSuccess(w, r, http.StatusOK, "Paper.id webhook accepted", nil)
}
