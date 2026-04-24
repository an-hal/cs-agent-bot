package entity

import "time"

// Extraction lifecycle.
const (
	FirefliesStatusPending   = "pending"
	FirefliesStatusRunning   = "running"
	FirefliesStatusSucceeded = "succeeded"
	FirefliesStatusFailed    = "failed"
)

// FirefliesTranscript stores Fireflies webhook payloads + extraction state.
type FirefliesTranscript struct {
	ID               string         `json:"id"`
	WorkspaceID      string         `json:"workspace_id"`
	FirefliesID      string         `json:"fireflies_id"`
	MeetingTitle     string         `json:"meeting_title,omitempty"`
	MeetingDate      *time.Time     `json:"meeting_date,omitempty"`
	DurationSeconds  int            `json:"duration_seconds"`
	HostEmail        string         `json:"host_email,omitempty"`
	Participants     []string       `json:"participants"`
	TranscriptText   string         `json:"transcript_text,omitempty"`
	RawPayload       map[string]any `json:"raw_payload,omitempty"`
	ExtractionStatus string         `json:"extraction_status"`
	ExtractionError  string         `json:"extraction_error,omitempty"`
	ExtractedAt      *time.Time     `json:"extracted_at,omitempty"`
	MasterDataID     string         `json:"master_data_id,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}
