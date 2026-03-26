package apperror

import "github.com/rs/zerolog"

// Err creates an error log event with stacktrace automatically included if available.
// Returns a *zerolog.Event that can be chained with .Msg() or additional fields.
//
// Usage:
//
//	apperror.Err(logger, err).Msg("Failed to create user")
//	apperror.Err(logger, err).Str("user_id", id).Msg("Failed to update user")
func Err(logger zerolog.Logger, err error) *zerolog.Event {
	event := logger.Error().Err(err)

	if appErr := GetAppError(err); appErr != nil && appErr.Stack != "" {
		event = event.Str("stacktrace", appErr.Stack)
	}

	return event
}
