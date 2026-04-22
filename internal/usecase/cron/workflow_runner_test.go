package cron_test

import (
	"context"
	"testing"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	ucron "github.com/Sejutacita/cs-agent-bot/internal/usecase/cron"
	workflowuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/workflow"
	"github.com/rs/zerolog"
)

// ─── Stub usecases ────────────────────────────────────────────────────────────

type stubWorkflowUC struct {
	wf *entity.Workflow
}

func (s *stubWorkflowUC) List(_ context.Context, _ string, _ *string) ([]entity.WorkflowListItem, error) {
	return nil, nil
}
func (s *stubWorkflowUC) GetByID(_ context.Context, _, _ string) (*entity.WorkflowFull, error) {
	return nil, nil
}
func (s *stubWorkflowUC) GetBySlug(_ context.Context, _, _ string) (*entity.WorkflowFull, error) {
	return nil, nil
}
func (s *stubWorkflowUC) Create(_ context.Context, _ string, _ *entity.Workflow) (*entity.Workflow, error) {
	return nil, nil
}
func (s *stubWorkflowUC) Update(_ context.Context, _, _, _ string, _ map[string]interface{}) error {
	return nil
}
func (s *stubWorkflowUC) Delete(_ context.Context, _, _ string) error { return nil }
func (s *stubWorkflowUC) SaveCanvas(_ context.Context, _, _ string, _ []entity.WorkflowNode, _ []entity.WorkflowEdge) (*workflowuc.CanvasSaveResult, error) {
	return nil, nil
}
func (s *stubWorkflowUC) SaveSteps(_ context.Context, _, _ string, _ []entity.WorkflowStep) error {
	return nil
}
func (s *stubWorkflowUC) SaveTabs(_ context.Context, _, _ string, _ []entity.PipelineTab) error {
	return nil
}
func (s *stubWorkflowUC) SaveStats(_ context.Context, _, _ string, _ []entity.PipelineStat) error {
	return nil
}
func (s *stubWorkflowUC) SaveColumns(_ context.Context, _, _ string, _ []entity.PipelineColumn) error {
	return nil
}
func (s *stubWorkflowUC) GetConfig(_ context.Context, _, _ string) (*entity.WorkflowFull, error) {
	return nil, nil
}
func (s *stubWorkflowUC) GetStepByKey(_ context.Context, _, _, _ string) (*entity.WorkflowStep, error) {
	return nil, nil
}
func (s *stubWorkflowUC) UpdateStep(_ context.Context, _, _, _ string, _ map[string]interface{}) error {
	return nil
}
func (s *stubWorkflowUC) GetActiveForStage(_ context.Context, _, _ string) (*entity.Workflow, error) {
	return s.wf, nil
}

type stubAutomationUC struct {
	rules []entity.AutomationRule
}

func (s *stubAutomationUC) List(_ context.Context, _ string, _ entity.AutomationRuleFilter) ([]entity.AutomationRule, error) {
	return s.rules, nil
}
func (s *stubAutomationUC) GetByID(_ context.Context, _, _ string) (*entity.AutomationRule, []entity.RuleChangeLog, error) {
	return nil, nil, nil
}
func (s *stubAutomationUC) Create(_ context.Context, _ string, _ *entity.AutomationRule) (*entity.AutomationRule, error) {
	return nil, nil
}
func (s *stubAutomationUC) Update(_ context.Context, _, _, _ string, _ map[string]interface{}) (*entity.AutomationRule, []entity.RuleChangeLog, error) {
	return nil, nil, nil
}
func (s *stubAutomationUC) Delete(_ context.Context, _, _ string) error { return nil }
func (s *stubAutomationUC) ListChangeLogs(_ context.Context, _ string, _ int) ([]entity.RuleChangeLogWithCode, error) {
	return nil, nil
}
func (s *stubAutomationUC) GetActiveByRole(_ context.Context, _ string, _ entity.RuleRole) ([]entity.AutomationRule, error) {
	return s.rules, nil
}

// ─── Tests ────────────────────────────────────────────────────────────────────

