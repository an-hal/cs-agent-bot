package entity

import "time"

// ActivityLog represents a single audit trail entry in the activity_log table.
// It covers all app activity categories: bot, data, and team.
type ActivityLog struct {
	ID          int64     `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Category    string    `json:"category"`
	ActorType   string    `json:"actor_type"`
	Actor       string    `json:"actor"`
	Action      string    `json:"action"`
	Target      string    `json:"target"`
	Detail      string    `json:"detail"`
	RefID       string    `json:"ref_id"`
	Status      string    `json:"status,omitempty"`
	OccurredAt  time.Time `json:"occurred_at"`
}

// ActivityFilter holds query parameters for filtering activity logs.
type ActivityFilter struct {
	WorkspaceID string
	Category    string     // empty = all categories
	Since       *time.Time // return entries after this timestamp
	Limit       int
	Offset      int
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
