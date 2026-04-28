package entity

import "time"

// Import session lifecycle states.
const (
	ImportSessionStatusPending   = "pending"
	ImportSessionStatusSubmitted = "submitted"
	ImportSessionStatusExpired   = "expired"
)

// ImportSession is the wizard-side scratchpad for an in-progress bulk import.
// Phase C of the OneSchema-style flow: a maker uploads a file, gets a session,
// edits bad cells via PATCH, then Submits to create the actual approval. The
// file (base64) and final mapping + per-cell overrides live here so the
// parser can reproduce the maker's intent on every read.
type ImportSession struct {
	ID            string                       `json:"id"`
	WorkspaceID   string                       `json:"workspace_id"`
	CreatedBy     string                       `json:"created_by"`
	Status        string                       `json:"status"`
	FileName      string                       `json:"file_name"`
	FileB64       string                       `json:"file_b64,omitempty"` // omitempty so list responses don't carry full payload
	SheetName     string                       `json:"sheet_name"`
	Mode          string                       `json:"mode"`
	Mapping       map[string]string            `json:"mapping"`
	CellOverrides map[string]map[string]string `json:"cell_overrides"` // {row_num_str: {target_key: corrected_value}}
	ApprovalID    string                       `json:"approval_id,omitempty"`
	CreatedAt     time.Time                    `json:"created_at"`
	UpdatedAt     time.Time                    `json:"updated_at"`
	ExpiresAt     time.Time                    `json:"expires_at"`
}
