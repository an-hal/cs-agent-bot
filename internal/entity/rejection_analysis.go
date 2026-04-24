package entity

import "time"

// Rejection categories (canonical list; workspace can add custom categories
// via system_config if needed).
const (
	RejectionCategoryPrice     = "price"
	RejectionCategoryAuthority = "authority"
	RejectionCategoryTiming    = "timing"
	RejectionCategoryFeature   = "feature"
	RejectionCategoryTone      = "tone"
	RejectionCategoryOther     = "other"
)

// Severity.
const (
	RejectionSeverityLow  = "low"
	RejectionSeverityMid  = "mid"
	RejectionSeverityHigh = "high"
)

// Analyst source.
const (
	RejectionAnalystRule   = "rule"
	RejectionAnalystClaude = "claude"
	RejectionAnalystHuman  = "human"
)

// RejectionAnalysis captures a single rejection reply + its classification.
type RejectionAnalysis struct {
	ID                string    `json:"id"`
	WorkspaceID       string    `json:"workspace_id"`
	MasterDataID      string    `json:"master_data_id"`
	SourceChannel     string    `json:"source_channel"`
	SourceMessage     string    `json:"source_message,omitempty"`
	RejectionCategory string    `json:"rejection_category,omitempty"`
	Severity          string    `json:"severity"`
	AnalysisSummary   string    `json:"analysis_summary,omitempty"`
	SuggestedResponse string    `json:"suggested_response,omitempty"`
	Analyst           string    `json:"analyst"`
	AnalystVersion    string    `json:"analyst_version,omitempty"`
	DetectedAt        time.Time `json:"detected_at"`
	CreatedAt         time.Time `json:"created_at"`
}

type RejectionAnalysisFilter struct {
	WorkspaceID       string
	MasterDataID      string
	RejectionCategory string
	Limit             int
	Offset            int
}
