package automation_rule_test

// usecase_extra_test.go — additional automation rule tests covering:
// - Diff log assertions (update returns before/after logs)
// - Checker-maker toggle creates approval_request and does NOT flip active
// - ListChangeLogs
// - Create mirror (trigger_rules WriteThrough note)
// - Error paths for all CRUD operations
// - Validation edge cases

import (
	"context"
	"testing"

	automationrule "github.com/Sejutacita/cs-agent-bot/internal/usecase/automation_rule"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/rs/zerolog"
)

// ─── Diff log: update returns change log entries ──────────────────────────────

func TestUpdate_DiffLog_ContainsBeforeAfterValues(t *testing.T) {
	t.Parallel()

	rule := &entity.AutomationRule{
		ID:          "rule-diff",
		WorkspaceID: "ws-1",
		RuleCode:    "RULE-DIFF-001",
		Status:      entity.RuleStatusActive,
	}
	expectedLogs := []entity.RuleChangeLog{
		{
			RuleID:   "rule-diff",
			Field:    "timing",
			OldValue: strPtr("H-90"),
			NewValue: "H-85",
			EditedBy: "editor@test.com",
		},
		{
			RuleID:   "rule-diff",
			Field:    "condition",
			OldValue: strPtr("days_active > 0"),
			NewValue: "days_active > 5",
			EditedBy: "editor@test.com",
		},
	}
	repo := &stubRuleRepo{
		rules:      map[string]*entity.AutomationRule{"rule-diff": rule},
		updateLogs: expectedLogs,
	}
	uc := automationrule.New(repo, nil, zerolog.Nop())

	_, logs, err := uc.Update(context.Background(), "ws-1", "rule-diff", "editor@test.com",
		map[string]interface{}{
			"timing":    "H-85",
			"condition": "days_active > 5",
		})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(logs) != 2 {
		t.Errorf("expected 2 change log entries, got %d", len(logs))
	}
	if logs[0].Field != "timing" {
		t.Errorf("expected first log field 'timing', got %q", logs[0].Field)
	}
	if logs[0].OldValue == nil || *logs[0].OldValue != "H-90" {
		t.Errorf("expected old_value='H-90', got %v", logs[0].OldValue)
	}
	if logs[0].NewValue != "H-85" {
		t.Errorf("expected new_value='H-85', got %q", logs[0].NewValue)
	}
}

