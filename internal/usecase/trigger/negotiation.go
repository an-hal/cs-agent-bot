package trigger

import (
	"context"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

// EvalNegotiation evaluates P1 renewal negotiation triggers.
// REN60/45 halt on any reply OR checkin_replied=TRUE.
// REN30 onwards ignores reply status (Rule 4).
func (t *TriggerService) EvalNegotiation(ctx context.Context, c entity.Client, f entity.ClientFlags) (bool, error) {
	dte := c.DaysToExpiry()

	// REN60 at H-60
	if dte <= 60 && !f.Ren60Sent {
		// Skip if checkin_replied
		if f.CheckinReplied {
			f.Ren60Sent = true
			f.Ren45Sent = true // also skip REN45 per Rule 6
			if err := t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f); err != nil {
				t.Logger.Error().Err(err).Str("company_id", c.CompanyID).Msg("Failed to update flags")
			}
			return false, nil
		}
		if c.ResponseStatus == entity.ResponseStatusReplied {
			return false, nil // P1 stops on reply for REN60/45
		}
		if err := t.sendMessage(ctx, "REN60", "REN60_SENT", c, nil); err != nil {
			return false, err
		}
		f.Ren60Sent = true
		return true, t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f)
	}

	// REN45 at H-45
	if dte <= 45 && !f.Ren45Sent {
		if f.CheckinReplied {
			f.Ren45Sent = true
			if err := t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f); err != nil {
				t.Logger.Error().Err(err).Str("company_id", c.CompanyID).Msg("Failed to update flags")
			}
			return false, nil
		}
		if c.ResponseStatus == entity.ResponseStatusReplied {
			return false, nil
		}

		// Check PROMO_DEADLINE before REN45 (Rule 11)
		if t.Cfg.PromoDeadline != "" {
			deadline, err := time.Parse("2006-01-02", t.Cfg.PromoDeadline)
			if err == nil && time.Now().After(deadline) {
				// Promo expired, skip REN45 and alert AE Lead
				alertMsg := fmt.Sprintf("PROMO_DEADLINE expired (%s). REN45 skipped for %s (%s).",
					t.Cfg.PromoDeadline, c.CompanyName, c.CompanyID)
				if err := t.Telegram.SendMessage(ctx, t.Cfg.TelegramAELeadID, alertMsg); err != nil {
					t.Logger.Error().Err(err).Msg("Failed to send Telegram alert to AE Lead")
				}
				f.Ren45Sent = true
				if err := t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f); err != nil {
					t.Logger.Error().Err(err).Str("company_id", c.CompanyID).Msg("Failed to update flags")
				}
				return false, nil
			}
		}

		if err := t.sendMessage(ctx, "REN45", "REN45_SENT", c, nil); err != nil {
			return false, err
		}
		f.Ren45Sent = true
		return true, t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f)
	}

	// REN30 at H-30 — ignores reply status
	if dte <= 30 && !f.Ren30Sent {
		// TODO: post-CRM-refactor — quotation_link moved to
		// clients.custom_fields. Re-add the empty-link gate once
		// entity.Client exposes a CustomFields map.
		if err := t.sendMessage(ctx, "REN30", "REN30_SENT", c, nil); err != nil {
			return false, err
		}
		f.Ren30Sent = true
		return true, t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f)
	}

	// REN15 at H-15
	if dte <= 15 && !f.Ren15Sent {
		if err := t.sendMessage(ctx, "REN15", "REN15_SENT", c, nil); err != nil {
			return false, err
		}
		f.Ren15Sent = true
		return true, t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f)
	}

	// REN0 at H-0
	if dte <= 0 && !f.Ren0Sent {
		if err := t.sendMessage(ctx, "REN0", "REN0_SENT", c, nil); err != nil {
			return false, err
		}
		f.Ren0Sent = true
		if err := t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f); err != nil {
			t.Logger.Error().Err(err).Str("company_id", c.CompanyID).Msg("Failed to update flags")
		}

		// REN0 with no reply → ESC-004. Originally gated to Mid/High
		// segment but `segment` moved to clients.custom_fields. Until
		// entity.Client exposes a CustomFields map, escalate on every
		// REN0-no-reply event regardless of segment (more aggressive but
		// safe — no false negatives).
		if c.ResponseStatus != entity.ResponseStatusReplied {
			if err := t.Escalation.TriggerEscalation(ctx, entity.EscRen0NoReply, c,
				"REN0 sent, no reply from client", entity.EscPriorityP2High); err != nil {
				t.Logger.Error().Err(err).Str("company_id", c.CompanyID).Msg("Failed to trigger REN0 no-reply escalation")
			}
		}

		return true, nil
	}

	return false, nil
}
