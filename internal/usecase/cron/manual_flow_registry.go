package cron

import (
	"context"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/rs/zerolog"
)

// ManualFlowTriggers maps each manual-flow trigger_id to a UX category.
// Canonical list from 06-workflow-engine/07-manual-flows.md §"20 Flow Inventory".
// Adding a trigger here changes routing at the next cron run — no deploys.
var ManualFlowTriggers = map[string]string{
	"SDR_BANT_QUALIFY_REVIEW":   "bant_qualification",
	"SDR_ENTERPRISE_FOLLOWUP":   "enterprise_personalisation",
	"BD_D10_DM_ESCALATION":      "internal_politics_escalation",
	"BD_D14_FINAL_CHECKIN":      "final_check_in",
	"BD_AE_HANDOFF_INTRO":       "bd_ae_handoff",
	"AE_P02_CHECKIN_D14":        "onboarding_checkin",
	"AE_P22_CALL_INVITE":        "warmup_call_invite",
	"AE_P33_REFERRAL_ASK":       "referral_pitch",
	"AE_P4_REN90_OPENER":        "renewal_opener",
	"AE_P42_REN_CALL":           "renewal_call_invite",
	"AE_P45_REN60":              "renewal_followup",
	"AE_P47_REN45_DECIDE":       "renewal_decision",
	"AE_P6_OVERDUE_D8":          "overdue_empathy",
	"AE_P6_OVERDUE_D15":         "overdue_final",
	"ADMIN_PRICING_EDIT":        "admin_pricing_edit",
	"ADMIN_BLACKLIST_EDIT":      "admin_blacklist_edit",
	"BD_DM_ABSENT_FOLLOWUP_A":   "bd_dm_absent",
	"BD_DM_ABSENT_FOLLOWUP_B":   "bd_dm_absent",
	"BD_DM_ABSENT_FOLLOWUP_C":   "bd_dm_absent",
	"BD_DM_ABSENT_FOLLOWUP_D":   "bd_dm_absent",
}

// IsManualFlow reports whether the trigger_id should enqueue a manual action
// instead of firing an automated send.
func IsManualFlow(triggerID string) bool {
	_, ok := ManualFlowTriggers[triggerID]
	return ok
}

// manualFlowPriority maps trigger_id to P0/P1/P2. Default P2.
// Rules of thumb: renewal + overdue late stages are P0, admin approvals P0,
// first-touch invitations P2, mid-sequence nudges P1.
func manualFlowPriority(triggerID string) string {
	switch triggerID {
	case "AE_P4_REN90_OPENER", "AE_P6_OVERDUE_D15", "AE_P47_REN45_DECIDE",
		"ADMIN_PRICING_EDIT", "ADMIN_BLACKLIST_EDIT":
		return entity.ManualActionPriorityP0
	case "AE_P42_REN_CALL", "AE_P45_REN60", "AE_P6_OVERDUE_D8",
		"BD_D10_DM_ESCALATION", "BD_D14_FINAL_CHECKIN":
		return entity.ManualActionPriorityP1
	}
	return entity.ManualActionPriorityP2
}

// manualFlowRole derives role from trigger_id prefix.
func manualFlowRole(triggerID string) string {
	switch {
	case strings.HasPrefix(triggerID, "SDR_"):
		return "sdr"
	case strings.HasPrefix(triggerID, "BD_"):
		return "bd"
	case strings.HasPrefix(triggerID, "AE_"):
		return "ae"
	case strings.HasPrefix(triggerID, "ADMIN_"):
		return "admin"
	}
	return "ae"
}

// ManualActionEnqueuer is the external port the dispatcher uses to create a
// manual_action_queue row. It is intentionally minimal — concrete impl lives
// in the manual_action usecase; wiring is done in main.go.
type ManualActionEnqueuer interface {
	Enqueue(ctx context.Context, in ManualActionEnqueueInput) error
}

// ManualActionEnqueueInput is a thin DTO — narrower than the usecase request
// so cron layer doesn't depend on the manual_action package.
type ManualActionEnqueueInput struct {
	WorkspaceID    string
	MasterDataID   string
	TriggerID      string
	FlowCategory   string
	Role           string
	AssignedToUser string
	Priority       string
	DueAt          time.Time
	SuggestedDraft string
	ContextSummary map[string]any
}

// buildManualActionInput compiles the queue input from a rule + master_data.
// Used by the dispatcher (and reusable by tests) to keep the mapping logic
// in one place.
func buildManualActionInput(rule entity.AutomationRule, md entity.MasterData, draft string, ctxSummary map[string]any) ManualActionEnqueueInput {
	// OwnerName is the assignee identifier (email or display name, per
	// workspace convention). FE stores the email when available.
	owner := md.OwnerName
	category := ManualFlowTriggers[rule.TriggerID]
	// Default to fire immediately — real cron timing parser owns due_at; this
	// is a safe fallback if the rule doesn't provide one.
	due := time.Now().UTC().Add(24 * time.Hour)
	return ManualActionEnqueueInput{
		WorkspaceID:    md.WorkspaceID,
		MasterDataID:   md.ID,
		TriggerID:      rule.TriggerID,
		FlowCategory:   category,
		Role:           manualFlowRole(rule.TriggerID),
		AssignedToUser: owner,
		Priority:       manualFlowPriority(rule.TriggerID),
		DueAt:          due,
		SuggestedDraft: draft,
		ContextSummary: ctxSummary,
	}
}

// logManualFlowSkip is used when an enqueuer is not configured — we log and
// no-op so the bot never accidentally sends a manual-flow template.
func logManualFlowSkip(logger zerolog.Logger, rule entity.AutomationRule, md entity.MasterData) {
	logger.Warn().
		Str("trigger_id", rule.TriggerID).
		Str("company_id", md.CompanyID).
		Msg("manual-flow trigger hit but no enqueuer wired — skipping send")
}
