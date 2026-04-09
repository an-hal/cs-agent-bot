package trigger

import (
	"context"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/config"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/escalation"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/haloai"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/telegram"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/template"
	"github.com/rs/zerolog"
)

// ActionExecutor dispatches actions for matched trigger rules.
type ActionExecutor struct {
	clientRepo       repository.ClientRepository
	invoiceRepo      repository.InvoiceRepository
	flagsRepo        repository.FlagsRepository
	convStateRepo    repository.ConversationStateRepository
	logRepo          repository.LogRepository
	configRepo       repository.ConfigRepository
	templateResolver template.TemplateResolver
	haloAI           haloai.HaloAIClient
	telegram         telegram.TelegramNotifier
	escalation       escalation.EscalationHandler
	cfg              *config.AppConfig
	logger           zerolog.Logger
}

// NewActionExecutor creates a new action executor with all required dependencies.
func NewActionExecutor(
	clientRepo repository.ClientRepository,
	invoiceRepo repository.InvoiceRepository,
	flagsRepo repository.FlagsRepository,
	convStateRepo repository.ConversationStateRepository,
	logRepo repository.LogRepository,
	configRepo repository.ConfigRepository,
	templateResolver template.TemplateResolver,
	haloAI haloai.HaloAIClient,
	telegramNotifier telegram.TelegramNotifier,
	escalationHandler escalation.EscalationHandler,
	cfg *config.AppConfig,
	logger zerolog.Logger,
) *ActionExecutor {
	return &ActionExecutor{
		clientRepo:       clientRepo,
		invoiceRepo:      invoiceRepo,
		flagsRepo:        flagsRepo,
		convStateRepo:    convStateRepo,
		logRepo:          logRepo,
		configRepo:       configRepo,
		templateResolver: templateResolver,
		haloAI:           haloAI,
		telegram:         telegramNotifier,
		escalation:       escalationHandler,
		cfg:              cfg,
		logger:           logger,
	}
}

// Execute dispatches the action for a matched trigger rule.
func (e *ActionExecutor) Execute(ctx context.Context, rule entity.TriggerRule, clientCtx *ClientContext) error {
	switch rule.ActionType {
	case entity.ActionSendWA:
		return e.executeSendWA(ctx, rule, clientCtx)
	case entity.ActionEscalate:
		return e.executeEscalate(ctx, rule, clientCtx)
	case entity.ActionAlertTelegram:
		return e.executeAlertTelegram(ctx, rule, clientCtx)
	case entity.ActionCreateInvoice:
		return e.executeCreateInvoice(ctx, rule, clientCtx)
	case entity.ActionSkipAndSetFlag:
		return e.executeSkipAndSetFlag(ctx, rule, clientCtx)
	default:
		return fmt.Errorf("unknown action type: %s", rule.ActionType)
	}
}

