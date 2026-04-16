package entity

import "time"

// ActivityLog represents a single audit trail entry in the activity_log table.
// It covers all app activity categories: bot, data, and team.
type ActivityLog struct {
	ID           int64     `json:"id"`
	WorkspaceID  string    `json:"workspace_id"`
	Category     string    `json:"category"`
	ActorType    string    `json:"actor_type"`
	Actor        string    `json:"actor"`
	Action       string    `json:"action"`
	Target       string    `json:"target"`
	Detail       string    `json:"detail"`
	RefID        string    `json:"ref_id"`
	ResourceType string    `json:"resource_type,omitempty"`
	Status       string    `json:"status,omitempty"`
	OccurredAt   time.Time `json:"occurred_at"`

	// Actor identity (feat/08)
	ActorName  string `json:"actor_name,omitempty"`
	ActorEmail string `json:"actor_email,omitempty"`

	// Bot-category trace fields (feat/08)
	TriggerID  string `json:"trigger_id,omitempty"`
	TemplateID string `json:"template_id,omitempty"`
	Phase      string `json:"phase,omitempty"`
	Channel    string `json:"channel,omitempty"`
	Replied    bool   `json:"replied,omitempty"`
	ReplyText  string `json:"reply_text,omitempty"`

	// Data-mutation category fields (feat/08)
	CompanyID      string         `json:"company_id,omitempty"`
	CompanyName    string         `json:"company_name,omitempty"`
	ChangedFields  []string       `json:"changed_fields,omitempty"`
	PreviousValues map[string]any `json:"previous_values,omitempty"`
	NewValues      map[string]any `json:"new_values,omitempty"`
	BulkCount      *int           `json:"bulk_count,omitempty"`
	Note           string         `json:"note,omitempty"`

	// Team-category fields (feat/08)
	TargetName  string `json:"target_name,omitempty"`
	TargetEmail string `json:"target_email,omitempty"`
	TargetID    string `json:"target_id,omitempty"`
}

// ActivityStats holds 7 stat counters for the unified feed response.
type ActivityStats struct {
	Total         int `json:"total"`
	Today         int `json:"today"`
	Bot           int `json:"bot"`
	Human         int `json:"human"`
	DataMutations int `json:"data_mutations"`
	TeamActions   int `json:"team_actions"`
	Escalations   int `json:"escalations"`
}

// ActivityFeedResponse wraps the unified feed payload.
type ActivityFeedResponse struct {
	Data  []ActivityLog `json:"data"`
	Meta  interface{}   `json:"meta"`
	Stats ActivityStats `json:"stats"`
}

// CompanySummary aggregates per-company action log data.
type CompanySummary struct {
	CompanyID           string  `json:"company_id"`
	CompanyName         string  `json:"company_name"`
	TotalSent           int     `json:"total_sent"`
	TotalReplied        int     `json:"total_replied"`
	ReplyRate           float64 `json:"reply_rate"`
	LastSentAt          string  `json:"last_sent_at,omitempty"`
	LastTriggerID       string  `json:"last_trigger_id,omitempty"`
	LastStatus          string  `json:"last_status,omitempty"`
	HasActiveEscalation bool    `json:"has_active_escalation"`
	CurrentPhase        string  `json:"current_phase,omitempty"`
}

// ActivityFilter holds query parameters for filtering activity logs.
type ActivityFilter struct {
	WorkspaceIDs []string   // one or more workspace IDs (holding workspaces expand to member IDs)
	Category     string     // empty = all categories
	ResourceType string     // empty = all resource types; non-empty = exact match on resource_type
	RefID        string     // empty = no filter; non-empty = exact match on ref_id
	Since        *time.Time // return entries after this timestamp
	Limit        int
	Offset       int
}

// Activity category constants
const (
	ActivityCategoryBot  = "bot"
	ActivityCategoryData = "data"
	ActivityCategoryTeam = "team"
)

// Activity actor type constants
const (
	ActivityActorBot   = "bot"
	ActivityActorHuman = "human"
)

// Activity resource type constants — used to power per-module activity feeds.
const (
	ActivityResourceClient       = "client"
	ActivityResourceInvoice      = "invoice"
	ActivityResourceTemplate     = "template"
	ActivityResourceTriggerRule  = "trigger_rule"
	ActivityResourceSystemConfig = "system_config"
	ActivityResourceBot          = "bot"
	ActivityResourceTeam         = "team"
)
