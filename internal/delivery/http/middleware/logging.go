package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/rs/zerolog"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func LoggingMiddleware(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip logging for high-frequency/static endpoints to reduce log noise
			if strings.HasSuffix(r.URL.Path, "/readiness") ||
				strings.HasPrefix(r.URL.Path, "/swagger") {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()

			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			duration := time.Since(start)
			requestID := ctxutil.GetRequestID(r.Context())
			traceID := response.GetTraceID(r)

			logEvent := logger.Info().
				Str("requestId", requestID).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("remote", r.RemoteAddr).
				Str("user_agent", r.UserAgent()).
				Int("status", rec.status).
				Int64("duration_ms", duration.Milliseconds())

			if traceID != "" {
				logEvent = logEvent.Str("traceId", traceID)
			}

			logEvent.Msg("HTTP request completed")
		})
	}
}
