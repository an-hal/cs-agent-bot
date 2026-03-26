package example

import (
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/queryparams"
)

// Delete godoc
// @Summary      Delete an example
// @Description  Soft deletes an example by its UUID
// @Tags         example
// @Accept       json
// @Produce      json
// @Param        id_eq  query     string  true  "Example ID (UUID)"
// @Success      200    {object}  response.StandardResponse
// @Failure      400    {object}  response.StandardResponse
// @Failure      404    {object}  response.StandardResponse
// @Failure      500    {object}  response.StandardResponse
// @Router       /example/one [delete]
func (h *ExampleHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.Tracer.Start(r.Context(), "example.handler.Delete")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.Logger)

	logger.Info().Msg("Incoming Delete example request")

	// Parse ID from query parameter
	id, err := queryparams.GetUUIDEq(r, "id")
	if err != nil {
		return apperror.BadRequest("id_eq parameter is required and must be a valid UUID")
	}

	err = h.ExampleUC.Delete(ctx, id)
	if err != nil {
		return err
	}

	logger.Info().Str("example_id", id.String()).Msg("Successfully deleted example")
	return response.StandardSuccess(w, r, http.StatusOK, "Example deleted successfully", nil)
}
