package dashboard

import (
	"encoding/json"
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

type WorkspaceThemeHandler struct {
	repo   repository.WorkspaceThemeRepository
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewWorkspaceThemeHandler(repo repository.WorkspaceThemeRepository, logger zerolog.Logger, tr tracer.Tracer) *WorkspaceThemeHandler {
	return &WorkspaceThemeHandler{repo: repo, logger: logger, tracer: tr}
}

// Get godoc
// @Summary  Get theme config for the current workspace
// @Tags     Workspace
// @Param    X-Workspace-ID header string true "Workspace ID"
// @Router   /api/workspace/theme [get]
func (h *WorkspaceThemeHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.WorkspaceThemeGet")
	defer span.End()
	out, err := h.repo.Get(ctx, ctxutil.GetWorkspaceID(ctx))
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Workspace theme", out)
}

type upsertThemeBody struct {
	Theme map[string]any `json:"theme"`
}

// Upsert godoc
// @Summary  Replace the theme config for the current workspace
// @Tags     Workspace
// @Accept   json
// @Param    X-Workspace-ID header string true "Workspace ID"
// @Param    body body upsertThemeBody true "Theme JSON (opaque)"
// @Router   /api/workspace/theme [put]
func (h *WorkspaceThemeHandler) Upsert(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.WorkspaceThemeUpsert")
	defer span.End()
	var b upsertThemeBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.repo.Upsert(ctx, &repository.WorkspaceTheme{
		WorkspaceID: ctxutil.GetWorkspaceID(ctx),
		Theme:       b.Theme,
		UpdatedBy:   callerEmail(r),
	})
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Theme saved", out)
}

// ExpandHolding godoc
// @Summary  Resolve the full workspace set a holding covers (self + siblings)
// @Description  Returns [workspace_id] if no holding is configured, else all siblings sharing the same holding_id.
// @Tags     Workspace
// @Param    X-Workspace-ID header string true "Workspace ID"
// @Router   /api/workspace/holding/expand [get]
func (h *WorkspaceThemeHandler) ExpandHolding(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.HoldingExpand")
	defer span.End()
	ids, err := h.repo.ExpandHolding(ctx, ctxutil.GetWorkspaceID(ctx))
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Holding workspaces", map[string]any{"workspace_ids": ids, "count": len(ids)})
}
