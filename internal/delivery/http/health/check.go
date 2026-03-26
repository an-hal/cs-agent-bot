package health

import (
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
)

// Health godoc
// @Summary      Health Check
// @Description  Health check for service
// @Tags         health
// @Success      200  {object}  response.StandardResponse
// @Router       /readiness [get]
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.Tracer.Start(r.Context(), "health.handler.Check")
	defer span.End()

	return response.StandardSuccess(w, r.WithContext(ctx), 200, "Service is healthy", map[string]string{
		"status": "ok",
	})
}