func (e *ActionExecutor) executeSendWA(ctx context.Context, rule entity.TriggerRule, clientCtx *ClientContext) error {
	if rule.TemplateID == nil {
		return fmt.Errorf("rule %s: send_wa requires template_id", rule.RuleID)
	}

	cfg := template.TemplateConfig{
		SurveyPlatformURL: e.cfg.SurveyPlatformURL,
		CheckinFormURL:    e.cfg.CheckinFormURL,
		ReferralBenefit:   e.cfg.ReferralBenefit,
	}

	body, err := e.templateResolver.ResolveTemplate(ctx, *rule.TemplateID, clientCtx.Client, clientCtx.Invoice, cfg)
	if err != nil {
		alertMsg := fmt.Sprintf("Template resolution failed for %s (%s): %v",
			clientCtx.Client.CompanyName, clientCtx.Client.CompanyID, err)
		if alertErr := e.telegram.SendMessage(ctx, e.cfg.TelegramAELeadID, alertMsg); alertErr != nil {
			e.logger.Error().Err(alertErr).Msg("Failed to send template failure alert")
		}
		return fmt.Errorf("resolve template %s: %w", *rule.TemplateID, err)
	}

	if _, err := e.haloAI.SendWA(ctx, clientCtx.Client.PICWA, body); err != nil {
		return fmt.Errorf("send WA for rule %s: %w", rule.RuleID, err)
	}

	// Log action
	logEntry := entity.ActionLog{
		Timestamp:   time.Now(),
		CompanyID:   clientCtx.Client.CompanyID,
		CompanyName: clientCtx.Client.CompanyName,
		TriggerType: rule.FlagKey,
		TemplateID:  *rule.TemplateID,
		Channel:     entity.ChannelWhatsApp,
		MessageSent: true,
		LogNotes:    fmt.Sprintf("Rule: %s · Template: %s", rule.RuleID, *rule.TemplateID),
	}
	if err := e.logRepo.AppendLog(ctx, logEntry); err != nil {
		e.logger.Error().Err(err).Str("rule_id", rule.RuleID).Msg("Failed to append action log")
	}

	// Activity trail
	if err := e.logRepo.AppendActivity(ctx, entity.ActivityLog{
		WorkspaceID: clientCtx.Client.WorkspaceID,
		Category:    entity.ActivityCategoryBot,
		ActorType:   entity.ActivityActorBot,
		Actor:       "bot",
		Action:      rule.FlagKey,
		Target:      clientCtx.Client.CompanyName,
		Detail:      fmt.Sprintf("Rule: %s · Template: %s · Channel: WA", rule.RuleID, *rule.TemplateID),
		RefID:       clientCtx.Client.CompanyID,
		Status:      "delivered",
	}); err != nil {
		e.logger.Error().Err(err).Str("rule_id", rule.RuleID).Msg("Failed to append activity log")
	}

	// Set primary flag
	if err := e.setFlag(ctx, rule, clientCtx); err != nil {
		return err
	}

	// Record conversation state for invoice/overdue rules
	if rule.RuleGroup == entity.RuleGroupInvoice || rule.RuleGroup == entity.RuleGroupOverdue {
		if rule.TemplateID != nil {
			msgType := "INVOICE_REMINDER"
			if rule.RuleGroup == entity.RuleGroupOverdue {
				msgType = "OVERDUE_REMINDER"
			}
			if err := e.convStateRepo.RecordMessage(ctx, clientCtx.Client.CompanyID, msgType, *rule.TemplateID); err != nil {
				e.logger.Error().Err(err).Msg("Failed to record message in conversation state")
			}
		}
	}

	// Update invoice reminder flags for payment/overdue rules
	if rule.RuleGroup == entity.RuleGroupInvoice || rule.RuleGroup == entity.RuleGroupOverdue {
		flagMap := map[string]bool{rule.FlagKey: true}
		if err := e.clientRepo.UpdateInvoiceReminderFlags(ctx, clientCtx.Client.CompanyID, flagMap); err != nil {
			e.logger.Error().Err(err).Str("flag", rule.FlagKey).Msg("Failed to update invoice reminder flags")
		}
	}

	return nil
}

func (e *ActionExecutor) executeEscalate(ctx context.Context, rule entity.TriggerRule, clientCtx *ClientContext) error {
	if rule.EscalationID == nil || rule.EscPriority == nil {
		return fmt.Errorf("rule %s: escalate requires escalation_id and esc_priority", rule.RuleID)
	}

	reason := rule.RuleID
	if rule.EscReason != nil {
		reason = *rule.EscReason
	}

	if err := e.escalation.TriggerEscalation(ctx, *rule.EscalationID, clientCtx.Client, reason, *rule.EscPriority); err != nil {
		e.logger.Error().Err(err).
			Str("rule_id", rule.RuleID).
			Str("esc_id", *rule.EscalationID).
			Msg("Failed to trigger escalation")
		return err
	}

	return e.setFlag(ctx, rule, clientCtx)
}

func (e *ActionExecutor) executeAlertTelegram(ctx context.Context, rule entity.TriggerRule, clientCtx *ClientContext) error {
	desc := rule.RuleID
	if rule.Description != nil {
		desc = *rule.Description
	}

	alertMsg := fmt.Sprintf("[%s] %s for %s (%s)",
		rule.RuleID, desc, clientCtx.Client.CompanyName, clientCtx.Client.CompanyID)

	telegramID := e.cfg.TelegramAELeadID
	if clientCtx.Client.OwnerTelegramID != "" {
		telegramID = clientCtx.Client.OwnerTelegramID
	}

	if err := e.telegram.SendMessage(ctx, telegramID, alertMsg); err != nil {
		e.logger.Error().Err(err).Str("rule_id", rule.RuleID).Msg("Failed to send Telegram alert")
	}

	return e.setFlag(ctx, rule, clientCtx)
}

