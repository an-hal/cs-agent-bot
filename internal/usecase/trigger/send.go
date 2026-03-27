package trigger

import (
	"context"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/template"
)

// sendMessage is the shared send helper for all triggers.
// It resolves a template, sends via HaloAI, logs the action, and sets the flag.
func (t *TriggerService) sendMessage(ctx context.Context, templateID string, triggerType string, client entity.Client, invoice *entity.Invoice) error {
	cfg := template.TemplateConfig{
		SurveyPlatformURL: t.Cfg.SurveyPlatformURL,
		CheckinFormURL:    t.Cfg.CheckinFormURL,
		ReferralBenefit:   t.Cfg.ReferralBenefit,
	}

	body, err := t.TemplateResolver.ResolveTemplate(ctx, templateID, client, invoice, cfg)
	if err != nil {
		// Alert AE if template resolution fails
		alertMsg := fmt.Sprintf("Template resolution failed for %s (%s): %v", client.CompanyName, client.CompanyID, err)
		_ = t.Telegram.SendMessage(ctx, client.OwnerTelegramID, alertMsg)
		return err
	}

	_, err = t.HaloAI.SendWA(ctx, client.PICWA, body)
	if err != nil {
		return fmt.Errorf("failed to send WA for %s: %w", triggerType, err)
	}

	// Append to action log
	logEntry := entity.ActionLog{
		Timestamp:   time.Now(),
		CompanyID:   client.CompanyID,
		CompanyName: client.CompanyName,
		TriggerType: triggerType,
		TemplateID:  templateID,
		Channel:     entity.ChannelWhatsApp,
		Details:     templateID,
	}
	if err := t.LogRepo.AppendLog(ctx, logEntry); err != nil {
		t.Logger.Error().Err(err).Str("trigger", triggerType).Msg("Failed to append action log")
	}

	return nil
}
