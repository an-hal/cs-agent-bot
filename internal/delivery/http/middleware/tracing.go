package middleware

import (
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// TracingMiddleware creates OpenTelemetry spans for HTTP requests
func TracingMiddleware(t tracer.Tracer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Start a new span for this HTTP request
			spanName := r.Method + " " + r.URL.Path
			ctx, span := t.Start(r.Context(), spanName)
			defer span.End()

			// Extract traceID and store in context
			traceID := span.SpanContext().TraceID().String()
			ctx = ctxutil.SetTraceID(ctx, traceID)

			// Add HTTP request attributes
			span.SetAttributes(
				semconv.HTTPRequestMethodKey.String(r.Method),
				semconv.HTTPRoute(r.URL.Path),
				semconv.URLScheme(r.URL.Scheme),
				semconv.URLPath(r.URL.Path),
				semconv.URLQuery(r.URL.RawQuery),
				semconv.ServerAddress(r.Host),
				attribute.String("http.user_agent", r.UserAgent()),
				attribute.String("http.remote_addr", r.RemoteAddr),
			)

			// Create a response recorder to capture status code
			rec := &tracingResponseRecorder{ResponseWriter: w, statusCode: http.StatusOK}

			// Continue with the request using the new context
			next.ServeHTTP(rec, r.WithContext(ctx))

			// Add response status code
			span.SetAttributes(semconv.HTTPResponseStatusCode(rec.statusCode))

			// Set span status based on HTTP status code
			if rec.statusCode >= 400 {
				span.SetStatus(codes.Error, http.StatusText(rec.statusCode))
			} else {
				span.SetStatus(codes.Ok, "")
			}
		})
	}
}

// tracingResponseRecorder wraps http.ResponseWriter to capture status code
type tracingResponseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *tracingResponseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}
