package ctxutil

import (
	"context"

	"github.com/rs/zerolog"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// RequestIDKey is the context key for request ID
	RequestIDKey contextKey = "request_id"
	// TraceIDKey is the context key for trace ID
	TraceIDKey contextKey = "trace_id"
)

// RequestIDHeader is the HTTP header name for request ID
const RequestIDHeader = "X-Request-ID"

// GetRequestID extracts the request ID from context.
// Returns empty string if not found.
func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if reqID, ok := ctx.Value(RequestIDKey).(string); ok {
		return reqID
	}
	return ""
}

// SetRequestID stores the request ID in context.
func SetRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// GetTraceID extracts the trace ID from context.
// Returns empty string if not found.
func GetTraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok {
		return traceID
	}
	return ""
}

// SetTraceID stores the trace ID in context.
func SetTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}

// LoggerWithRequestID returns a new logger with the request ID field added.
// If no request ID is found in context, returns the original logger.
func LoggerWithRequestID(ctx context.Context, logger zerolog.Logger) zerolog.Logger {
	reqID := GetRequestID(ctx)
	if reqID == "" {
		return logger
	}
	return logger.With().Str("request_id", reqID).Logger()
}
