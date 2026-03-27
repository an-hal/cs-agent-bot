package escalation

import (
	"context"
	"fmt"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/telegram"
	"github.com/rs/zerolog"
)

type EscalationHandler interface {
	TriggerEscalation(ctx context.Context, escID string, client entity.Client, reason string, priority string) error
}

type escalationHandler struct {
	flagsRepo      repository.FlagsRepository
	logRepo        repository.LogRepository
	escalationRepo repository.EscalationRepository
	telegram       telegram.TelegramNotifier
	logger         zerolog.Logger
}

func NewEscalationHandler(
	flagsRepo repository.FlagsRepository,
	logRepo repository.LogRepository,
	escalationRepo repository.EscalationRepository,
	telegramNotifier telegram.TelegramNotifier,
	logger zerolog.Logger,
) EscalationHandler {
	return &escalationHandler{
		flagsRepo:      flagsRepo,
		logRepo:        logRepo,
		escalationRepo: escalationRepo,
		telegram:       telegramNotifier,
		logger:         logger,
	}
}

func (h *escalationHandler) TriggerEscalation(ctx context.Context, escID string, client entity.Client, reason string, priority string) error {
	// Check for existing Open escalation — never create duplicates
	existing, err := h.escalationRepo.GetOpenByCompanyAndEscID(ctx, client.CompanyID, escID)
	if err != nil {
		return fmt.Errorf("failed to check existing escalation: %w", err)
	}

	if existing != nil {
		// Send reminder instead of creating new row
		msg := fmt.Sprintf("Reminder: %s escalation for %s (%s) is still open.\nReason: %s",
			escID, client.CompanyName, client.CompanyID, reason)
		return h.telegram.SendMessage(ctx, client.OwnerTelegramID, msg)
	}

	esc := entity.Escalation{
		CompanyID: client.CompanyID,
		EscID:     escID,
		Status:    entity.EscalationStatusOpen,
		Priority:  priority,
		Reason:    reason,
	}

	// Step 1: Set BotActive = FALSE
	if err := h.flagsRepo.SetBotActive(ctx, client.CompanyID, false); err != nil {
		return fmt.Errorf("escalation step 1 (set bot_active=false) failed: %w", err)
	}

	// Step 2: Append to Action Log
	logEntry := entity.ActionLog{
		CompanyID:   client.CompanyID,
		TriggerType: "ESCALATED_" + escID,
		Channel:     entity.ChannelTelegram,
		Details:     reason,
	}
	if err := h.logRepo.AppendLog(ctx, logEntry); err != nil {
		return fmt.Errorf("escalation step 2 (append log) failed: %w", err)
	}

	// Step 3: Send Telegram to AE
	formatted := h.telegram.FormatEscalation(esc, client)
	if err := h.telegram.SendMessage(ctx, client.OwnerTelegramID, formatted); err != nil {
		return fmt.Errorf("escalation step 3 (telegram) failed: %w", err)
	}

	// Step 4: Write escalation row
	if err := h.escalationRepo.OpenEscalation(ctx, esc); err != nil {
		return fmt.Errorf("escalation step 4 (open escalation) failed: %w", err)
	}

	h.logger.Info().
		Str("esc_id", escID).
		Str("company_id", client.CompanyID).
		Str("priority", priority).
		Msg("Escalation triggered")

	return nil
}
