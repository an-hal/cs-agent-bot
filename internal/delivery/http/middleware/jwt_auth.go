package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/rs/zerolog"
)

// JWTAuthResponse represents the response from the self-validation endpoint.
type JWTAuthResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    struct {
		SessionID    string   `json:"sessionId"`
		ID           string   `json:"_id"`
		Email        string   `json:"email"`
		Roles        []string `json:"roles"`
		Platform     string   `json:"platform"`
		NormalizedID string   `json:"normalizedId"`
	} `json:"data"`
}

// JWTUser holds the authenticated user information.
type JWTUser struct {
	SessionID    string
	ID           string
	Email        string
	Roles        []string
	Platform     string
	NormalizedID string
}

type jwtContextKey string

const jwtUserKey jwtContextKey = "jwt_user"

// WithJWTUser stores JWTUser in context.
func WithJWTUser(ctx context.Context, user JWTUser) context.Context {
	return context.WithValue(ctx, jwtUserKey, user)
}

// GetJWTUser retrieves JWTUser from context.
func GetJWTUser(ctx context.Context) (*JWTUser, bool) {
	if ctx == nil {
		return nil, false
	}
	val := ctx.Value(jwtUserKey)
	if val == nil {
		return nil, false
	}
	user, ok := val.(JWTUser)
	if !ok {
		return nil, false
	}
	return &user, true
}

// devBypassPrefix is the sentinel that marks a dev-mode bypass token.
// Format: "Bearer DEV.<email>" — only honored when env is dev/local AND
// JWT_DEV_BYPASS_ENABLED=true. Optional roles via "X-Dev-Roles" header
// (comma-separated). See features/00-shared/13-dev-bypass.md.
const devBypassPrefix = "DEV."

func isDevEnv(env string) bool {
	return env == "development" || env == "local"
}

// tryDevBypass returns a synthesized JWTUser when the request carries a
// DEV.<email> token AND the environment + flag permit bypass. Returns nil
// otherwise so the caller falls through to real validation.
func tryDevBypass(token, env string, enabled bool, devRolesHeader string, logger zerolog.Logger) *JWTUser {
	if !enabled || !isDevEnv(env) {
		return nil
	}
	if !strings.HasPrefix(token, devBypassPrefix) {
		return nil
	}
	email := strings.TrimSpace(strings.TrimPrefix(token, devBypassPrefix))
	if email == "" || !strings.Contains(email, "@") {
		return nil
	}

	roles := []string{"admin"}
	if devRolesHeader = strings.TrimSpace(devRolesHeader); devRolesHeader != "" {
		roles = roles[:0]
		for _, r := range strings.Split(devRolesHeader, ",") {
			if r = strings.TrimSpace(r); r != "" {
				roles = append(roles, r)
			}
		}
	}

	logger.Warn().
		Str("email", email).
		Strs("roles", roles).
		Str("env", env).
		Msg("JWT: DEV BYPASS active — Sejutacita validation skipped")

	return &JWTUser{
		SessionID:    "dev-session",
		ID:           "dev-user",
		Email:        email,
		Roles:        roles,
		Platform:     "dev",
		NormalizedID: "dev-user",
	}
}

// JWTAuthMiddleware validates JWT tokens by calling the Sejutacita self-validation endpoint.
// It extracts the Bearer token from the Authorization header and forwards it to the
// validation service. On success, user info is stored in the request context.
//
// When env is "development" or "local" AND devBypassEnabled is true, a token
// of the form "DEV.<email>" short-circuits validation and synthesizes a
// JWTUser. This is gated by both the env and the flag to make accidental
// activation in production impossible.
func JWTAuthMiddleware(validateURL, env string, devBypassEnabled bool, logger zerolog.Logger) func(ErrorHandler) ErrorHandler {
	client := &http.Client{Timeout: 10 * time.Second}

	return func(next ErrorHandler) ErrorHandler {
		return func(w http.ResponseWriter, r *http.Request) error {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				logger.Warn().Msg("JWT: missing Authorization header")
				return apperror.Unauthorized("missing authorization header")
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == authHeader {
				logger.Warn().Msg("JWT: invalid Authorization header format")
				return apperror.Unauthorized("invalid authorization header format")
			}

			// Dev-mode bypass: triple-gated (env + flag + token prefix).
			if user := tryDevBypass(token, env, devBypassEnabled, r.Header.Get("X-Dev-Roles"), logger); user != nil {
				return next(w, r.WithContext(WithJWTUser(r.Context(), *user)))
			}

			// Call the self-validation endpoint
			req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, validateURL, nil)
			if err != nil {
				logger.Error().Err(err).Msg("JWT: failed to create validation request")
				return apperror.InternalError(err)
			}
			req.Header.Set("Authorization", "Bearer "+token)

			resp, err := client.Do(req)
			if err != nil {
				logger.Error().Err(err).Msg("JWT: validation request failed")
				return apperror.InternalError(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				logger.Warn().
					Int("status", resp.StatusCode).
					Str("body", string(body)).
					Msg("JWT: token validation failed")
				return apperror.Unauthorized("unauthorized")
			}

			var authResp JWTAuthResponse
			if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
				logger.Error().Err(err).Msg("JWT: failed to decode validation response")
				return apperror.InternalError(err)
			}

			ctx := WithJWTUser(r.Context(), JWTUser{
				SessionID:    authResp.Data.SessionID,
				ID:           authResp.Data.ID,
				Email:        authResp.Data.Email,
				Roles:        authResp.Data.Roles,
				Platform:     authResp.Data.Platform,
				NormalizedID: authResp.Data.NormalizedID,
			})

			logger.Debug().
				Str("user_id", authResp.Data.ID).
				Str("email", authResp.Data.Email).
				Msg("JWT: authenticated")

			return next(w, r.WithContext(ctx))
		}
	}
}

// HasRole checks if the user has a specific role.
func (u JWTUser) HasRole(role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// String returns a string representation of the JWTUser.
func (u JWTUser) String() string {
	return fmt.Sprintf("JWTUser{id=%s, email=%s, roles=%v}", u.ID, u.Email, u.Roles)
}
