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
	ActivityResourceClient      = "client"
	ActivityResourceInvoice     = "invoice"
	ActivityResourceTemplate    = "template"
	ActivityResourceTriggerRule = "trigger_rule"
	ActivityResourceBot         = "bot"
	ActivityResourceTeam        = "team"
)
