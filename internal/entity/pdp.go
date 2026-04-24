package entity

import "time"

// PDP erasure request lifecycle.
const (
	PDPErasureStatusPending   = "pending"
	PDPErasureStatusApproved  = "approved"
	PDPErasureStatusExecuted  = "executed"
	PDPErasureStatusRejected  = "rejected"
	PDPErasureStatusExpired   = "expired"
)

// PDP retention action types.
const (
	PDPRetentionActionDelete    = "delete"
	PDPRetentionActionAnonymize = "anonymize"
	PDPRetentionActionArchive   = "archive"
)

// PDPErasureRequest is a subject-access / right-to-be-forgotten request.
// Admin reviews, approves, and a cron processor scrubs the subject's PII from
// downstream tables listed in `scope`.
type PDPErasureRequest struct {
	ID               string         `json:"id"`
	WorkspaceID      string         `json:"workspace_id"`
	SubjectEmail     string         `json:"subject_email"`
	SubjectKind      string         `json:"subject_kind"`
	Requester        string         `json:"requester"`
	Reason           string         `json:"reason,omitempty"`
	Scope            []string       `json:"scope"`
	Status           string         `json:"status"`
	RejectionReason  string         `json:"rejection_reason,omitempty"`
	ReviewedBy       string         `json:"reviewed_by,omitempty"`
	ReviewedAt       *time.Time     `json:"reviewed_at,omitempty"`
	ExecutedAt       *time.Time     `json:"executed_at,omitempty"`
	ExecutionSummary map[string]any `json:"execution_summary,omitempty"`
	ExpiresAt        time.Time      `json:"expires_at"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// PDPRetentionPolicy describes the lifespan + disposition of a data class
// (table or logical group) per workspace. A nightly cron sweeps the matching
// rows and deletes/anonymizes/archives based on `action`.
type PDPRetentionPolicy struct {
	ID            string    `json:"id"`
	WorkspaceID   string    `json:"workspace_id"`
	DataClass     string    `json:"data_class"`
	RetentionDays int       `json:"retention_days"`
	Action        string    `json:"action"`
	IsActive      bool      `json:"is_active"`
	LastRunAt     *time.Time `json:"last_run_at,omitempty"`
	LastRunRows   int       `json:"last_run_rows"`
	CreatedBy     string    `json:"created_by,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// PDPErasureRequestFilter for List.
type PDPErasureRequestFilter struct {
	WorkspaceID  string
	Status       string
	SubjectEmail string
	Limit        int
	Offset       int
}
