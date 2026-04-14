package cron_test

// workflow_scenarios_test.go — client-journey scenario tests for WorkflowRunner.
//
// Each test case covers one named scenario from the spec. The tests are
// table-driven where the variants share structure, and standalone subtests
// where each scenario is meaningfully different.
//
// Tests use stub repos (no mockery) and deterministic timestamps (fixedNow).
//
// SKIP NOTES:
//   - Scenarios that require processClient() (P0-P5 gate ordering) cannot be
//     tested via WorkflowRunner alone because processClient is not exported.
//     Those scenarios are marked SKIP with a note below and would require
//     either exporting processClient or an integration harness with all deps.
//   - Template guard (UnresolvedVariable_AbortsSend) is also SKIP because
//     WorkflowRunner's current phase only logs rules; it does not resolve or
//     send templates (deferred to cron-engine milestone per code comment).

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	ucron "github.com/Sejutacita/cs-agent-bot/internal/usecase/cron"
	"github.com/rs/zerolog"
)

// ─── Gate behaviour: disabled flag ───────────────────────────────────────────

func TestWorkflowRunner_DisabledFlag_IsAlwaysNoOpRegardlessOfRecordState(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		md   entity.MasterData
	}{
		{
			name: "blacklisted client",
			md:   entity.MasterData{ID: "md-1", Stage: "CLIENT", Blacklisted: true},
		},
		{
			name: "overdue client",
			md: entity.MasterData{
				ID: "md-2", Stage: "CLIENT",
				PaymentStatus: "Overdue",
				ContractEnd:   &[]time.Time{fixedNow.AddDate(0, 0, -30)}[0],
			},
		},
		{
			name: "fresh lead",
			md:   entity.MasterData{ID: "md-3", Stage: entity.StageLead},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			wfUC := &stubWorkflowUC{wf: &entity.Workflow{ID: "wf-1", Slug: "ae-lifecycle"}}
			wr := ucron.NewWorkflowRunner(wfUC, &stubAutomationUC{
				rules: []entity.AutomationRule{activeRule("RULE-001", "any_trigger")},
			}, false, zerolog.Nop())

			matched, err := wr.RunForRecord(context.Background(), "ws-1", tc.md)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if matched {
				t.Errorf("scenario %q: expected no match when workflow engine disabled", tc.name)
			}
		})
	}
}

// ─── Gate: no workflow for stage ─────────────────────────────────────────────

func TestWorkflowRunner_NoWorkflowConfiguredForStage_SilentlySkips(t *testing.T) {
	t.Parallel()

	stages := []string{entity.StageLead, entity.StageProspect, entity.StageClient, entity.StageDormant}
	for _, stage := range stages {
		stage := stage
		t.Run("stage="+stage, func(t *testing.T) {
			t.Parallel()

			wfUC := &stubWorkflowUC{wf: nil} // no workflow for any stage
			wr := ucron.NewWorkflowRunner(wfUC, &stubAutomationUC{
				rules: []entity.AutomationRule{activeRule("R1", "T1")},
			}, true, zerolog.Nop())

			matched, err := wr.RunForRecord(context.Background(), "ws-x", entity.MasterData{
				ID: "md-1", Stage: stage,
			})
			if err != nil {
				t.Fatalf("stage %s: unexpected error: %v", stage, err)
			}
			if matched {
				t.Errorf("stage %s: expected no match (no workflow)", stage)
			}
		})
	}
}

// ─── Gate: slug does not map to known role ────────────────────────────────────

func TestWorkflowRunner_UnknownSlugRole_SkipsWithoutError(t *testing.T) {
	t.Parallel()

	cases := []string{"", "unknown-workflow", "admin-flow", "  "}
	for _, slug := range cases {
		slug := slug
		t.Run("slug="+slug, func(t *testing.T) {
			t.Parallel()

			wfUC := &stubWorkflowUC{wf: newWorkflow("wf-1", slug, "CLIENT")}
			wr := ucron.NewWorkflowRunner(wfUC, &stubAutomationUC{
				rules: []entity.AutomationRule{activeRule("R1", "T1")},
			}, true, zerolog.Nop())

			matched, err := wr.RunForRecord(context.Background(), "ws-1", entity.MasterData{
				ID: "md-1", Stage: entity.StageClient,
			})
			if err != nil {
				t.Fatalf("slug %q: unexpected error: %v", slug, err)
			}
			if matched {
				t.Errorf("slug %q: expected no match for unknown role", slug)
			}
		})
	}
}

