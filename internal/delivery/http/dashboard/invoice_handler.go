package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/dashboard"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/invoice"
	"github.com/rs/zerolog"
)

// InvoiceHandler serves both legacy /data-master/invoices routes (via dashboard.Usecase)
// and the full-featured /invoices routes (via invoice.Usecase).
type InvoiceHandler struct {
	uc        dashboard.DashboardUsecase
	invoiceUC invoice.Usecase
	pdfGen    invoice.PDFGenerator
	logger    zerolog.Logger
	tracer    tracer.Tracer
}

func NewInvoiceHandler(uc dashboard.DashboardUsecase, invoiceUC invoice.Usecase, logger zerolog.Logger, tr tracer.Tracer) *InvoiceHandler {
	return &InvoiceHandler{uc: uc, invoiceUC: invoiceUC, pdfGen: invoice.NewPDFGenerator(invoiceUC), logger: logger, tracer: tr}
}

// Activity godoc
// @Summary      Unified activity timeline for one invoice (payment_logs + mutations)
// @Tags         Invoices
// @Security     BearerAuth
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Param        invoice_id     path   string true "Invoice ID"
// @Param        limit          query  int    false "Max items (default 50, max 200)"
// @Router       /api/invoices/{invoice_id}/activity [get]
func (h *InvoiceHandler) Activity(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.InvoiceActivity")
	defer span.End()
	id := router.GetParam(r, "invoice_id")
	if id == "" {
		return apperror.BadRequest("invoice_id required")
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.invoiceUC.Activity(ctx, ctxutil.GetWorkspaceID(ctx), id, limit)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Invoice activity", out)
}

// UpdateStage godoc
// @Summary      Manually set collection_stage for an invoice (AE-only)
// @Tags         Invoices
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Param        invoice_id     path   string true "Invoice ID"
// @Param        body           body   entity.UpdateStageReq true "New stage"
// @Router       /api/invoices/{invoice_id}/update-stage [post]
func (h *InvoiceHandler) UpdateStage(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.InvoiceUpdateStage")
	defer span.End()
	id := router.GetParam(r, "invoice_id")
	var req entity.UpdateStageReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	req.Actor = callerEmail(r)
	out, err := h.invoiceUC.UpdateStage(ctx, ctxutil.GetWorkspaceID(ctx), id, req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Stage updated", out)
}

// ConfirmPartial godoc
// @Summary      Confirm one termin as paid on a multi-termin invoice (AE-only)
// @Tags         Invoices
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Param        invoice_id     path   string true "Invoice ID"
// @Param        body           body   entity.ConfirmPartialReq true "Termin confirmation"
// @Router       /api/invoices/{invoice_id}/confirm-partial [put]
func (h *InvoiceHandler) ConfirmPartial(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.InvoiceConfirmPartial")
	defer span.End()
	id := router.GetParam(r, "invoice_id")
	var req entity.ConfirmPartialReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	req.Actor = callerEmail(r)
	out, err := h.invoiceUC.ConfirmPartial(ctx, ctxutil.GetWorkspaceID(ctx), id, req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Termin confirmed", out)
}

// PDF godoc
// @Summary      Download invoice as HTML (printable → PDF via headless Chrome)
// @Description  Returns a styled HTML document ready for client-side PDF rendering.
// @Description  A server-side Chrome/wkhtmltopdf step is optional; this endpoint is
// @Description  deliberately dependency-free so it works in every env.
// @Tags         Invoices
// @Security     BearerAuth
// @Param        X-Workspace-ID header string true  "Workspace ID"
// @Param        invoice_id     path   string true  "Invoice ID"
// @Produce      text/html
// @Success      200  {string}  string  "HTML body"
// @Router       /api/invoices/{invoice_id}/pdf [get]
func (h *InvoiceHandler) PDF(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.InvoicePDF")
	defer span.End()
	invoiceID := router.GetParam(r, "invoice_id")
	if invoiceID == "" {
		return apperror.BadRequest("invoice_id required")
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="invoice-`+invoiceID+`.html"`)
	return h.pdfGen.GeneratePDF(ctx, ctxutil.GetWorkspaceID(ctx), invoiceID, w)
}

// List godoc
// @Summary      List invoices
// @Description  Returns paginated invoices for the workspace specified in the X-Workspace-ID header.
// @Tags         Dashboard
// @Param        X-Workspace-ID  header    string  true   "Workspace ID"
// @Param        company_id      query     string  false  "Filter by company ID"
// @Param        status          query     string  false  "Filter by payment status"
// @Param        offset          query     int     false  "Pagination offset (default 0)"
// @Param        limit           query     int     false  "Limit per page (default 10, max 100)"
// @Success      200  {object}  response.StandardResponseWithMeta{data=[]entity.Invoice}
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/data-master/invoices [get]
func (h *InvoiceHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.InvoiceList")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	params := pagination.FromRequest(r)
	q := r.URL.Query()

	wsID := ctxutil.GetWorkspaceID(ctx)
	filter := entity.InvoiceFilter{
		WorkspaceIDs:    []string{wsID},
		CompanyID:       q.Get("company_id"),
		Status:          q.Get("status"),
		Search:          q.Get("search"),
		CollectionStage: q.Get("collection_stage"),
		SortBy:          q.Get("sort_by"),
		SortDir:         q.Get("sort_dir"),
	}

	logger.Info().Str("workspace_id", wsID).Str("status", filter.Status).Msg("Incoming list invoices request")

	invoices, total, err := h.uc.GetInvoices(ctx, filter, params)
	if err != nil {
		return err
	}
	if invoices == nil {
		invoices = []entity.Invoice{}
	}

	logger.Info().Int("count", len(invoices)).Int64("total", total).Msg("Successfully fetched invoices")

	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Invoices", pagination.NewMeta(params, total), invoices)
}

// Get godoc
// @Summary      Get invoice detail
// @Description  Returns a single invoice with its line items and payment logs.
// @Tags         Dashboard
// @Param        invoice_id  path      string  true  "Invoice ID"
// @Success      200  {object}  response.StandardResponse{data=entity.InvoiceDetail}
// @Failure      404  {object}  response.StandardResponse
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/invoices/{invoice_id} [get]
func (h *InvoiceHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.InvoiceGet")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	invoiceID := router.GetParam(r, "invoice_id")

	logger.Info().Str("invoice_id", invoiceID).Msg("Incoming get invoice request")

	// When the full invoice usecase is wired, prefer the rich detail.
	if h.invoiceUC != nil {
		detail, err := h.invoiceUC.Get(ctx, invoiceID)
		if err != nil {
			return err
		}
		return response.StandardSuccess(w, r, http.StatusOK, "Invoice", detail)
	}

	// Fallback: legacy dashboard usecase.
	inv, err := h.uc.GetInvoice(ctx, invoiceID)
	if err != nil {
		return err
	}
	if inv == nil {
		return apperror.NotFound("invoice", "Invoice not found")
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Invoice", inv)
}

// Update godoc
// @Summary      Update invoice
// @Description  Partially updates an invoice (safe fields only — payment_status is rejected).
// @Tags         Dashboard
// @Param        invoice_id  path      string                  true  "Invoice ID"
// @Param        body        body      map[string]interface{}  true  "Fields to update"
// @Success      200  {object}  response.StandardResponse
// @Failure      400  {object}  response.StandardResponse
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/invoices/{invoice_id} [put]
func (h *InvoiceHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.InvoiceUpdate")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	invoiceID := router.GetParam(r, "invoice_id")

	logger.Info().Str("invoice_id", invoiceID).Msg("Incoming update invoice request")

	var patch map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		return apperror.ValidationError("Invalid request body")
	}

	if h.invoiceUC != nil {
		if err := h.invoiceUC.Update(ctx, invoiceID, patch); err != nil {
			return err
		}
	} else {
		if err := h.uc.UpdateInvoice(ctx, invoiceID, patch); err != nil {
			return err
		}
	}

	if err := h.uc.RecordActivity(ctx, entity.ActivityLog{
		WorkspaceID:  ctxutil.GetWorkspaceID(ctx),
		Category:     entity.ActivityCategoryData,
		ActorType:    entity.ActivityActorHuman,
		Actor:        actorFromCtx(r),
		Action:       "edit_invoice",
		Target:       invoiceID,
		RefID:        invoiceID,
		ResourceType: entity.ActivityResourceInvoice,
	}); err != nil {
		return err
	}

	logger.Info().Str("invoice_id", invoiceID).Msg("Successfully updated invoice")

	return response.StandardSuccess(w, r, http.StatusOK, "Invoice updated", nil)
}

// Create godoc
// @Summary      Create invoice (creates approval)
// @Description  Queues an approval request for a new invoice. Returns 202 with the approval request.
// @Tags         Dashboard
// @Param        X-Workspace-ID  header   string                    true  "Workspace ID"
// @Param        body            body     entity.CreateInvoiceReq   true  "Invoice create request"
// @Success      202  {object}  response.StandardResponse{data=entity.ApprovalRequest}
// @Failure      400  {object}  response.StandardResponse
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/invoices [post]
func (h *InvoiceHandler) Create(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.InvoiceCreate")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)

	var req entity.CreateInvoiceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationError("Invalid request body")
	}
	if req.CreatedBy == "" {
		req.CreatedBy = actorFromCtx(r)
	}

	ar, err := h.invoiceUC.Create(ctx, req)
	if err != nil {
		return err
	}

	logger.Info().Str("approval_id", ar.ID).Str("company_id", req.CompanyID).Msg("Invoice create approval requested")
	return response.StandardSuccess(w, r, http.StatusAccepted, "Invoice create approval requested", ar)
}

// Delete godoc
// @Summary      Delete invoice
// @Description  Deletes an invoice. Only invoices with status "Belum bayar" may be deleted.
// @Tags         Dashboard
// @Param        invoice_id  path      string  true  "Invoice ID"
// @Success      204  "No Content"
// @Failure      400  {object}  response.StandardResponse
// @Failure      404  {object}  response.StandardResponse
// @Router       /api/invoices/{invoice_id} [delete]
func (h *InvoiceHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.InvoiceDelete")
	defer span.End()

	invoiceID := router.GetParam(r, "invoice_id")
	if err := h.invoiceUC.Delete(ctx, invoiceID); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Invoice deleted", nil)
}

// MarkPaid godoc
// @Summary      Mark invoice as paid (creates approval)
// @Description  Queues an approval request to mark an invoice as paid. Returns 202 with the approval request.
// @Tags         Dashboard
// @Param        invoice_id  path   string              true  "Invoice ID"
// @Param        body        body   entity.MarkPaidReq  true  "Mark-paid request"
// @Success      202  {object}  response.StandardResponse{data=entity.ApprovalRequest}
// @Failure      400  {object}  response.StandardResponse
// @Router       /api/invoices/{invoice_id}/mark-paid [post]
func (h *InvoiceHandler) MarkPaid(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.InvoiceMarkPaid")
	defer span.End()

	invoiceID := router.GetParam(r, "invoice_id")

	var req entity.MarkPaidReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationError("Invalid request body")
	}
	if req.Actor == "" {
		req.Actor = actorFromCtx(r)
	}

	ar, err := h.invoiceUC.MarkPaid(ctx, invoiceID, req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusAccepted, "Mark-paid approval requested", ar)
}

// SendReminder godoc
// @Summary      Send payment reminder
// @Description  Sends a payment reminder for an invoice, records the reminder log.
// @Tags         Dashboard
// @Param        invoice_id  path   string                   true  "Invoice ID"
// @Param        body        body   entity.SendReminderReq   true  "Reminder request"
// @Success      200  {object}  response.StandardResponse
// @Failure      400  {object}  response.StandardResponse
// @Router       /api/invoices/{invoice_id}/send-reminder [post]
func (h *InvoiceHandler) SendReminder(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.InvoiceSendReminder")
	defer span.End()

	invoiceID := router.GetParam(r, "invoice_id")

	var req entity.SendReminderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationError("Invalid request body")
	}
	if req.Actor == "" {
		req.Actor = actorFromCtx(r)
	}

	if err := h.invoiceUC.SendReminder(ctx, invoiceID, req); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Reminder sent", nil)
}

// Stats godoc
// @Summary      Invoice stats
// @Description  Returns aggregated invoice stats for the workspace.
// @Tags         Dashboard
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse{data=entity.InvoiceStats}
// @Router       /api/invoices/stats [get]
func (h *InvoiceHandler) Stats(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.InvoiceStats")
	defer span.End()

	wsID := ctxutil.GetWorkspaceID(ctx)
	stats, err := h.invoiceUC.Stats(ctx, []string{wsID})
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Invoice stats", stats)
}

// ByStage godoc
// @Summary      Invoice counts by collection stage
// @Description  Returns map of collection_stage → count for the workspace.
// @Tags         Dashboard
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse{data=map[string]int64}
// @Router       /api/invoices/by-stage [get]
func (h *InvoiceHandler) ByStage(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.InvoiceByStage")
	defer span.End()

	wsID := ctxutil.GetWorkspaceID(ctx)
	data, err := h.invoiceUC.ByStage(ctx, []string{wsID})
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Invoices by stage", data)
}

// PaymentLogs godoc
// @Summary      List payment logs for an invoice
// @Description  Returns recent payment log entries for a single invoice.
// @Tags         Dashboard
// @Param        invoice_id  path  string  true  "Invoice ID"
// @Success      200  {object}  response.StandardResponse{data=[]entity.PaymentLog}
// @Router       /api/invoices/{invoice_id}/payment-logs [get]
func (h *InvoiceHandler) PaymentLogs(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.InvoicePaymentLogs")
	defer span.End()

	invoiceID := router.GetParam(r, "invoice_id")
	logs, err := h.invoiceUC.PaymentLogs(ctx, invoiceID)
	if err != nil {
		return err
	}
	if logs == nil {
		logs = []entity.PaymentLog{}
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Payment logs", logs)
}
