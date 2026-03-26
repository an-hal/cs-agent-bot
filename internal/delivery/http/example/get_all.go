package example

import (
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
)

// GetAll godoc
// @Summary      Get list of examples
// @Description  Retrieves paginated examples from the database
// @Tags         example
// @Accept       json
// @Produce      json
// @Param        offset  query     int  false  "Offset for pagination (default: 0)"
// @Param        limit   query     int  false  "Limit per page (default: 10, max: 100)"
// @Success      200     {object}  response.StandardResponseWithMeta
// @Failure      500     {object}  response.StandardResponse
// @Router       /example/list [get]
func (h *ExampleHandler) GetAll(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.Tracer.Start(r.Context(), "example.handler.GetAll")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.Logger)

	// Parse pagination params from query string
	params := pagination.FromRequest(r)

	logger.Info().
		Int("offset", params.Offset).
		Int("limit", params.Limit).
		Msg("Incoming GetAll examples request")

	result, err := h.ExampleUC.GetAll(ctx, params)
	if err != nil {
		return err
	}

	// Ensure we return empty array instead of null
	examples := result.Examples
	if examples == nil {
		examples = []entity.Example{}
	}

	logger.Info().
		Int("count", len(examples)).
		Int64("total", result.Meta.Total).
		Msg("Successfully fetched examples")

	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Successfully retrieved examples", result.Meta, examples)
}
