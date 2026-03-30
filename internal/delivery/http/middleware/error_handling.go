package middleware

import (
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
)

// ErrorHandler is a handler function that can return an error
type ErrorHandler func(http.ResponseWriter, *http.Request) error

// ErrorHandlingMiddleware wraps an ErrorHandler and catches any returned errors
func ErrorHandlingMiddleware(exceptionHandler *response.HTTPExceptionHandler) func(ErrorHandler) http.HandlerFunc {
	return func(handler ErrorHandler) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			err := handler(w, r)
			if err != nil {
				exceptionHandler.HandleError(w, r, err)
			}
		}
	}
}

// WrapErrorHandler converts an error-returning handler to http.HandlerFunc
func WrapErrorHandler(handler ErrorHandler, exceptionHandler *response.HTTPExceptionHandler) http.HandlerFunc {
	return ErrorHandlingMiddleware(exceptionHandler)(handler)
}
