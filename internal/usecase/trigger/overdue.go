package trigger

import (
	"context"
	"fmt"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

// EvalOverdue evaluates P3 overdue triggers.
// Post-due: D+1, D+4, D+8. D+15 → ESC-001 (AE takes over).
// Never checks ResponseStatus (Rule 4).
func (t *TriggerService) EvalOverdue(ctx context.Context, c entity.Client, f entity.ClientFlags, inv *entity.Invoice, convState *entity.ConversationState) (bool, error) {
	// Check conversation state first (anti-spam)
	if !convState.ShouldSend() {
		fmt.Println("Client bot is not active, reason: ", convState.ReasonBotPaused)
		return false, nil
	}

	// Primary check: Client-based overdue calculation
	if !c.IsPaymentOverdue() {
		return false, nil
	}

	daysPast := c.DaysPastDue()

	// D+1 reminder - using Client flags
	if daysPast >= 1 && !c.Post1Sent {
		if err := t.sendMessage(ctx, "TPL-PAY-POST1", "PAY_POST1_SENT", c, inv); err != nil {
			return false, err
		}
		if err := t.ClientRepo.UpdateInvoiceReminderFlags(ctx, c.CompanyID, map[string]bool{"post1_sent": true}); err != nil {
			return true, err
		}
		// Record in conversation state
		_ = t.ConvStateRepo.RecordMessage(ctx, c.CompanyID, "OVERDUE_REMINDER", "TPL-PAY-POST1")
		return true, nil
	}

	// D+4 reminder - using Client flags
	if daysPast >= 4 && !c.Post4Sent {
		if err := t.sendMessage(ctx, "TPL-PAY-POST4", "PAY_POST4_SENT", c, inv); err != nil {
			return false, err
		}
		if err := t.ClientRepo.UpdateInvoiceReminderFlags(ctx, c.CompanyID, map[string]bool{"post4_sent": true}); err != nil {
			return true, err
		}
		// Record in conversation state
		_ = t.ConvStateRepo.RecordMessage(ctx, c.CompanyID, "OVERDUE_REMINDER", "TPL-PAY-POST4")
		return true, nil
	}

	// D+8 reminder - using Client flags
	if daysPast >= 8 && !c.Post8Sent {
		if err := t.sendMessage(ctx, "TPL-PAY-POST8", "PAY_POST8_SENT", c, inv); err != nil {
			return false, err
		}
		if err := t.ClientRepo.UpdateInvoiceReminderFlags(ctx, c.CompanyID, map[string]bool{"post8_sent": true}); err != nil {
			return true, err
		}
		// Record in conversation state
		_ = t.ConvStateRepo.RecordMessage(ctx, c.CompanyID, "OVERDUE_REMINDER", "TPL-PAY-POST8")
		return true, nil
	}

	// D+15 reminder - using Client flags
	if daysPast >= 15 && !c.Post15Sent {
		if err := t.sendMessage(ctx, "TPL-PAY-POST15", "PAY_POST15_SENT", c, inv); err != nil {
			return false, err
		}
		if err := t.ClientRepo.UpdateInvoiceReminderFlags(ctx, c.CompanyID, map[string]bool{"post15_sent": true}); err != nil {
			return true, err
		}
		// Record in conversation state
		_ = t.ConvStateRepo.RecordMessage(ctx, c.CompanyID, "OVERDUE_REMINDER", "TPL-PAY-POST15")
		return true, nil
	}

	// D+15: ESC-001 — AE takes over
	if daysPast >= 15 {
		_ = t.Escalation.TriggerEscalation(ctx, entity.EscInvoiceOverdue15, c,
			"Invoice overdue D+15+", entity.EscPriorityP1Critical)
		return true, nil
	}

	return false, nil
}
