package trigger

import (
	"github.com/Sejutacita/cs-agent-bot/config"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/service/cache"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/escalation"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/haloai"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/telegram"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/template"
	"github.com/rs/zerolog"
)

type TriggerService struct {
	ClientRepo       repository.ClientRepository
	InvoiceRepo      repository.InvoiceRepository
	FlagsRepo        repository.FlagsRepository
	LogRepo          repository.LogRepository
	ConfigRepo       repository.ConfigRepository
	EscalationRepo   repository.EscalationRepository
	TemplateResolver template.TemplateResolver
	HaloAI           haloai.HaloAIClient
	Telegram         telegram.TelegramNotifier
	Escalation       escalation.EscalationHandler
	Cache            cache.SheetCache
	Logger           zerolog.Logger
	Cfg              *config.AppConfig
}

func NewTriggerService(
	clientRepo repository.ClientRepository,
	invoiceRepo repository.InvoiceRepository,
	flagsRepo repository.FlagsRepository,
	logRepo repository.LogRepository,
	configRepo repository.ConfigRepository,
	escalationRepo repository.EscalationRepository,
	templateResolver template.TemplateResolver,
	haloaiClient haloai.HaloAIClient,
	telegramNotifier telegram.TelegramNotifier,
	escalationHandler escalation.EscalationHandler,
	cacheService cache.SheetCache,
	cfg *config.AppConfig,
	logger zerolog.Logger,
) *TriggerService {
	return &TriggerService{
		ClientRepo:       clientRepo,
		InvoiceRepo:      invoiceRepo,
		FlagsRepo:        flagsRepo,
		LogRepo:          logRepo,
		ConfigRepo:       configRepo,
		EscalationRepo:   escalationRepo,
		TemplateResolver: templateResolver,
		HaloAI:           haloaiClient,
		Telegram:         telegramNotifier,
		Escalation:       escalationHandler,
		Cache:            cacheService,
		Cfg:              cfg,
		Logger:           logger,
	}
}