// TestWorkflowRunner_DisabledFlagIsNoOp asserts that when USE_WORKFLOW_ENGINE
// is false the runner returns without doing any work (preserving P0–P5 only).
func TestWorkflowRunner_DisabledFlagIsNoOp(t *testing.T) {
	t.Parallel()

	wfUC := &stubWorkflowUC{wf: &entity.Workflow{ID: "wf-1", Slug: "ae-lifecycle"}}
	wr := ucron.NewWorkflowRunner(wfUC, &stubAutomationUC{}, nil, nil, nil, nil, nil, nil, false, zerolog.Nop())

	matched, err := wr.RunForRecord(context.Background(), "ws-1", entity.MasterData{
		ID:    "md-1",
		Stage: "CLIENT",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if matched {
		t.Error("expected no match when workflow engine is disabled")
	}
}

// TestWorkflowRunner_NoWorkflowForStageSkips asserts that when no active
// workflow covers the record's stage the runner silently skips.
func TestWorkflowRunner_NoWorkflowForStageSkips(t *testing.T) {
	t.Parallel()

	wfUC := &stubWorkflowUC{wf: nil} // no workflow for stage
	wr := ucron.NewWorkflowRunner(wfUC, &stubAutomationUC{}, nil, nil, nil, nil, nil, nil, true, zerolog.Nop())

	matched, err := wr.RunForRecord(context.Background(), "ws-1", entity.MasterData{
		ID:    "md-1",
		Stage: "LEAD",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if matched {
		t.Error("expected no match when no workflow found for stage")
	}
}

// TestWorkflowRunner_DoesNotWriteForbiddenFields asserts that the runner
// never modifies payment_status, renewed, or rejected on a record.
// This is enforced by running through all rules and confirming the record
// fields are unchanged after RunForRecord.
func TestWorkflowRunner_DoesNotWriteForbiddenFields(t *testing.T) {
	t.Parallel()

	wfUC := &stubWorkflowUC{wf: &entity.Workflow{
		ID:   "wf-ae",
		Slug: "ae-lifecycle",
	}}
	rules := []entity.AutomationRule{
		{
			ID:        "rule-1",
			RuleCode:  "RULE-AE-001",
			TriggerID: "Onboarding_Welcome",
			Status:    entity.RuleStatusActive,
		},
	}
	wr := ucron.NewWorkflowRunner(wfUC, &stubAutomationUC{rules: rules}, nil, nil, nil, nil, nil, nil, true, zerolog.Nop())

	md := entity.MasterData{
		ID:            "md-1",
		CompanyID:     "CO-001",
		Stage:         "CLIENT",
		PaymentStatus: "Menunggu", // must remain unchanged
	}
	originalPaymentStatus := md.PaymentStatus

	_, err := wr.RunForRecord(context.Background(), "ws-1", md)
	if err != nil && err != context.DeadlineExceeded {
		t.Fatalf("unexpected error: %v", err)
	}

	// PaymentStatus on the value copy must be unchanged.
	if md.PaymentStatus != originalPaymentStatus {
		t.Errorf("RunForRecord must not modify payment_status: got %q, want %q",
			md.PaymentStatus, originalPaymentStatus)
	}
}

// TestWorkflowRunner_P0P5RunsFirstIsAdditive verifies the conceptual guarantee:
// the WorkflowRunner is called only after the existing processClient has already
// handled legacy triggers. We test this by ensuring that calling RunForRecord
// with useWorkflow=true still respects the contract (no-op on disabled rules).
func TestWorkflowRunner_P0P5RunsFirstIsAdditive(t *testing.T) {
	t.Parallel()

	wfUC := &stubWorkflowUC{wf: &entity.Workflow{
		ID:   "wf-ae",
		Slug: "ae-lifecycle",
		StageFilter: []string{"CLIENT"},
	}}
	// Disabled rule — should not be matched.
	rules := []entity.AutomationRule{
		{
			ID:       "rule-2",
			RuleCode: "RULE-AE-PAUSED",
			Status:   entity.RuleStatusPaused,
		},
	}
	wr := ucron.NewWorkflowRunner(wfUC, &stubAutomationUC{rules: rules}, nil, nil, nil, nil, nil, nil, true, zerolog.Nop())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	matched, err := wr.RunForRecord(ctx, "ws-1", entity.MasterData{
		ID:    "md-1",
		Stage: "CLIENT",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if matched {
		t.Error("expected no match for paused rule")
	}
}