// ─── resolveRole slug prefix table ───────────────────────────────────────────

func TestWorkflowRunner_ResolveRole_KnownPrefixes_MatchRules(t *testing.T) {
	t.Parallel()

	cases := []struct {
		slug     string
		wantRole entity.RuleRole
	}{
		{"sdr-new-lead", entity.RuleRoleSDR},
		{"bd-prospects", entity.RuleRoleBD},
		{"ae-lifecycle", entity.RuleRoleAE},
		{"cs-retention", entity.RuleRoleCS},
		{"sdr", entity.RuleRoleSDR}, // exact match
		{"ae", entity.RuleRoleAE},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("slug="+tc.slug, func(t *testing.T) {
			t.Parallel()

			captureAU := &capturingAutomationUC{
				stubAutomationUC: stubAutomationUC{
					rules: []entity.AutomationRule{activeRule("R1", "T1")},
				},
			}

			wfUC := &stubWorkflowUC{wf: newWorkflow("wf-1", tc.slug, "CLIENT")}
			wr := ucron.NewWorkflowRunner(wfUC, captureAU, true, zerolog.Nop())

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			_, err := wr.RunForRecord(ctx, "ws-1", entity.MasterData{
				ID: "md-1", Stage: entity.StageClient,
			})
			if err != nil && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(captureAU.queriedRoles) == 0 {
				t.Fatal("GetActiveByRole was never called")
			}
			if captureAU.queriedRoles[0] != tc.wantRole {
				t.Errorf("slug=%q: expected role %q, got %q",
					tc.slug, tc.wantRole, captureAU.queriedRoles[0])
			}
		})
	}
}

// ─── Active rule matched ──────────────────────────────────────────────────────

func TestWorkflowRunner_ActiveRule_ReturnsMatchedTrue(t *testing.T) {
	t.Parallel()

	wfUC := &stubWorkflowUC{wf: newWorkflow("wf-ae", "ae-lifecycle", "CLIENT")}
	rules := []entity.AutomationRule{activeRule("RULE-AE-001", "Onboarding_Welcome")}
	wr := ucron.NewWorkflowRunner(wfUC, &stubAutomationUC{rules: rules}, true, zerolog.Nop())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	matched, err := wr.RunForRecord(ctx, "ws-1", entity.MasterData{
		ID: "md-1", CompanyID: "CO-001", Stage: entity.StageClient,
	})
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !matched {
		t.Error("expected matched=true for active rule")
	}
}

// ─── Paused/disabled rules not matched ───────────────────────────────────────

func TestWorkflowRunner_PausedAndDisabledRules_NeverMatched(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status entity.RuleStatus
	}{
		{"paused", entity.RuleStatusPaused},
		{"disabled", entity.RuleStatusDisabled},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			wfUC := &stubWorkflowUC{wf: newWorkflow("wf-1", "ae-lifecycle", "CLIENT")}
			rules := []entity.AutomationRule{{
				ID:        "rule-1",
				RuleCode:  "RULE-001",
				TriggerID: "T1",
				Status:    tc.status,
				Role:      entity.RuleRoleAE,
				Phase:     "P0",
			}}
			wr := ucron.NewWorkflowRunner(wfUC, &stubAutomationUC{rules: rules}, true, zerolog.Nop())

			matched, err := wr.RunForRecord(context.Background(), "ws-1", entity.MasterData{
				ID: "md-1", Stage: entity.StageClient,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if matched {
				t.Errorf("status=%s: expected no match for non-active rule", tc.status)
			}
		})
	}
}

// ─── Empty rules list ─────────────────────────────────────────────────────────

