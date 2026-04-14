package http

import (
	"github.com/Sejutacita/cs-agent-bot/config"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/validator"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/auth"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/cron"
	customfield "github.com/Sejutacita/cs-agent-bot/internal/usecase/custom_field"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/dashboard"
	masterdata "github.com/Sejutacita/cs-agent-bot/internal/usecase/master_data"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/notification"
	usecasePayment "github.com/Sejutacita/cs-agent-bot/internal/usecase/payment"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/trigger"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/webhook"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/workspace"
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
	PaymentVerifier  usecasePayment.PaymentVerifier
	DashboardUsecase dashboard.DashboardUsecase
	LogRepo          repository.LogRepository
	TriggerRuleRepo  repository.TriggerRuleRepository
	SystemConfigRepo repository.SystemConfigRepository
	RuleEngine       *trigger.RuleEngine
	AuthUsecase      auth.AuthUsecase
	WorkspaceUC      workspace.Usecase
	NotificationUC   notification.Usecase
	MasterDataUC     masterdata.Usecase
	CustomFieldUC    customfield.Usecase
}
