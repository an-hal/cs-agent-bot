package trigger

import (
	"context"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

// EvalInvoice evaluates P2 invoice + payment triggers.
// Creates invoice at H-30. Sends pre-due reminders at H-14/7/3.
// Never checks ResponseStatus (Rule 4).
func (t *TriggerService) EvalInvoice(ctx context.Context, c entity.Client, f entity.ClientFlags, inv *entity.Invoice) (bool, error) {
	dte := c.DaysToExpiry()

	// At H-30: create invoice if none exists
	if dte <= 30 && inv == nil {
		// Check quotation_link must exist (Rule 10)
		if c.QuotationLink == "" {
			alertMsg := fmt.Sprintf("quotation_link is empty for %s (%s). Invoice creation delayed.", c.CompanyName, c.CompanyID)
			_ = t.Telegram.SendMessage(ctx, c.OwnerTelegramID, alertMsg)
			return false, nil
		}

		newInv := entity.Invoice{
			InvoiceID:     fmt.Sprintf("INV-%s-%s", time.Now().Format("2006"), c.CompanyID),
			CompanyID:     c.CompanyID,
			DueDate:       c.ContractEnd,
			PaymentStatus: entity.PaymentStatusPending,
		}

		if err := t.InvoiceRepo.CreateInvoice(ctx, newInv); err != nil {
			return false, fmt.Errorf("failed to create invoice: %w", err)
		}

		inv = &newInv
	}

	if inv == nil {
		return false, nil
	}

	daysUntilDue := inv.DaysUntilDue()

	// H-14 reminder
	if daysUntilDue <= 14 && !inv.Pre14Sent {
		if err := t.sendMessage(ctx, "TPL-PAY-PRE14", "PAY_PRE14_SENT", c, inv); err != nil {
			return false, err
		}
		if err := t.InvoiceRepo.UpdateFlags(ctx, inv.InvoiceID, map[string]bool{"pre14_sent": true}); err != nil {
			return true, err
		}
		return true, nil
	}

	// H-7 reminder
	if daysUntilDue <= 7 && !inv.Pre7Sent {
		if err := t.sendMessage(ctx, "TPL-PAY-PRE7", "PAY_PRE7_SENT", c, inv); err != nil {
			return false, err
		}
		if err := t.InvoiceRepo.UpdateFlags(ctx, inv.InvoiceID, map[string]bool{"pre7_sent": true}); err != nil {
			return true, err
		}
		return true, nil
	}

	// H-3 reminder
	if daysUntilDue <= 3 && !inv.Pre3Sent {
		if err := t.sendMessage(ctx, "TPL-PAY-PRE3", "PAY_PRE3_SENT", c, inv); err != nil {
			return false, err
		}
		if err := t.InvoiceRepo.UpdateFlags(ctx, inv.InvoiceID, map[string]bool{"pre3_sent": true}); err != nil {
			return true, err
		}
		return true, nil
	}

	return false, nil
}
