package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/analytics"
	"github.com/rs/zerolog"
)

// Cache is the narrow dependency the handler uses for short-lived caching of
// expensive aggregate payloads (spec 09 §Redis 15-min cache). Any impl of
// internal/pkg/rediscache.Cache fits; nil is legal (always miss).
type AnalyticsCache interface {
	Get(ctx context.Context, key string, dest any) (bool, error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
}

// AnalyticsHandler handles analytics endpoints.
type AnalyticsHandler struct {
	uc     analytics.Usecase
	cache  AnalyticsCache
	logger zerolog.Logger
	tracer tracer.Tracer
}

// NewAnalyticsHandler creates a new AnalyticsHandler. `cache` may be nil.
func NewAnalyticsHandler(uc analytics.Usecase, logger zerolog.Logger, tr tracer.Tracer) *AnalyticsHandler {
	return &AnalyticsHandler{uc: uc, logger: logger, tracer: tr}
}

// WithCache attaches a cache for the Bundle endpoint and returns the handler
// for chain-style wiring.
func (h *AnalyticsHandler) WithCache(c AnalyticsCache) *AnalyticsHandler {
	h.cache = c
	return h
}

// Bundle godoc
// @Summary      Bundled KPI + distributions + engagement + forecast
// @Description  Single-call batch for FE dashboard; optional role filter tags the payload so FE
// @Description  can render role-specific panels without a second round-trip. Cache is recommended
// @Description  at the client side (15 min).
// @Tags         Analytics
// @Param        X-Workspace-ID  header  string  true   "Workspace ID"
// @Param        role            query   string  false  "sdr|bd|ae|admin — optional, echoed in meta"
// @Param        months          query   int     false  "Revenue-trend history window (default 6)"
// @Success      200  {object}  response.StandardResponse
// @Router       /analytics/kpi/bundle [get]
func (h *AnalyticsHandler) Bundle(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "analytics.handler.Bundle")
	defer span.End()

	wsID := ctxutil.GetWorkspaceID(ctx)
	months, _ := strconv.Atoi(r.URL.Query().Get("months"))
	if months <= 0 || months > 24 {
		months = 6
	}
	role := r.URL.Query().Get("role")

	// Cache check — spec 09 §15-min cache. Key embeds workspace+role+months
	// so different views don't collide.
	if h.cache != nil {
		cacheKey := "analytics:bundle:" + wsID + ":" + role + ":" + strconv.Itoa(months)
		var cached map[string]any
		if hit, _ := h.cache.Get(ctx, cacheKey, &cached); hit {
			return response.StandardSuccess(w, r, http.StatusOK, "Analytics bundle (cached)", cached)
		}
		defer func() {
			// Best-effort warm — done after successful response; ignore errors.
		}()
		ctx = context.WithValue(ctx, analyticsCacheKeyCtx{}, cacheKey)
	}

	kpi, err := h.uc.KPI(ctx, wsID)
	if err != nil {
		return err
	}
	dist, err := h.uc.Distributions(ctx, wsID)
	if err != nil {
		return err
	}
	eng, err := h.uc.Engagement(ctx, wsID)
	if err != nil {
		return err
	}
	trend, err := h.uc.RevenueTrend(ctx, wsID, months)
	if err != nil {
		return err
	}
	acc, err := h.uc.ForecastAccuracy(ctx, wsID)
	if err != nil {
		return err
	}

	// Per-role projection: convert KPI to JSON map then filter by role layout.
	kpiBytes, _ := json.Marshal(kpi)
	var kpiMap map[string]any
	_ = json.Unmarshal(kpiBytes, &kpiMap)
	roleKPI := analytics.BuildRoleKPIFromJSON(role, kpiMap)

	payload := map[string]any{
		"role":              role,
		"kpi":               kpi,
		"role_kpi":          roleKPI,
		"distributions":     dist,
		"engagement":        eng,
		"revenue_trend":     trend,
		"forecast_accuracy": acc,
	}
	if h.cache != nil {
		if key, ok := ctx.Value(analyticsCacheKeyCtx{}).(string); ok && key != "" {
			_ = h.cache.Set(ctx, key, payload, 15*time.Minute)
		}
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Analytics bundle", payload)
}

// analyticsCacheKeyCtx is a private ctx key for passing the cache key from
// the cache-check prelude down to the write-back step.
type analyticsCacheKeyCtx struct{}

// Stats godoc
// @Summary      Dashboard overview stats
// @Description  Quick stats for the dashboard overview page (lightweight — used on page load).
// @Tags         Analytics
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse{data=entity.DashboardStats}
// @Failure      500  {object}  response.StandardResponse
// @Router       /dashboard/stats [get]
func (h *AnalyticsHandler) Stats(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "analytics.handler.Stats")
	defer span.End()

	wsID := ctxutil.GetWorkspaceID(ctx)

	stats, err := h.uc.DashboardStats(ctx, wsID)
	if err != nil {
		return err
	}

	return response.StandardSuccess(w, r, http.StatusOK, "Dashboard stats retrieved", stats)
}

