package auth

import (
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

// WhitelistEntry re-exports entity.WhitelistEntry so swag can resolve the type
// from within this package without requiring a runtime import in the handler.
type WhitelistEntry = entity.WhitelistEntry

// LoginRequest is the request body for POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email"    example:"user@dealls.com"`
	Password string `json:"password" example:"secret123"`
}

// LoginResponse is the data payload returned on a successful email/password login.
type LoginResponse struct {
	AccessToken string    `json:"access_token" example:"eyJhbGci..."`
	Expires     time.Time `json:"expires"`
	Provider    string    `json:"provider"     example:"email"`
	Admin       bool      `json:"admin"`
	User        any       `json:"user"`
}

// GoogleLoginRequest is the request body for POST /auth/google.
type GoogleLoginRequest struct {
	Credential string `json:"credential" example:"eyJhbGci..."`
}

// GoogleLoginResponse is the data payload returned on a successful Google login.
type GoogleLoginResponse struct {
	User     any    `json:"user"`
	Admin    bool   `json:"admin"`
	Provider string `json:"provider" example:"google"`
}

// AddWhitelistRequest is the request body for POST /whitelist.
type AddWhitelistRequest struct {
	Email string `json:"email" example:"user@dealls.com"`
	Notes string `json:"notes" example:"Added by admin"`
}

// WhitelistCheckResponse is the response payload for GET /whitelist/check.
type WhitelistCheckResponse struct {
	Allowed bool `json:"allowed"`
}

// DeleteWhitelistResponse is the response payload for DELETE /whitelist/{id}.
type DeleteWhitelistResponse struct {
	ID string `json:"id" example:"abc123"`
}
