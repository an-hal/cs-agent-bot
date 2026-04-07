package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/rs/zerolog"
)

// HMACAuthMiddleware validates the X-Handoff-Secret header using HMAC-SHA256 over the request body.
// Skips validation when ENV=development or ENV=local for local testing.
func HMACAuthMiddleware(secret, env string, logger zerolog.Logger) func(ErrorHandler) ErrorHandler {
	return func(next ErrorHandler) ErrorHandler {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Skip HMAC validation for local development
			if env == "development" || env == "local" {
				logger.Debug().Msg("HMAC: skipping validation for local development")
				return next(w, r)
			}
			headerVal := r.Header.Get("X-Handoff-Secret")
			if headerVal == "" {
				logger.Warn().Msg("HMAC: missing X-Handoff-Secret header")
				return apperror.Unauthorized("missing X-Handoff-Secret header")
			}

			body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
			if err != nil {
				logger.Error().Err(err).Msg("HMAC: failed to read body")
				return apperror.InternalError(err)
			}
			r.Body = io.NopCloser(bytes.NewReader(body))

			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(body)
			expectedSig := hex.EncodeToString(mac.Sum(nil))

			if !hmac.Equal([]byte(headerVal), []byte(expectedSig)) {
				logger.Warn().Msg("HMAC: secret mismatch")
				return apperror.Unauthorized("secret mismatch")
			}

			return next(w, r)
		}
	}
}