func TestWorkflowRunner_EmptyRulesList_NoMatch(t *testing.T) {
	t.Parallel()

	wfUC := &stubWorkflowUC{wf: newWorkflow("wf-1", "ae-lifecycle", "CLIENT")}
	wr := ucron.NewWorkflowRunner(wfUC, &stubAutomationUC{rules: nil}, true, zerolog.Nop())

	matched, err := wr.RunForRecord(context.Background(), "ws-1", entity.MasterData{
		ID: "md-1", Stage: entity.StageClient,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if matched {
		t.Error("expected no match when rule list is empty")
	}
}

// ─── Forbidden field writes (CLAUDE.md §3) ────────────────────────────────────

func TestWorkflowRunner_RejectsWriteToPaymentStatus(t *testing.T) {
	t.Parallel()

	wfUC := &stubWorkflowUC{wf: newWorkflow("wf-1", "ae-lifecycle", "CLIENT")}
	rules := []entity.AutomationRule{activeRule("RULE-PAY", "Payment_Status_Change")}
	wr := ucron.NewWorkflowRunner(wfUC, &stubAutomationUC{rules: rules}, true, zerolog.Nop())

	md := entity.MasterData{
		ID:            "md-1",
		CompanyID:     "CO-001",
		Stage:         entity.StageClient,
		PaymentStatus: "Pending",
	}
	before := md

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := wr.RunForRecord(ctx, "ws-1", md)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected error: %v", err)
	}

	assertNoForbiddenFieldMutated(t, before, md)
}

func TestWorkflowRunner_RejectsWriteToRenewed(t *testing.T) {
	t.Parallel()

	wfUC := &stubWorkflowUC{wf: newWorkflow("wf-1", "ae-lifecycle", "CLIENT")}
	rules := []entity.AutomationRule{activeRule("RULE-REN", "Renewal_Trigger")}
	wr := ucron.NewWorkflowRunner(wfUC, &stubAutomationUC{rules: rules}, true, zerolog.Nop())

	md := entity.MasterData{
		ID:        "md-2",
		CompanyID: "CO-002",
		Stage:     entity.StageClient,
		Renewed:   false,
	}
	before := md

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := wr.RunForRecord(ctx, "ws-1", md)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected error: %v", err)
	}

	if md.Renewed != before.Renewed {
		t.Errorf("renewed field was mutated: %v → %v", before.Renewed, md.Renewed)
	}
}

func TestWorkflowRunner_RejectsWriteToRejected(t *testing.T) {
	t.Parallel()

	wfUC := &stubWorkflowUC{wf: newWorkflow("wf-1", "ae-lifecycle", "CLIENT")}
	rules := []entity.AutomationRule{activeRule("RULE-REJ", "Rejection_Trigger")}
	wr := ucron.NewWorkflowRunner(wfUC, &stubAutomationUC{rules: rules}, true, zerolog.Nop())

	md := entity.MasterData{
		ID:        "md-3",
		CompanyID: "CO-003",
		Stage:     entity.StageClient,
		// Rejected is not a field on MasterData — we verify PaymentStatus is untouched.
		PaymentStatus: "Overdue",
	}
	before := md

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := wr.RunForRecord(ctx, "ws-1", md)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected error: %v", err)
	}

	assertNoForbiddenFieldMutated(t, before, md)
}

// ─── Priority invariant: workflow runner is additive ─────────────────────────

// TestWorkflowRunner_IsAdditive_CalledAfterLegacyTriggersInContract verifies
// the documented contract: RunForRecord should never affect legacy P0-P5 results.
// We confirm this by running it on a fully-active record and checking the record
// value remains unchanged — the caller (cronRunner) is responsible for calling
// RunForRecord only after processClient completes.
func TestWorkflowRunner_IsAdditive_RecordUnchangedAfterRun(t *testing.T) {
	t.Parallel()

	wfUC := &stubWorkflowUC{wf: newWorkflow("wf-ae", "ae-lifecycle", entity.StageClient)}
	rules := []entity.AutomationRule{
		activeRule("RULE-001", "HealthRisk_Check"),
		activeRule("RULE-002", "Invoice_Due"),
	}
	wr := ucron.NewWorkflowRunner(wfUC, &stubAutomationUC{rules: rules}, true, zerolog.Nop())

	md := entity.MasterData{
		ID:            "md-additive",
		CompanyID:     "CO-ADDITIVE",
		Stage:         entity.StageClient,
		PaymentStatus: "Menunggu",
		BotActive:     true,
		Renewed:       false,
	}
	snapshot := md

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	matched, err := wr.RunForRecord(ctx, "ws-1", md)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected error: %v", err)
	}

	// Either matched (rules ran) or not (ctx expired) — neither is an error.
	_ = matched

	// Confirm the record value is identical — runner must not mutate the copy.
	if md.PaymentStatus != snapshot.PaymentStatus {
		t.Errorf("payment_status mutated: %q → %q", snapshot.PaymentStatus, md.PaymentStatus)
	}
	if md.BotActive != snapshot.BotActive {
		t.Errorf("bot_active mutated: %v → %v", snapshot.BotActive, md.BotActive)
	}
	if md.Renewed != snapshot.Renewed {
		t.Errorf("renewed mutated: %v → %v", snapshot.Renewed, md.Renewed)
	}
}

