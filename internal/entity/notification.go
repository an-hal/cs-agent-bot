package entity

import "time"

// Notification is a cross-cutting in-app/email/telegram notification record
// scoped to a workspace and a recipient (by email).
type Notification struct {
	ID             string     `json:"id"`
	WorkspaceID    string     `json:"workspace_id"`
	RecipientEmail string     `json:"recipient_email"`
	Type           string     `json:"type"`
	Icon           string     `json:"icon"`
	Message        string     `json:"message"`
	Href           string     `json:"href"`
	SourceFeature  string     `json:"source_feature"`
	SourceID       string     `json:"source_id"`
	Read           bool       `json:"read"`
	ReadAt         *time.Time `json:"read_at,omitempty"`
	TelegramSent   bool       `json:"telegram_sent"`
	EmailSent      bool       `json:"email_sent"`
	CreatedAt      time.Time  `json:"created_at"`
}

// NotificationFilter filters the notification list endpoint.
type NotificationFilter struct {
	WorkspaceID    string
	RecipientEmail string
	UnreadOnly     bool
	Type           string
	Limit          int
	Offset         int
}