func (e *ActionExecutor) executeCreateInvoice(ctx context.Context, rule entity.TriggerRule, clientCtx *ClientContext) error {
	if clientCtx.Invoice != nil {
		return nil // Invoice already exists
	}

	if clientCtx.Client.QuotationLink == "" {
		alertMsg := fmt.Sprintf("quotation_link is empty for %s (%s). Invoice creation delayed.",
			clientCtx.Client.CompanyName, clientCtx.Client.CompanyID)
		if err := e.telegram.SendMessage(ctx, clientCtx.Client.OwnerTelegramID, alertMsg); err != nil {
			e.logger.Error().Err(err).Msg("Failed to send quotation link alert")
		}
		return nil
	}

	newInv := entity.Invoice{
		InvoiceID:     fmt.Sprintf("INV-%s-%s", time.Now().Format("2006"), clientCtx.Client.CompanyID),
		CompanyID:     clientCtx.Client.CompanyID,
		DueDate:       clientCtx.Client.ContractEnd,
		PaymentStatus: entity.PaymentStatusPending,
	}

	if err := e.invoiceRepo.CreateInvoice(ctx, newInv); err != nil {
		return fmt.Errorf("create invoice: %w", err)
	}

	clientCtx.Invoice = &newInv
	return e.setFlag(ctx, rule, clientCtx)
}

func (e *ActionExecutor) executeSkipAndSetFlag(ctx context.Context, rule entity.TriggerRule, clientCtx *ClientContext) error {
	return e.setFlag(ctx, rule, clientCtx)
}

// setFlag sets the primary flag and any extra flags defined in the rule.
func (e *ActionExecutor) setFlag(ctx context.Context, rule entity.TriggerRule, clientCtx *ClientContext) error {
	e.setFlagValue(&clientCtx.Flags, rule.FlagKey, true)

	// Set extra flags
	for key, val := range rule.GetExtraFlags() {
		e.setFlagValue(&clientCtx.Flags, key, val)
	}

	return e.flagsRepo.UpdateFlags(ctx, clientCtx.Client.CompanyID, clientCtx.Flags)
}

// setFlagValue sets a flag by name on the ClientFlags struct.
func (e *ActionExecutor) setFlagValue(f *entity.ClientFlags, key string, val bool) {
	switch key {
	case "low_usage_msg_sent":
		f.LowUsageMsgSent = val
	case "low_nps_msg_sent":
		f.LowNPSMsgSent = val
	case "ren60_sent":
		f.Ren60Sent = val
	case "ren45_sent":
		f.Ren45Sent = val
	case "ren30_sent":
		f.Ren30Sent = val
	case "ren15_sent":
		f.Ren15Sent = val
	case "ren0_sent":
		f.Ren0Sent = val
	case "checkin_a1_form_sent":
		f.CheckinA1FormSent = val
	case "checkin_a1_call_sent":
		f.CheckinA1CallSent = val
	case "checkin_a2_form_sent":
		f.CheckinA2FormSent = val
	case "checkin_a2_call_sent":
		f.CheckinA2CallSent = val
	case "checkin_b1_form_sent":
		f.CheckinB1FormSent = val
	case "checkin_b1_call_sent":
		f.CheckinB1CallSent = val
	case "checkin_b2_form_sent":
		f.CheckinB2FormSent = val
	case "checkin_b2_call_sent":
		f.CheckinB2CallSent = val
	case "checkin_replied":
		f.CheckinReplied = val
	case "nps1_sent":
		f.NPS1Sent = val
	case "nps2_sent":
		f.NPS2Sent = val
	case "nps3_sent":
		f.NPS3Sent = val
	case "nps_replied":
		f.NPSReplied = val
	case "referral_sent_this_cycle":
		f.ReferralSentThisCycle = val
	case "cs_h7":
		f.CSH7 = val
	case "cs_h14":
		f.CSH14 = val
	case "cs_h21":
		f.CSH21 = val
	case "cs_h30":
		f.CSH30 = val
	case "cs_h45":
		f.CSH45 = val
	case "cs_h60":
		f.CSH60 = val
	case "cs_h75":
		f.CSH75 = val
	case "cs_h90":
		f.CSH90 = val
	case "cs_lt1":
		f.CSLT1 = val
	case "cs_lt2":
		f.CSLT2 = val
	case "cs_lt3":
		f.CSLT3 = val
	case "feature_update_sent":
		f.FeatureUpdateSent = val
	case "quotation_acknowledged":
		f.QuotationAcknowledged = val
	}
}
