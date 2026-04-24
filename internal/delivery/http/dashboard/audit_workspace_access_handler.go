package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	auditws "github.com/Sejutacita/cs-agent-bot/internal/usecase/audit_workspace_access"
	"github.com/rs/zerolog"
)

type AuditWorkspaceAccessHandler struct {
	uc     auditws.Usecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewAuditWorkspaceAccessHandler(uc auditws.Usecase, logger zerolog.Logger, tr tracer.Tracer) *AuditWorkspaceAccessHandler {
	return &AuditWorkspaceAccessHandler{uc: uc, logger: logger, tracer: tr}
}

// List godoc
// @Summary      List cross-workspace access audit entries
// @Tags         AuditLogs
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true   "Workspace ID"
// @Param        actor           query   string  false  "Filter by actor_email"
// @Param        kind            query   string  false  "read|write|admin"
// @Param        resource        query   string  false  "Resource filter"
// @Param        limit           query   int     false  "Max items (default 50, max 200)"
// @Param        offset          query   int     false  "Offset"
// @Router       /api/audit-logs/workspace-access [get]
func (h *AuditWorkspaceAccessHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.AuditWorkspaceAccessList")
	defer span.End()

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	out, total, err := h.uc.List(ctx, entity.AuditWorkspaceAccessFilter{
		WorkspaceID: ctxutil.GetWorkspaceID(ctx),
		ActorEmail:  r.URL.Query().Get("actor"),
		Kind:        r.URL.Query().Get("kind"),
		Resource:    r.URL.Query().Get("resource"),
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		return err
	}
	if out == nil {
		out = []entity.AuditWorkspaceAccess{}
	}
	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Workspace access audit",
		pagination.Meta{Total: total, Offset: offset, Limit: limit}, out)
}

type recordWorkspaceAccessBody struct {
	Kind       string `json:"access_kind"`
	Resource   string `json:"resource"`
	ResourceID string `json:"resource_id"`
	Reason     string `json:"reason"`
}

// Record godoc
// @Summary      Record a cross-workspace access event
// @Tags         AuditLogs
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID  header  string                       true  "Workspace ID"
// @Param        body            body    recordWorkspaceAccessBody    true  "Event"
// @Router       /api/audit-logs/workspace-access [post]
func (h *AuditWorkspaceAccessHandler) Record(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.AuditWorkspaceAccessRecord")
	defer span.End()

	var body recordWorkspaceAccessBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.Record(ctx, auditws.RecordRequest{
		WorkspaceID: ctxutil.GetWorkspaceID(ctx),
		ActorEmail:  callerEmail(r),
		Kind:        body.Kind,
		Resource:    body.Resource,
		ResourceID:  body.ResourceID,
		IPAddress:   clientIP(r),
		UserAgent:   r.UserAgent(),
		Reason:      body.Reason,
	})
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusCreated, "Workspace access recorded", out)
}

// clientIP extracts the caller's IP, honoring X-Forwarded-For when present.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	return r.RemoteAddr
}
