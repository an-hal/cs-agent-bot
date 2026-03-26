package example

import (
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/validator"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase"
	"github.com/rs/zerolog"
)

// ExampleHandler handles HTTP requests for the example resource.
type ExampleHandler struct {
	ExampleUC usecase.ExampleUseCase
	Logger    zerolog.Logger
	Validator *validator.Validator
	Tracer    tracer.Tracer
}

// NewExampleHandler creates a new instance of ExampleHandler.
func NewExampleHandler(exampleUC usecase.ExampleUseCase, logger zerolog.Logger, validator *validator.Validator, tracer tracer.Tracer) *ExampleHandler {
	return &ExampleHandler{
		ExampleUC: exampleUC,
		Logger:    logger,
		Validator: validator,
		Tracer:    tracer,
	}
}
