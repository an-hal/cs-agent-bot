package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/rs/zerolog"
)

// ErrNotWhitelisted is returned when an authenticated user is not on the dashboard whitelist.
var ErrNotWhitelisted = errors.New("auth: email is not whitelisted")

// LoginResult captures the data needed to set a session cookie + return user info.
type LoginResult struct {
	SessionToken string
	ExpiresAt    time.Time
	User         map[string]interface{}
	Provider     string
	IsAdmin      bool
}

// AuthUsecase coordinates login flows: whitelist check, upstream proxy, session minting.
type AuthUsecase interface {
	LoginEmailPassword(ctx context.Context, email, password string) (*LoginResult, error)
	LoginGoogle(ctx context.Context, credential string) (*LoginResult, error)

	IsWhitelisted(ctx context.Context, email string) (bool, error)
	ListWhitelist(ctx context.Context) ([]entity.WhitelistEntry, error)
	AddWhitelist(ctx context.Context, email, addedBy, notes string) (*entity.WhitelistEntry, error)
	RemoveWhitelist(ctx context.Context, id string) error
}

type authUsecase struct {
	whitelist     repository.WhitelistRepository
	proxy         AuthProxyClient
	google        GoogleTokenVerifier
	sessionSecret string
	logger        zerolog.Logger
}

// NewAuthUsecase wires the auth usecase. proxy and google may be nil in tests
// that only exercise whitelist CRUD.
func NewAuthUsecase(
	whitelist repository.WhitelistRepository,
	proxy AuthProxyClient,
	google GoogleTokenVerifier,
	sessionSecret string,
	logger zerolog.Logger,
) AuthUsecase {
	return &authUsecase{
		whitelist:     whitelist,
		proxy:         proxy,
		google:        google,
		sessionSecret: sessionSecret,
		logger:        logger,
	}
}

// LoginEmailPassword runs the email/password flow:
//  1. validate inputs
//  2. delegate to ms-auth-proxy (FIRST — to prevent whitelist email enumeration)
//  3. whitelist gate (DB)
//  4. return cookie token + user
//
// Order matters: calling the proxy first ensures attackers cannot probe which
// emails are on the whitelist by observing 403 vs 401 responses. Both invalid
// credentials and non-whitelisted accounts yield ErrInvalidCredentials.
func (u *authUsecase) LoginEmailPassword(ctx context.Context, email, password string) (*LoginResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	if u.proxy == nil {
		return nil, errors.New("auth: proxy client not configured")
	}

	resp, err := u.proxy.Login(ctx, email, password)
	if err != nil {
		return nil, err
	}

	allowed, err := u.whitelist.IsAllowed(ctx, email)
	if err != nil {
		return nil, err
	}
	if !allowed {
		// Mask as invalid credentials so non-whitelisted accounts are
		// indistinguishable from wrong passwords to the caller.
		return nil, ErrInvalidCredentials
	}

	expiresAt := parseExpiry(resp.Expire)
	return &LoginResult{
		SessionToken: resp.AccessToken,
		ExpiresAt:    expiresAt,
		User:         resp.User,
		Provider:     "email",
		IsAdmin:      isAdminFromProxyUser(resp.User),
	}, nil
}

// LoginGoogle runs the Google OAuth flow:
//  1. verify Google credential
//  2. whitelist gate (DB)
//  3. mint HMAC session token
func (u *authUsecase) LoginGoogle(ctx context.Context, credential string) (*LoginResult, error) {
	if u.google == nil {
		return nil, errors.New("auth: google verifier not configured")
	}
	info, err := u.google.Verify(ctx, credential)
	if err != nil {
		return nil, err
	}

	allowed, err := u.whitelist.IsAllowed(ctx, info.Email)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrNotWhitelisted
	}

	now := time.Now()
	token, err := SignGoogleSession(u.sessionSecret, info.Sub, now)
	if err != nil {
		return nil, err
	}
	user := map[string]interface{}{
		"_id":     info.Sub,
		"email":   info.Email,
		"name":    info.Name,
		"picture": info.Picture,
	}
	return &LoginResult{
		SessionToken: token,
		ExpiresAt:    now.Add(GoogleSessionTTL),
		User:         user,
		Provider:     "google",
		IsAdmin:      false,
	}, nil
}

// IsWhitelisted exposes the whitelist gate to the public probe endpoint.
func (u *authUsecase) IsWhitelisted(ctx context.Context, email string) (bool, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return false, nil
	}
	return u.whitelist.IsAllowed(ctx, email)
}

// isAdminFromProxyUser inspects the proxy /login response payload for an
// admin role marker. Falls back to false when no recognisable signal exists.
func isAdminFromProxyUser(user map[string]interface{}) bool {
	if user == nil {
		return false
	}
	if v, ok := user["is_admin"].(bool); ok {
		return v
	}
	if v, ok := user["isAdmin"].(bool); ok {
		return v
	}
	switch roles := user["roles"].(type) {
	case []interface{}:
		for _, r := range roles {
			if s, ok := r.(string); ok && strings.EqualFold(s, "admin") {
				return true
			}
		}
	case []string:
		for _, s := range roles {
			if strings.EqualFold(s, "admin") {
				return true
			}
		}
	}
	return false
}

func (u *authUsecase) ListWhitelist(ctx context.Context) ([]entity.WhitelistEntry, error) {
	out, err := u.whitelist.List(ctx)
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []entity.WhitelistEntry{}
	}
	return out, nil
}

func (u *authUsecase) AddWhitelist(ctx context.Context, email, addedBy, notes string) (*entity.WhitelistEntry, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, errors.New("auth: email is required")
	}
	return u.whitelist.Create(ctx, email, addedBy, notes)
}

func (u *authUsecase) RemoveWhitelist(ctx context.Context, id string) error {
	return u.whitelist.Delete(ctx, id)
}

// parseExpiry tries common formats from the proxy: RFC3339 string, or unix
// milliseconds as a string. Returns zero time on failure.
func parseExpiry(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t
	}
	return time.Time{}
}