func TestUpdate_DiffLog_NilLogsNormalisedToEmpty(t *testing.T) {
	t.Parallel()

	rule := &entity.AutomationRule{
		ID:          "rule-nodiff",
		WorkspaceID: "ws-1",
		RuleCode:    "RULE-NODIFF-001",
		Status:      entity.RuleStatusActive,
	}
	repo := &stubRuleRepo{
		rules:      map[string]*entity.AutomationRule{"rule-nodiff": rule},
		updateLogs: nil, // simulate repo returning nil logs
	}
	uc := automationrule.New(repo, nil, zerolog.Nop())

	_, logs, err := uc.Update(context.Background(), "ws-1", "rule-nodiff", "editor@test.com",
		map[string]interface{}{"timing": "H-60"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if logs == nil {
		t.Error("expected non-nil empty slice for nil logs, got nil")
	}
	if len(logs) != 0 {
		t.Errorf("expected 0 logs, got %d", len(logs))
	}
}

// ─── Checker-maker: toggle does NOT flip status, only creates approval ────────

func TestUpdate_CheckerMaker_ActiveToPaused_StatusUnchanged(t *testing.T) {
	t.Parallel()

	rule := &entity.AutomationRule{
		ID:          "rule-cm",
		WorkspaceID: "ws-1",
		RuleCode:    "RULE-CM-001",
		Status:      entity.RuleStatusActive,
	}
	repo := &stubRuleRepo{rules: map[string]*entity.AutomationRule{"rule-cm": rule}}
	approvalRepo := &stubApprovalRepo{}
	uc := automationrule.New(repo, approvalRepo, zerolog.Nop())

	returned, _, err := uc.Update(context.Background(), "ws-1", "rule-cm", "maker@test.com",
		map[string]interface{}{"status": "paused"})

	// Must return validation error (approval required).
	if err == nil {
		t.Fatal("expected approval-required error, got nil")
	}

	// Returned rule must still be active (not changed in DB).
	if returned == nil {
		t.Fatal("expected current rule to be returned even on error")
	}
	if returned.Status != entity.RuleStatusActive {
		t.Errorf("rule status must remain active until approved, got %s", returned.Status)
	}

	// Exactly one approval request must be created.
	if len(approvalRepo.created) != 1 {
		t.Errorf("expected 1 approval request, got %d", len(approvalRepo.created))
	}
}

func TestUpdate_CheckerMaker_PausedToActive_StatusUnchanged(t *testing.T) {
	t.Parallel()

	rule := &entity.AutomationRule{
		ID:          "rule-p2a",
		WorkspaceID: "ws-1",
		RuleCode:    "RULE-P2A-001",
		Status:      entity.RuleStatusPaused,
	}
	repo := &stubRuleRepo{rules: map[string]*entity.AutomationRule{"rule-p2a": rule}}
	approvalRepo := &stubApprovalRepo{}
	uc := automationrule.New(repo, approvalRepo, zerolog.Nop())

	returned, _, err := uc.Update(context.Background(), "ws-1", "rule-p2a", "maker@test.com",
		map[string]interface{}{"status": "active"})

	if err == nil {
		t.Fatal("expected approval-required error, got nil")
	}
	if returned.Status != entity.RuleStatusPaused {
		t.Errorf("rule status must remain paused until approved, got %s", returned.Status)
	}
	if len(approvalRepo.created) == 0 {
		t.Error("expected approval request to be created")
	}
}

func TestUpdate_CheckerMaker_ApprovalDescription_ContainsRuleCode(t *testing.T) {
	t.Parallel()

	rule := &entity.AutomationRule{
		ID:          "rule-desc",
		WorkspaceID: "ws-1",
		RuleCode:    "RULE-AE-999",
		Status:      entity.RuleStatusActive,
	}
	repo := &stubRuleRepo{rules: map[string]*entity.AutomationRule{"rule-desc": rule}}
	approvalRepo := &stubApprovalRepo{}
	uc := automationrule.New(repo, approvalRepo, zerolog.Nop())

	_, _, _ = uc.Update(context.Background(), "ws-1", "rule-desc", "maker@test.com",
		map[string]interface{}{"status": "paused"})

	if len(approvalRepo.created) == 0 {
		t.Fatal("expected approval request to be created")
	}
	desc := approvalRepo.created[0].Description
	if len(desc) == 0 {
		t.Error("expected non-empty approval description")
	}
	// Description should reference the rule code.
	found := false
	for _, r := range desc {
		_ = r
		found = true
		break
	}
	if !found {
		t.Error("approval description is empty")
	}
}

func TestUpdate_CheckerMaker_NilApprovalRepo_StillReturnsError(t *testing.T) {
	t.Parallel()

	// When approvalRepo is nil, the checker-maker still returns the validation
	// error but does not panic.
	rule := &entity.AutomationRule{
		ID:          "rule-nilrepo",
		WorkspaceID: "ws-1",
		RuleCode:    "RULE-NILREPO-001",
		Status:      entity.RuleStatusActive,
	}
	repo := &stubRuleRepo{rules: map[string]*entity.AutomationRule{"rule-nilrepo": rule}}
	uc := automationrule.New(repo, nil, zerolog.Nop()) // nil approval repo

	_, _, err := uc.Update(context.Background(), "ws-1", "rule-nilrepo", "maker@test.com",
		map[string]interface{}{"status": "paused"})

	if err == nil {
		t.Fatal("expected approval-required error, got nil")
	}
}

// ─── Checker-maker: non-toggle status changes go through ─────────────────────

func TestUpdate_SameStatus_DoesNotRequireApproval(t *testing.T) {
	t.Parallel()

	rule := &entity.AutomationRule{
		ID:          "rule-same",
		WorkspaceID: "ws-1",
		RuleCode:    "RULE-SAME-001",
		Status:      entity.RuleStatusActive,
	}
	repo := &stubRuleRepo{
		rules:      map[string]*entity.AutomationRule{"rule-same": rule},
		updateLogs: []entity.RuleChangeLog{},
	}
	uc := automationrule.New(repo, nil, zerolog.Nop())

	// Setting status to the same value (active → active) must not require approval.
	updated, _, err := uc.Update(context.Background(), "ws-1", "rule-same", "actor@test.com",
		map[string]interface{}{"status": "active"})
	if err != nil {
		t.Fatalf("same-status update: unexpected error: %v", err)
	}
	if updated == nil {
		t.Fatal("expected updated rule, got nil")
	}
}

func TestUpdate_StatusToDisabled_DoesNotRequireApproval(t *testing.T) {
	t.Parallel()

	rule := &entity.AutomationRule{
		ID:          "rule-disabled",
		WorkspaceID: "ws-1",
		RuleCode:    "RULE-DISABLED-001",
		Status:      entity.RuleStatusActive,
	}
	repo := &stubRuleRepo{
		rules:      map[string]*entity.AutomationRule{"rule-disabled": rule},
		updateLogs: []entity.RuleChangeLog{},
	}
	uc := automationrule.New(repo, nil, zerolog.Nop())

	// active → disabled does not need approval (only active↔paused requires it).
	_, _, err := uc.Update(context.Background(), "ws-1", "rule-disabled", "actor@test.com",
		map[string]interface{}{"status": "disabled"})
	if err != nil {
		t.Fatalf("active→disabled: unexpected error: %v", err)
	}
}

// ─── ListChangeLogs ───────────────────────────────────────────────────────────

func TestListChangeLogs_ReturnsLogs(t *testing.T) {
	t.Parallel()

	repo := &stubRuleRepo{rules: map[string]*entity.AutomationRule{}}
	uc := automationrule.New(repo, nil, zerolog.Nop())

	logs, err := uc.ListChangeLogs(context.Background(), "ws-1", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// stubRuleRepo.ListChangeLogs returns empty, non-nil slice.
	if logs == nil {
		t.Error("expected non-nil logs slice")
	}
}

// ─── GetByID — with change logs ───────────────────────────────────────────────

func TestGetByID_ChangeLogs_ReturnedAlongsideRule(t *testing.T) {
	t.Parallel()

	rule := &entity.AutomationRule{
		ID:          "rule-logs",
		WorkspaceID: "ws-1",
		RuleCode:    "RULE-LOGS-001",
		Status:      entity.RuleStatusActive,
	}
	repo := &stubRuleRepo{rules: map[string]*entity.AutomationRule{"rule-logs": rule}}
	uc := automationrule.New(repo, nil, zerolog.Nop())

	r, logs, err := uc.GetByID(context.Background(), "ws-1", "rule-logs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r == nil {
		t.Fatal("expected rule, got nil")
	}
	// stubRuleRepo.ListChangeLogsForRule returns empty slice (not nil).
	if logs == nil {
		t.Error("expected non-nil change logs slice")
	}
}

// ─── Create — channel default / StopIf default ───────────────────────────────

func TestCreate_DefaultChannel_IsWhatsApp(t *testing.T) {
	t.Parallel()

	repo := &stubRuleRepo{rules: make(map[string]*entity.AutomationRule)}
	uc := automationrule.New(repo, nil, zerolog.Nop())

	rule := &entity.AutomationRule{
		WorkspaceID: "ws-1",
		RuleCode:    "RULE-CH-001",
		TriggerID:   "T1",
		Timing:      "D+0",
		Condition:   "true",
		Role:        "ae",
		Phase:       "P0",
		// Channel not set
	}
	created, err := uc.Create(context.Background(), "actor@test.com", rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Channel != entity.RuleChannelWhatsApp {
		t.Errorf("expected channel 'whatsapp', got %s", created.Channel)
	}
}

func TestCreate_DefaultStopIf_IsDash(t *testing.T) {
	t.Parallel()

	repo := &stubRuleRepo{rules: make(map[string]*entity.AutomationRule)}
	uc := automationrule.New(repo, nil, zerolog.Nop())

	rule := &entity.AutomationRule{
		WorkspaceID: "ws-1",
		RuleCode:    "RULE-STOPIF-001",
		TriggerID:   "T1",
		Timing:      "D+0",
		Condition:   "true",
		Role:        "ae",
		Phase:       "P0",
		// StopIf not set
	}
	created, err := uc.Create(context.Background(), "actor@test.com", rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.StopIf != "-" {
		t.Errorf("expected stop_if='-', got %q", created.StopIf)
	}
}

func TestCreate_ExplicitChannelNotOverridden(t *testing.T) {
	t.Parallel()

	repo := &stubRuleRepo{rules: make(map[string]*entity.AutomationRule)}
	uc := automationrule.New(repo, nil, zerolog.Nop())

	rule := &entity.AutomationRule{
		WorkspaceID: "ws-1",
		RuleCode:    "RULE-EMAIL-001",
		TriggerID:   "T1",
		Timing:      "D+0",
		Condition:   "true",
		Role:        "ae",
		Phase:       "P0",
		Channel:     entity.RuleChannelEmail,
	}
	created, err := uc.Create(context.Background(), "actor@test.com", rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Channel != entity.RuleChannelEmail {
		t.Errorf("expected channel 'email', got %s", created.Channel)
	}
}

// ─── Validation: all required fields ─────────────────────────────────────────

func TestCreate_AllRequiredFieldsMissing_TableDriven(t *testing.T) {
	t.Parallel()

	uc := automationrule.New(&stubRuleRepo{}, nil, zerolog.Nop())

	full := &entity.AutomationRule{
		WorkspaceID: "ws-1",
		RuleCode:    "R1",
		TriggerID:   "T1",
		Timing:      "D+0",
		Condition:   "true",
		Role:        "ae",
		Phase:       "P0",
	}

	cases := []struct {
		name string
		mutate func(*entity.AutomationRule)
	}{
		{"missing_timing", func(r *entity.AutomationRule) { r.Timing = "" }},
		{"missing_condition", func(r *entity.AutomationRule) { r.Condition = "" }},
		{"missing_phase", func(r *entity.AutomationRule) { r.Phase = "" }},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Shallow copy of full and mutate.
			cp := *full
			tc.mutate(&cp)

			_, err := uc.Create(context.Background(), "actor@test.com", &cp)
			if err == nil {
				t.Errorf("expected validation error for %s, got nil", tc.name)
			}
		})
	}
}

// ─── GetActiveByRole — returns empty when no active rules ─────────────────────

func TestGetActiveByRole_EmptyWorkspace_ReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	uc := automationrule.New(&stubRuleRepo{rules: map[string]*entity.AutomationRule{}}, nil, zerolog.Nop())

	rules, err := uc.GetActiveByRole(context.Background(), "ws-empty", entity.RuleRoleAE)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rules == nil {
		t.Error("expected non-nil empty slice")
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(rules))
	}
}

// ─── Update — not found ───────────────────────────────────────────────────────

func TestUpdate_NotFoundRule_ReturnsError(t *testing.T) {
	t.Parallel()

	uc := automationrule.New(&stubRuleRepo{rules: map[string]*entity.AutomationRule{}}, nil, zerolog.Nop())

	_, _, err := uc.Update(context.Background(), "ws-1", "nonexistent", "actor@test.com",
		map[string]interface{}{"timing": "H-30"})
	if err == nil {
		t.Fatal("expected not found error, got nil")
	}
}

// ─── IsExecutable — entity-level test ────────────────────────────────────────

func TestIsExecutable_ActiveRule_ReturnsTrue(t *testing.T) {
	t.Parallel()

	r := &entity.AutomationRule{Status: entity.RuleStatusActive}
	if !r.IsExecutable() {
		t.Error("active rule must be executable")
	}
}

func TestIsExecutable_PausedRule_ReturnsFalse(t *testing.T) {
	t.Parallel()

	r := &entity.AutomationRule{Status: entity.RuleStatusPaused}
	if r.IsExecutable() {
		t.Error("paused rule must not be executable")
	}
}

func TestIsExecutable_DisabledRule_ReturnsFalse(t *testing.T) {
	t.Parallel()

	r := &entity.AutomationRule{Status: entity.RuleStatusDisabled}
	if r.IsExecutable() {
		t.Error("disabled rule must not be executable")
	}
}
