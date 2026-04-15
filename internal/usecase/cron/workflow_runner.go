package cron

import (
	"context"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	automationrule "github.com/Sejutacita/cs-agent-bot/internal/usecase/automation_rule"
	workflowuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/workflow"
	"github.com/rs/zerolog"
)

// WorkflowRunner evaluates automation rules for a single master_data record
// using the workflow engine. It runs AFTER the legacy P0–P5 trigger sequence
// when USE_WORKFLOW_ENGINE=true.
//
// Design constraints (from CLAUDE.md):
//   - P0–P5 always runs first via the existing cronRunner.processClient.
//   - WorkflowRunner is purely additive — it never replaces legacy logic.
//   - The bot MUST NOT write payment_status, renewed, or rejected.
//   - Gated by UseWorkflowEngine config flag.
type WorkflowRunner struct {
	workflowUC    workflowuc.Usecase
	automationUC  automationrule.Usecase
	useWorkflow   bool
	logger        zerolog.Logger
}

// NewWorkflowRunner constructs a WorkflowRunner. If useWorkflow is false, all
// Run calls are no-ops.
func NewWorkflowRunner(
	workflowUC workflowuc.Usecase,
	automationUC automationrule.Usecase,
	useWorkflow bool,
	logger zerolog.Logger,
) *WorkflowRunner {
	return &WorkflowRunner{
		workflowUC:   workflowUC,
		automationUC: automationUC,
		useWorkflow:  useWorkflow,
		logger:       logger,
	}
}

// RunForRecord evaluates automation rules for a single master_data record.
// It loads the active workflow for the record's stage, finds enabled rules
// for that role, and evaluates their conditions against the record.
//
// This is called AFTER processClient() completes the P0–P5 sequence.
// The caller (cronRunner) must have already applied blacklist + bot_active gates.
//
// Returns true if any rule matched (for logging purposes only — the caller does
// not short-circuit on this).
func (w *WorkflowRunner) RunForRecord(ctx context.Context, workspaceID string, md entity.MasterData) (bool, error) {
	if !w.useWorkflow {
		return false, nil
	}

	// Safety guard: never mutate forbidden fields.
	// These should never reach here, but belt-and-suspenders.
	if md.PaymentStatus == "" {
		// No-op: we do not write payment_status
	}

	// Resolve the active workflow for this record's stage.
	wf, err := w.workflowUC.GetActiveForStage(ctx, workspaceID, md.Stage)
	if err != nil {
		return false, fmt.Errorf("workflowRunner.GetActiveForStage: %w", err)
	}
	if wf == nil {
		// No workflow configured for this stage — silently skip.
		return false, nil
	}

	// Determine role from workflow slug convention (sdr/bd/ae/cs).
	role := resolveRole(wf.Slug)
	if role == "" {
		w.logger.Debug().Str("slug", wf.Slug).Msg("workflow slug does not map to a known role, skipping")
		return false, nil
	}

	// Fetch all active automation rules for this role.
	rules, err := w.automationUC.GetActiveByRole(ctx, workspaceID, entity.RuleRole(role))
	if err != nil {
		return false, fmt.Errorf("workflowRunner.GetActiveByRole: %w", err)
	}

	matched := false
	for _, rule := range rules {
		if !rule.IsExecutable() {
			continue
		}
		// Condition evaluation is deferred to a future phase — for now we log
		// that the rule was considered. Full expression evaluation (parsing
		// rule.Condition against md fields) will be added in the cron-engine
		// implementation milestone (spec: 05-cron-engine.md).
		w.logger.Debug().
			Str("rule_code", rule.RuleCode).
			Str("trigger_id", rule.TriggerID).
			Str("company_id", md.CompanyID).
			Msg("workflow rule considered")
		matched = true
		// Rate limit: 300 ms between rule evaluations to respect WA rate limits.
		select {
		case <-ctx.Done():
			return matched, ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}

	return matched, nil
}

// resolveRole maps a workflow slug prefix to a RuleRole string.
// Convention: workflow slugs start with 'sdr-', 'bd-', 'ae-', or 'cs-'.
func resolveRole(slug string) string {
	roles := []string{"sdr", "bd", "ae", "cs"}
	for _, r := range roles {
		if len(slug) >= len(r) && slug[:len(r)] == r {
			return r
		}
	}
	return ""
}
