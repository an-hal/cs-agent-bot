package cron

import (
	"context"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/trigger"
	"github.com/rs/zerolog"
)

type CronRunner interface {
	RunAll(ctx context.Context) error
}

type cronRunner struct {
	clientRepo  repository.ClientRepository
	flagsRepo   repository.FlagsRepository
	invoiceRepo repository.InvoiceRepository
	logRepo     repository.LogRepository
	triggers    *trigger.TriggerService
	logger      zerolog.Logger
}

func NewCronRunner(
	clientRepo repository.ClientRepository,
	flagsRepo repository.FlagsRepository,
	invoiceRepo repository.InvoiceRepository,
	logRepo repository.LogRepository,
	triggers *trigger.TriggerService,
	logger zerolog.Logger,
) CronRunner {
	return &cronRunner{
		clientRepo:  clientRepo,
		flagsRepo:   flagsRepo,
		invoiceRepo: invoiceRepo,
		logRepo:     logRepo,
		triggers:    triggers,
		logger:      logger,
	}
}

func (cr *cronRunner) RunAll(ctx context.Context) error {
	clients, err := cr.clientRepo.GetAll(ctx)
	if err != nil {
		return err
	}

	cr.logger.Info().Int("total_clients", len(clients)).Msg("Cron run started")

	for _, c := range clients {
		if err := cr.processClient(ctx, c); err != nil {
			cr.logger.Error().Err(err).Str("company_id", c.CompanyID).Msg("Error processing client")
		}
		time.Sleep(300 * time.Millisecond) // inter-client throttle
	}

	cr.logger.Info().Msg("Cron run completed")
	return nil
}

func (cr *cronRunner) processClient(ctx context.Context, c entity.Client) error {
	// Gate 1: blacklisted — checked BEFORE bot_active. Always exit. (Rule 2)
	if c.Blacklisted {
		cr.logger.Warn().Str("company_id", c.CompanyID).Msg("Client is blacklisted")
		return nil
	}

	// Gate 2: bot suspended by AE or escalation
	if !c.BotActive {
		cr.logger.Warn().Str("company_id", c.CompanyID).Msg("Client bot is not active")
		return nil
	}

	// Gate 3: rejected by client
	if c.Rejected {
		cr.logger.Warn().Str("company_id", c.CompanyID).Msg("Client is rejected")
		return nil
	}

	// Gate 4: max 1 WA per client per calendar day
	sentToday, err := cr.logRepo.SentTodayAlready(ctx, c.CompanyID)
	if err != nil {
		cr.logger.Error().Err(err).Str("company_id", c.CompanyID).Msg("Failed to check if client already sent today")
		return err
	}
	if sentToday {
		cr.logger.Warn().Str("company_id", c.CompanyID).Msg("Client already sent today")
		return nil
	}

	// Get flags
	f, err := cr.flagsRepo.GetByCompanyID(ctx, c.CompanyID)
	if err != nil {
		cr.logger.Error().Err(err).Str("company_id", c.CompanyID).Msg("Failed to get flags")
		return err
	}

	// Check if renewed — triggers resetCycleFlags before evaluation
	if c.Renewed {
		if err := cr.flagsRepo.ResetCycleFlags(ctx, c.CompanyID); err != nil {
			cr.logger.Error().Err(err).Str("company_id", c.CompanyID).Msg("Failed to reset cycle flags")
			return err
		}
		// Re-fetch flags after reset
		f, err = cr.flagsRepo.GetByCompanyID(ctx, c.CompanyID)
		if err != nil {
			cr.logger.Error().Err(err).Str("company_id", c.CompanyID).Msg("Failed to get flags after reset")
			return err
		}
	}

	// Get active invoice
	inv, err := cr.invoiceRepo.GetActiveByCompanyID(ctx, c.CompanyID)
	if err != nil {
		cr.logger.Warn().Err(err).Str("company_id", c.CompanyID).Msg("Failed to get active invoice")
	}

	// Strict priority order. First match fires and returns — no further evaluation.
	if sent, _ := cr.triggers.EvalHealthRisk(ctx, c, *f); sent {
		cr.logger.Info().Str("company_id", c.CompanyID).Msg("Health risk trigger fired")
		return nil
	}
	if sent, _ := cr.triggers.EvalCheckIn(ctx, c, *f); sent {
		cr.logger.Info().Str("company_id", c.CompanyID).Msg("Check-in trigger fired")
		return nil
	}
	if sent, _ := cr.triggers.EvalNegotiation(ctx, c, *f); sent {
		cr.logger.Info().Str("company_id", c.CompanyID).Msg("Negotiation trigger fired")
		return nil
	}
	if sent, _ := cr.triggers.EvalInvoice(ctx, c, *f, inv); sent {
		cr.logger.Info().Str("company_id", c.CompanyID).Msg("Invoice trigger fired")
		return nil
	}
	if sent, _ := cr.triggers.EvalOverdue(ctx, c, *f, inv); sent {
		cr.logger.Info().Str("company_id", c.CompanyID).Msg("Overdue trigger fired")
		return nil
	}
	if sent, _ := cr.triggers.EvalExpansion(ctx, c, *f); sent {
		cr.logger.Info().Str("company_id", c.CompanyID).Msg("Expansion trigger fired")
		return nil
	}
	if sent, _ := cr.triggers.EvalCrossSell(ctx, c, *f); sent {
		cr.logger.Info().Str("company_id", c.CompanyID).Msg("Cross-sell trigger fired")
		return nil
	}

	return nil
}
