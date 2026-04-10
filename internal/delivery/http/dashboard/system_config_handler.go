package dashboard

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Sejutacita/cs-agent-bot/config"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// SystemConfigHandler handles CRUD for the system_config table via the dashboard.
type SystemConfigHandler struct {
	repo   repository.SystemConfigRepository
	logRepo repository.LogRepository
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewSystemConfigHandler(
	repo repository.SystemConfigRepository,
	logRepo repository.LogRepository,
	logger zerolog.Logger,
	tr tracer.Tracer,
) *SystemConfigHandler {
	return &SystemConfigHandler{repo: repo, logRepo: logRepo, logger: logger, tracer: tr}
}

// List godoc
// @Summary      List all system config entries
// @Description  Returns all system_config entries. Sensitive values (tokens) are masked.
// @Tags         Dashboard
// @Success      200  {object}  response.StandardResponse{data=[]entity.SystemConfig}
// @Router       /api/data-master/system-config [get]
func (h *SystemConfigHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.SystemConfigList")
	defer span.End()

	configs, err := h.repo.ListAll(ctx)
	if err != nil {
		return err
	}
	if configs == nil {
		configs = []entity.SystemConfig{}
	}

	// Mask sensitive values so tokens are never exposed via the API.
	for i := range configs {
		if config.IsSensitiveConfigKey(configs[i].Key) && configs[i].Value != "" {
			configs[i].Value = "***"
		}
	}

	return response.StandardSuccess(w, r, http.StatusOK, "System config", configs)
}

// Update godoc
// @Summary      Update a system config entry
// @Description  Updates the value of a single system_config key.
// @Tags         Dashboard
// @Param        key   path  string  true  "Config key (e.g. HALOAI_API_URL)"
// @Param        body  body  object  true  "{ \"value\": \"new-value\" }"
// @Success      200  {object}  response.StandardResponse
// @Router       /api/data-master/system-config/{key} [put]
func (h *SystemConfigHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.SystemConfigUpdate")
	defer span.End()

	key := router.GetParam(r, "key")
	if strings.TrimSpace(key) == "" {
		return apperror.ValidationError("key path param is required")
	}

	var body struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return apperror.ValidationError("Invalid request body")
	}

	actor := actorFromCtx(r)
	if err := h.repo.Upsert(ctx, key, body.Value, actor); err != nil {
		return err
	}

	// Log the change; mask sensitive key values in the activity detail.
	detail := key + "=<updated>"
	if !config.IsSensitiveConfigKey(key) {
		detail = key + "=" + body.Value
	}

	if err := h.logRepo.AppendActivity(ctx, entity.ActivityLog{
		WorkspaceID:  ctxutil.GetWorkspaceID(ctx),
		Category:     entity.ActivityCategoryData,
		ActorType:    entity.ActivityActorHuman,
		Actor:        actor,
		Action:       "edit_system_config",
		Target:       key,
		RefID:        key,
		ResourceType: entity.ActivityResourceSystemConfig,
		Detail:       detail,
	}); err != nil {
		return err
	}

	return response.StandardSuccess(w, r, http.StatusOK, "System config updated", nil)
}
