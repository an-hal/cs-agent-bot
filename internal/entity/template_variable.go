package entity

import "time"

// TemplateVariable defines a variable available for template substitution.
type TemplateVariable struct {
	ID           string    `json:"id" db:"id"`
	WorkspaceID  string    `json:"workspace_id" db:"workspace_id"`
	VariableKey  string    `json:"variable_key" db:"variable_key"`
	DisplayLabel string    `json:"display_label" db:"display_label"`
	SourceType   string    `json:"source_type" db:"source_type"`
	SourceField  *string   `json:"source_field,omitempty" db:"source_field"`
	Description  *string   `json:"description,omitempty" db:"description"`
	ExampleValue *string   `json:"example_value,omitempty" db:"example_value"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}
