package webhook

import (
	"context"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/rs/zerolog"
)

type HandoffHandler interface {
	HandleNewClient(ctx context.Context, payload NewClientPayload) error
}

type handoffHandler struct {
	clientRepo repository.ClientRepository
	flagsRepo  repository.FlagsRepository
	logRepo    repository.LogRepository
	logger     zerolog.Logger
}

func NewHandoffHandler(
	clientRepo repository.ClientRepository,
	flagsRepo repository.FlagsRepository,
	logRepo repository.LogRepository,
	logger zerolog.Logger,
) HandoffHandler {
	return &handoffHandler{
		clientRepo: clientRepo,
		flagsRepo:  flagsRepo,
		logRepo:    logRepo,
		logger:     logger,
	}
}

func (h *handoffHandler) HandleNewClient(ctx context.Context, payload NewClientPayload) error {
	contractStart, err := time.Parse("2006-01-02", payload.ContractStart)
	if err != nil {
		return fmt.Errorf("invalid contract_start: %w", err)
	}
	contractEnd, err := time.Parse("2006-01-02", payload.ContractEnd)
	if err != nil {
		return fmt.Errorf("invalid contract_end: %w", err)
	}

	var activationDate time.Time
	if payload.ActivationDate != "" {
		activationDate, err = time.Parse("2006-01-02", payload.ActivationDate)
		if err != nil {
			h.logger.Warn().Err(err).Str("activation_date", payload.ActivationDate).Msg("Failed to parse activation_date, using zero value")
		}
	}

	client := entity.Client{
		CompanyID:       payload.CompanyID,
		CompanyName:     payload.CompanyName,
		PICName:         payload.PICName,
		PICWA:           payload.PICWA,
		OwnerName:       payload.OwnerName,
		OwnerWA:         entity.StringPtr(payload.OwnerWA),
		ContractMonths:  payload.ContractMonths,
		ContractStart:   contractStart,
		ContractEnd:     contractEnd,
		ActivationDate:  activationDate,
		PaymentStatus:   entity.PaymentStatusPending,
		BotActive:       true,
		OwnerTelegramID: payload.OwnerTelegramID,
		SequenceCS:      entity.SequenceCSActive,
		ResponseStatus:  entity.ResponseStatusPending,
	}

	if err := h.clientRepo.CreateClient(ctx, client); err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	flags := entity.ClientFlags{
		CompanyID: payload.CompanyID,
	}
	if err := h.flagsRepo.UpdateFlags(ctx, payload.CompanyID, flags); err != nil {
		h.logger.Warn().Err(err).Msg("Failed to update flags, client flags may need manual init")
	}

	logEntry := entity.ActionLog{
		CompanyID:              payload.CompanyID,
		TriggerType:            "NEW_CLIENT_HANDOFF",
		Channel:                entity.ChannelWhatsApp,
		MessageSent:            false,
		ResponseReceived:       false,
		ResponseClassification: "HANDOFF",
		NextActionTriggered:    "ONBOARD",
		LogNotes:               fmt.Sprintf("New client onboarded: %s", payload.CompanyName),
	}
	if err := h.logRepo.AppendLog(ctx, logEntry); err != nil {
		h.logger.Error().Err(err).Str("company_id", payload.CompanyID).Msg("Failed to append handoff log")
	}

	h.logger.Info().Str("company_id", payload.CompanyID).Msg("New client onboarded via handoff")

	return nil
}
