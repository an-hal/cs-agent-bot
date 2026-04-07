package middleware

import (
	"net/http"
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/rs/zerolog"
	"google.golang.org/api/idtoken"
)

// OIDCAuthMiddleware validates GCP Cloud Scheduler OIDC tokens.
// Skips validation when ENV=development or ENV=local for local testing.
func OIDCAuthMiddleware(appURL, schedulerEmail, env string, logger zerolog.Logger) func(ErrorHandler) ErrorHandler {
	audience := appURL + "/cron/run"

	return func(next ErrorHandler) ErrorHandler {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Skip OIDC validation for local development
			if env == "development" || env == "local" {
				logger.Debug().Msg("OIDC: skipping validation for local development")
				return next(w, r)
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				logger.Warn().Msg("OIDC: missing Authorization header")
				return apperror.Unauthorized("missing authorization header")
			}

			rawToken := strings.TrimPrefix(authHeader, "Bearer ")
			if rawToken == authHeader {
				logger.Warn().Msg("OIDC: invalid Authorization header format")
				return apperror.Unauthorized("invalid authorization header format")
			}

			payload, err := idtoken.Validate(r.Context(), rawToken, audience)
			if err != nil {
				logger.Warn().Err(err).Msg("OIDC: token validation failed")
				return apperror.Unauthorized("unauthorized")
			}

			// Verify email claim
			email, ok := payload.Claims["email"].(string)
			if !ok || email != schedulerEmail {
				logger.Warn().Str("email", email).Msg("OIDC: email mismatch")
				return apperror.Forbidden("forbidden")
			}

			return next(w, r)
		}
	}
}
