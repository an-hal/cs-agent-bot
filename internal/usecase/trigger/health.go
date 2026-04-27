package trigger

import (
	"context"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

// EvalHealthRisk evaluates P0 health & risk triggers.
// TODO: post-CRM-refactor — UsageScore + NPSScore moved to clients.custom_fields.
// Until entity.Client exposes a CustomFields map, both gates below no-op.
// The trigger fires never; re-enable after wiring custom_fields read path.
func (t *TriggerService) EvalHealthRisk(ctx context.Context, c entity.Client, f entity.ClientFlags) (bool, error) {
	_ = c
	_ = f
	return false, nil
}
