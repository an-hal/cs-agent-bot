package dashboard

import (
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
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
// @Description  toggle_automation_rule.
// @Description  bulk_import_master_data requires the row payload — use
// @Description  POST /data-master/import/commit/{approval_id} instead.
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
