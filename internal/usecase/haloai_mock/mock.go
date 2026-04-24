// Package haloaimock is the mock WhatsApp sender used by the cron action
// dispatcher when MOCK_EXTERNAL_APIS=true or HaloAI is not configured. It
// records every "send" to the mock outbox so FE/QA can inspect the would-be
// WA messages without contacting HaloAI.
package haloaimock

import (
	"context"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/usecase/mockoutbox"
	"github.com/rs/zerolog"
)

// Sender is the narrow interface the cron dispatcher consumes. Intentionally
// minimal — decoupled from the real haloai.Client so the mock doesn't need to
// mirror that entire surface.
type Sender interface {
	Send(ctx context.Context, req SendRequest) (*SendResponse, error)
}

// SendRequest describes a WA message to dispatch. The `BusinessID` and
// `ChannelID` are normally workspace-scoped; for the mock we accept whatever.
type SendRequest struct {
	WorkspaceID string            `json:"workspace_id"`
	To          string            `json:"to"`
	TemplateID  string            `json:"template_id,omitempty"`
	Body        string            `json:"body"`
	Variables   map[string]string `json:"variables,omitempty"`
	BusinessID  string            `json:"business_id,omitempty"`
	ChannelID   string            `json:"channel_id,omitempty"`
}

// SendResponse mirrors the shape the cron layer cares about — a message id +
// provider ack. The mock fabricates both.
type SendResponse struct {
	MessageID string    `json:"message_id"`
	Status    string    `json:"status"`
	SentAt    time.Time `json:"sent_at"`
}

// NewSender returns a mock sender backed by the given outbox.
func NewSender(outbox *mockoutbox.Outbox, logger zerolog.Logger) Sender {
	return &mockSender{outbox: outbox, logger: logger}
}

type mockSender struct {
	outbox *mockoutbox.Outbox
	logger zerolog.Logger
}

func (s *mockSender) Send(ctx context.Context, req SendRequest) (*SendResponse, error) {
	// Simulate rate-limit + network latency (real HaloAI ≈ 200ms + 300ms WA API).
	time.Sleep(150 * time.Millisecond)

	resp := &SendResponse{
		MessageID: "mock-wamid-" + nowTag(),
		Status:    "delivered",
		SentAt:    time.Now().UTC(),
	}
	s.logger.Info().
		Str("to", req.To).
		Str("template_id", req.TemplateID).
		Str("workspace_id", req.WorkspaceID).
		Msg("HaloAI mock: recorded WA send")

	if s.outbox != nil {
		s.outbox.Record(
			mockoutbox.ProviderHaloAI, "send_wa",
			map[string]any{
				"workspace_id": req.WorkspaceID,
				"to":           req.To,
				"template_id":  req.TemplateID,
				"body":         req.Body,
				"variables":    req.Variables,
				"business_id":  req.BusinessID,
				"channel_id":   req.ChannelID,
			},
			map[string]any{
				"message_id": resp.MessageID,
				"status":     resp.Status,
				"sent_at":    resp.SentAt,
			},
			"success", "",
		)
	}
	return resp, nil
}

func nowTag() string {
	return time.Now().UTC().Format("20060102150405.000")
}
