package automation_rule_test

import (
	"context"
	"testing"

	automationrule "github.com/Sejutacita/cs-agent-bot/internal/usecase/automation_rule"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/rs/zerolog"
)

// ─── Stub repositories ────────────────────────────────────────────────────────

type stubRuleRepo struct {
	rules     map[string]*entity.AutomationRule
	createErr error
	updateLogs []entity.RuleChangeLog
}

func (s *stubRuleRepo) List(_ context.Context, _ string, _ entity.AutomationRuleFilter) ([]entity.AutomationRule, error) {
	out := make([]entity.AutomationRule, 0, len(s.rules))
	for _, r := range s.rules {
		out = append(out, *r)
	}
	return out, nil
}

func (s *stubRuleRepo) GetByTriggerID(_ context.Context, _, _ string) (*entity.AutomationRule, error) {
	return nil, nil
}

func (s *stubRuleRepo) GetByID(_ context.Context, _ string, id string) (*entity.AutomationRule, error) {
	if r, ok := s.rules[id]; ok {
		return r, nil
	}
	return nil, nil
}

func (s *stubRuleRepo) Create(_ context.Context, r *entity.AutomationRule) error {
	if s.createErr != nil {
		return s.createErr
	}
	r.ID = "new-rule-id"
	if s.rules == nil {
		s.rules = make(map[string]*entity.AutomationRule)
	}
	cp := *r
	s.rules[r.ID] = &cp
	return nil
}

func (s *stubRuleRepo) Update(_ context.Context, _, _ string, _ map[string]interface{}, _ string) ([]entity.RuleChangeLog, error) {
	return s.updateLogs, nil
}

func (s *stubRuleRepo) Delete(_ context.Context, _, _ string) error { return nil }

func (s *stubRuleRepo) ListChangeLogsForRule(_ context.Context, _ string) ([]entity.RuleChangeLog, error) {
	return []entity.RuleChangeLog{}, nil
}

func (s *stubRuleRepo) ListChangeLogs(_ context.Context, _ string, _ int) ([]entity.RuleChangeLogWithCode, error) {
	return []entity.RuleChangeLogWithCode{}, nil
}

func (s *stubRuleRepo) GetActiveByRole(_ context.Context, _ string, _ entity.RuleRole) ([]entity.AutomationRule, error) {
	var out []entity.AutomationRule
	for _, r := range s.rules {
		if r.Status == entity.RuleStatusActive {
			out = append(out, *r)
		}
	}
	return out, nil
}

// stubApprovalRepo satisfies the ApprovalRequestRepository interface.
type stubApprovalRepo struct {
	created []*entity.ApprovalRequest
}

func (s *stubApprovalRepo) Create(_ context.Context, req *entity.ApprovalRequest) (*entity.ApprovalRequest, error) {
	req.ID = "approval-id"
	s.created = append(s.created, req)
	return req, nil
}

func (s *stubApprovalRepo) GetByID(_ context.Context, _, _ string) (*entity.ApprovalRequest, error) {
	return nil, nil
}

