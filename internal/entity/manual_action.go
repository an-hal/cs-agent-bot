package entity

import "time"

// Manual action queue statuses.
const (
	ManualActionStatusPending    = "pending"
	ManualActionStatusInProgress = "in_progress"
	ManualActionStatusSent       = "sent"
	ManualActionStatusSkipped    = "skipped"
	ManualActionStatusExpired    = "expired"
)

// Manual action priority tiers (not enforced by DB, used for UX grouping).
const (
	ManualActionPriorityP0 = "P0"
	ManualActionPriorityP1 = "P1"
	ManualActionPriorityP2 = "P2"
)

// Manual action channels (sent_channel values).
const (
	ManualActionChannelWA      = "wa"
	ManualActionChannelEmail   = "email"
	ManualActionChannelCall    = "call"
	ManualActionChannelMeeting = "meeting"
)

// ManualAction is a reminder row that the bot inserts when a human-composition
// trigger fires (see 06-workflow-engine/07-manual-flows.md). FE shows the
// suggested_draft and context_summary; user composes + sends via personal
// channels, then marks the row sent or skipped.
type ManualAction struct {
	ID             string         `json:"id"`
	WorkspaceID    string         `json:"workspace_id"`
	MasterDataID   string         `json:"master_data_id"`
	TriggerID      string         `json:"trigger_id"`
	FlowCategory   string         `json:"flow_category"`
	Role           string         `json:"role"`
	AssignedToUser string         `json:"assigned_to_user"`
	SuggestedDraft string         `json:"suggested_draft"`
	ContextSummary map[string]any `json:"context_summary"`
	Status         string         `json:"status"`
	Priority       string         `json:"priority"`
	DueAt          time.Time      `json:"due_at"`
	SentAt         *time.Time     `json:"sent_at,omitempty"`
	SentChannel    string         `json:"sent_channel,omitempty"`
	ActualMessage  string         `json:"actual_message,omitempty"`
	SkippedReason  string         `json:"skipped_reason,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// ManualActionFilter is used by List queries.
type ManualActionFilter struct {
	WorkspaceID    string
	Status         string // empty = any
	AssignedToUser string // empty = any
	Role           string // empty = any
	Priority       string // empty = any
	FlowCategory   string // empty = any
	Limit          int
	Offset         int
}
