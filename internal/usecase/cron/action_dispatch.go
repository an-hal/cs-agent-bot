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

// WASender is the narrow outbound-WA port the cron dispatcher calls. Real
// impls: haloai.Client (production); haloaimock.Sender (mock/dev). Nil is
// legal — dispatcher logs + no-ops.
type WASender interface {
	Send(ctx context.Context, req WASendRequest) (messageID string, err error)
}

// WASendRequest is a thin adapter shape the dispatcher produces. Concrete
// adapters in main.go convert to their SDK's request type.
type WASendRequest struct {
	WorkspaceID string
	To          string
	TemplateID  string
	Body        string
	Variables   map[string]string
}

// channelDispatcher dispatches by rule channel.
type channelDispatcher struct {
	stageHandler  StageTransitionHandler
	manualEnqueue ManualActionEnqueuer // optional — nil = log+skip manual flows
	waSender      WASender             // optional — nil = log-only WA
	logger        zerolog.Logger
}

// DispatcherOptions bundles optional dependencies for NewChannelDispatcherWith.
// Nil fields are legal — dispatcher degrades gracefully for each.
type DispatcherOptions struct {
	ManualEnqueuer ManualActionEnqueuer
	WASender       WASender
}

// NewChannelDispatcher creates a WorkflowActionDispatcher.
func NewChannelDispatcher(stageHandler StageTransitionHandler, logger zerolog.Logger) WorkflowActionDispatcher {
	return &channelDispatcher{stageHandler: stageHandler, logger: logger}
}

// NewChannelDispatcherWithManual is like NewChannelDispatcher but attaches a
// manual_action_queue enqueuer. When `enq` is non-nil, trigger_ids listed in
// ManualFlowTriggers are routed to the queue instead of being sent by the
// bot — matches feat/06 §"Manual-Flow Overlay (GUARD)".
func NewChannelDispatcherWithManual(stageHandler StageTransitionHandler, enq ManualActionEnqueuer, logger zerolog.Logger) WorkflowActionDispatcher {
	return &channelDispatcher{stageHandler: stageHandler, manualEnqueue: enq, logger: logger}
}

// NewChannelDispatcherWith accepts an options struct — preferred constructor
// when multiple optional deps need wiring (manual queue + WA sender).
func NewChannelDispatcherWith(stageHandler StageTransitionHandler, opts DispatcherOptions, logger zerolog.Logger) WorkflowActionDispatcher {
	return &channelDispatcher{
		stageHandler:  stageHandler,
		manualEnqueue: opts.ManualEnqueuer,
		waSender:      opts.WASender,
		logger:        logger,
	}
}

func (d *channelDispatcher) Dispatch(ctx context.Context, rule entity.AutomationRule, md entity.MasterData) error {
	// Low-intent shortening: skip courtesy BD nudges on cold prospects so we
	// don't spam clients who have clearly disengaged (feat/06 §cron-engine).
	if IsLowIntentSkip(rule, md) {
		d.logger.Info().
			Str("trigger_id", rule.TriggerID).
			Str("company_id", md.CompanyID).
			Msg("skipping low-intent BD trigger (cold / buying_intent=low)")
		return nil
	}

	// GUARD: intercept manual-flow trigger_ids BEFORE channel routing. These
	// are flows that must be composed + sent by a human; the bot only enqueues
	// a reminder.
	if IsManualFlow(rule.TriggerID) {
		if d.manualEnqueue == nil {
			logManualFlowSkip(d.logger, rule, md)
			return nil
		}
		in := buildManualActionInput(rule, md, "", map[string]any{
			"company_id":   md.CompanyID,
			"company_name": md.CompanyName,
			"trigger_id":   rule.TriggerID,
			"rule_code":    rule.RuleCode,
		})
		if err := d.manualEnqueue.Enqueue(ctx, in); err != nil {
			d.logger.Warn().Err(err).Str("trigger_id", rule.TriggerID).
				Msg("manual-flow enqueue failed — continuing")
		}
		return nil
	}

	switch rule.Channel {
	case entity.RuleChannelWhatsApp:
		if d.waSender == nil {
			d.logger.Info().
				Str("trigger_id", rule.TriggerID).
				Str("template_id", stringPtrOr(rule.TemplateID, "")).
				Str("company_id", md.CompanyID).
				Msg("dispatching WhatsApp action — no sender wired, logging only")
			return nil
		}
		msgID, err := d.waSender.Send(ctx, WASendRequest{
			WorkspaceID: md.WorkspaceID,
			To:          md.PICWA,
			TemplateID:  stringPtrOr(rule.TemplateID, ""),
			Body:        "", // populated upstream when template resolver runs; mock sender accepts empty
		})
		if err != nil {
			d.logger.Warn().Err(err).Str("trigger_id", rule.TriggerID).
				Str("company_id", md.CompanyID).Msg("WA send failed")
			return err
		}
		d.logger.Info().
			Str("trigger_id", rule.TriggerID).
			Str("template_id", stringPtrOr(rule.TemplateID, "")).
			Str("company_id", md.CompanyID).
			Str("wa_message_id", msgID).
			Msg("WA sent")
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
