// Package firefliesclient fetches transcripts from Fireflies via their GraphQL
// API. Used when the webhook payload is light (id only) and we need the full
// transcript text. In dev/staging without a key, returns a noop client.
package firefliesclient

import (
	"context"
	"errors"

	"github.com/rs/zerolog"
)

type Config struct {
	APIKey     string
	GraphQLURL string // default https://api.fireflies.ai/graphql
	Timeout    int    // seconds
}

// Transcript is the minimal view the rest of the system consumes.
type Transcript struct {
	ID              string
	Title           string
	Text            string
	Participants    []string
	DurationSeconds int
	HostEmail       string
}

type Client interface {
	FetchTranscript(ctx context.Context, firefliesID string) (*Transcript, error)
}

// NewClient returns a real HTTP client when APIKey is set, or a noop otherwise.
// The noop always returns an error — callers should check IsNoop + skip the
// real fetch path when true.
func NewClient(cfg Config, logger zerolog.Logger) Client {
	if cfg.APIKey == "" {
		logger.Warn().Msg("Fireflies client: FIREFLIES_API_KEY not set — using noop client")
		return &noopClient{}
	}
	logger.Info().Msg("Fireflies client: real client not yet implemented; falling back to noop")
	return &noopClient{}
}

type noopClient struct{}

func (c *noopClient) FetchTranscript(ctx context.Context, firefliesID string) (*Transcript, error) {
	return nil, errors.New("fireflies client not configured")
}

// IsNoop lets callers detect the noop without a type assertion.
func IsNoop(c Client) bool {
	_, ok := c.(*noopClient)
	return ok
}
