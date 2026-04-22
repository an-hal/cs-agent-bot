package cron

import (
	"context"
	"fmt"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/rs/zerolog"
)

// WorkflowActionDispatcher executes an automation rule's action for a given
// master_data record. Implementations handle channel-specific dispatch
// (WhatsApp, Email, Telegram, etc.).
type WorkflowActionDispatcher interface {
	Dispatch(ctx context.Context, rule entity.AutomationRule, md entity.MasterData) error
}

// channelDispatcher dispatches by rule channel.
type channelDispatcher struct {
	stageHandler StageTransitionHandler
	logger       zerolog.Logger
}

// NewChannelDispatcher creates a WorkflowActionDispatcher.
func NewChannelDispatcher(stageHandler StageTransitionHandler, logger zerolog.Logger) WorkflowActionDispatcher {
	return &channelDispatcher{stageHandler: stageHandler, logger: logger}
}

func (d *channelDispatcher) Dispatch(ctx context.Context, rule entity.AutomationRule, md entity.MasterData) error {
	switch rule.Channel {
	case entity.RuleChannelWhatsApp:
		d.logger.Info().
			Str("trigger_id", rule.TriggerID).
			Str("template_id", stringPtrOr(rule.TemplateID, "")).
			Str("company_id", md.CompanyID).
			Msg("dispatching WhatsApp action")
		// TODO: wire HaloAI client for WA send. For now, log only.
		return nil

	case entity.RuleChannelEmail:
		d.logger.Info().
			Str("trigger_id", rule.TriggerID).
			Str("company_id", md.CompanyID).
			Msg("dispatching email action")
		return nil

	case entity.RuleChannelTelegram:
		d.logger.Info().
			Str("trigger_id", rule.TriggerID).
			Str("company_id", md.CompanyID).
			Msg("dispatching telegram action")
		return nil

	case "escalate":
		d.logger.Info().
			Str("trigger_id", rule.TriggerID).
			Str("company_id", md.CompanyID).
			Msg("dispatching escalation")
		return nil

	case "alert":
		d.logger.Info().
			Str("trigger_id", rule.TriggerID).
			Str("company_id", md.CompanyID).
			Msg("dispatching alert")
		return nil

	case "skip_and_set_flag":
		// No-op action body — the sent flag write happens in tryNode.
		d.logger.Info().
			Str("trigger_id", rule.TriggerID).
			Str("company_id", md.CompanyID).
			Msg("skip_and_set_flag — flag written by tryNode")
		return nil

	case "handoff":
		if d.stageHandler == nil {
			return fmt.Errorf("stage handler not configured for handoff: %s", rule.TriggerID)
		}
		return d.stageHandler.HandleTransition(ctx, rule.TriggerID, &md)

	default:
		return fmt.Errorf("unknown channel: %s for rule %s", rule.Channel, rule.RuleCode)
	}
}

func stringPtrOr(s *string, fallback string) string {
	if s == nil {
		return fallback
	}
	return *s
}
