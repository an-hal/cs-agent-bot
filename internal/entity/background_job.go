package entity

import "time"

// BackgroundJob represents an async task (import, export, etc.) tracked in the background_jobs table.
type BackgroundJob struct {
	ID          string         `json:"job_id"`
	WorkspaceID string         `json:"workspace_id"`
	JobType     string         `json:"job_type"`
	Status      string         `json:"status"`
	EntityType  string         `json:"entity_type"`
	Filename    string         `json:"filename,omitempty"`
	StoragePath string         `json:"-"`
	TotalRows   int            `json:"total_rows"`
	Processed   int            `json:"processed"`
	Success     int            `json:"success"`
	Failed      int            `json:"failed"`
	Skipped     int            `json:"skipped"`
	Errors      []JobRowError  `json:"errors,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedBy   string         `json:"created_by"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// JobRowError represents a per-row processing error in an import/export job.
type JobRowError struct {
	Row    int    `json:"row,omitempty"`
	RefID  string `json:"ref_id,omitempty"`
	Reason string `json:"reason"`
}

const (
	JobTypeImport = "import"
	JobTypeExport = "export"
	JobTypeCron   = "cron"

	JobStatusPending    = "pending"
	JobStatusProcessing = "processing"
	JobStatusDone       = "done"
	JobStatusFailed     = "failed"

	JobEntityClient  = "client"
	JobEntityCronRun = "cron_run"
)
