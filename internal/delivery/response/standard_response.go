package response

import (
	"encoding/json"
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
)

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

// WriteJSON writes a JSON response with the given status code
func WriteJSON(w http.ResponseWriter, r *http.Request, code int, data interface{}) error {
	return StandardSuccess(w, r, code, "Success", data)
}
