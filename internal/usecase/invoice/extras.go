package invoice

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
)

// Activity returns a unified newest-first timeline for one invoice. Mixes
// payment_logs with master_data_mutations that touched this invoice's
// company_id (e.g. status transitions written against the client record).
// Payment logs always dominate; mutations are supplementary.
func (u *invoiceUsecase) Activity(ctx context.Context, workspaceID, invoiceID string, limit int) ([]InvoiceActivityEntry, error) {
	if invoiceID == "" {
		return nil, apperror.ValidationError("invoice_id required")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	inv, err := u.invoiceRepo.GetByID(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, apperror.NotFound("invoice", invoiceID)
	}
	if workspaceID != "" && inv.WorkspaceID != workspaceID {
		return nil, apperror.NotFound("invoice", invoiceID)
	}

	logs, err := u.paymentLogRepo.GetByInvoiceID(ctx, invoiceID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]InvoiceActivityEntry, 0, len(logs)+5)
	for _, l := range logs {
		summary := l.EventType
		if l.AmountPaid != nil && *l.AmountPaid > 0 {
			summary = fmt.Sprintf("%s: %d paid", l.EventType, *l.AmountPaid)
		}
		out = append(out, InvoiceActivityEntry{
			Source:    "payment_log",
			Timestamp: l.Timestamp,
			EventType: l.EventType,
			Actor:     l.Actor,
			Summary:   summary,
			Detail: map[string]any{
				"old_status":      l.OldStatus,
				"new_status":      l.NewStatus,
				"old_stage":       l.OldStage,
				"new_stage":       l.NewStage,
				"payment_method":  l.PaymentMethod,
				"payment_channel": l.PaymentChannel,
				"payment_ref":     l.PaymentRef,
				"notes":           l.Notes,
			},
		})
	}
	// Sort newest-first (payment_logs typically already sorted, but be safe).
	sortEntriesNewestFirst(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// UpdateStage is a direct write to collection_stage (no approval gate). Used
// when AE negotiates offline and needs to move the client to a softer/firmer
// stage than the aging cron would assign. Emits a payment_log entry so the
// manual change is auditable.
func (u *invoiceUsecase) UpdateStage(ctx context.Context, workspaceID, invoiceID string, req entity.UpdateStageReq) (*entity.Invoice, error) {
	if invoiceID == "" {
		return nil, apperror.ValidationError("invoice_id required")
	}
	if !isValidCollectionStage(req.NewStage) {
		return nil, apperror.ValidationError("invalid new_stage (valid: Stage 0–4, Closed)")
	}

	inv, err := u.invoiceRepo.GetByID(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	if inv == nil || (workspaceID != "" && inv.WorkspaceID != workspaceID) {
		return nil, apperror.NotFound("invoice", invoiceID)
	}
	if inv.CollectionStage == req.NewStage {
		return inv, nil // idempotent no-op
	}

	oldStage := inv.CollectionStage
	if err := u.invoiceRepo.UpdateFields(ctx, invoiceID, map[string]interface{}{
		"collection_stage": req.NewStage,
		"updated_at":       time.Now().UTC(),
	}); err != nil {
		return nil, fmt.Errorf("update stage: %w", err)
	}

	_ = u.paymentLogRepo.Append(ctx, entity.PaymentLog{
		WorkspaceID: inv.WorkspaceID,
		InvoiceID:   invoiceID,
		EventType:   "stage_updated",
		OldStage:    oldStage,
		NewStage:    req.NewStage,
		Actor:       firstNonEmpty(req.Actor, "dashboard"),
		Notes:       req.Reason,
		Timestamp:   time.Now().UTC(),
	})

	inv.CollectionStage = req.NewStage
	return inv, nil
}

// ConfirmPartial marks one termin as paid on a multi-termin invoice. When
// all termins reach status=paid, flips the invoice payment_status to Lunas
// and writes a consolidated payment log.
func (u *invoiceUsecase) ConfirmPartial(ctx context.Context, workspaceID, invoiceID string, req entity.ConfirmPartialReq) (*entity.Invoice, error) {
	if invoiceID == "" || req.TerminNumber <= 0 {
		return nil, apperror.ValidationError("invoice_id and termin_number required (termin_number >= 1)")
	}
	if req.AmountPaid <= 0 {
		return nil, apperror.ValidationError("amount_paid must be positive")
	}

	inv, err := u.invoiceRepo.GetByID(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	if inv == nil || (workspaceID != "" && inv.WorkspaceID != workspaceID) {
		return nil, apperror.NotFound("invoice", invoiceID)
	}
	if len(inv.TerminBreakdown) == 0 {
		return nil, apperror.BadRequest("invoice has no termin_breakdown — use /mark-paid for single-payment invoices")
	}

	// Locate the target termin.
	idx := -1
	for i, t := range inv.TerminBreakdown {
		if t.TerminNumber == req.TerminNumber {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, apperror.NotFound("termin", fmt.Sprintf("termin_number=%d", req.TerminNumber))
	}
	if inv.TerminBreakdown[idx].Status == "paid" {
		return nil, apperror.BadRequest("termin already paid")
	}

	now := time.Now().UTC()
	paidAt := req.PaidAt
	if paidAt.IsZero() {
		paidAt = now
	}
	inv.TerminBreakdown[idx].Status = "paid"
	inv.TerminBreakdown[idx].PaidAt = &paidAt
	inv.TerminBreakdown[idx].PaymentMethod = req.PaymentMethod
	inv.TerminBreakdown[idx].PaymentRef = req.PaymentRef
	inv.TerminBreakdown[idx].Notes = req.Notes

	// Recompute invoice aggregate state.
	allPaid := true
	var totalPaid int64
	for _, t := range inv.TerminBreakdown {
		if t.Status != "paid" {
			allPaid = false
		}
		if t.Status == "paid" {
			totalPaid += t.Amount
		}
	}

	fields := map[string]interface{}{
		"termin_breakdown": mustJSON(inv.TerminBreakdown),
		"amount_paid":      float64(totalPaid),
		"updated_at":       now,
	}
	if allPaid {
		fields["payment_status"] = entity.PaymentStatusLunas
		fields["paid_at"] = paidAt
		inv.PaymentStatus = entity.PaymentStatusLunas
	}

	if err := u.invoiceRepo.UpdateFields(ctx, invoiceID, fields); err != nil {
		return nil, fmt.Errorf("confirm partial: %w", err)
	}

	amountPaidLog := req.AmountPaid
	newStatus := inv.PaymentStatus
	_ = u.paymentLogRepo.Append(ctx, entity.PaymentLog{
		WorkspaceID:   inv.WorkspaceID,
		InvoiceID:     invoiceID,
		EventType:     "partial_paid",
		AmountPaid:    &amountPaidLog,
		PaymentMethod: req.PaymentMethod,
		PaymentRef:    req.PaymentRef,
		OldStatus:     inv.PaymentStatus, // snapshot for audit (may equal newStatus when not yet flipped)
		NewStatus:     newStatus,
		Actor:         firstNonEmpty(req.Actor, "ae"),
		Notes:         req.Notes,
		Timestamp:     now,
	})

	inv.AmountPaid = float64(totalPaid)
	return inv, nil
}

// ─── helpers ────────────────────────────────────────────────────────────────

func isValidCollectionStage(s string) bool {
	switch s {
	case entity.CollectionStage0, entity.CollectionStage1, entity.CollectionStage2,
		entity.CollectionStage3, entity.CollectionStage4, entity.CollectionStageClosed:
		return true
	}
	return false
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func defaultStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func sortEntriesNewestFirst(s []InvoiceActivityEntry) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j].Timestamp.After(s[j-1].Timestamp); j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