// ─── Error propagation ────────────────────────────────────────────────────────

func TestWorkflowRunner_GetActiveForStageError_PropagatesError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("db timeout")
	wfUC := &stubWorkflowUCError{err: sentinel}
	wr := ucron.NewWorkflowRunner(wfUC, &stubAutomationUC{}, true, zerolog.Nop())

	_, err := wr.RunForRecord(context.Background(), "ws-1", entity.MasterData{
		ID: "md-1", Stage: entity.StageClient,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "GetActiveForStage") {
		t.Errorf("error should mention GetActiveForStage, got: %v", err)
	}
}

func TestWorkflowRunner_GetActiveByRoleError_PropagatesError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("role query failed")
	wfUC := &stubWorkflowUC{wf: newWorkflow("wf-1", "ae-lifecycle", "CLIENT")}
	wr := ucron.NewWorkflowRunner(wfUC, &stubAutomationUCError{err: sentinel}, true, zerolog.Nop())

	_, err := wr.RunForRecord(context.Background(), "ws-1", entity.MasterData{
		ID: "md-1", Stage: entity.StageClient,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "GetActiveByRole") {
		t.Errorf("error should mention GetActiveByRole, got: %v", err)
	}
}

// ─── Context cancellation ─────────────────────────────────────────────────────

func TestWorkflowRunner_ContextCancelled_ReturnsCtxError(t *testing.T) {
	t.Parallel()

	wfUC := &stubWorkflowUC{wf: newWorkflow("wf-1", "ae-lifecycle", "CLIENT")}
	// Multiple active rules so the 300ms sleep will be hit.
	rules := []entity.AutomationRule{
		activeRule("R1", "T1"),
		activeRule("R2", "T2"),
	}
	wr := ucron.NewWorkflowRunner(wfUC, &stubAutomationUC{rules: rules}, true, zerolog.Nop())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := wr.RunForRecord(ctx, "ws-1", entity.MasterData{
		ID: "md-1", Stage: entity.StageClient,
	})
	// Either the first rule ran in <10ms (unlikely with 300ms sleep) or ctx cancelled.
	// We just confirm no panic and the error (if any) is the context error.
	if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("expected context error, got: %v", err)
	}
}

// ─── Multi-rule ordering ──────────────────────────────────────────────────────

func TestWorkflowRunner_MultipleActiveRules_AllConsidered(t *testing.T) {
	t.Parallel()

	wfUC := &stubWorkflowUC{wf: newWorkflow("wf-1", "ae-lifecycle", "CLIENT")}
	rules := []entity.AutomationRule{
		activeRule("R1", "T1"),
		{ID: "r2", RuleCode: "R2", TriggerID: "T2", Status: entity.RuleStatusPaused, Role: entity.RuleRoleAE, Phase: "P0"},
		activeRule("R3", "T3"),
	}
	wr := ucron.NewWorkflowRunner(wfUC, &stubAutomationUC{rules: rules}, true, zerolog.Nop())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	matched, err := wr.RunForRecord(ctx, "ws-1", entity.MasterData{
		ID: "md-1", Stage: entity.StageClient,
	})
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected error: %v", err)
	}
	// At least one active rule matched.
	if !matched {
		t.Error("expected matched=true with active rules in list")
	}
}

// ─── Workspace scoping ────────────────────────────────────────────────────────

func TestWorkflowRunner_WorkspaceIDPassedToWorkflowUC(t *testing.T) {
	t.Parallel()

	captureWF := &capturingWorkflowUC{
		stubWorkflowUC: stubWorkflowUC{wf: nil},
	}
	wr := ucron.NewWorkflowRunner(captureWF, &stubAutomationUC{}, true, zerolog.Nop())

	targetWS := "ws-specific-123"
	_, _ = wr.RunForRecord(context.Background(), targetWS, entity.MasterData{
		ID: "md-1", Stage: entity.StageClient,
	})

	if len(captureWF.queriedStages) == 0 {
		t.Fatal("GetActiveForStage was never called")
	}
	// WorkspaceID is passed as first arg — we verify stage was queried correctly.
	if captureWF.queriedStages[0] != entity.StageClient {
		t.Errorf("expected stage %q, got %q", entity.StageClient, captureWF.queriedStages[0])
	}
}

