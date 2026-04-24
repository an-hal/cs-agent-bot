package entity

import "time"

// Claude extraction lifecycle.
const (
	ClaudeExtractionStatusPending    = "pending"
	ClaudeExtractionStatusRunning    = "running"
	ClaudeExtractionStatusSucceeded  = "succeeded"
	ClaudeExtractionStatusFailed     = "failed"
	ClaudeExtractionStatusSuperseded = "superseded"
)

// Source type tags.
const (
	ClaudeSourceFireflies  = "fireflies"
	ClaudeSourceManualNote = "manual_note"
	ClaudeSourceEmail      = "email"
)

// BANTS classification buckets.
const (
	BANTSHot  = "hot"
	BANTSWarm = "warm"
	BANTSCold = "cold"
)

// ClaudeExtraction stores a single Claude extraction attempt (stage-1 fields
// + stage-2 BANTS). One row per attempt; retries create new rows and mark
// prior ones `superseded`.
type ClaudeExtraction struct {
	ID                   string         `json:"id"`
	WorkspaceID          string         `json:"workspace_id"`
	SourceType           string         `json:"source_type"`
	SourceID             string         `json:"source_id"`
	MasterDataID         string         `json:"master_data_id,omitempty"`
	ExtractedFields      map[string]any `json:"extracted_fields"`
	ExtractionPrompt     string         `json:"extraction_prompt,omitempty"`
	ExtractionModel      string         `json:"extraction_model,omitempty"`
	BANTSBudget          *int           `json:"bants_budget,omitempty"`
	BANTSAuthority       *int           `json:"bants_authority,omitempty"`
	BANTSNeed            *int           `json:"bants_need,omitempty"`
	BANTSTiming          *int           `json:"bants_timing,omitempty"`
	BANTSSentiment       *int           `json:"bants_sentiment,omitempty"`
	BANTSScore           *float64       `json:"bants_score,omitempty"`
	BANTSClassification  string         `json:"bants_classification,omitempty"`
	BuyingIntent         string         `json:"buying_intent,omitempty"`
	CoachingNotes        string         `json:"coaching_notes,omitempty"`
	Status               string         `json:"status"`
	ErrorMessage         string         `json:"error_message,omitempty"`
	PromptTokens         int            `json:"prompt_tokens"`
	CompletionTokens     int            `json:"completion_tokens"`
	LatencyMS            int            `json:"latency_ms"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}
