package entity

import "time"

// MessageTemplate is a workspace-scoped WhatsApp/Telegram template.
// ID is a human-readable string (e.g. "TPL-OB-WELCOME") — not a UUID.
type MessageTemplate struct {
	ID          string    `json:"id" db:"id"`
	WorkspaceID string    `json:"workspace_id" db:"workspace_id"`
	TriggerID   string    `json:"trigger_id" db:"trigger_id"`
	Phase       string    `json:"phase" db:"phase"`
	PhaseLabel  string    `json:"phase_label" db:"phase_label"`
	Channel     string    `json:"channel" db:"channel"`
	Role        string    `json:"role" db:"role"`
	Category    string    `json:"category" db:"category"`
	Action      string    `json:"action" db:"action"`
	Timing      string    `json:"timing" db:"timing"`
	Condition   string    `json:"condition" db:"condition"`
	Message     string    `json:"message" db:"message"`
	Variables   []string  `json:"variables" db:"variables"`
	StopIf      *string   `json:"stop_if,omitempty" db:"stop_if"`
	SentFlag    string    `json:"sent_flag" db:"sent_flag"`
	Priority    *string   `json:"priority,omitempty" db:"priority"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty" db:"updated_at"`
	UpdatedBy   *string   `json:"updated_by,omitempty" db:"updated_by"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// MessageTemplateFilter holds optional filters for listing message templates.
type MessageTemplateFilter struct {
	Role     string
	Phases   []string
	Channel  string
	Category string
	Search   string
}

// Allowed enumerations.
const (
	MsgTplChannelWhatsApp = "whatsapp"
	MsgTplChannelTelegram = "telegram"

	MsgTplRoleSDR = "sdr"
	MsgTplRoleBD  = "bd"
	MsgTplRoleAE  = "ae"
)
