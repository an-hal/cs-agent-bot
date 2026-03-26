package example

import (
	"encoding/json"
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
)

// CreateRequest represents the request body for creating an example.
type CreateRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=255"`
	Description string `json:"description" validate:"max=1000"`
	Status      string `json:"status,omitempty" validate:"omitempty,oneof=active inactive pending"`
}

// Create godoc
// @Summary      Create a new example
// @Description  Creates a new example with the provided name, description, and status
// @Tags         example
// @Accept       json
// @Produce      json
// @Param        request  body      CreateRequest  true  "Example data to create"
// @Success      201      {object}  response.StandardResponse
// @Failure      400      {object}  response.StandardResponse
// @Failure      422      {object}  response.StandardResponse
// @Failure      500      {object}  response.StandardResponse
// @Router       /example/one [post]
func (h *ExampleHandler) Create(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.Tracer.Start(r.Context(), "example.handler.Create")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.Logger)

	logger.Info().Msg("Incoming Create example request")

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationError("Invalid request body")
	}

	// Validate request using struct tags
	if err := h.Validator.Validate(&req); err != nil {
		return err
	}

	example := entity.Example{
		Name:        req.Name,
		Description: req.Description,
		Status:      req.Status,
	}

	created, err := h.ExampleUC.Create(ctx, example)
	if err != nil {
		return err
	}

	logger.Info().Str("example_id", created.ID.String()).Msg("Successfully created example")
	return response.StandardSuccess(w, r, http.StatusCreated, "Example created successfully", created)
}
