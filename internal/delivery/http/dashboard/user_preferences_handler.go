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
	userpreferences "github.com/Sejutacita/cs-agent-bot/internal/usecase/user_preferences"
	"github.com/rs/zerolog"
)

type UserPreferencesHandler struct {
	uc     userpreferences.Usecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewUserPreferencesHandler(uc userpreferences.Usecase, logger zerolog.Logger, tr tracer.Tracer) *UserPreferencesHandler {
	return &UserPreferencesHandler{uc: uc, logger: logger, tracer: tr}
}

// List godoc
// @Summary      List all preferences for the caller in this workspace
// @Tags         UserPreferences
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse{data=[]entity.UserPreference}
// @Router       /api/preferences [get]
func (h *UserPreferencesHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.UserPreferencesList")
	defer span.End()

	wsID := ctxutil.GetWorkspaceID(ctx)
	out, err := h.uc.List(ctx, wsID, callerEmail(r))
	if err != nil {
		return err
	}
	if out == nil {
		out = []entity.UserPreference{}
	}
	return response.StandardSuccess(w, r, http.StatusOK, "User preferences", out)
}

// Get godoc
// @Summary      Get a single preference by namespace
// @Tags         UserPreferences
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        namespace       path    string  true  "Preference namespace (e.g. theme, sidebar, columns.clients)"
// @Success      200  {object}  response.StandardResponse{data=entity.UserPreference}
// @Router       /api/preferences/{namespace} [get]
func (h *UserPreferencesHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.UserPreferencesGet")
	defer span.End()

	namespace := router.GetParam(r, "namespace")
	out, err := h.uc.Get(ctx, ctxutil.GetWorkspaceID(ctx), callerEmail(r), namespace)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "User preference", out)
}

type upsertPreferenceBody struct {
	Value map[string]any `json:"value"`
}

// Upsert godoc
// @Summary      Create or update a preference by namespace (full replace of value)
// @Tags         UserPreferences
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID  header  string                 true  "Workspace ID"
// @Param        namespace       path    string                 true  "Preference namespace"
// @Param        body            body    upsertPreferenceBody   true  "Preference value"
// @Success      200  {object}  response.StandardResponse{data=entity.UserPreference}
// @Router       /api/preferences/{namespace} [put]
func (h *UserPreferencesHandler) Upsert(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.UserPreferencesUpsert")
	defer span.End()

	namespace := router.GetParam(r, "namespace")
	var body upsertPreferenceBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.Upsert(ctx, userpreferences.UpsertRequest{
		WorkspaceID: ctxutil.GetWorkspaceID(ctx),
		UserEmail:   callerEmail(r),
		Namespace:   namespace,
		Value:       body.Value,
	})
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Preference saved", out)
}

// Delete godoc
// @Summary      Delete a preference by namespace
// @Tags         UserPreferences
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        namespace       path    string  true  "Preference namespace"
// @Success      200  {object}  response.StandardResponse
// @Router       /api/preferences/{namespace} [delete]
func (h *UserPreferencesHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.UserPreferencesDelete")
	defer span.End()

	namespace := router.GetParam(r, "namespace")
	if err := h.uc.Delete(ctx, ctxutil.GetWorkspaceID(ctx), callerEmail(r), namespace); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Preference deleted", nil)
}
