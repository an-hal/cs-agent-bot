package middleware

import (
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/google/uuid"
)

// RequestIDMiddleware generates or extracts a request ID and adds it to the request context.
// If X-Request-ID header is present, it uses that value; otherwise, it generates a new UUID.
// The request ID is also added to the response header.
func RequestIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get(ctxutil.RequestIDHeader)
			if requestID == "" {
				requestID = uuid.New().String()
			}

			w.Header().Set(ctxutil.RequestIDHeader, requestID)

			ctx := ctxutil.SetRequestID(r.Context(), requestID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
