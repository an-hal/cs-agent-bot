package middleware

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog"
	"google.golang.org/api/idtoken"
)

// OIDCAuthMiddleware validates GCP Cloud Scheduler OIDC tokens.
// Skips validation when ENV=development or ENV=local for local testing.
func OIDCAuthMiddleware(appURL, schedulerEmail, env string, logger zerolog.Logger) func(http.HandlerFunc) http.HandlerFunc {
	audience := appURL + "/cron/run"

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Skip OIDC validation for local development
			if env == "development" || env == "local" {
				logger.Debug().Msg("OIDC: skipping validation for local development")
				next(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				logger.Warn().Msg("OIDC: missing Authorization header")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			rawToken := strings.TrimPrefix(authHeader, "Bearer ")
			if rawToken == authHeader {
				logger.Warn().Msg("OIDC: invalid Authorization header format")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			payload, err := idtoken.Validate(r.Context(), rawToken, audience)
			if err != nil {
				logger.Warn().Err(err).Msg("OIDC: token validation failed")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Verify email claim
			email, ok := payload.Claims["email"].(string)
			if !ok || email != schedulerEmail {
				logger.Warn().Str("email", email).Msg("OIDC: email mismatch")
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next(w, r)
		}
	}
}
