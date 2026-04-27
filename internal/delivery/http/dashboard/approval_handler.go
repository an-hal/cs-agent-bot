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
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/approval"
	"github.com/rs/zerolog"
)

// ApprovalHandler exposes the central checker-maker apply endpoint. The
// dispatcher routes by request_type to the correct feature Apply method —
// FE does not need to know per-feature endpoints.
type ApprovalHandler struct {
	dispatcher approval.Dispatcher
	logger     zerolog.Logger
	tracer     tracer.Tracer
}

func NewApprovalHandler(d approval.Dispatcher, logger zerolog.Logger, tr tracer.Tracer) *ApprovalHandler {
	return &ApprovalHandler{dispatcher: d, logger: logger, tracer: tr}
}

// Apply godoc
// @Summary      Apply a pending approval (central dispatcher)
// @Description  Routes by request_type: create_invoice, mark_invoice_paid,
// @Description  collection_schema_change, delete_client_record,
// @Description  toggle_automation_rule, bulk_import_master_data,
// @Description  stage_transition, integration_key_change.
// @Description  For bulk_import the xlsx is rehydrated from the approval
// @Description  payload (file_b64) — no second upload needed.
// @Tags         Approvals
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        id              path    string  true  "Approval request ID"
// @Success      200  {object}  response.StandardResponse{data=entity.ApprovalRequest}
// @Router       /api/approvals/{id}/apply [post]
func (h *ApprovalHandler) Apply(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ApprovalApply")
	defer span.End()

	id := router.GetParam(r, "id")
	out, err := h.dispatcher.Apply(ctx, ctxutil.GetWorkspaceID(ctx), id, callerEmail(r))
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Approval applied", out)
}

// List godoc
// @Summary      List approval requests for the current workspace
// @Tags         Approvals
// @Security     BearerAuth
// @Param        X-Workspace-ID header string true  "Workspace ID"
// @Param        status         query  string false "pending | approved | rejected | expired"
// @Param        request_type   query  string false "Filter by approval type"
// @Param        maker_email    query  string false "Filter by maker"
// @Param        limit          query  int    false "Default 50, max 200"
// @Param        offset         query  int    false "Default 0"
// @Success      200 {object} response.StandardResponse
// @Router       /api/approvals [get]
func (h *ApprovalHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ApprovalList")
	defer span.End()

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	filter := repository.ApprovalFilter{
		Status:      r.URL.Query().Get("status"),
		RequestType: r.URL.Query().Get("request_type"),
		MakerEmail:  r.URL.Query().Get("maker_email"),
		Limit:       limit,
		Offset:      offset,
	}
	items, total, err := h.dispatcher.List(ctx, ctxutil.GetWorkspaceID(ctx), filter)
	if err != nil {
		return err
	}
	if items == nil {
		items = []entity.ApprovalRequest{}
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Approvals", map[string]any{
		"items":  items,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

// Get godoc
// @Summary      Get a single approval request
// @Tags         Approvals
// @Security     BearerAuth
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Param        id             path   string true "Approval request UUID"
// @Success      200 {object} response.StandardResponse{data=entity.ApprovalRequest}
// @Router       /api/approvals/{id} [get]
func (h *ApprovalHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ApprovalGet")
	defer span.End()

	id := router.GetParam(r, "id")
	out, err := h.dispatcher.Get(ctx, ctxutil.GetWorkspaceID(ctx), id)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Approval", out)
}

// Reject godoc
// @Summary      Reject a pending approval (sets status=rejected)
// @Tags         Approvals
// @Security     BearerAuth
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Param        id             path   string true "Approval request UUID"
// @Param        body           body   ApprovalRejectRequest true "Rejection reason"
// @Success      200 {object} response.StandardResponse{data=entity.ApprovalRequest}
// @Router       /api/approvals/{id}/reject [post]
func (h *ApprovalHandler) Reject(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ApprovalReject")
	defer span.End()

	id := router.GetParam(r, "id")
	var req ApprovalRejectRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return apperror.BadRequest("invalid JSON body")
		}
	}
	out, err := h.dispatcher.Reject(ctx, ctxutil.GetWorkspaceID(ctx), id, callerEmail(r), req.Reason)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Approval rejected", out)
}

// ApprovalRejectRequest is the body for POST /approvals/{id}/reject.
type ApprovalRejectRequest struct {
	Reason string `json:"reason"`
}
