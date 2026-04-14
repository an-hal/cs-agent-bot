// Package entity contains domain types for automation rules (feat/06).
package entity

import "time"

// ─── Rule enums ───────────────────────────────────────────────────────────────

// RuleStatus is the execution state of an automation rule.
type RuleStatus string

const (
	RuleStatusActive   RuleStatus = "active"
	RuleStatusPaused   RuleStatus = "paused"
	RuleStatusDisabled RuleStatus = "disabled"
)

// RuleChannel is the delivery channel for an automation rule.
type RuleChannel string

const (
	RuleChannelWhatsApp RuleChannel = "whatsapp"
	RuleChannelEmail    RuleChannel = "email"
	RuleChannelTelegram RuleChannel = "telegram"
)

// RuleRole is the pipeline role associated with a rule.
type RuleRole string

const (
	RuleRoleSDR RuleRole = "sdr"
	RuleRoleBD  RuleRole = "bd"
	RuleRoleAE  RuleRole = "ae"
	RuleRoleCS  RuleRole = "cs"
)

// ─── AutomationRule ───────────────────────────────────────────────────────────

// AutomationRule is an executable rule evaluated by the workflow cron engine.
// Rules are workspace-scoped and referenced by workflow nodes via trigger_id.
type AutomationRule struct {
	ID          string `db:"id"           json:"id"`
	WorkspaceID string `db:"workspace_id" json:"workspace_id"`

	RuleCode   string  `db:"rule_code"   json:"rule_code"`
	TriggerID  string  `db:"trigger_id"  json:"trigger_id"`
	TemplateID *string `db:"template_id" json:"template_id"`

	Role       RuleRole `db:"role"        json:"role"`
	Phase      string   `db:"phase"       json:"phase"`
	PhaseLabel *string  `db:"phase_label" json:"phase_label"`
	Priority   *string  `db:"priority"    json:"priority"`

	Timing    string      `db:"timing"    json:"timing"`
	Condition string      `db:"condition" json:"condition"`
	StopIf    string      `db:"stop_if"   json:"stop_if"`
	SentFlag  *string     `db:"sent_flag" json:"sent_flag"`
	Channel   RuleChannel `db:"channel"   json:"channel"`

	Status RuleStatus `db:"status" json:"status"`

	UpdatedAt *time.Time `db:"updated_at" json:"updated_at"`
	UpdatedBy *string    `db:"updated_by" json:"updated_by"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
}

// IsExecutable reports whether the rule should be evaluated by the cron engine.
func (r *AutomationRule) IsExecutable() bool {
	return r.Status == RuleStatusActive
}

// ─── RuleChangeLog ────────────────────────────────────────────────────────────

// RuleChangeLog is an append-only audit entry for automation rule edits.
// The underlying table has UPDATE and DELETE revoked via SQL migration.
type RuleChangeLog struct {
	ID          string  `db:"id"           json:"id"`
	RuleID      string  `db:"rule_id"      json:"rule_id"`
	WorkspaceID string  `db:"workspace_id" json:"workspace_id"`
	Field       string  `db:"field"        json:"field"`
	OldValue    *string `db:"old_value"    json:"old_value"`
	NewValue    string  `db:"new_value"    json:"new_value"`
	EditedBy    string  `db:"edited_by"    json:"edited_by"`
	EditedAt    time.Time `db:"edited_at"  json:"edited_at"`
}

// RuleChangeLogWithCode extends RuleChangeLog with the rule_code for display.
type RuleChangeLogWithCode struct {
	RuleChangeLog
	RuleCode string `db:"rule_code" json:"rule_code"`
}

// ─── Filters ──────────────────────────────────────────────────────────────────

// AutomationRuleFilter holds query parameters for listing automation rules.
type AutomationRuleFilter struct {
	Role   string
	Status string
	Phase  string
	Search string
}
