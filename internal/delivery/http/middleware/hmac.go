package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/rs/zerolog"
)

// HMACAuthMiddleware validates the X-Handoff-Secret header for the BD handoff endpoint.
// Skips validation when ENV=development or ENV=local for local testing.
func HMACAuthMiddleware(secret, env string, logger zerolog.Logger) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Skip HMAC validation for local development
			if env == "development" || env == "local" {
				logger.Debug().Msg("HMAC: skipping validation for local development")
				next(w, r)
				return
			}
			headerVal := r.Header.Get("X-Handoff-Secret")
			if headerVal == "" {
				logger.Warn().Msg("HMAC: missing X-Handoff-Secret header")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			mac := hmac.New(sha256.New, []byte(secret))
			expectedSig := hex.EncodeToString(mac.Sum(nil))

			if !hmac.Equal([]byte(headerVal), []byte(expectedSig)) {
				logger.Warn().Msg("HMAC: secret mismatch")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next(w, r)
		}
	}
}
