package trigger

import (
	"context"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

// EvalExpansion evaluates P4 NPS + Referral triggers.
// NPS surveys at D+15, D+45, D+60 post-activation.
// Referral only if NPSScore >= 8 and NPSReplied=TRUE.
func (t *TriggerService) EvalExpansion(ctx context.Context, c entity.Client, f entity.ClientFlags) (bool, error) {
	if c.ActivationDate.IsZero() {
		return false, nil
	}

	dsa := c.DaysSinceActivation()

	// NPS1 at D+15
	if dsa >= 15 && !f.NPS1Sent {
		if err := t.sendMessage(ctx, "NPS1", "NPS1_SENT", c, nil); err != nil {
			return false, err
		}
		f.NPS1Sent = true
		return true, t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f)
	}

	// NPS2 at D+45
	if dsa >= 45 && !f.NPS2Sent {
		if err := t.sendMessage(ctx, "NPS2", "NPS2_SENT", c, nil); err != nil {
			return false, err
		}
		f.NPS2Sent = true
		return true, t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f)
	}

	// NPS3 at D+60
	if dsa >= 60 && !f.NPS3Sent {
		if err := t.sendMessage(ctx, "NPS3", "NPS3_SENT", c, nil); err != nil {
			return false, err
		}
		f.NPS3Sent = true
		return true, t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f)
	}

	// Referral: only if NPSReplied=TRUE and not already sent this cycle.
	// TODO: post-CRM-refactor — re-add NPSScore >= 8 check by reading
	// from clients.custom_fields once entity.Client exposes the JSONB.
	if f.NPSReplied && !f.ReferralSentThisCycle {
		if err := t.sendMessage(ctx, "REFERRAL", "REFERRAL_SENT", c, nil); err != nil {
			return false, err
		}
		f.ReferralSentThisCycle = true
		return true, t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f)
	}

	return false, nil
}
