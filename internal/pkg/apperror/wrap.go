package apperror

import "github.com/rs/zerolog"

// WrapInternal logs and wraps error as InternalError (500).
// Use for: Database errors, external service failures, unexpected system errors.
func WrapInternal(logger zerolog.Logger, err error, msg string) *AppError {
	if err != nil {
		Err(logger, err).Msg(msg)
	} else {
		logger.Warn().Msg(msg)
	}
	return InternalError(err)
}

// WrapNotFound logs and wraps error as NotFound (404).
// Use for: sql.ErrNoRows, missing resources.
func WrapNotFound(logger zerolog.Logger, err error, entity, msg string) *AppError {
	if err != nil {
		Err(logger, err).Msg(msg)
	} else {
		logger.Warn().Str("entity", entity).Msg(msg)
	}
	return NotFound(entity, msg)
}

// WrapBadRequest logs and wraps error as BadRequest (400).
// Use for: Invalid input, business logic violations, duplicate checks.
func WrapBadRequest(logger zerolog.Logger, err error, msg string) *AppError {
	if err != nil {
		Err(logger, err).Msg(msg)
	} else {
		logger.Warn().Msg(msg)
	}
	return BadRequest(msg)
}

// WrapValidation logs and wraps error as ValidationError (422).
// Use for: Input validation failures.
func WrapValidation(logger zerolog.Logger, err error, msg string) *AppError {
	if err != nil {
		Err(logger, err).Msg(msg)
	} else {
		logger.Warn().Msg(msg)
	}
	return ValidationError(msg)
}

// WrapUnauthorized logs and wraps error as Unauthorized (401).
// Use for: Authentication failures.
func WrapUnauthorized(logger zerolog.Logger, err error, msg string) *AppError {
	if err != nil {
		Err(logger, err).Msg(msg)
	} else {
		logger.Warn().Msg(msg)
	}
	return Unauthorized(msg)
}

// WrapForbidden logs and wraps error as Forbidden (403).
// Use for: Authorization failures, insufficient permissions.
func WrapForbidden(logger zerolog.Logger, err error, msg string) *AppError {
	if err != nil {
		Err(logger, err).Msg(msg)
	} else {
		logger.Warn().Msg(msg)
	}
	return Forbidden(msg)
}
