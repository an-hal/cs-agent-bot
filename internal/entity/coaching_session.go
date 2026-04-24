package entity

import "time"

const (
	CoachingTypePeerReview    = "peer_review"
	CoachingTypeSelfReview    = "self_review"
	CoachingTypeManagerReview = "manager_review"
)

const (
	CoachingStatusDraft     = "draft"
	CoachingStatusSubmitted = "submitted"
	CoachingStatusReviewed  = "reviewed"
)

// CoachingSession captures a peer-review or manager-review critique of a BD's
// discovery/renewal conversation. Scoring fields are 1–5 scale; nil until the
// reviewer submits.
type CoachingSession struct {
	ID                    string     `json:"id"`
	WorkspaceID           string     `json:"workspace_id"`
	BDEmail               string     `json:"bd_email"`
	CoachEmail            string     `json:"coach_email"`
	MasterDataID          string     `json:"master_data_id,omitempty"`
	ClaudeExtractionID    string     `json:"claude_extraction_id,omitempty"`
	SessionType           string     `json:"session_type"`
	SessionDate           time.Time  `json:"session_date"`
	BANTSClarityScore     *int       `json:"bants_clarity_score,omitempty"`
	DiscoveryDepthScore   *int       `json:"discovery_depth_score,omitempty"`
	ToneFitScore          *int       `json:"tone_fit_score,omitempty"`
	NextStepClarityScore  *int       `json:"next_step_clarity_score,omitempty"`
	OverallScore          *float64   `json:"overall_score,omitempty"`
	Strengths             string     `json:"strengths,omitempty"`
	Improvements          string     `json:"improvements,omitempty"`
	ActionItems           string     `json:"action_items,omitempty"`
	Status                string     `json:"status"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

// CoachingSessionFilter is used by List.
type CoachingSessionFilter struct {
	WorkspaceID string
	BDEmail     string
	CoachEmail  string
	Status      string
	Limit       int
	Offset      int
}
