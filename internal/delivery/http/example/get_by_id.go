package example

import (
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/queryparams"
)

// GetByID godoc
// @Summary      Get example by ID
// @Description  Retrieves a single example by its UUID
// @Tags         example
// @Accept       json
// @Produce      json
// @Param        id_eq  query     string  true  "Example ID (UUID)"
// @Success      200    {object}  response.StandardResponse
// @Failure      400    {object}  response.StandardResponse
// @Failure      404    {object}  response.StandardResponse
// @Failure      500    {object}  response.StandardResponse
// @Router       /example/one [get]
func (h *ExampleHandler) GetByID(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.Tracer.Start(r.Context(), "example.handler.GetByID")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.Logger)

	logger.Info().Msg("Incoming GetByID example request")

	// Parse ID from query parameter
	id, err := queryparams.GetUUIDEq(r, "id")
	if err != nil {
		logger.Warn().Msg("Missing or invalid id_eq parameter")
		return apperror.BadRequest("id_eq parameter is required and must be a valid UUID")
	}

	example, err := h.ExampleUC.GetByID(ctx, id)
	if err != nil {
		return err
	}

	logger.Info().Str("example_id", example.ID.String()).Msg("Successfully fetched example")
	return response.StandardSuccess(w, r, http.StatusOK, "Successfully retrieved example", example)
}
