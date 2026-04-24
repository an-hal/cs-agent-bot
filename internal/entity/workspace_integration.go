package entity

import "time"

// Supported integration providers.
const (
	IntegrationProviderHaloAI   = "haloai"
	IntegrationProviderTelegram = "telegram"
	IntegrationProviderPaperID  = "paper_id"
	IntegrationProviderSMTP     = "smtp"
)

// WorkspaceIntegration holds per-workspace credentials/config for a third-party provider.
// The `Config` shape is provider-specific. Secret fields (keys containing
// "token", "secret", "password", "api_key", "key") are redacted on read.
type WorkspaceIntegration struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspace_id"`
	Provider    string         `json:"provider"`
	DisplayName string         `json:"display_name"`
	Config      map[string]any `json:"config"`
	IsActive    bool           `json:"is_active"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	CreatedBy   string         `json:"created_by,omitempty"`
	UpdatedBy   string         `json:"updated_by,omitempty"`
}
