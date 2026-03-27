package webhook

import (
	"context"
	"fmt"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/telegram"
	"github.com/rs/zerolog"
)

type CheckinFormHandler interface {
	HandleCheckinForm(ctx context.Context, companyID string) error
}

type checkinFormHandler struct {
	clientRepo repository.ClientRepository
	flagsRepo  repository.FlagsRepository
	logRepo    repository.LogRepository
	telegram   telegram.TelegramNotifier
	logger     zerolog.Logger
}

func NewCheckinFormHandler(
	clientRepo repository.ClientRepository,
	flagsRepo repository.FlagsRepository,
	logRepo repository.LogRepository,
	telegramNotifier telegram.TelegramNotifier,
	logger zerolog.Logger,
) CheckinFormHandler {
	return &checkinFormHandler{
		clientRepo: clientRepo,
		flagsRepo:  flagsRepo,
		logRepo:    logRepo,
		telegram:   telegramNotifier,
		logger:     logger,
	}
}

func (h *checkinFormHandler) HandleCheckinForm(ctx context.Context, companyID string) error {
	// Get client
	client, err := h.clientRepo.GetByID(ctx, companyID)
	if err != nil {
		return fmt.Errorf("client not found: %s: %w", companyID, err)
	}

	// Set CheckinReplied=TRUE on flags
	flags, err := h.flagsRepo.GetByCompanyID(ctx, companyID)
	if err != nil {
		return fmt.Errorf("flags not found: %s: %w", companyID, err)
	}
	flags.CheckinReplied = true
	if err := h.flagsRepo.UpdateFlags(ctx, companyID, *flags); err != nil {
		return err
	}

	// Append to action log
	logEntry := entity.ActionLog{
		CompanyID:              companyID,
		TriggerType:            "CHECKIN_FORM_REPLIED",
		Channel:                entity.ChannelWhatsApp,
		MessageSent:            false,
		ResponseReceived:       true,
		ResponseClassification: "CHECKIN",
		NextActionTriggered:    "",
		LogNotes:               "Check-in form submitted",
	}
	if err := h.logRepo.AppendLog(ctx, logEntry); err != nil {
		h.logger.Error().Err(err).Msg("Failed to append checkin log")
	}

	// Notify AE via Telegram
	msg := fmt.Sprintf("Check-in form submitted by %s (%s).", client.CompanyName, client.CompanyID)
	return h.telegram.SendMessage(ctx, client.OwnerTelegramID, msg)
}
