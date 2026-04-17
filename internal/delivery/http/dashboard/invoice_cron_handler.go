package dashboard

import (
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/invoice"
	"github.com/rs/zerolog"
)

// InvoiceCronHandler exposes the invoice cron jobs for Cloud Scheduler.
type InvoiceCronHandler struct {
	cron   invoice.CronInvoice
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewInvoiceCronHandler(cron invoice.CronInvoice, logger zerolog.Logger, tr tracer.Tracer) *InvoiceCronHandler {
	return &InvoiceCronHandler{cron: cron, logger: logger, tracer: tr}
}

// UpdateOverdue godoc
// @Summary      Cron: mark overdue invoices
// @Description  Marks invoices past due_date as Terlambat and writes status_change payment logs.
// @Tags         Cron
// @Success      200  {object}  response.StandardResponse
// @Router       /api/cron/invoices/overdue [get]
func (h *InvoiceCronHandler) UpdateOverdue(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "cron.handler.InvoiceUpdateOverdue")
	defer span.End()
	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)

	if err := h.cron.UpdateOverdueStatuses(ctx); err != nil {
		logger.Error().Err(err).Msg("cron: UpdateOverdueStatuses failed")
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Overdue statuses updated", nil)
}

// EscalateStages godoc
// @Summary      Cron: auto-escalate collection stages
// @Description  Promotes collection_stage of overdue invoices based on days_overdue thresholds.
// @Tags         Cron
// @Success      200  {object}  response.StandardResponse
// @Router       /api/cron/invoices/escalate [get]
func (h *InvoiceCronHandler) EscalateStages(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "cron.handler.InvoiceEscalateStages")
	defer span.End()
	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)

	if err := h.cron.AutoEscalateStages(ctx); err != nil {
		logger.Error().Err(err).Msg("cron: AutoEscalateStages failed")
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Stages escalated", nil)
}
