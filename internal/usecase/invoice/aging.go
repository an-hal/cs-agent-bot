package invoice

import (
	"context"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

// ReminderCadence defines when an auto-reminder should be queued for an
// invoice based on its days-to-due. Positive = days before due; negative =
// days after due (overdue). Each cadence point only fires once per invoice
// per day — the cron reruns safely.
//
// Ordering of points doesn't matter; the cron loops through and emits a
// SendReminder for any match on the current day's offset.
var ReminderCadence = []int{
	14,  // pre-due friendly heads-up
	7,   // pre-due reminder
	3,   // pre-due urgent
	0,   // due today
	-3,  // soft follow-up
	-8,  // empathetic firm
	-15, // final pre-suspend
}

// AutoResendReminders is the cron hook that emits a SendReminder for every
// invoice whose `(due_date - today)` matches one of the cadence points above.
// Called once daily; idempotent by design (SendReminder logs a PaymentLog
// with `EventType=reminder_sent`; if the invoice is already Lunas we skip).
func (c *cronInvoice) AutoResendReminders(ctx context.Context) error {
	// Scope: everything within 60 days past due (safer than an unbounded scan).
	// The cadence never fires beyond -15 days so this window is plenty.
	cutoff := time.Now().UTC().Add(-60 * 24 * time.Hour)
	invoices, err := c.uc.invoiceRepo.ListOverdue(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("aging: list invoices: %w", err)
	}
	today := truncateToDay(time.Now().UTC())
	sent := 0
	for _, inv := range invoices {
		if inv.PaymentStatus == entity.PaymentStatusLunas {
			continue
		}
		if inv.DueDate.IsZero() {
			continue
		}
		due := truncateToDay(inv.DueDate)
		offset := int(due.Sub(today).Hours() / 24)
		if !cadenceMatches(offset) {
			continue
		}
		if err := c.uc.SendReminder(ctx, inv.InvoiceID, entity.SendReminderReq{
			Channel: "wa",
			Actor:   "cron-auto-resend",
		}); err != nil {
			c.logger.Warn().Err(err).
				Str("invoice_id", inv.InvoiceID).
				Int("offset", offset).
				Msg("auto-resend reminder failed — continuing")
			continue
		}
		sent++
	}
	c.logger.Info().
		Int("sent", sent).
		Int("invoices_checked", len(invoices)).
		Msg("invoice auto-resend cadence complete")
	return nil
}

func cadenceMatches(offset int) bool {
	for _, p := range ReminderCadence {
		if p == offset {
			return true
		}
	}
	return false
}

func truncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
