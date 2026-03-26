package middleware

import (
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
)

// ErrorHandler is a handler function that can return an error
type ErrorHandler func(http.ResponseWriter, *http.Request) error

// ErrorHandlingMiddleware wraps an ErrorHandler and catches any returned errors
// This is used for per-route error handling where handlers return errors
func ErrorHandlingMiddleware(exceptionHandler *response.HTTPExceptionHandler) func(ErrorHandler) http.HandlerFunc {
	return func(handler ErrorHandler) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Execute the handler
			err := handler(w, r)

			// If an error occurred, let the exception handler deal with it
			if err != nil {
				exceptionHandler.HandleError(w, r, err)
			}
		}
	}
}
