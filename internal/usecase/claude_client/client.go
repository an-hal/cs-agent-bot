// Package claudeclient is a thin wrapper around the Anthropic API used by the
// Claude extraction pipeline. The real HTTP call is gated behind a 2-stage
// prompt (field extraction + BANTS scoring).
//
// When ANTHROPIC_API_KEY is empty, NewClient returns a noop implementation
// that returns nil result — the extraction usecase treats that as "succeeded
// empty" so the pipeline degrades gracefully in dev/staging without the key.
package claudeclient

import (
	"context"

	claudeextraction "github.com/Sejutacita/cs-agent-bot/internal/usecase/claude_extraction"
	"github.com/rs/zerolog"
)

// Config mirrors config.ClaudeConfig shape. Empty APIKey → noop client.
// For the mock client with realistic BANTS output, use NewMockClient instead.
type Config struct {
	APIKey              string
	Model               string // e.g. claude-sonnet-4-6
	ExtractionPromptKey string
	BANTSPromptKey      string
	Timeout             int // seconds; 0 = default 30
}

// NewClient returns a Claude client that implements claudeextraction.Client.
// When the API key is missing, returns a noop that yields nil result + nil error.
func NewClient(cfg Config, logger zerolog.Logger) claudeextraction.Client {
	if cfg.APIKey == "" {
		logger.Warn().Msg("Claude client: ANTHROPIC_API_KEY not set — using noop client")
		return &noopClient{logger: logger}
	}
	// Real HTTP client stub — TODO: implement with anthropic SDK when the
	// key is provisioned. Keeping the interface stable means wiring later is
	// a single-line config change.
	logger.Info().Str("model", cfg.Model).Msg("Claude client: real client not yet implemented; falling back to noop")
	return &noopClient{logger: logger}
}

type noopClient struct {
	logger zerolog.Logger
}

func (c *noopClient) Extract(ctx context.Context, transcriptText string, hints map[string]any) (*claudeextraction.Result, error) {
	c.logger.Debug().Int("len", len(transcriptText)).Msg("Claude noop: returning empty result")
	return nil, nil
}
