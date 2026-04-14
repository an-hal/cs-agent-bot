package entity

import "time"

// TemplateEditLog is an INSERT-only audit record for template field changes.
type TemplateEditLog struct {
	ID           string    `json:"id" db:"id"`
	WorkspaceID  string    `json:"workspace_id" db:"workspace_id"`
	TemplateID   string    `json:"template_id" db:"template_id"`
	TemplateType string    `json:"template_type" db:"template_type"`
	Field        string    `json:"field" db:"field"`
	OldValue     *string   `json:"old_value,omitempty" db:"old_value"`
	NewValue     *string   `json:"new_value,omitempty" db:"new_value"`
	EditedBy     string    `json:"edited_by" db:"edited_by"`
	EditedAt     time.Time `json:"edited_at" db:"edited_at"`
}

const (
	TemplateTypeMessage = "message"
	TemplateTypeEmail   = "email"

	TemplateEditFieldCreated = "created"
	TemplateEditFieldDeleted = "deleted"
)

// TemplateEditLogFilter holds optional filters for listing edit logs.
type TemplateEditLogFilter struct {
	TemplateID   string
	TemplateType string
	Since        *time.Time
	Limit        int
}
