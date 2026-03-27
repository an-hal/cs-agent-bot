package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/rs/zerolog"
)

// RecoveryMiddleware catches panics and returns a 500 error response
// This should be applied at the router level to protect all routes
func RecoveryMiddleware(logger zerolog.Logger, exceptionHandler *response.HTTPExceptionHandler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					requestID := response.GetRequestID(r)
					traceID := response.GetTraceID(r)

					logger.Error().
						Interface("panic", err).
						Str("requestId", requestID).
						Str("traceId", traceID).
						Str("method", r.Method).
						Str("path", r.URL.Path).
						Str("stack", string(debug.Stack())).
						Msg("Panic recovered")

					panicErr := apperror.InternalError(fmt.Errorf("panic: %v", err))
					exceptionHandler.HandleError(w, r, panicErr)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
