package response

import (
	"encoding/json"
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
)

// StandardSuccess writes a successful StandardResponse
// Reads requestID and traceID from context automatically
func StandardSuccess(w http.ResponseWriter, r *http.Request, code int, message string, data interface{}) error {
	requestID := GetRequestID(r)
	traceID := GetTraceID(r)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	return json.NewEncoder(w).Encode(StandardResponse{
		RequestID: requestID,
		TraceID:   traceID,
		Status:    "success",
		Message:   message,
		Data:      toSafeData(data),
	})
}

// StandardSuccessWithMeta writes a successful StandardResponseWithMeta
// Reads requestID and traceID from context automatically
func StandardSuccessWithMeta(w http.ResponseWriter, r *http.Request, code int, message string, meta, data interface{}) error {
	requestID := GetRequestID(r)
	traceID := GetTraceID(r)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	return json.NewEncoder(w).Encode(StandardResponseWithMeta{
		RequestID: requestID,
		TraceID:   traceID,
		Status:    "success",
		Message:   message,
		Meta:      toSafeData(meta),
		Data:      toSafeData(data),
	})
}

// StandardError writes an error StandardResponse
// Reads requestID and traceID from context automatically
func StandardError(w http.ResponseWriter, r *http.Request, code int, message, errorCode string, validationErrors []apperror.FieldError, stackTrace string) error {
	requestID := GetRequestID(r)
	traceID := GetTraceID(r)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	return json.NewEncoder(w).Encode(StandardResponse{
		RequestID:        requestID,
		TraceID:          traceID,
		Status:           "error",
		Message:          message,
		ErrorCode:        errorCode,
		ValidationErrors: validationErrors,
		StackTrace:       stackTrace,
	})
}

// StandardErrorWithMeta writes an error StandardResponseWithMeta
// Reads requestID and traceID from context automatically
func StandardErrorWithMeta(w http.ResponseWriter, r *http.Request, code int, message, errorCode string, meta interface{}, validationErrors []apperror.FieldError, stackTrace string) error {
	requestID := GetRequestID(r)
	traceID := GetTraceID(r)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	return json.NewEncoder(w).Encode(StandardResponseWithMeta{
		RequestID:        requestID,
		TraceID:          traceID,
		Status:           "error",
		Message:          message,
		Meta:             toSafeData(meta),
		ErrorCode:        errorCode,
		ValidationErrors: validationErrors,
		StackTrace:       stackTrace,
	})
}
