package webhook

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/fireflies"
	"github.com/rs/zerolog"
)

// FirefliesWebhookHandler accepts the Fireflies transcript webhook. Signature
// verification is handled by middleware (HMAC). The handler is intentionally
// non-blocking — ingest + goroutine dispatch for extraction live in the
// Usecase layer.
type FirefliesWebhookHandler struct {
	uc     fireflies.Usecase
	logger zerolog.Logger
}

func NewFirefliesWebhookHandler(uc fireflies.Usecase, logger zerolog.Logger) *FirefliesWebhookHandler {
	return &FirefliesWebhookHandler{uc: uc, logger: logger}
}

// Inbound payload. The real Fireflies webhook body is richer; we accept both
// a flat normalized form and a `raw` passthrough so FE/Zapier pre-processors
// can transform upstream.
type firefliesWebhookBody struct {
	FirefliesID     string         `json:"fireflies_id"`
	MeetingTitle    string         `json:"meeting_title"`
	MeetingDate     string         `json:"meeting_date"`
	DurationSeconds int            `json:"duration_seconds"`
	HostEmail       string         `json:"host_email"`
	Participants    []string       `json:"participants"`
	TranscriptText  string         `json:"transcript_text"`
	Raw             map[string]any `json:"raw,omitempty"`
}

// Handle godoc
// @Summary      Receive a Fireflies transcript webhook
// @Description  Idempotent — repeat deliveries of the same fireflies_id return the existing record.
// @Tags         Webhooks
// @Accept       json
// @Param        workspace_id  path  string                  true  "Target workspace ID"
// @Param        body          body  firefliesWebhookBody    true  "Transcript payload"
// @Success      202  {object}  response.StandardResponse
// @Router       /api/webhook/fireflies/{workspace_id} [post]
func (h *FirefliesWebhookHandler) Handle(w http.ResponseWriter, r *http.Request) error {
	workspaceID := router.GetParam(r, "workspace_id")
	if strings.TrimSpace(workspaceID) == "" {
		return apperror.BadRequest("workspace_id required in path")
	}
	var body firefliesWebhookBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.IngestWebhook(r.Context(), fireflies.IngestWebhookRequest{
		WorkspaceID:     workspaceID,
		FirefliesID:     body.FirefliesID,
		MeetingTitle:    body.MeetingTitle,
		MeetingDateISO:  body.MeetingDate,
		DurationSeconds: body.DurationSeconds,
		HostEmail:       body.HostEmail,
		Participants:    body.Participants,
		TranscriptText:  body.TranscriptText,
		RawPayload:      body.Raw,
	})
	if err != nil {
		return err
	}
	// 202 — extraction runs async if wired; FE polls /fireflies/transcripts/{id} for status.
	return response.StandardSuccess(w, r, http.StatusAccepted, "Transcript ingested", map[string]any{
		"transcript_id":     out.ID,
		"extraction_status": out.ExtractionStatus,
	})
}
