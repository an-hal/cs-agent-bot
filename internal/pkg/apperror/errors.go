package apperror

import (
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
)

// FieldError represents a validation error for a specific field
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// AppError represents a structured application error
type AppError struct {
	Code       string       // Internal error code (e.g., "NOT_FOUND")
	Message    string       // User-friendly message
	HTTPStatus int          // HTTP status code
	Err        error        // Original underlying error (optional)
	Fields     []FieldError // Field-level validation errors (optional)
	Stack      string       // Server-side stacktrace (never exposed to clients)
}

// captureStack captures the current stack trace.
// skip: number of frames to skip (to exclude captureStack and caller functions)
func captureStack(skip int) string {
	const maxFrames = 15
	pcs := make([]uintptr, maxFrames)
	n := runtime.Callers(skip, pcs)
	if n == 0 {
		return ""
	}

	frames := runtime.CallersFrames(pcs[:n])
	var builder strings.Builder

	for {
		frame, more := frames.Next()

		// Skip runtime internals
		if strings.Contains(frame.File, "runtime/") {
			if !more {
				break
			}
			continue
		}

		fmt.Fprintf(&builder, "%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)

		if !more {
			break
		}
	}

	return builder.String()
}

// StackTrace returns the captured stack trace (for logging purposes only)
func (e *AppError) StackTrace() string {
	return e.Stack
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error for errors.Is/As support
func (e *AppError) Unwrap() error {
	return e.Err
}

// NotFound creates a 404 Not Found error
func NotFound(entity, message string) *AppError {
	if message == "" {
		message = fmt.Sprintf("%s not found", entity)
	}
	return &AppError{
		Code:       CodeNotFound,
		Message:    message,
		HTTPStatus: http.StatusNotFound,
	}
}

// BadRequest creates a 400 Bad Request error
func BadRequest(message string) *AppError {
	return &AppError{
		Code:       CodeBadRequest,
		Message:    message,
		HTTPStatus: http.StatusBadRequest,
	}
}

// ValidationError creates a 422 Unprocessable Entity error
func ValidationError(message string) *AppError {
	return &AppError{
		Code:       CodeValidation,
		Message:    message,
		HTTPStatus: http.StatusUnprocessableEntity,
	}
}

// ValidationErrorWithFields creates a 422 error with field-level details
func ValidationErrorWithFields(message string, fields []FieldError) *AppError {
	return &AppError{
		Code:       CodeValidation,
		Message:    message,
		HTTPStatus: http.StatusUnprocessableEntity,
		Fields:     fields,
	}
}

// Unauthorized creates a 401 Unauthorized error
func Unauthorized(message string) *AppError {
	if message == "" {
		message = "Unauthorized access"
	}
	return &AppError{
		Code:       CodeUnauthorized,
		Message:    message,
		HTTPStatus: http.StatusUnauthorized,
	}
}

// Forbidden creates a 403 Forbidden error
func Forbidden(message string) *AppError {
	if message == "" {
		message = "Access forbidden"
	}
	return &AppError{
		Code:       CodeForbidden,
		Message:    message,
		HTTPStatus: http.StatusForbidden,
	}
}

// InternalError creates a 500 Internal Server Error with stack trace
func InternalError(err error) *AppError {
	return &AppError{
		Code:       CodeInternal,
		Message:    "An internal error occurred",
		HTTPStatus: http.StatusInternalServerError,
		Err:        err,
		Stack:      captureStack(3), // Skip: runtime.Callers, captureStack, InternalError
	}
}

// InternalErrorWithMessage creates a 500 Internal Server Error with custom message and stack trace
func InternalErrorWithMessage(message string, err error) *AppError {
	return &AppError{
		Code:       CodeInternal,
		Message:    message,
		HTTPStatus: http.StatusInternalServerError,
		Err:        err,
		Stack:      captureStack(3), // Skip: runtime.Callers, captureStack, InternalErrorWithMessage
	}
}

// GetAppError extracts *AppError from an error if possible
// Returns nil if the error is not an *AppError
func GetAppError(err error) *AppError {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return nil
}

// IsNotFound checks if the error is a NotFound error
func IsNotFound(err error) bool {
	appErr := GetAppError(err)
	return appErr != nil && appErr.Code == CodeNotFound
}

// IsValidationError checks if the error is a ValidationError
func IsValidationError(err error) bool {
	appErr := GetAppError(err)
	return appErr != nil && appErr.Code == CodeValidation
}

// IsBadRequest checks if the error is a BadRequest error
func IsBadRequest(err error) bool {
	appErr := GetAppError(err)
	return appErr != nil && appErr.Code == CodeBadRequest
}
