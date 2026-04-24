package entity

import "time"

// Built-in reactivation trigger codes. Workspaces can add custom codes too.
const (
	ReactivationCodePriceChange  = "price_change"
	ReactivationCodeNewFeature   = "new_feature"
	ReactivationCodeAnniversary  = "anniversary"
	ReactivationCodeManual       = "manual"
)

// Reactivation event outcomes.
const (
	ReactivationOutcomeSent    = "sent"
	ReactivationOutcomeSkipped = "skipped"
	ReactivationOutcomeReplied = "replied"
	ReactivationOutcomeBounced = "bounced"
)

// ReactivationTrigger is a workspace-level rule that decides when a dormant
// client is re-engaged. The `Condition` is evaluated by pkg/conditiondsl.
type ReactivationTrigger struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspace_id"`
	Code         string    `json:"code"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	Condition    string    `json:"condition"`
	TemplateCode string    `json:"template_code,omitempty"`
	IsActive     bool      `json:"is_active"`
	CreatedBy    string    `json:"created_by,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ReactivationEvent records a single firing — which trigger fired for which
// client + outcome. Used for rate-limiting and auditing.
type ReactivationEvent struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspace_id"`
	TriggerID    string    `json:"trigger_id"`
	MasterDataID string    `json:"master_data_id"`
	FiredAt      time.Time `json:"fired_at"`
	Outcome      string    `json:"outcome"`
	Note         string    `json:"note,omitempty"`
}
