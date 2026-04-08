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
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/dashboard"
	"github.com/rs/zerolog"
)

type TemplateHandler struct {
	uc     dashboard.DashboardUsecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewTemplateHandler(uc dashboard.DashboardUsecase, logger zerolog.Logger, tr tracer.Tracer) *TemplateHandler {
	return &TemplateHandler{uc: uc, logger: logger, tracer: tr}
}

// List godoc
// @Summary      List message templates
// @Description  Returns paginated message templates with optional filters.
// @Tags         Dashboard
// @Param        category  query     string  false  "Filter by template category"
// @Param        language  query     string  false  "Filter by language"
// @Param        active    query     string  false  "Filter by active status (true/false)"
// @Param        offset    query     int     false  "Pagination offset (default 0)"
// @Param        limit     query     int     false  "Limit per page (default 10, max 100)"
// @Success      200  {object}  response.StandardResponseWithMeta{data=[]entity.Template}
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/message-template [get]
func (h *TemplateHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.TemplateList")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	params := pagination.FromRequest(r)
	q := r.URL.Query()

	filter := entity.TemplateFilter{
		Category: q.Get("category"),
		Language: q.Get("language"),
	}
	if activeStr := q.Get("active"); activeStr != "" {
		v, err := strconv.ParseBool(activeStr)
		if err == nil {
			filter.Active = &v
		}
	}

	logger.Info().Str("category", filter.Category).Str("language", filter.Language).Msg("Incoming list templates request")

	templates, total, err := h.uc.GetTemplates(ctx, filter, params)
	if err != nil {
		return err
	}
	if templates == nil {
		templates = []entity.Template{}
	}

	logger.Info().Int("count", len(templates)).Int64("total", total).Msg("Successfully fetched templates")

	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Templates", pagination.NewMeta(params, total), templates)
}

// Get godoc
// @Summary      Get message template by ID
// @Description  Returns a single message template by template_id.
// @Tags         Dashboard
// @Param        template_id  path      string  true  "Template ID"
// @Success      200  {object}  response.StandardResponse{data=entity.Template}
// @Failure      404  {object}  response.StandardResponse
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/message-template/{template_id} [get]
func (h *TemplateHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.TemplateGet")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	templateID := router.GetParam(r, "template_id")

	logger.Info().Str("template_id", templateID).Msg("Incoming get template request")

	tmpl, err := h.uc.GetTemplate(ctx, templateID)
	if err != nil {
		return err
	}
	if tmpl == nil {
		return apperror.NotFound("template", "Template not found")
	}

	logger.Info().Str("template_id", templateID).Msg("Successfully fetched template")

	return response.StandardSuccess(w, r, http.StatusOK, "Template", tmpl)
}

// Update godoc
// @Summary      Update message template
// @Description  Partially updates a message template (safe fields only).
// @Tags         Dashboard
// @Param        template_id  path      string                  true  "Template ID"
// @Param        body         body      map[string]interface{}  true  "Fields to update"
// @Success      200  {object}  response.StandardResponse
// @Failure      400  {object}  response.StandardResponse
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/dashboard/message-template/{template_id} [put]
func (h *TemplateHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.TemplateUpdate")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	templateID := router.GetParam(r, "template_id")

	logger.Info().Str("template_id", templateID).Msg("Incoming update template request")

	var patch map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		return apperror.ValidationError("Invalid request body")
	}

	if err := h.uc.UpdateTemplate(ctx, templateID, patch); err != nil {
		return err
	}

	if err := h.uc.RecordActivity(ctx, entity.ActivityLog{
		Category:  entity.ActivityCategoryData,
		ActorType: entity.ActivityActorHuman,
		Actor:     actorFromCtx(r),
		Action:    "edit_template",
		Target:    templateID,
		RefID:     templateID,
	}); err != nil {
		return err
	}

	logger.Info().Str("template_id", templateID).Msg("Successfully updated template")

	return response.StandardSuccess(w, r, http.StatusOK, "Template updated", nil)
}
