package template

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

// ResolvePriority picks the first template ID in `candidates` that exists and
// resolves cleanly against the given client + invoice + config. Returns the
// rendered body AND the selected template ID (so callers can audit which
// branch was chosen).
//
// Usage matches spec 05-messaging + 06-workflow-engine: pass candidates in
// priority order — typically renewal_branch → intent_branch → legacy_branch → default.
// If every candidate fails (missing template / unresolved variable), the last
// error is returned.
func ResolvePriority(
	ctx context.Context,
	r TemplateResolver,
	candidates []string,
	client entity.Client,
	invoice *entity.Invoice,
	cfg TemplateConfig,
) (body, templateID string, err error) {
	if r == nil {
		return "", "", errors.New("template resolver is nil")
	}
	if len(candidates) == 0 {
		return "", "", errors.New("no template candidates provided")
	}
	var lastErr error
	for _, id := range candidates {
		if id == "" {
			continue
		}
		b, e := r.ResolveTemplate(ctx, id, client, invoice, cfg)
		if e == nil {
			return b, id, nil
		}
		lastErr = fmt.Errorf("candidate %q: %w", id, e)
	}
	if lastErr == nil {
		lastErr = errors.New("all template candidates empty")
	}
	return "", "", lastErr
}
