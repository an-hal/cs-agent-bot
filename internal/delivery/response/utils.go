package response

import (
	"net/http"
	"reflect"

	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
)

func toSafeData(data any) any {
	if data == nil {
		return nil
	}
	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Slice && val.IsNil() {
		return []any{}
	}
	return data
}

// GetRequestID extracts request ID from context (stored by RequestIDMiddleware)
func GetRequestID(r *http.Request) string {
	return ctxutil.GetRequestID(r.Context())
}

// GetTraceID extracts trace ID from context (stored by TracingMiddleware)
func GetTraceID(r *http.Request) string {
	return ctxutil.GetTraceID(r.Context())
}
