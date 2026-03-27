package trigger

import (
	"context"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

// EvalCheckIn evaluates P0.5 check-in triggers.
// Branch A (ContractMonths >= 9): H-120 form, H-113 call, H-90 form, H-83 call
// Branch B (ContractMonths < 9):  H-90 form, H-83 call, H-60 form, H-53 call
func (t *TriggerService) EvalCheckIn(ctx context.Context, c entity.Client, f entity.ClientFlags) (bool, error) {
	if f.CheckinReplied {
		return false, nil // already replied, skip all check-in
	}

	dte := c.DaysToExpiry()

	if c.ContractMonths >= 9 {
		return t.evalBranchA(ctx, c, f, dte)
	}
	return t.evalBranchB(ctx, c, f, dte)
}

func (t *TriggerService) evalBranchA(ctx context.Context, c entity.Client, f entity.ClientFlags, dte int) (bool, error) {
	// H-120 form
	if dte <= 120 && !f.CheckinA1FormSent {
		if err := t.sendMessage(ctx, "CHECKIN_A1_FORM", "CHECKIN_A1_FORM_SENT", c, nil); err != nil {
			return false, err
		}
		f.CheckinA1FormSent = true
		return true, t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f)
	}

	// H-113 call (7 days after form if no reply)
	if dte <= 113 && f.CheckinA1FormSent && !f.CheckinA1CallSent {
		if err := t.sendMessage(ctx, "CHECKIN_A1_CALL", "CHECKIN_A1_CALL_SENT", c, nil); err != nil {
			return false, err
		}
		f.CheckinA1CallSent = true
		return true, t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f)
	}

	// H-90 form
	if dte <= 90 && !f.CheckinA2FormSent {
		if err := t.sendMessage(ctx, "CHECKIN_A2_FORM", "CHECKIN_A2_FORM_SENT", c, nil); err != nil {
			return false, err
		}
		f.CheckinA2FormSent = true
		return true, t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f)
	}

	// H-83 call
	if dte <= 83 && f.CheckinA2FormSent && !f.CheckinA2CallSent {
		if err := t.sendMessage(ctx, "CHECKIN_A2_CALL", "CHECKIN_A2_CALL_SENT", c, nil); err != nil {
			return false, err
		}
		f.CheckinA2CallSent = true
		return true, t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f)
	}

	return false, nil
}

func (t *TriggerService) evalBranchB(ctx context.Context, c entity.Client, f entity.ClientFlags, dte int) (bool, error) {
	// H-90 form
	if dte <= 90 && !f.CheckinB1FormSent {
		if err := t.sendMessage(ctx, "CHECKIN_B1_FORM", "CHECKIN_B1_FORM_SENT", c, nil); err != nil {
			return false, err
		}
		f.CheckinB1FormSent = true
		return true, t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f)
	}

	// H-83 call
	if dte <= 83 && f.CheckinB1FormSent && !f.CheckinB1CallSent {
		if err := t.sendMessage(ctx, "CHECKIN_B1_CALL", "CHECKIN_B1_CALL_SENT", c, nil); err != nil {
			return false, err
		}
		f.CheckinB1CallSent = true
		return true, t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f)
	}

	// H-60 form
	if dte <= 60 && !f.CheckinB2FormSent {
		if err := t.sendMessage(ctx, "CHECKIN_B2_FORM", "CHECKIN_B2_FORM_SENT", c, nil); err != nil {
			return false, err
		}
		f.CheckinB2FormSent = true
		return true, t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f)
	}

	// H-53 call
	if dte <= 53 && f.CheckinB2FormSent && !f.CheckinB2CallSent {
		if err := t.sendMessage(ctx, "CHECKIN_B2_CALL", "CHECKIN_B2_CALL_SENT", c, nil); err != nil {
			return false, err
		}
		f.CheckinB2CallSent = true
		return true, t.FlagsRepo.UpdateFlags(ctx, c.CompanyID, f)
	}

	return false, nil
}
