package entity

import "time"

// WhitelistEntry represents a row in the whitelist table that gates dashboard access.
// Authentication itself is delegated to ms-auth-proxy; this table only decides which
// authenticated users may access the dashboard.
type WhitelistEntry struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	IsActive  bool      `json:"is_active"`
	AddedBy   string    `json:"added_by,omitempty"`
	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