func (s *stubApprovalRepo) UpdateStatus(_ context.Context, _, _, _, _, _ string) error {
	return nil
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestCreate_RequiredFieldValidation(t *testing.T) {
	t.Parallel()

	uc := automationrule.New(&stubRuleRepo{}, nil, zerolog.Nop())

	cases := []struct {
		name string
		rule *entity.AutomationRule
	}{
		{
			name: "missing rule_code",
			rule: &entity.AutomationRule{
				TriggerID: "T1", Timing: "D+1", Condition: "x=1",
				Role: "ae", Phase: "P0",
			},
		},
		{
			name: "missing trigger_id",
			rule: &entity.AutomationRule{
				RuleCode: "R1", Timing: "D+1", Condition: "x=1",
				Role: "ae", Phase: "P0",
			},
		},
		{
			name: "missing role",
			rule: &entity.AutomationRule{
				RuleCode: "R1", TriggerID: "T1", Timing: "D+1",
				Condition: "x=1", Phase: "P0",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.rule.WorkspaceID = "ws-1"
			_, err := uc.Create(context.Background(), "actor@test.com", tc.rule)
			if err == nil {
				t.Errorf("expected validation error for %s", tc.name)
			}
		})
	}
}

func TestCreate_SetsDefaults(t *testing.T) {
	t.Parallel()

	repo := &stubRuleRepo{rules: make(map[string]*entity.AutomationRule)}
	uc := automationrule.New(repo, nil, zerolog.Nop())

	rule := &entity.AutomationRule{
		WorkspaceID: "ws-1",
		RuleCode:    "RULE-TEST-001",
		TriggerID:   "Test_Trigger",
		Timing:      "D+0 to D+5",
		Condition:   "days_active BETWEEN 0 AND 5",
		Role:        "ae",
		Phase:       "P0",
	}

	created, err := uc.Create(context.Background(), "actor@test.com", rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Channel == "" {
		t.Error("expected default channel to be set")
	}
	if created.Status == "" {
		t.Error("expected default status to be set")
	}
	if created.StopIf == "" {
		t.Error("expected default stop_if to be set")
	}
}

func TestGetByID_NotFoundReturnsError(t *testing.T) {
	t.Parallel()

	uc := automationrule.New(&stubRuleRepo{rules: map[string]*entity.AutomationRule{}}, nil, zerolog.Nop())

	_, _, err := uc.GetByID(context.Background(), "ws-1", "nonexistent")
	if err == nil {
		t.Fatal("expected not found error, got nil")
	}
}

func TestGetByID_ReturnsRuleAndLogs(t *testing.T) {
	t.Parallel()

	rule := &entity.AutomationRule{
		ID:          "rule-1",
		WorkspaceID: "ws-1",
		RuleCode:    "RULE-AE-001",
		Status:      entity.RuleStatusActive,
	}
	repo := &stubRuleRepo{rules: map[string]*entity.AutomationRule{"rule-1": rule}}
	uc := automationrule.New(repo, nil, zerolog.Nop())

	r, logs, err := uc.GetByID(context.Background(), "ws-1", "rule-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r == nil {
		t.Fatal("expected rule, got nil")
	}
	if r.RuleCode != "RULE-AE-001" {
		t.Errorf("unexpected rule_code: %s", r.RuleCode)
	}
	if logs == nil {
		t.Error("expected non-nil logs slice")
	}
}

func TestUpdate_StatusToggleActiveToToPaused_RequiresApproval(t *testing.T) {
	t.Parallel()

	rule := &entity.AutomationRule{
		ID:          "rule-1",
		WorkspaceID: "ws-1",
		RuleCode:    "RULE-AE-001",
		Status:      entity.RuleStatusActive,
	}
	repo := &stubRuleRepo{rules: map[string]*entity.AutomationRule{"rule-1": rule}}
	approvalRepo := &stubApprovalRepo{}
	uc := automationrule.New(repo, approvalRepo, zerolog.Nop())

	_, _, err := uc.Update(context.Background(), "ws-1", "rule-1", "actor@test.com", map[string]interface{}{
		"status": "paused",
	})
	// Should return a validation error indicating approval is required.
	if err == nil {
		t.Fatal("expected approval-required error, got nil")
	}
	// An approval request should have been created.
	if len(approvalRepo.created) == 0 {
		t.Error("expected approval request to be created for active->paused toggle")
	}
}

func TestUpdate_StatusTogglePausedToActive_RequiresApproval(t *testing.T) {
	t.Parallel()

	rule := &entity.AutomationRule{
		ID:          "rule-2",
		WorkspaceID: "ws-1",
		RuleCode:    "RULE-AE-002",
		Status:      entity.RuleStatusPaused,
	}
	repo := &stubRuleRepo{rules: map[string]*entity.AutomationRule{"rule-2": rule}}
	approvalRepo := &stubApprovalRepo{}
	uc := automationrule.New(repo, approvalRepo, zerolog.Nop())

	_, _, err := uc.Update(context.Background(), "ws-1", "rule-2", "actor@test.com", map[string]interface{}{
		"status": "active",
	})
	if err == nil {
		t.Fatal("expected approval-required error, got nil")
	}
	if len(approvalRepo.created) == 0 {
		t.Error("expected approval request to be created for paused->active toggle")
	}
}

func TestUpdate_NonStatusFieldDoesNotRequireApproval(t *testing.T) {
	t.Parallel()

	rule := &entity.AutomationRule{
		ID:          "rule-3",
		WorkspaceID: "ws-1",
		RuleCode:    "RULE-AE-003",
		Status:      entity.RuleStatusActive,
	}
	repo := &stubRuleRepo{
		rules: map[string]*entity.AutomationRule{"rule-3": rule},
		updateLogs: []entity.RuleChangeLog{
			{Field: "timing", OldValue: strPtr("H-90"), NewValue: "H-85"},
		},
	}
	uc := automationrule.New(repo, nil, zerolog.Nop())

	updated, logs, err := uc.Update(context.Background(), "ws-1", "rule-3", "actor@test.com", map[string]interface{}{
		"timing": "H-85",
	})
	if err != nil {
		t.Fatalf("unexpected error updating timing: %v", err)
	}
	if updated == nil {
		t.Fatal("expected updated rule, got nil")
	}
	if len(logs) == 0 {
		t.Error("expected change log entries for timing update")
	}
}

func TestList_ReturnsEmpty_WhenNoRules(t *testing.T) {
	t.Parallel()

	uc := automationrule.New(&stubRuleRepo{rules: map[string]*entity.AutomationRule{}}, nil, zerolog.Nop())

	rules, err := uc.List(context.Background(), "ws-1", entity.AutomationRuleFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(rules))
	}
}

func TestDelete_NotFoundReturnsError(t *testing.T) {
	t.Parallel()

	uc := automationrule.New(&stubRuleRepo{rules: map[string]*entity.AutomationRule{}}, nil, zerolog.Nop())

	err := uc.Delete(context.Background(), "ws-1", "nonexistent")
	if err == nil {
		t.Fatal("expected not found error for delete of nonexistent rule")
	}
}

func TestGetActiveByRole_FiltersToActiveOnly(t *testing.T) {
	t.Parallel()

	rules := map[string]*entity.AutomationRule{
		"r1": {ID: "r1", WorkspaceID: "ws-1", Status: entity.RuleStatusActive},
		"r2": {ID: "r2", WorkspaceID: "ws-1", Status: entity.RuleStatusPaused},
	}
	uc := automationrule.New(&stubRuleRepo{rules: rules}, nil, zerolog.Nop())

	active, err := uc.GetActiveByRole(context.Background(), "ws-1", "ae")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range active {
		if r.Status != entity.RuleStatusActive {
			t.Errorf("GetActiveByRole returned non-active rule: %s status=%s", r.ID, r.Status)
		}
	}
}

func strPtr(s string) *string { return &s }
