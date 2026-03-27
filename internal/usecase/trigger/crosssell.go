package trigger

import (
	"context"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

// EvalCrossSell evaluates P5 cross-sell ATS triggers.
// 90-day sequence starting D+30 post-activation, then rotating long-term cycles.
func (t *TriggerService) EvalCrossSell(ctx context.Context, c entity.Client, f entity.ClientFlags) (bool, error) {
	if c.ActivationDate.IsZero() {
		return false, nil
	}

	// Stop conditions
	if c.CrossSellRejected || c.CrossSellInterested {
		return false, nil
	}

	// NPS gate: if NPSReplied=TRUE AND NPSScore < 8, block all P5
	if f.NPSReplied && c.NPSScore < 8 {
		return false, nil
	}

	// No-overlap rule: skip if feature_update_sent this week
	if f.FeatureUpdateSent {
		return false, nil
	}

	dsa := c.DaysSinceActivation()

	// 90-day active sequence
	if c.SequenceCS == entity.SequenceCSActive || c.SequenceCS == "" {
		return t.eval90DaySequence(ctx, c, f, dsa)
	}

	// Long-term rotation
	if c.SequenceCS == entity.SequenceCSLongterm {
		return t.evalLongTermSequence(ctx, c, f)
	}

	return false, nil
}

func (t *TriggerService) eval90DaySequence(ctx context.Context, c entity.Client, f entity.ClientFlags, dsa int) (bool, error) {
	type step struct {
		dayStart   int
		flag       *bool
		templateID string
		triggerType string
	}

	steps := []step{
		{30, &f.CSH7, "CS_H7", "CS_H7_SENT"},
		{37, &f.CSH14, "CS_H14", "CS_H14_SENT"},
		{44, &f.CSH21, "CS_H21", "CS_H21_SENT"},
		{52, &f.CSH30, "CS_H30", "CS_H30_SENT"},
		{60, &f.CSH45, "CS_H45", "CS_H45_SENT"},
		{67, &f.CSH60, "CS_H60", "CS_H60_SENT"},
		{75, &f.CSH75, "CS_H75", "CS_H75_SENT"},
		{90, &f.CSH90, "CS_H90", "CS_H90_SENT"},
	}

	for _, s := range steps {
		if dsa >= s.dayStart && !*s.flag {
			// Check PROMO_DEADLINE before CS_H60
			if s.templateID == "CS_H60" && t.Cfg.PromoDeadline != "" {
				deadline, err := time.Parse("2006-01-02", t.Cfg.PromoDeadline)
				if err == nil && time.Now().After(deadline) {
					*s.flag = true
					_ = t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f)
					continue
				}
			}

			if err := t.sendMessage(ctx, s.templateID, s.triggerType, c, nil); err != nil {
				return false, err
			}
			*s.flag = true

			// After CS_H90: transition to LONGTERM if no interest
			if s.templateID == "CS_H90" {
				// Update sequence_cs to LONGTERM
				// This is stored on Sheet 1 (Master Client), column for sequence_cs
				t.Logger.Info().
					Str("company_id", c.CompanyID).
					Msg("Cross-sell 90-day complete, transitioning to LONGTERM")
			}

			return true, t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f)
		}
	}

	return false, nil
}

func (t *TriggerService) evalLongTermSequence(ctx context.Context, c entity.Client, f entity.ClientFlags) (bool, error) {
	type step struct {
		flag        *bool
		templateID  string
		triggerType string
	}

	steps := []step{
		{&f.CSLT1, "CS_LT1", "CS_LT1_SENT"},
		{&f.CSLT2, "CS_LT2", "CS_LT2_SENT"},
		{&f.CSLT3, "CS_LT3", "CS_LT3_SENT"},
	}

	for _, s := range steps {
		if !*s.flag {
			if err := t.sendMessage(ctx, s.templateID, s.triggerType, c, nil); err != nil {
				return false, err
			}
			*s.flag = true

			// After CSLT3: reset all LT flags to restart rotation
			if s.templateID == "CS_LT3" {
				f.CSLT1 = false
				f.CSLT2 = false
				f.CSLT3 = false
				t.Logger.Info().
					Str("company_id", c.CompanyID).
					Msg("Cross-sell LT3 complete, resetting LT flags for rotation")
			}

			return true, t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f)
		}
	}

	// All LT flags sent but not yet reset (shouldn't happen if CSLT3 resets above)
	_ = fmt.Sprintf("") // suppress unused import
	return false, nil
}
