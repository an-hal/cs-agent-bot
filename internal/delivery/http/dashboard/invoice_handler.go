package dashboard

import (
	"encoding/json"
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/dashboard"
	"github.com/rs/zerolog"
)

type InvoiceHandler struct {
	uc     dashboard.DashboardUsecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewInvoiceHandler(uc dashboard.DashboardUsecase, logger zerolog.Logger, tr tracer.Tracer) *InvoiceHandler {
	return &InvoiceHandler{uc: uc, logger: logger, tracer: tr}
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
// @Summary      Get invoice by ID
// @Description  Returns a single invoice by invoice_id.
// @Tags         Dashboard
// @Param        invoice_id  path      string  true  "Invoice ID"
// @Success      200  {object}  response.StandardResponse{data=entity.Invoice}
// @Failure      404  {object}  response.StandardResponse
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/invoices/{invoice_id} [get]
func (h *InvoiceHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.InvoiceGet")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	invoiceID := router.GetParam(r, "invoice_id")

	logger.Info().Str("invoice_id", invoiceID).Msg("Incoming get invoice request")

	inv, err := h.uc.GetInvoice(ctx, invoiceID)
	if err != nil {
		return err
	}
	if inv == nil {
		return apperror.NotFound("invoice", "Invoice not found")
	}

	logger.Info().Str("invoice_id", invoiceID).Msg("Successfully fetched invoice")

	return response.StandardSuccess(w, r, http.StatusOK, "Invoice", inv)
}

// Update godoc
// @Summary      Update invoice
// @Description  Partially updates an invoice (safe fields only).
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

	if err := h.uc.UpdateInvoice(ctx, invoiceID, patch); err != nil {
		return err
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
