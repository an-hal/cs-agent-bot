package http

import (
	"github.com/Sejutacita/cs-agent-bot/config"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/validator"
	"github.com/Sejutacita/cs-agent-bot/internal/service/session"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase"
	"github.com/rs/zerolog"
)

type Deps struct {
	Cfg              *config.AppConfig
	Logger           zerolog.Logger
	Validator        *validator.Validator
	Tracer           tracer.Tracer
	ExceptionHandler *response.HTTPExceptionHandler
	ExampleUC        usecase.ExampleUseCase
	SessionStore     session.Store
}
