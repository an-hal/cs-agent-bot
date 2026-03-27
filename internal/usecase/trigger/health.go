package trigger

import (
	"context"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

// EvalHealthRisk evaluates P0 health & risk triggers.
// Fires when UsageScore < 40 or NPSScore <= 5.
func (t *TriggerService) EvalHealthRisk(ctx context.Context, c entity.Client, f entity.ClientFlags) (bool, error) {
	// Low usage check
	if c.UsageScore < 40 && !f.LowUsageMsgSent {
		if err := t.sendMessage(ctx, "LOW_USAGE", "LOW_USAGE_SENT", c, nil); err != nil {
			return false, err
		}
		f.LowUsageMsgSent = true
		if err := t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f); err != nil {
			return true, err
		}
		return true, nil
	}

	// Low NPS check
	if c.NPSScore > 0 && c.NPSScore <= 5 && !f.LowNPSMsgSent {
		if err := t.sendMessage(ctx, "LOW_NPS", "LOW_NPS_SENT", c, nil); err != nil {
			return false, err
		}
		f.LowNPSMsgSent = true
		if err := t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f); err != nil {
			return true, err
		}

		// NPSScore <= 5 also triggers ESC-003
		if err := t.Escalation.TriggerEscalation(ctx, entity.EscLowNPS, c, "NPS score <= 5", entity.EscPriorityP1Critical); err != nil {
			t.Logger.Error().Err(err).Msg("Failed to trigger ESC-003")
		}

		return true, nil
	}

	return false, nil
}