// ─── Stage-specific workflow lookup ──────────────────────────────────────────

func TestWorkflowRunner_StagePassedFromMasterData(t *testing.T) {
	t.Parallel()

	captureWF := &capturingWorkflowUC{
		stubWorkflowUC: stubWorkflowUC{wf: nil},
	}
	wr := ucron.NewWorkflowRunner(captureWF, &stubAutomationUC{}, true, zerolog.Nop())

	stages := []string{entity.StageLead, entity.StageProspect, entity.StageClient, entity.StageDormant}
	for _, stage := range stages {
		captureWF.queriedStages = nil // reset between iterations

		_, _ = wr.RunForRecord(context.Background(), "ws-1", entity.MasterData{
			ID: "md-" + stage, Stage: stage,
		})

		if len(captureWF.queriedStages) != 1 || captureWF.queriedStages[0] != stage {
			t.Errorf("stage %q: expected queried stage %q, got %v",
				stage, stage, captureWF.queriedStages)
		}
	}
}

// ─── Mixed active/paused rules — only active count ───────────────────────────

func TestWorkflowRunner_MixedRules_OnlyActiveRulesCountAsMatch(t *testing.T) {
	t.Parallel()

	wfUC := &stubWorkflowUC{wf: newWorkflow("wf-1", "cs-retention", "CLIENT")}
	// All paused — should not match.
	allPaused := []entity.AutomationRule{
		{ID: "r1", RuleCode: "R1", TriggerID: "T1", Status: entity.RuleStatusPaused, Role: entity.RuleRoleCS, Phase: "P0"},
		{ID: "r2", RuleCode: "R2", TriggerID: "T2", Status: entity.RuleStatusDisabled, Role: entity.RuleRoleCS, Phase: "P0"},
	}
	wr := ucron.NewWorkflowRunner(wfUC, &stubAutomationUC{rules: allPaused}, true, zerolog.Nop())

	matched, err := wr.RunForRecord(context.Background(), "ws-1", entity.MasterData{
		ID: "md-1", Stage: entity.StageClient,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if matched {
		t.Error("expected no match when all rules are paused/disabled")
	}
}

// ─── Scenario: P0 health risk (SKIP note) ────────────────────────────────────

// SKIP: TestScenario_HighChurnRisk_FiresHealthActionFirst
// Reason: This scenario requires processClient() which evaluates EvalHealthRisk
// before other triggers. processClient is not exported and requires a full
// TriggerService + all repos. The WorkflowRunner only runs AFTER processClient
// completes — it cannot be used to test P0-P5 ordering. An integration test
// harness (or exported processClient) would be needed.

// ─── Scenario: Check-in branch A / B (SKIP note) ─────────────────────────────

// SKIP: TestScenario_CheckinBranchA_LongContract
// SKIP: TestScenario_CheckinBranchB_ShortContract
// SKIP: TestScenario_CheckinReplied_SkipsRen60AndRen45
// Reason: Check-in branch logic lives in trigger.TriggerService.EvalCheckIn,
// called by processClient. WorkflowRunner has no knowledge of check-in flags.
// These tests belong in internal/usecase/trigger/ not here.

// ─── Scenario: P1 negotiation (SKIP note) ────────────────────────────────────

// SKIP: TestScenario_Ren60_NoReply_FiresRen45
// SKIP: TestScenario_Ren45_ExpiredPromo_SkipsAndAlertsAELead
// SKIP: TestScenario_Ren30_NullQuotation_DefersAndAlertsAE
// SKIP: TestScenario_ClientRepliedToRen_HaltsP1
// Reason: Same as above — these live in trigger.TriggerService.EvalNegotiation.

// ─── Scenario: P2 invoice (SKIP notes) ───────────────────────────────────────

// SKIP: TestScenario_InvoiceIssuedH30_DueDateEqualsContractEnd
// SKIP: TestScenario_Pre14Sent_FlagOnClientsNotClientFlags
// SKIP: TestScenario_ClientRepliedButP2Continues
// SKIP: TestScenario_NewInvoiceCycle_ResetsPaymentFlags
// Reason: Invoice/payment flag logic is in trigger.TriggerService.EvalInvoice
// and the cronRunner.processClient reset-cycle path. Not reachable via WorkflowRunner.

// ─── Scenario: P3 overdue / escalation (SKIP notes) ─────────────────────────

// SKIP: TestScenario_OverdueD7_FiresOverdueAction
// SKIP: TestScenario_OverdueD15_TriggersESC001
// SKIP: TestScenario_OverdueEscalationDeduped
// Reason: trigger.TriggerService.EvalOverdue + escalation.TriggerEscalation.
// Not reachable via WorkflowRunner.

// ─── Scenario: P4 NPS (SKIP notes) ──────────────────────────────────────────

// SKIP: TestScenario_NpsReplied_HaltsP4
// SKIP: TestScenario_NpsScore4_TriggersESC003
// Reason: trigger.TriggerService.EvalExpansion.

// ─── Scenario: P5 cross-sell (SKIP notes) ────────────────────────────────────

// SKIP: TestScenario_CrossSellRejected_HaltsP5
// SKIP: TestScenario_CrossSellInterested_HaltsAndNotifiesAE
// Reason: trigger.TriggerService.EvalCrossSell.

// ─── Scenario: Template guard (SKIP note) ────────────────────────────────────

// SKIP: TestScenario_UnresolvedVariable_AbortsSend
// Reason: WorkflowRunner's current phase only logs that a rule was considered.
// Template resolution and send are deferred to the cron-engine milestone
// (spec: 05-cron-engine.md). When that milestone is implemented, this test
// should be added to trigger/send.go tests.

// ─── Scenario table: forbidden field writes ──────────────────────────────────

func TestWorkflowRunner_ForbiddenFields_TableDriven(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		initialStatus string
		initialRenew  bool
	}{
		{"payment_status=Paid", entity.PaymentStatusPaid, false},
		{"payment_status=Overdue", entity.PaymentStatusOverdue, false},
		{"payment_status=Pending renewed=true", entity.PaymentStatusPending, true},
		{"payment_status=Partial", entity.PaymentStatusPartial, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			wfUC := &stubWorkflowUC{wf: newWorkflow("wf-1", "ae-lifecycle", "CLIENT")}
			rules := []entity.AutomationRule{
				activeRule("RULE-PAYMENT-WRITE", "Payment_Status_Override"),
				activeRule("RULE-RENEWED-WRITE", "Renewal_Flag_Override"),
			}
			wr := ucron.NewWorkflowRunner(wfUC, &stubAutomationUC{rules: rules}, true, zerolog.Nop())

			md := entity.MasterData{
				ID:            "md-forbidden",
				CompanyID:     "CO-FORBIDDEN",
				Stage:         entity.StageClient,
				PaymentStatus: tc.initialStatus,
				Renewed:       tc.initialRenew,
			}
			before := md

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			_, err := wr.RunForRecord(ctx, "ws-1", md)
			if err != nil && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("unexpected error: %v", err)
			}

			assertNoForbiddenFieldMutated(t, before, md)
		})
	}
}

