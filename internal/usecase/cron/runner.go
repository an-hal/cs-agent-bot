package cron

import (
	"context"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/trigger"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type CronRunner interface {
	RunAll(ctx context.Context) error
	StartRunAll(ctx context.Context) ([]*entity.BackgroundJob, error)
}

type cronRunner struct {
	clientRepo    repository.ClientRepository
	flagsRepo     repository.FlagsRepository
	convStateRepo repository.ConversationStateRepository
	invoiceRepo   repository.InvoiceRepository
	logRepo       repository.LogRepository
	bgJobRepo     repository.BackgroundJobRepository
	workspaceRepo repository.WorkspaceRepository
	triggers      *trigger.TriggerService
	logger        zerolog.Logger
}

func NewCronRunner(
	clientRepo repository.ClientRepository,
	flagsRepo repository.FlagsRepository,
	convStateRepo repository.ConversationStateRepository,
	invoiceRepo repository.InvoiceRepository,
	logRepo repository.LogRepository,
	bgJobRepo repository.BackgroundJobRepository,
	workspaceRepo repository.WorkspaceRepository,
	triggers *trigger.TriggerService,
	logger zerolog.Logger,
) CronRunner {
	return &cronRunner{
		clientRepo:    clientRepo,
		flagsRepo:     flagsRepo,
		convStateRepo: convStateRepo,
		invoiceRepo:   invoiceRepo,
		logRepo:       logRepo,
		bgJobRepo:     bgJobRepo,
		workspaceRepo: workspaceRepo,
		triggers:      triggers,
		logger:        logger,
	}
}

// StartRunAll fetches all non-holding workspaces, creates one background job per workspace,
// and launches a goroutine for each. Returns immediately with the list of created jobs.
func (cr *cronRunner) StartRunAll(ctx context.Context) ([]*entity.BackgroundJob, error) {
	workspaces, err := cr.workspaceRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("cron.StartRunAll: fetch workspaces: %w", err)
	}

	var jobs []*entity.BackgroundJob
	for _, ws := range workspaces {
		if ws.IsHolding {
			continue
		}

		job := &entity.BackgroundJob{
			ID:          uuid.NewString(),
			WorkspaceID: ws.ID,
			JobType:     entity.JobTypeCron,
			Status:      entity.JobStatusPending,
			EntityType:  entity.JobEntityCronRun,
			CreatedBy:   "scheduler",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Metadata: map[string]any{
				"workspace_name": ws.Name,
				"workspace_slug": ws.Slug,
			},
		}

		if err := cr.bgJobRepo.Create(ctx, job); err != nil {
			cr.logger.Error().Err(err).Str("workspace_id", ws.ID).Msg("cron: failed to create job for workspace")
			continue
		}

		go cr.runWorkspaceBackground(job.ID, ws.ID)
		jobs = append(jobs, job)
	}

	cr.logger.Info().Int("total_jobs", len(jobs)).Msg("Cron jobs dispatched")
	return jobs, nil
}

