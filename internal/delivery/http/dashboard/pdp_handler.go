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
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/pdp"
	"github.com/rs/zerolog"
)

type PDPHandler struct {
	uc     pdp.Usecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewPDPHandler(uc pdp.Usecase, logger zerolog.Logger, tr tracer.Tracer) *PDPHandler {
	return &PDPHandler{uc: uc, logger: logger, tracer: tr}
}

// ─── Erasure requests ───────────────────────────────────────────────────────

type createErasureBody struct {
	SubjectEmail string   `json:"subject_email"`
	SubjectKind  string   `json:"subject_kind"`
	Reason       string   `json:"reason"`
	Scope        []string `json:"scope"`
}

// CreateErasure godoc
// @Tags  PDP
// @Accept json
// @Param  X-Workspace-ID header string true  "Workspace ID"
// @Param  body           body   createErasureBody true "Subject + scope"
// @Router /api/pdp/erasure-requests [post]
func (h *PDPHandler) CreateErasure(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.PDPCreateErasure")
	defer span.End()
	var b createErasureBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.CreateErasure(ctx, pdp.CreateErasureRequest{
		WorkspaceID:  ctxutil.GetWorkspaceID(ctx),
		SubjectEmail: b.SubjectEmail,
		SubjectKind:  b.SubjectKind,
		Reason:       b.Reason,
		Scope:        b.Scope,
		Requester:    callerEmail(r),
	})
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusCreated, "Erasure request created", out)
}

// ListErasure godoc
// @Tags  PDP
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param status query string false "pending|approved|executed|rejected|expired"
// @Param subject_email query string false "Filter by subject"
// @Param limit query int false "Max items"
// @Param offset query int false "Offset"
// @Router /api/pdp/erasure-requests [get]
func (h *PDPHandler) ListErasure(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.PDPListErasure")
	defer span.End()
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	out, total, err := h.uc.ListErasure(ctx, entity.PDPErasureRequestFilter{
		WorkspaceID:  ctxutil.GetWorkspaceID(ctx),
		Status:       r.URL.Query().Get("status"),
		SubjectEmail: r.URL.Query().Get("subject_email"),
		Limit:        limit,
		Offset:       offset,
	})
	if err != nil {
		return err
	}
	if out == nil {
		out = []entity.PDPErasureRequest{}
	}
	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Erasure requests",
		pagination.Meta{Total: total, Offset: offset, Limit: limit}, out)
}

// GetErasure godoc
// @Tags  PDP
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param id path string true "Erasure request ID"
// @Router /api/pdp/erasure-requests/{id} [get]
func (h *PDPHandler) GetErasure(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.PDPGetErasure")
	defer span.End()
	out, err := h.uc.GetErasure(ctx, ctxutil.GetWorkspaceID(ctx), router.GetParam(r, "id"))
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Erasure request", out)
}

// ApproveErasure godoc
// @Tags  PDP
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param id path string true "Erasure request ID"
// @Router /api/pdp/erasure-requests/{id}/approve [post]
func (h *PDPHandler) ApproveErasure(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.PDPApproveErasure")
	defer span.End()
	out, err := h.uc.ApproveErasure(ctx, ctxutil.GetWorkspaceID(ctx), router.GetParam(r, "id"), callerEmail(r))
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Erasure approved", out)
}

type rejectErasureBody struct {
	Reason string `json:"reason"`
}

// RejectErasure godoc
// @Tags  PDP
// @Accept json
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param id path string true "Erasure request ID"
// @Param body body rejectErasureBody true "Reason"
// @Router /api/pdp/erasure-requests/{id}/reject [post]
func (h *PDPHandler) RejectErasure(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.PDPRejectErasure")
	defer span.End()
	var b rejectErasureBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.RejectErasure(ctx, ctxutil.GetWorkspaceID(ctx), router.GetParam(r, "id"), callerEmail(r), b.Reason)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Erasure rejected", out)
}

// ExecuteErasure godoc
// @Tags  PDP
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param id path string true "Erasure request ID"
// @Router /api/pdp/erasure-requests/{id}/execute [post]
func (h *PDPHandler) ExecuteErasure(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.PDPExecuteErasure")
	defer span.End()
	out, err := h.uc.ExecuteErasure(ctx, ctxutil.GetWorkspaceID(ctx), router.GetParam(r, "id"))
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Erasure executed", out)
}

// ─── Retention policies ─────────────────────────────────────────────────────

type upsertPolicyBody struct {
	DataClass     string `json:"data_class"`
	RetentionDays int    `json:"retention_days"`
	Action        string `json:"action"`
	IsActive      *bool  `json:"is_active,omitempty"`
}

// UpsertPolicy godoc
// @Tags  PDP
// @Accept json
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param body body upsertPolicyBody true "Policy"
// @Router /api/pdp/retention-policies [post]
func (h *PDPHandler) UpsertPolicy(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.PDPUpsertPolicy")
	defer span.End()
	var b upsertPolicyBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.UpsertPolicy(ctx, pdp.UpsertPolicyRequest{
		WorkspaceID:   ctxutil.GetWorkspaceID(ctx),
		DataClass:     b.DataClass,
		RetentionDays: b.RetentionDays,
		Action:        b.Action,
		IsActive:      b.IsActive,
		ActorEmail:    callerEmail(r),
	})
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Policy saved", out)
}

// ListPolicies godoc
// @Tags  PDP
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param active query bool false "Filter to is_active=true only"
// @Router /api/pdp/retention-policies [get]
func (h *PDPHandler) ListPolicies(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.PDPListPolicies")
	defer span.End()
	active, _ := strconv.ParseBool(r.URL.Query().Get("active"))
	out, err := h.uc.ListPolicies(ctx, ctxutil.GetWorkspaceID(ctx), active)
	if err != nil {
		return err
	}
	if out == nil {
		out = []entity.PDPRetentionPolicy{}
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Retention policies", out)
}

// DeletePolicy godoc
// @Tags PDP
// @Param X-Workspace-ID header string true "Workspace ID"
// @Param id path string true "Policy ID"
// @Router /api/pdp/retention-policies/{id} [delete]
func (h *PDPHandler) DeletePolicy(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.PDPDeletePolicy")
	defer span.End()
	if err := h.uc.DeletePolicy(ctx, ctxutil.GetWorkspaceID(ctx), router.GetParam(r, "id")); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Policy deleted", nil)
}

// RunRetention godoc
// @Tags PDP
// @Param X-Workspace-ID header string true "Workspace ID"
// @Router /api/cron/pdp/retention [get]
func (h *PDPHandler) RunRetention(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.PDPRunRetention")
	defer span.End()
	out, err := h.uc.RunRetention(ctx, ctxutil.GetWorkspaceID(ctx))
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Retention run", out)
}