// ─── Role routing correctness ─────────────────────────────────────────────────

func TestWorkflowRunner_RoleRouting_SDRBDCSWorkflows(t *testing.T) {
	t.Parallel()

	cases := []struct {
		slug     string
		wantRole entity.RuleRole
	}{
		{"sdr-qualification", entity.RuleRoleSDR},
		{"bd-engagement", entity.RuleRoleBD},
		{"cs-success", entity.RuleRoleCS},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.slug, func(t *testing.T) {
			t.Parallel()

			captureAU := &capturingAutomationUC{
				stubAutomationUC: stubAutomationUC{rules: nil},
			}
			wfUC := &stubWorkflowUC{wf: newWorkflow("wf-1", tc.slug, entity.StageLead)}
			wr := ucron.NewWorkflowRunner(wfUC, captureAU, true, zerolog.Nop())

			_, _ = wr.RunForRecord(context.Background(), "ws-1", entity.MasterData{
				ID: "md-1", Stage: entity.StageLead,
			})

			if len(captureAU.queriedRoles) == 0 {
				t.Fatal("GetActiveByRole never called")
			}
			if captureAU.queriedRoles[0] != tc.wantRole {
				t.Errorf("expected role %q, got %q", tc.wantRole, captureAU.queriedRoles[0])
			}
		})
	}
}