// runWorkspaceBackground processes all clients for a single workspace, tracking progress in the background job.
func (cr *cronRunner) runWorkspaceBackground(jobID, workspaceID string) {
	ctx := context.Background()
	logger := cr.logger.With().Str("job_id", jobID).Str("workspace_id", workspaceID).Logger()

	if err := cr.bgJobRepo.UpdateStatus(ctx, jobID, entity.JobStatusProcessing); err != nil {
		logger.Error().Err(err).Msg("cron: failed to set processing status")
		return
	}

	clients, err := cr.clientRepo.GetAllByWorkspaceIDs(ctx, []string{workspaceID})
	if err != nil {
		logger.Error().Err(err).Msg("cron: failed to fetch clients")
		_ = cr.bgJobRepo.UpdateStatus(ctx, jobID, entity.JobStatusFailed)
		return
	}

	totalRows := len(clients)
	_ = cr.bgJobRepo.UpdateProgress(ctx, jobID, totalRows, 0, 0, 0, 0, nil)

	logger.Info().Int("total_clients", totalRows).Msg("Cron workspace run started")

	var jobErrs []entity.JobRowError
	success, failed, skipped := 0, 0, 0

	for i, c := range clients {
		processed := i + 1

		if err := cr.processClient(ctx, c); err != nil {
			logger.Error().Err(err).Str("company_id", c.CompanyID).Msg("Error processing client")
			jobErrs = append(jobErrs, entity.JobRowError{RefID: c.CompanyID, Reason: err.Error()})
			failed++
		} else {
			success++
		}

		if processed%10 == 0 {
			_ = cr.bgJobRepo.UpdateProgress(ctx, jobID, totalRows, processed, success, failed, skipped, jobErrs)
		}

		time.Sleep(300 * time.Millisecond)
	}

	finalStatus := entity.JobStatusDone
	if failed > 0 && success == 0 {
		finalStatus = entity.JobStatusFailed
	}

	_ = cr.bgJobRepo.UpdateProgress(ctx, jobID, totalRows, totalRows, success, failed, skipped, jobErrs)
	if err := cr.bgJobRepo.UpdateStatus(ctx, jobID, finalStatus); err != nil {
		logger.Error().Err(err).Msg("cron: failed to set final status")
	}

	logger.Info().
		Str("status", finalStatus).
		Int("success", success).
		Int("failed", failed).
		Msg("Cron workspace run completed")
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

	f, err := cr.flagsRepo.GetByCompanyID(ctx, c.CompanyID)
	if err != nil {
		cr.logger.Error().Err(err).Str("company_id", c.CompanyID).Msg("Failed to get flags")
		return err
	}

	// Check if renewed — triggers resetCycleFlags before evaluation
	if c.Renewed {
		if resetErr := cr.flagsRepo.ResetCycleFlags(ctx, c.CompanyID); resetErr != nil {
			cr.logger.Error().Err(resetErr).Str("company_id", c.CompanyID).Msg("Failed to reset cycle flags")
			return resetErr
		}
		// Re-fetch flags after reset
		f, err = cr.flagsRepo.GetByCompanyID(ctx, c.CompanyID)
		if err != nil {
			cr.logger.Error().Err(err).Str("company_id", c.CompanyID).Msg("Failed to get flags after reset")
			return err
		}
	}

	inv, err := cr.invoiceRepo.GetActiveByCompanyID(ctx, c.CompanyID)
	if err != nil {
		cr.logger.Warn().Err(err).Str("company_id", c.CompanyID).Msg("Failed to get active invoice")
	}

	convState, err := cr.convStateRepo.GetByCompanyID(ctx, c.CompanyID)
	if err != nil {
		cr.logger.Warn().Err(err).Str("company_id", c.CompanyID).Msg("Failed to get conversation state")
		// Continue with default state
		convState = &entity.ConversationState{BotActive: true}
	}

	// Strict priority order. First match fires and returns — no further evaluation.
	if sent, err := cr.triggers.EvalHealthRisk(ctx, c, *f); err != nil {
		cr.logger.Error().Err(err).Str("company_id", c.CompanyID).Msg("Health risk trigger error")
		return err
	} else if sent {
		cr.logger.Info().Str("company_id", c.CompanyID).Msg("Health risk trigger fired")
		return nil
	}
	if sent, err := cr.triggers.EvalCheckIn(ctx, c, *f); err != nil {
		cr.logger.Error().Err(err).Str("company_id", c.CompanyID).Msg("Check-in trigger error")
		return err
	} else if sent {
		cr.logger.Info().Str("company_id", c.CompanyID).Msg("Check-in trigger fired")
		return nil
	}
	if sent, err := cr.triggers.EvalNegotiation(ctx, c, *f); err != nil {
		cr.logger.Error().Err(err).Str("company_id", c.CompanyID).Msg("Negotiation trigger error")
		return err
	} else if sent {
		cr.logger.Info().Str("company_id", c.CompanyID).Msg("Negotiation trigger fired")
		return nil
	}
	if sent, err := cr.triggers.EvalInvoice(ctx, c, *f, inv, convState); err != nil {
		cr.logger.Error().Err(err).Str("company_id", c.CompanyID).Msg("Invoice trigger error")
		return err
	} else if sent {
		cr.logger.Info().Str("company_id", c.CompanyID).Msg("Invoice trigger fired")
		return nil
	}
	if sent, err := cr.triggers.EvalOverdue(ctx, c, *f, inv, convState); err != nil {
		cr.logger.Error().Err(err).Str("company_id", c.CompanyID).Msg("Overdue trigger error")
		return err
	} else if sent {
		cr.logger.Info().Str("company_id", c.CompanyID).Msg("Overdue trigger fired")
		return nil
	}
	if sent, err := cr.triggers.EvalExpansion(ctx, c, *f); err != nil {
		cr.logger.Error().Err(err).Str("company_id", c.CompanyID).Msg("Expansion trigger error")
		return err
	} else if sent {
		cr.logger.Info().Str("company_id", c.CompanyID).Msg("Expansion trigger fired")
		return nil
	}
	if sent, err := cr.triggers.EvalCrossSell(ctx, c, *f); err != nil {
		cr.logger.Error().Err(err).Str("company_id", c.CompanyID).Msg("Cross-sell trigger error")
		return err
	} else if sent {
		cr.logger.Info().Str("company_id", c.CompanyID).Msg("Cross-sell trigger fired")
		return nil
	}

	return nil
}
