package entity

import "time"

// UserPreference stores a per-user, per-workspace key-value preference.
// The `value` is a free-form JSON object; FE owns its shape.
// Typical namespaces: "theme", "sidebar", "columns.clients", "feed_interval".
type UserPreference struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspace_id"`
	UserEmail   string         `json:"user_email"`
	Namespace   string         `json:"namespace"`
	Value       map[string]any `json:"value"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}
