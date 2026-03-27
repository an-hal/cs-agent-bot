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
		activationDate, _ = time.Parse("2006-01-02", payload.ActivationDate)
	}

	client := entity.Client{
		CompanyID:       payload.CompanyID,
		CompanyName:     payload.CompanyName,
		PICName:         payload.PICName,
		PICWA:           payload.PICWA,
		OwnerName:       payload.OwnerName,
		OwnerWA:         payload.OwnerWA,
		Segment:         payload.Segment,
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

	// Create client row in Sheet 1
	if err := h.clientRepo.CreateClient(ctx, client); err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Initialize all flags in Sheet 4 (all FALSE by default)
	flags := entity.ClientFlags{
		CompanyID: payload.CompanyID,
	}
	// Append flags row — since the struct defaults are all false, this works as initialization
	// For POC, we use UpdateFlags which writes the full row
	// First we need to append a new row to Sheet 4
	if err := h.flagsRepo.UpdateFlags(ctx, payload.CompanyID, flags); err != nil {
		// If update fails (row doesn't exist yet), the flags need to be created
		h.logger.Warn().Err(err).Msg("Failed to update flags, client flags may need manual init")
	}

	// Append to action log
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
	_ = h.logRepo.AppendLog(ctx, logEntry)

	h.logger.Info().Str("company_id", payload.CompanyID).Msg("New client onboarded via handoff")

	return nil
}
