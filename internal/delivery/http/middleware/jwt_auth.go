package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
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

// JWTAuthMiddleware validates JWT tokens by calling the Sejutacita self-validation endpoint.
// It extracts the Bearer token from the Authorization header and forwards it to the
// validation service. On success, user info is stored in the request context.
func JWTAuthMiddleware(validateURL string, logger zerolog.Logger) func(http.Handler) http.Handler {
	client := &http.Client{Timeout: 10 * time.Second}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				logger.Warn().Msg("JWT: missing Authorization header")
				response.StandardError(w, r, http.StatusUnauthorized, "missing authorization header", "UNAUTHORIZED", nil, "")
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == authHeader {
				logger.Warn().Msg("JWT: invalid Authorization header format")
				response.StandardError(w, r, http.StatusUnauthorized, "invalid authorization header format", "UNAUTHORIZED", nil, "")
				return
			}

			// Call the self-validation endpoint
			req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, validateURL, nil)
			if err != nil {
				logger.Error().Err(err).Msg("JWT: failed to create validation request")
				response.StandardError(w, r, http.StatusInternalServerError, "internal server error", "INTERNAL_SERVER_ERROR", nil, "")
				return
			}
			req.Header.Set("Authorization", "Bearer "+token)

			resp, err := client.Do(req)
			if err != nil {
				logger.Error().Err(err).Msg("JWT: validation request failed")
				response.StandardError(w, r, http.StatusInternalServerError, "internal server error", "INTERNAL_SERVER_ERROR", nil, "")
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				logger.Warn().
					Int("status", resp.StatusCode).
					Str("body", string(body)).
					Msg("JWT: token validation failed")
				response.StandardError(w, r, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED", nil, "")
				return
			}

			var authResp JWTAuthResponse
			if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
				logger.Error().Err(err).Msg("JWT: failed to decode validation response")
				response.StandardError(w, r, http.StatusInternalServerError, "internal server error", "INTERNAL_SERVER_ERROR", nil, "")
				return
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

			next.ServeHTTP(w, r.WithContext(ctx))
		})
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
