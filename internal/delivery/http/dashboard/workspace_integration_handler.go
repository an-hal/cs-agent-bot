package dashboard

import (
	"encoding/json"
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	workspaceintegration "github.com/Sejutacita/cs-agent-bot/internal/usecase/workspace_integration"
	"github.com/rs/zerolog"
)

type WorkspaceIntegrationHandler struct {
	uc     workspaceintegration.Usecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewWorkspaceIntegrationHandler(uc workspaceintegration.Usecase, logger zerolog.Logger, tr tracer.Tracer) *WorkspaceIntegrationHandler {
	return &WorkspaceIntegrationHandler{uc: uc, logger: logger, tracer: tr}
}

// List godoc
// @Summary      List integrations configured for this workspace (secrets redacted)
// @Tags         WorkspaceIntegrations
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse{data=[]entity.WorkspaceIntegration}
// @Router       /api/integrations [get]
func (h *WorkspaceIntegrationHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.IntegrationsList")
	defer span.End()

	out, err := h.uc.List(ctx, ctxutil.GetWorkspaceID(ctx))
	if err != nil {
		return err
	}
	if out == nil {
		out = []entity.WorkspaceIntegration{}
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Workspace integrations", out)
}

// Get godoc
// @Summary      Get a single integration by provider (secrets redacted)
// @Tags         WorkspaceIntegrations
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        provider        path    string  true  "Provider (haloai|telegram|paper_id|smtp)"
// @Success      200  {object}  response.StandardResponse{data=entity.WorkspaceIntegration}
// @Router       /api/integrations/{provider} [get]
func (h *WorkspaceIntegrationHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.IntegrationsGet")
	defer span.End()

	provider := router.GetParam(r, "provider")
	out, err := h.uc.Get(ctx, ctxutil.GetWorkspaceID(ctx), provider)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Workspace integration", out)
}

type upsertIntegrationBody struct {
	DisplayName string         `json:"display_name"`
	Config      map[string]any `json:"config"`
	IsActive    *bool          `json:"is_active,omitempty"`
}

// Upsert godoc
// @Summary      Create or update an integration for a provider
// @Tags         WorkspaceIntegrations
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID  header  string                  true  "Workspace ID"
// @Param        provider        path    string                  true  "Provider (haloai|telegram|paper_id|smtp)"
// @Param        body            body    upsertIntegrationBody   true  "Integration config"
// @Success      200  {object}  response.StandardResponse{data=entity.WorkspaceIntegration}
// @Router       /api/integrations/{provider} [put]
func (h *WorkspaceIntegrationHandler) Upsert(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.IntegrationsUpsert")
	defer span.End()

	provider := router.GetParam(r, "provider")
	var body upsertIntegrationBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.Upsert(ctx, workspaceintegration.UpsertRequest{
		WorkspaceID: ctxutil.GetWorkspaceID(ctx),
		Provider:    provider,
		DisplayName: body.DisplayName,
		Config:      body.Config,
		IsActive:    body.IsActive,
		ActorEmail:  callerEmail(r),
	})
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Integration saved", out)
}

// Delete godoc
// @Summary      Delete an integration by provider
// @Tags         WorkspaceIntegrations
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        provider        path    string  true  "Provider"
// @Success      200  {object}  response.StandardResponse
// @Router       /api/integrations/{provider} [delete]
func (h *WorkspaceIntegrationHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.IntegrationsDelete")
	defer span.End()

	provider := router.GetParam(r, "provider")
	if err := h.uc.Delete(ctx, ctxutil.GetWorkspaceID(ctx), provider); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Integration deleted", nil)
}
