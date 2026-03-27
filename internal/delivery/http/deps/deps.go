package http

import (
	"github.com/Sejutacita/cs-agent-bot/config"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/validator"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/cron"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/webhook"
	"github.com/rs/zerolog"
)

type Deps struct {
	Cfg              *config.AppConfig
	Logger           zerolog.Logger
	Validator        *validator.Validator
	Tracer           tracer.Tracer
	ExceptionHandler *response.HTTPExceptionHandler
	CronRunner       cron.CronRunner
	ReplyHandler     webhook.ReplyHandler
	CheckinHandler   webhook.CheckinFormHandler
	HandoffHandler   webhook.HandoffHandler
}
