package smtpclient

import (
	"context"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/usecase/mockoutbox"
	"github.com/rs/zerolog"
)

// MockConfig tunes the mock sender.
type MockConfig struct {
	BaseLatencyMS int
	Outbox        *mockoutbox.Outbox
}

// NewMockClient returns a Client that records outbound mail to the outbox
// instead of contacting an SMTP server. Use when SMTP_HOST is absent or
// MOCK_EXTERNAL_APIS=true.
func NewMockClient(cfg MockConfig, logger zerolog.Logger) Client {
	if cfg.BaseLatencyMS <= 0 {
		cfg.BaseLatencyMS = 200
	}
	return &mockSMTPClient{cfg: cfg, logger: logger}
}

type mockSMTPClient struct {
	cfg    MockConfig
	logger zerolog.Logger
}

func (c *mockSMTPClient) Send(ctx context.Context, m Message) error {
	time.Sleep(time.Duration(c.cfg.BaseLatencyMS) * time.Millisecond)

	c.logger.Info().
		Strs("to", m.To).
		Strs("cc", m.Cc).
		Str("subject", m.Subject).
		Msg("SMTP mock: recorded email send")

	if c.cfg.Outbox != nil {
		bodyPreview := m.BodyText
		if bodyPreview == "" {
			bodyPreview = m.BodyHTML
		}
		// Trim long bodies so the outbox payload stays readable.
		if len(bodyPreview) > 2000 {
			bodyPreview = bodyPreview[:2000] + "… (truncated)"
		}
		c.cfg.Outbox.Record(
			mockoutbox.ProviderSMTP, "send_email",
			map[string]any{
				"to":           m.To,
				"cc":           m.Cc,
				"bcc":          m.Bcc,
				"from":         m.FromAddr,
				"subject":      m.Subject,
				"body_preview": bodyPreview,
				"is_html":      m.BodyHTML != "",
			},
			map[string]any{
				"message_id": "mock-smtp-" + time.Now().UTC().Format("20060102150405.000"),
				"status":     "queued",
			},
			"success", "",
		)
	}
	return nil
}
