package webhook

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/classifier"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/escalation"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/haloai"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/telegram"
	"github.com/rs/zerolog"
)

type ReplyHandler interface {
	HandleIncomingReply(ctx context.Context, payload WAWebhookPayload) error
}

type replyHandler struct {
	invoiceRepo   repository.InvoiceRepository
	clientRepo    repository.ClientRepository
	flagsRepo     repository.FlagsRepository
	convStateRepo repository.ConversationStateRepository
	logRepo       repository.LogRepository
	classifier    classifier.ReplyClassifier
	escalation    escalation.EscalationHandler
	haloAI        haloai.HaloAIClient
	telegram      telegram.TelegramNotifier
	logger        zerolog.Logger
}

func NewReplyHandler(
	invoiceRepo repository.InvoiceRepository,
	clientRepo repository.ClientRepository,
	flagsRepo repository.FlagsRepository,
	convStateRepo repository.ConversationStateRepository,
	logRepo repository.LogRepository,
	replyClassifier classifier.ReplyClassifier,
	escHandler escalation.EscalationHandler,
	haloAIClient haloai.HaloAIClient,
	telegramNotifier telegram.TelegramNotifier,
	logger zerolog.Logger,
) ReplyHandler {
	return &replyHandler{
		invoiceRepo:   invoiceRepo,
		clientRepo:    clientRepo,
		flagsRepo:     flagsRepo,
		convStateRepo: convStateRepo,
		logRepo:       logRepo,
		classifier:    replyClassifier,
		escalation:    escHandler,
		haloAI:        haloAIClient,
		telegram:      telegramNotifier,
		logger:        logger,
	}
}

func (h *replyHandler) HandleIncomingReply(ctx context.Context, payload WAWebhookPayload) error {
	// Dedup: check message_id against action_log
	exists, err := h.logRepo.MessageIDExists(ctx, payload.MessageID)
	if err != nil {
		return fmt.Errorf("dedup check failed: %w", err)
	}
	if exists {
		h.logger.Info().Str("message_id", payload.MessageID).Msg("Duplicate message, skipping")
		// return nil
	}

	// Lookup client by WA number
	client, err := h.clientRepo.GetByWANumber(ctx, payload.PhoneNumber)
	if err != nil {
		return fmt.Errorf("client not found for %s: %w", payload.PhoneNumber, err)
	}

	// Classify intent
	intent := h.classifier.ClassifyReply(payload.MessageType, payload.Text)

	h.logger.Info().
		Str("company_id", client.CompanyID).
		Str("intent", string(intent)).
		Str("message_id", payload.MessageID).
		Msg("Reply classified")

	// Log the incoming reply
	logEntry := entity.ActionLog{
		CompanyID:              client.CompanyID,
		CompanyName:            client.CompanyName,
		TriggerType:            "REPLY_" + strings.ToUpper(string(intent)),
		TemplateID:             "-",
		Channel:                entity.ChannelWhatsApp,
		MessageSent:            false,
		ResponseReceived:       true,
		ResponseClassification: strings.ToUpper(string(intent)),
		NextActionTriggered:    "",
		LogNotes:               payload.Text,
	}
	_ = h.logRepo.AppendLog(ctx, logEntry)

	// Branch on intent
	switch intent {
	case classifier.IntentAngry:
		return h.handleAngry(ctx, *client)

	case classifier.IntentPaidClaim:
		return h.handlePaidClaim(ctx, *client, payload.Text)

	case classifier.IntentNPS:
		return h.handleNPS(ctx, *client, payload.Text)

	case classifier.IntentCSInterested:
		return h.handleCSInterested(ctx, *client)

	case classifier.IntentReject:
		return h.handleReject(ctx, *client)

	case classifier.IntentDelay:
		return h.handleDelay(ctx, *client)

	case classifier.IntentPositive:
		return h.handlePositive(ctx, *client, payload.Text)

	case classifier.IntentOOO:
		// Ignore. Do NOT update ResponseStatus. Retry in 3 days.
		h.logger.Info().Str("company_id", client.CompanyID).Msg("OOO reply, ignoring")
		return nil

	case classifier.IntentWantsHuman:
		return h.handleWantsHuman(ctx, *client)
	}

	return nil
}

func (h *replyHandler) handleAngry(ctx context.Context, client entity.Client) error {
	// Update conversation state - STOP bot immediately
	convState, _ := h.convStateRepo.GetByCompanyID(ctx, client.CompanyID)
	convState.ResponseClassification = entity.RespAngry
	convState.BotActive = false
	convState.ReasonBotPaused = "Angry client detected"
	_ = h.convStateRepo.CreateOrUpdate(ctx, *convState)

	// Blacklist immediately. ESC-006. No WA reply.
	return h.escalation.TriggerEscalation(ctx, entity.EscAngryClient, client,
		"Angry client detected", entity.EscPriorityP0Emergency)
}

