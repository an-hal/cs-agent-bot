package response

import "github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"

// StandardResponse is the new standardized API response format
// with requestId and traceId for observability
type StandardResponse struct {
	RequestID        string                `json:"requestId"`
	TraceID          string                `json:"traceId,omitempty"`
	Status           string                `json:"status"` // "success" or "error"
	Message          string                `json:"message"`
	Data             interface{}           `json:"data,omitempty"`
	ValidationErrors []apperror.FieldError `json:"validationErrors,omitempty"`
	ErrorCode        string                `json:"errorCode,omitempty"`
	StackTrace       string                `json:"stackTrace,omitempty"`
}

// StandardResponseWithMeta includes pagination/query metadata
type StandardResponseWithMeta struct {
	RequestID        string                `json:"requestId"`
	TraceID          string                `json:"traceId,omitempty"`
	Status           string                `json:"status"` // "success" or "error"
	Message          string                `json:"message"`
	Meta             interface{}           `json:"meta,omitempty"`
	Data             interface{}           `json:"data,omitempty"`
	ValidationErrors []apperror.FieldError `json:"validationErrors,omitempty"`
	ErrorCode        string                `json:"errorCode,omitempty"`
	StackTrace       string                `json:"stackTrace,omitempty"`
}
