package cron

import (
	"context"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/rs/zerolog"
)

func TestIsManualFlow_RegistryLookup(t *testing.T) {
	cases := map[string]bool{
		"AE_P4_REN90_OPENER":      true,
		"BD_D10_DM_ESCALATION":    true,
		"ADMIN_BLACKLIST_EDIT":    true,
		"AE_P00_DAILY_STATUS_BOT": false, // automated — not in the set
		"":                        false,
	}
	for id, want := range cases {
		if got := IsManualFlow(id); got != want {
			t.Errorf("IsManualFlow(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestManualFlowPriority_AssignsTierCorrectly(t *testing.T) {
	cases := map[string]string{
		"AE_P4_REN90_OPENER":  entity.ManualActionPriorityP0,
		"ADMIN_PRICING_EDIT":  entity.ManualActionPriorityP0,
		"AE_P42_REN_CALL":     entity.ManualActionPriorityP1,
		"AE_P22_CALL_INVITE":  entity.ManualActionPriorityP2,
		"SOMETHING_UNKNOWN":   entity.ManualActionPriorityP2,
	}
	for id, want := range cases {
		if got := manualFlowPriority(id); got != want {
			t.Errorf("manualFlowPriority(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestManualFlowRole_PrefixDerivation(t *testing.T) {
	cases := map[string]string{
		"SDR_BANT_QUALIFY_REVIEW": "sdr",
		"BD_D10_DM_ESCALATION":    "bd",
		"AE_P4_REN90_OPENER":      "ae",
		"ADMIN_PRICING_EDIT":      "admin",
		"UNKNOWN_TRIGGER":         "ae", // default
	}
	for id, want := range cases {
		if got := manualFlowRole(id); got != want {
			t.Errorf("manualFlowRole(%q) = %q, want %q", id, got, want)
		}
	}
}

// enqSpy captures Enqueue calls for assertion.
type enqSpy struct {
	calls []ManualActionEnqueueInput
}

func (s *enqSpy) Enqueue(_ context.Context, in ManualActionEnqueueInput) error {
	s.calls = append(s.calls, in)
	return nil
}

func TestChannelDispatcher_InterceptsManualFlow(t *testing.T) {
	spy := &enqSpy{}
	d := NewChannelDispatcherWithManual(nil, spy, zerolog.Nop())

	rule := entity.AutomationRule{
		TriggerID: "AE_P4_REN90_OPENER",
		Channel:   entity.RuleChannelWhatsApp,
		RuleCode:  "AE_REN90",
	}
	md := entity.MasterData{
		ID:          "md-1",
		WorkspaceID: "ws-1",
		CompanyID:   "ACME-001",
		CompanyName: "Acme",
		OwnerName:   "ae@acme.com",
	}

	if err := d.Dispatch(context.Background(), rule, md); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 enqueue, got %d", len(spy.calls))
	}
	call := spy.calls[0]
	if call.TriggerID != "AE_P4_REN90_OPENER" {
		t.Errorf("trigger_id mismatch: %q", call.TriggerID)
	}
	if call.FlowCategory != "renewal_opener" {
		t.Errorf("flow_category mismatch: %q", call.FlowCategory)
	}
	if call.Priority != entity.ManualActionPriorityP0 {
		t.Errorf("priority mismatch: %q", call.Priority)
	}
	if call.Role != "ae" {
		t.Errorf("role mismatch: %q", call.Role)
	}
	if call.AssignedToUser != "ae@acme.com" {
		t.Errorf("assignee mismatch: %q", call.AssignedToUser)
	}
}

func TestChannelDispatcher_NonManualFallsThrough(t *testing.T) {
	spy := &enqSpy{}
	d := NewChannelDispatcherWithManual(nil, spy, zerolog.Nop())

	rule := entity.AutomationRule{
		TriggerID: "AE_P00_DAILY_STATUS_BOT", // not manual
		Channel:   entity.RuleChannelWhatsApp,
		RuleCode:  "BOT_DAILY",
	}
	md := entity.MasterData{ID: "md-1", WorkspaceID: "ws-1"}

	if err := d.Dispatch(context.Background(), rule, md); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(spy.calls) != 0 {
		t.Errorf("expected no enqueue for non-manual trigger, got %d", len(spy.calls))
	}
}
