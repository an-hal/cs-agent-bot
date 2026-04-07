package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/rs/zerolog"
)

// HaloAISignatureMiddleware verifies HaloAI webhook signature.
// Skips validation when ENV=development or ENV=local for local testing.
func HaloAISignatureMiddleware(secret, env string, logger zerolog.Logger) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Skip signature validation for local development
			if env == "development" || env == "local" {
				logger.Debug().Msg("HaloAI signature: skipping validation for local development")
				next(w, r)
				return
			}
			signature := r.Header.Get("X-Signature")
			if signature == "" {
				logger.Warn().Msg("HaloAI signature: missing X-Signature header")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10MB limit
			if err != nil {
				logger.Error().Err(err).Msg("HaloAI signature: failed to read body")
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}
			// Restore body for downstream handler
			r.Body = io.NopCloser(bytes.NewReader(body))

			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(body)
			expectedSig := hex.EncodeToString(mac.Sum(nil))

			if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
				logger.Warn().Msg("HaloAI signature: HMAC mismatch")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next(w, r)
		}
	}
}
