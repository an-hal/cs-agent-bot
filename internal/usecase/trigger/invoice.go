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
func (t *TriggerService) EvalInvoice(ctx context.Context, c entity.Client, f entity.ClientFlags, inv *entity.Invoice, convState *entity.ConversationState) (bool, error) {
	// Check conversation state first (anti-spam)
	if !convState.ShouldSend() {
		return false, nil
	}

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

	if dte <= 14 && !c.Pre14Sent {
		if err := t.sendMessage(ctx, "TPL-PAY-PRE14", "PAY_PRE14_SENT", c, inv); err != nil {
			return false, err
		}
		if err := t.ClientRepo.UpdateInvoiceReminderFlags(ctx, c.CompanyID, map[string]bool{"pre14_sent": true}); err != nil {
			return true, err
		}
		_ = t.ConvStateRepo.RecordMessage(ctx, c.CompanyID, "INVOICE_REMINDER", "TPL-PAY-PRE14")
		return true, nil
	}

	if dte <= 7 && !c.Pre7Sent {
		if err := t.sendMessage(ctx, "TPL-PAY-PRE7", "PAY_PRE7_SENT", c, inv); err != nil {
			return false, err
		}
		if err := t.ClientRepo.UpdateInvoiceReminderFlags(ctx, c.CompanyID, map[string]bool{"pre7_sent": true}); err != nil {
			return true, err
		}
		_ = t.ConvStateRepo.RecordMessage(ctx, c.CompanyID, "INVOICE_REMINDER", "TPL-PAY-PRE7")
		return true, nil
	}

	if dte <= 3 && !c.Pre3Sent {
		if err := t.sendMessage(ctx, "TPL-PAY-PRE3", "PAY_PRE3_SENT", c, inv); err != nil {
			return false, err
		}
		if err := t.ClientRepo.UpdateInvoiceReminderFlags(ctx, c.CompanyID, map[string]bool{"pre3_sent": true}); err != nil {
			return true, err
		}
		_ = t.ConvStateRepo.RecordMessage(ctx, c.CompanyID, "INVOICE_REMINDER", "TPL-PAY-PRE3")
		return true, nil
	}

	return false, nil
}
