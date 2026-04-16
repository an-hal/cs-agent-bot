package entity

import "time"

// EmailTemplate is a workspace-scoped email template with HTML body.
type EmailTemplate struct {
	ID          string     `json:"id" db:"id"`
	WorkspaceID string     `json:"workspace_id" db:"workspace_id"`
	Name        string     `json:"name" db:"name"`
	Role        string     `json:"role" db:"role"`
	Category    string     `json:"category" db:"category"`
	Status      string     `json:"status" db:"status"`
	Subject     string     `json:"subject" db:"subject"`
	BodyHTML    string     `json:"body_html" db:"body_html"`
	Variables   []string   `json:"variables" db:"variables"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty" db:"updated_at"`
	UpdatedBy   *string    `json:"updated_by,omitempty" db:"updated_by"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
}

// EmailTemplateFilter holds optional filters for listing email templates.
type EmailTemplateFilter struct {
	Role     string
	Category string
	Status   string
	Search   string
}

const (
	EmailTplStatusActive   = "active"
	EmailTplStatusDraft    = "draft"
	EmailTplStatusArchived = "archived"
)
