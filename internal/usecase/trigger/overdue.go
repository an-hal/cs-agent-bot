package trigger

import (
	"context"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

// EvalOverdue evaluates P3 overdue triggers.
// Post-due: D+1, D+4, D+8. D+15 → ESC-001 (AE takes over).
// Never checks ResponseStatus (Rule 4).
func (t *TriggerService) EvalOverdue(ctx context.Context, c entity.Client, f entity.ClientFlags, inv *entity.Invoice) (bool, error) {
	if inv == nil {
		return false, nil
	}

	daysPast := inv.DaysPastDue()
	if daysPast < 1 {
		return false, nil // not yet overdue
	}

	// D+1 reminder
	if daysPast >= 1 && !inv.Post1Sent {
		if err := t.sendMessage(ctx, "TPL-PAY-POST1", "PAY_POST1_SENT", c, inv); err != nil {
			return false, err
		}
		if err := t.InvoiceRepo.UpdateFlags(ctx, inv.InvoiceID, map[string]bool{"post1_sent": true}); err != nil {
			return true, err
		}
		return true, nil
	}

	// D+4 reminder
	if daysPast >= 4 && !inv.Post4Sent {
		if err := t.sendMessage(ctx, "TPL-PAY-POST4", "PAY_POST4_SENT", c, inv); err != nil {
			return false, err
		}
		if err := t.InvoiceRepo.UpdateFlags(ctx, inv.InvoiceID, map[string]bool{"post4_sent": true}); err != nil {
			return true, err
		}
		return true, nil
	}

	// D+8 reminder
	if daysPast >= 8 && !inv.Post8Sent {
		if err := t.sendMessage(ctx, "TPL-PAY-POST8", "PAY_POST8_SENT", c, inv); err != nil {
			return false, err
		}
		if err := t.InvoiceRepo.UpdateFlags(ctx, inv.InvoiceID, map[string]bool{"post8_sent": true}); err != nil {
			return true, err
		}
		return true, nil
	}

	// D+15 reminder
	if daysPast >= 15 && !inv.Post15Sent {
		if err := t.sendMessage(ctx, "TPL-PAY-POST15", "PAY_POST15_SENT", c, inv); err != nil {
			return false, err
		}
		if err := t.InvoiceRepo.UpdateFlags(ctx, inv.InvoiceID, map[string]bool{"post15_sent": true}); err != nil {
			return true, err
		}
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