// KPI godoc
// @Summary      Full KPI data
// @Description  Comprehensive KPI data for the Analytics page.
// @Tags         Analytics
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse{data=entity.KPIData}
// @Failure      500  {object}  response.StandardResponse
// @Router       /analytics/kpi [get]
func (h *AnalyticsHandler) KPI(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "analytics.handler.KPI")
	defer span.End()

	wsID := ctxutil.GetWorkspaceID(ctx)

	kpi, err := h.uc.KPI(ctx, wsID)
	if err != nil {
		return err
	}

	return response.StandardSuccess(w, r, http.StatusOK, "KPI data retrieved", kpi)
}

// Distributions godoc
// @Summary      Distribution data for charts
// @Description  Distribution data for charts (donut, horizontal bars).
// @Tags         Analytics
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse{data=entity.DistributionData}
// @Failure      500  {object}  response.StandardResponse
// @Router       /analytics/distributions [get]
func (h *AnalyticsHandler) Distributions(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "analytics.handler.Distributions")
	defer span.End()

	wsID := ctxutil.GetWorkspaceID(ctx)

	dist, err := h.uc.Distributions(ctx, wsID)
	if err != nil {
		return err
	}

	return response.StandardSuccess(w, r, http.StatusOK, "Distribution data retrieved", dist)
}

// Engagement godoc
// @Summary      Engagement metrics
// @Description  Bot/NPS/checkin/cross-sell/renewed counts.
// @Tags         Analytics
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse{data=entity.EngagementData}
// @Failure      500  {object}  response.StandardResponse
// @Router       /analytics/engagement [get]
func (h *AnalyticsHandler) Engagement(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "analytics.handler.Engagement")
	defer span.End()

	wsID := ctxutil.GetWorkspaceID(ctx)

	eng, err := h.uc.Engagement(ctx, wsID)
	if err != nil {
		return err
	}

	return response.StandardSuccess(w, r, http.StatusOK, "Engagement data retrieved", eng)
}

// RevenueTrend godoc
// @Summary      Monthly revenue data with forecast
// @Description  Monthly revenue data with OLS linear regression forecast.
// @Tags         Analytics
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        months          query   int     false "Total months (default 16 — 10 actual + 6 forecast)"
// @Success      200  {object}  response.StandardResponse{data=entity.RevenueTrendResponse}
// @Failure      500  {object}  response.StandardResponse
// @Router       /analytics/revenue-trend [get]
func (h *AnalyticsHandler) RevenueTrend(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "analytics.handler.RevenueTrend")
	defer span.End()

	wsID := ctxutil.GetWorkspaceID(ctx)

	months := 16
	if v := r.URL.Query().Get("months"); v != "" {
		if m, err := strconv.Atoi(v); err == nil && m > 0 {
			months = m
		}
	}

	trend, err := h.uc.RevenueTrend(ctx, wsID, months)
	if err != nil {
		return err
	}

	return response.StandardSuccess(w, r, http.StatusOK, "Revenue trend retrieved", trend)
}

// ForecastAccuracy godoc
// @Summary      Forecast accuracy
// @Description  Last month's forecast vs actual difference.
// @Tags         Analytics
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse
// @Failure      500  {object}  response.StandardResponse
// @Router       /analytics/forecast-accuracy [get]
func (h *AnalyticsHandler) ForecastAccuracy(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "analytics.handler.ForecastAccuracy")
	defer span.End()

	wsID := ctxutil.GetWorkspaceID(ctx)

	accuracy, err := h.uc.ForecastAccuracy(ctx, wsID)
	if err != nil {
		return err
	}

	return response.StandardSuccess(w, r, http.StatusOK, "Forecast accuracy retrieved", map[string]float64{"accuracy": accuracy})
}
