package cron

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/conditiondsl"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/workday"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	automationrule "github.com/Sejutacita/cs-agent-bot/internal/usecase/automation_rule"
	workflowuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/workflow"
	"github.com/rs/zerolog"
)

// Fields the bot is NEVER allowed to write (CLAUDE.md rules 1 & 2).
var botWriteForbidden = map[string]bool{
	"payment_status": true,
	"renewed":        true,
	"rejected":       true,
}

// WorkflowRunner evaluates automation rules for a single master_data record
// using the workflow engine. It runs AFTER the legacy P0–P5 trigger sequence
// when USE_WORKFLOW_ENGINE=true.
type WorkflowRunner struct {
	workflowUC     workflowuc.Usecase
	automationUC   automationrule.Usecase
	conditionEval  *conditiondsl.Evaluator
	actionLogRepo  repository.ActionLogWorkflowRepository
	masterDataRepo repository.MasterDataRepository
	actionDispatch WorkflowActionDispatcher
	stageHandler   StageTransitionHandler
	workdayProv    *workday.Provider
	useWorkflow    bool
	logger         zerolog.Logger
}

// NewWorkflowRunner constructs a WorkflowRunner. If useWorkflow is false, all
// Run calls are no-ops.
func NewWorkflowRunner(
	workflowUC workflowuc.Usecase,
	automationUC automationrule.Usecase,
	conditionEval *conditiondsl.Evaluator,
	actionLogRepo repository.ActionLogWorkflowRepository,
	masterDataRepo repository.MasterDataRepository,
	actionDispatch WorkflowActionDispatcher,
	stageHandler StageTransitionHandler,
	workdayProv *workday.Provider,
	useWorkflow bool,
	logger zerolog.Logger,
) *WorkflowRunner {
	return &WorkflowRunner{
		workflowUC:     workflowUC,
		automationUC:   automationUC,
		conditionEval:  conditionEval,
		actionLogRepo:  actionLogRepo,
		masterDataRepo: masterDataRepo,
		actionDispatch: actionDispatch,
		stageHandler:   stageHandler,
		workdayProv:    workdayProv,
		useWorkflow:    useWorkflow,
		logger:         logger,
	}
}

// RunForRecord evaluates automation rules for a single master_data record.
// Returns true if any rule matched and executed.
func (w *WorkflowRunner) RunForRecord(ctx context.Context, workspaceID string, md entity.MasterData) (bool, error) {
	if !w.useWorkflow {
		return false, nil
	}

	// ── Gate 1: Blacklist ──
	if md.Blacklisted {
		return false, nil
	}

	// ── Gate 2: Bot active + snooze ──
	if !md.BotActive {
		if md.SnoozeUntil != nil && time.Now().After(*md.SnoozeUntil) {
			// Snooze expired — resume bot.
			if w.masterDataRepo != nil {
				_, _ = w.masterDataRepo.Patch(ctx, workspaceID, md.ID, repository.MasterDataPatch{
					BotActive:      boolPtr(true),
					SequenceStatus: strPtr(entity.SeqStatusActive),
				})
			}
			// Fall through — treat as bot-active after snooze expiry.
		} else if w.masterDataRepo != nil {
			// Bot explicitly off and no expired snooze — skip record.
			return false, nil
		}
		// When masterDataRepo is nil (test mode with no deps), fall through.
	}

	// ── Gate 3: Snooze check ──
	if md.SequenceStatus == entity.SeqStatusSnoozed {
		if md.SnoozeUntil != nil && time.Now().Before(*md.SnoozeUntil) {
			return false, nil // still snoozed
		}
		// Snooze expired — resume handled above in bot_active gate.
	}

	// ── Route by stage → workflow ──
	wf, err := w.workflowUC.GetActiveForStage(ctx, workspaceID, md.Stage)
	if err != nil {
		return false, fmt.Errorf("workflowRunner.GetActiveForStage: %w", err)
	}
	if wf == nil {
		return false, nil
	}

	// Fetch all active automation rules for this workflow's role.
	role := extractRoleFromSlug(wf.Slug)
	if role == "" {
		return false, nil
	}

	rules, err := w.automationUC.GetActiveByRole(ctx, workspaceID, entity.RuleRole(role))
	if err != nil {
		return false, fmt.Errorf("workflowRunner.GetActiveByRole: %w", err)
	}

	// ── Evaluate rules in phase order ──
	rec := wrapRecord(&md)
	for _, rule := range rules {
		if !rule.IsExecutable() {
			continue
		}

		matched, err := w.tryNode(ctx, workspaceID, rec, &md, &rule)
		if err != nil {
			w.logger.Error().Err(err).
				Str("rule_code", rule.RuleCode).
				Str("company_id", md.CompanyID).
				Msg("tryNode failed")
			continue
		}
		if matched {
			// ONE action per record per run.
			return true, nil
		}

		// Rate limit between evaluations.
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}

	return false, nil
}

