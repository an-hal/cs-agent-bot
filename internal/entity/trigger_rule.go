package entity

import (
	"encoding/json"
	"time"
)

// TriggerRule represents a dynamic trigger rule loaded from the database.
// Each rule replaces one hardcoded if-block in the trigger evaluators.
type TriggerRule struct {
	RuleID       string          `json:"rule_id" db:"rule_id"`
	RuleGroup    string          `json:"rule_group" db:"rule_group"`
	Priority     int             `json:"priority" db:"priority"`
	SubPriority  int             `json:"sub_priority" db:"sub_priority"`
	Condition    json.RawMessage `json:"condition" db:"condition"`
	ActionType   string          `json:"action_type" db:"action_type"`
	TemplateID   *string         `json:"template_id,omitempty" db:"template_id"`
	FlagKey      string          `json:"flag_key" db:"flag_key"`
	EscalationID *string         `json:"escalation_id,omitempty" db:"escalation_id"`
	EscPriority  *string         `json:"esc_priority,omitempty" db:"esc_priority"`
	EscReason    *string         `json:"esc_reason,omitempty" db:"esc_reason"`
	ExtraFlags   json.RawMessage `json:"extra_flags,omitempty" db:"extra_flags"`
	StopOnFire   bool            `json:"stop_on_fire" db:"stop_on_fire"`
	Active       bool            `json:"active" db:"active"`
	Description  *string         `json:"description,omitempty" db:"description"`
	WorkspaceID  *string         `json:"workspace_id,omitempty" db:"workspace_id"`
	CreatedAt    time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at" db:"updated_at"`
}

// TriggerRuleFilter holds optional filters for listing trigger rules.
type TriggerRuleFilter struct {
	RuleGroup   string
	ActionType  string
	Active      *bool
	WorkspaceID string
	Search      string // ILIKE across rule_id, rule_group, description
}

// Rule group constants
const (
	RuleGroupHealth      = "HEALTH"
	RuleGroupCheckIn     = "CHECKIN"
	RuleGroupNegotiation = "NEGOTIATION"
	RuleGroupInvoice     = "INVOICE"
	RuleGroupOverdue     = "OVERDUE"
	RuleGroupExpansion   = "EXPANSION"
	RuleGroupCrossSell   = "CROSS_SELL"
)

// Action type constants
const (
	ActionSendWA         = "send_wa"
	ActionSendEmail      = "send_email"
	ActionEscalate       = "escalate"
	ActionAlertTelegram  = "alert_telegram"
	ActionCreateInvoice  = "create_invoice"
	ActionSkipAndSetFlag = "skip_and_set_flag"
)

// GetExtraFlags parses the extra_flags JSONB into a map of flag keys to set.
func (r *TriggerRule) GetExtraFlags() map[string]bool {
	if r.ExtraFlags == nil {
		return nil
	}
	var flags map[string]bool
	if err := json.Unmarshal(r.ExtraFlags, &flags); err != nil {
		return nil
	}
	return flags
}
