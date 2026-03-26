package health

import (
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

type HealthHandler struct {
	Logger zerolog.Logger
	Tracer tracer.Tracer
}

func NewHealthHandler(logger zerolog.Logger, tracer tracer.Tracer) *HealthHandler {
	return &HealthHandler{
		Logger: logger,
		Tracer: tracer,
	}
}