// tryNode evaluates a single automation rule. Returns true if the rule matched
// and its action was executed. Implements the spec's tryNode flow:
// 1. Evaluate condition
// 2. Check stop_if
// 3. Check sent_flag (idempotency)
// 4. Execute action
// 5. Write results (sent flag to custom_fields)
// 6. Log action
func (w *WorkflowRunner) tryNode(ctx context.Context, workspaceID string, rec conditiondsl.Record, md *entity.MasterData, rule *entity.AutomationRule) (bool, error) {
	// 1. Evaluate condition.
	if w.conditionEval != nil {
		pass, err := w.conditionEval.Evaluate(ctx, rule.Condition, rec)
		if err != nil {
			return false, fmt.Errorf("evaluate condition %s: %w", rule.RuleCode, err)
		}
		if !pass {
			return false, nil
		}
	}

	// 2. Check stop_if.
	if rule.StopIf != "" && rule.StopIf != "-" {
		if w.conditionEval != nil {
			stop, err := w.conditionEval.Evaluate(ctx, rule.StopIf, rec)
			if err == nil && stop {
				return false, nil // stop condition met, skip.
			}
		}
	}

	// 3. Check sent_flag (idempotency — prevent duplicate sends).
	if rule.SentFlag != nil && *rule.SentFlag != "" {
		flags := strings.Split(*rule.SentFlag, "\n")
		primaryFlag := strings.TrimSpace(flags[0])
		if primaryFlag != "" {
			if val, ok := md.GetField(primaryFlag); ok && val == "true" {
				return false, nil // already sent
			}
		}
	}

	// 4. Execute action.
	if w.actionDispatch != nil {
		if err := w.actionDispatch.Dispatch(ctx, *rule, *md); err != nil {
			w.logAction(ctx, workspaceID, md.ID, rule.TriggerID, string(rule.Channel), "failed", nil)
			return false, fmt.Errorf("dispatch action %s: %w", rule.TriggerID, err)
		}
	}

	// 5. Write sent flag back to custom_fields.
	if rule.SentFlag != nil && *rule.SentFlag != "" {
		flags := strings.Split(*rule.SentFlag, "\n")
		updates := make(map[string]any, len(flags))
		for _, f := range flags {
			f = strings.TrimSpace(f)
			if f != "" && !botWriteForbidden[f] {
				updates[f] = true
			}
		}
		if len(updates) > 0 {
			if err := w.masterDataRepo.MergeCustomFields(ctx, workspaceID, md.ID, updates); err != nil {
				w.logger.Error().Err(err).Str("company_id", md.CompanyID).Msg("failed to write sent flags")
			}
		}
	}

	// 6. Log action.
	w.logAction(ctx, workspaceID, md.ID, rule.TriggerID, string(rule.Channel), "delivered", nil)

	return true, nil
}

// logAction appends an action log entry.
func (w *WorkflowRunner) logAction(ctx context.Context, workspaceID, masterDataID, triggerID, channel, status string, fieldsWritten map[string]any) {
	if w.actionLogRepo == nil {
		return
	}
	log := &entity.ActionLogWorkflow{
		WorkspaceID:  workspaceID,
		MasterDataID: masterDataID,
		TriggerID:    triggerID,
		Status:       status,
		Channel:      channel,
		Timestamp:    time.Now(),
	}
	if err := w.actionLogRepo.Append(ctx, log); err != nil {
		w.logger.Error().Err(err).Msg("failed to append action log")
	}
}

// IsWorkingDay reports whether today is a working day. Exposed for gate chain.
func (w *WorkflowRunner) IsWorkingDay(ctx context.Context) bool {
	if w.workdayProv == nil {
		return true // no provider → assume working day
	}
	return w.workdayProv.IsWorkingDay(ctx, time.Now())
}

// extractRoleFromSlug maps a workflow slug prefix to a role string.
func extractRoleFromSlug(slug string) string {
	roles := []string{"sdr", "bd", "ae", "cs"}
	for _, r := range roles {
		if slug == r || strings.HasPrefix(slug, r+"-") {
			return r
		}
	}
	return ""
}

func boolPtr(b bool) *bool       { return &b }
func strPtr(s string) *string    { return &s }