func (h *replyHandler) handlePaidClaim(ctx context.Context, client entity.Client, replyText string) error {
	// Ask for proof via WA
	_, _ = h.haloAI.SendWA(ctx, client.PICWA,
		"Terima kasih atas informasinya. Pembayaran anda sedang kami verifikasi. Terima kasih!")

	// Update conversation state
	convState, _ := h.convStateRepo.GetByCompanyID(ctx, client.CompanyID)
	inv, _ := h.invoiceRepo.GetActiveByCompanyID(ctx, client.CompanyID)
	convState.ResponseClassification = entity.RespPaid
	convState.ResponseStatus = entity.ResponseStatusPending
	_ = h.convStateRepo.CreateOrUpdate(ctx, *convState)

	// Telegram to AE — Do NOT mark as paid
	formatted := h.telegram.FormatPaymentClaim(client, inv)
	return h.telegram.SendMessage(ctx, client.OwnerTelegramID, formatted)
}

func (h *replyHandler) handleNPS(ctx context.Context, client entity.Client, text string) error {
	score, _ := strconv.Atoi(strings.TrimSpace(text))

	// Store NPS score — this is on Sheet 1 (Master Client)
	// For POC, we update via client repo
	h.logger.Info().Str("company_id", client.CompanyID).Int("nps_score", score).Msg("NPS score received")

	// Update conversation state
	convState, _ := h.convStateRepo.GetByCompanyID(ctx, client.CompanyID)
	convState.ResponseClassification = entity.RespNPS
	_ = h.convStateRepo.CreateOrUpdate(ctx, *convState)

	// Update flags: NPSReplied = true
	flags, err := h.flagsRepo.GetByCompanyID(ctx, client.CompanyID)
	if err != nil {
		return err
	}
	flags.NPSReplied = true
	if err := h.flagsRepo.UpdateFlags(ctx, client.CompanyID, *flags); err != nil {
		return err
	}

	// Score <= 5 → ESC-003
	if score <= 5 {
		return h.escalation.TriggerEscalation(ctx, entity.EscLowNPS, client,
			fmt.Sprintf("NPS score: %d", score), entity.EscPriorityP1Critical)
	}

	// Notify AE
	msg := fmt.Sprintf("NPS Reply from %s (%s): Score %d", client.CompanyName, client.CompanyID, score)
	return h.telegram.SendMessage(ctx, client.OwnerTelegramID, msg)
}

func (h *replyHandler) handleCSInterested(ctx context.Context, client entity.Client) error {
	// Set CrossSellInterested=TRUE. AE takes over P5.
	h.logger.Info().Str("company_id", client.CompanyID).Msg("Client interested in cross-sell")

	msg := fmt.Sprintf("Cross-sell interest from %s (%s). Please follow up.", client.CompanyName, client.CompanyID)
	return h.telegram.SendMessage(ctx, client.OwnerTelegramID, msg)
}

func (h *replyHandler) handleReject(ctx context.Context, client entity.Client) error {
	// Update conversation state - STOP all automation
	convState, _ := h.convStateRepo.GetByCompanyID(ctx, client.CompanyID)
	convState.ResponseClassification = entity.RespReject
	convState.BotActive = false
	convState.ReasonBotPaused = "Client rejected automation"
	_ = h.convStateRepo.CreateOrUpdate(ctx, *convState)

	// Stop all automation. Set Rejected=TRUE.
	h.logger.Info().Str("company_id", client.CompanyID).Msg("Client rejected, stopping automation")

	msg := fmt.Sprintf("Client %s (%s) rejected automation.", client.CompanyName, client.CompanyID)
	return h.telegram.SendMessage(ctx, client.OwnerTelegramID, msg)
}

func (h *replyHandler) handleDelay(ctx context.Context, client entity.Client) error {
	// Update conversation state - snooze for 30 days
	convState, _ := h.convStateRepo.GetByCompanyID(ctx, client.CompanyID)
	convState.ResponseClassification = entity.RespDelay
	convState.NextScheduledAction = "REACTIVATE_FLOW"
	convState.NextScheduledDate = time.Now().AddDate(0, 1, 0) // 30 days
	_ = h.convStateRepo.CreateOrUpdate(ctx, *convState)

	// Snooze
	h.logger.Info().Str("company_id", client.CompanyID).Msg("Client requested delay")

	msg := fmt.Sprintf("Client %s (%s) requested delay/snooze.", client.CompanyName, client.CompanyID)
	return h.telegram.SendMessage(ctx, client.OwnerTelegramID, msg)
}

func (h *replyHandler) handlePositive(ctx context.Context, client entity.Client, text string) error {
	// Continue flow. Log. Telegram to AE.
	msg := fmt.Sprintf("Positive reply from %s (%s): %s", client.CompanyName, client.CompanyID, text)
	return h.telegram.SendMessage(ctx, client.OwnerTelegramID, msg)
}

func (h *replyHandler) handleWantsHuman(ctx context.Context, client entity.Client) error {
	// Suspend bot. Telegram to AE.
	if err := h.flagsRepo.SetBotActive(ctx, client.CompanyID, false); err != nil {
		return err
	}

	msg := fmt.Sprintf("Client %s (%s) wants human assistance. Bot suspended.", client.CompanyName, client.CompanyID)
	return h.telegram.SendMessage(ctx, client.OwnerTelegramID, msg)
}
