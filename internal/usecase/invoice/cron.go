package invoice

import (
	"context"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/rs/zerolog"
)

// CronInvoice runs periodic invoice maintenance jobs.
// Wired to Cloud Scheduler (or any cron runner) via cron HTTP handlers.
type CronInvoice interface {
	// UpdateOverdueStatuses marks past-due invoices as Terlambat and writes payment logs.
	UpdateOverdueStatuses(ctx context.Context) error
	// AutoEscalateStages promotes collection_stage based on days_overdue thresholds.
	AutoEscalateStages(ctx context.Context) error
}

// Stage escalation thresholds (days overdue → stage).
// Stage 0 is pre-due; once overdue the stages escalate.
var stageThresholds = []struct {
	minDays int
	stage   string
}{
	{30, entity.CollectionStage4},
	{21, entity.CollectionStage3},
	{14, entity.CollectionStage2},
	{7, entity.CollectionStage1},
	{1, entity.CollectionStage1},
}

// cronInvoice implements CronInvoice using the invoice usecase deps.
type cronInvoice struct {
	uc     *invoiceUsecase
	logger zerolog.Logger
}

// NewCronInvoice wraps an invoiceUsecase as a CronInvoice.
// The Usecase interface returned by New() also satisfies CronInvoice via this helper.
func NewCronInvoice(uc Usecase) CronInvoice {
	u, ok := uc.(*invoiceUsecase)
	if !ok {
		panic("invoice.NewCronInvoice: expected *invoiceUsecase")
	}
	return &cronInvoice{uc: u, logger: u.logger}
}

// UpdateOverdueStatuses sets payment_status='Terlambat' and recalculates days_overdue
// for all invoices that are past due_date and not yet paid.
// Runs daily at 00:05 WIB.
func (c *cronInvoice) UpdateOverdueStatuses(ctx context.Context) error {
	now := time.Now().UTC()
	overdue, err := c.uc.invoiceRepo.ListOverdue(ctx, now)
	if err != nil {
		return fmt.Errorf("cron.UpdateOverdueStatuses list: %w", err)
	}
	if len(overdue) == 0 {
		return nil
	}

	ids := make([]string, 0, len(overdue))
	for _, inv := range overdue {
		ids = append(ids, inv.InvoiceID)
	}

	if err := c.uc.invoiceRepo.UpdateStatusBulk(ctx, ids, entity.PaymentStatusTerlambat); err != nil {
		return fmt.Errorf("cron.UpdateOverdueStatuses bulk update: %w", err)
	}

	// Append a payment_log per changed invoice (best-effort).
	for _, inv := range overdue {
		daysOverdue := int(now.Sub(inv.DueDate).Hours() / 24)
		lerr := c.uc.paymentLogRepo.Append(ctx, entity.PaymentLog{
			WorkspaceID: inv.WorkspaceID,
			InvoiceID:   inv.InvoiceID,
			EventType:   entity.EventStatusChange,
			OldStatus:   inv.PaymentStatus,
			NewStatus:   entity.PaymentStatusTerlambat,
			Actor:       "system",
			Notes:       fmt.Sprintf("overdue by %d days", daysOverdue),
			Timestamp:   now,
		})
		if lerr != nil {
			c.logger.Warn().Err(lerr).Str("invoice_id", inv.InvoiceID).Msg("cron: failed to append overdue log")
		}
	}

	c.logger.Info().Int("count", len(ids)).Msg("cron: marked invoices as Terlambat")
	return nil
}

// AutoEscalateStages promotes the collection_stage of overdue invoices
// based on how many days they have been overdue. Runs daily at 00:10 WIB.
func (c *cronInvoice) AutoEscalateStages(ctx context.Context) error {
	now := time.Now().UTC()

	// Fetch all overdue (already Terlambat) invoices that may need stage escalation.
	overdue, err := c.uc.invoiceRepo.ListOverdue(ctx, now)
	if err != nil {
		return fmt.Errorf("cron.AutoEscalateStages list: %w", err)
	}

	for _, inv := range overdue {
		daysOverdue := int(now.Sub(inv.DueDate).Hours() / 24)
		targetStage := inv.CollectionStage // default: no change

		for _, thresh := range stageThresholds {
			if daysOverdue >= thresh.minDays {
				targetStage = thresh.stage
				break
			}
		}

		if targetStage == inv.CollectionStage || targetStage == "" {
			continue
		}

		if err := c.uc.invoiceRepo.UpdateFields(ctx, inv.InvoiceID, map[string]interface{}{
			"collection_stage": targetStage,
			"days_overdue":     daysOverdue,
			"updated_at":       now,
		}); err != nil {
			c.logger.Warn().Err(err).Str("invoice_id", inv.InvoiceID).Msg("cron: failed to escalate stage")
			continue
		}

		_ = c.uc.paymentLogRepo.Append(ctx, entity.PaymentLog{
			WorkspaceID: inv.WorkspaceID,
			InvoiceID:   inv.InvoiceID,
			EventType:   entity.EventStageChange,
			OldStage:    inv.CollectionStage,
			NewStage:    targetStage,
			Actor:       "system",
			Notes:       fmt.Sprintf("auto-escalated after %d days overdue", daysOverdue),
			Timestamp:   now,
		})
	}

	c.logger.Info().Int("inspected", len(overdue)).Msg("cron: AutoEscalateStages complete")
	return nil
}
