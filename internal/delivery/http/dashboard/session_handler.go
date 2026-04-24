package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

type SessionHandler struct {
	repo   repository.RevokedSessionRepository
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewSessionHandler(repo repository.RevokedSessionRepository, logger zerolog.Logger, tr tracer.Tracer) *SessionHandler {
	return &SessionHandler{repo: repo, logger: logger, tracer: tr}
}

type revokeSessionBody struct {
	JTI             string `json:"jti"`
	UserEmail       string `json:"user_email"`
	Reason          string `json:"reason"`
	ExpiresInHours  int    `json:"expires_in_hours"`
}

// Revoke godoc
// @Summary  Revoke a user session / JWT jti
// @Tags     Sessions
// @Accept   json
// @Param    X-Workspace-ID header string true "Workspace ID"
// @Param    body body revokeSessionBody true "JTI + expiry"
// @Router   /api/sessions/revoke [post]
func (h *SessionHandler) Revoke(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.SessionRevoke")
	defer span.End()
	var b revokeSessionBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	if b.JTI == "" {
		return apperror.ValidationError("jti required")
	}
	if b.ExpiresInHours <= 0 {
		b.ExpiresInHours = 24 // default to the common JWT lifetime
	}
	err := h.repo.Revoke(ctx, &repository.RevokedSession{
		WorkspaceID: ctxutil.GetWorkspaceID(ctx),
		JTI:         b.JTI,
		UserEmail:   b.UserEmail,
		Reason:      b.Reason,
		RevokedBy:   callerEmail(r),
		ExpiresAt:   time.Now().UTC().Add(time.Duration(b.ExpiresInHours) * time.Hour),
	})
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusCreated, "Session revoked", nil)
}

// List godoc
// @Summary  List active revocations for a user
// @Tags     Sessions
// @Param    X-Workspace-ID header string true "Workspace ID"
// @Param    user_email query string true "User email"
// @Param    limit query int false "Max items"
// @Router   /api/sessions/revoked [get]
func (h *SessionHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.SessionList")
	defer span.End()
	email := r.URL.Query().Get("user_email")
	if email == "" {
		return apperror.ValidationError("user_email required")
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.repo.ListByUser(ctx, ctxutil.GetWorkspaceID(ctx), email, limit)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Revoked sessions", out)
}

// Cleanup godoc
// @Summary  Delete expired revocation rows (cron)
// @Tags     Sessions
// @Router   /api/cron/sessions/cleanup [get]
func (h *SessionHandler) Cleanup(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.SessionCleanup")
	defer span.End()
	n, err := h.repo.CleanupExpired(ctx)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Expired revocations cleaned", map[string]int64{"deleted": n})
}
