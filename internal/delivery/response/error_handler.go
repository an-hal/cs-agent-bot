package response

import (
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/rs/zerolog"
)

type HTTPExceptionHandler struct {
	logger           zerolog.Logger
	enableStackTrace bool
}

func NewHTTPExceptionHandler(logger zerolog.Logger, enableStackTrace bool) *HTTPExceptionHandler {
	return &HTTPExceptionHandler{
		logger:           logger,
		enableStackTrace: enableStackTrace,
	}
}

func (h *HTTPExceptionHandler) HandleError(w http.ResponseWriter, r *http.Request, err error) {
	requestID := GetRequestID(r)
	traceID := GetTraceID(r)

	var code int
	var message string
	var errorCode string
	var fields []apperror.FieldError
	var stackTrace string

	if appErr := apperror.GetAppError(err); appErr != nil {
		code = appErr.HTTPStatus
		message = appErr.Message
		errorCode = appErr.Code
		fields = appErr.Fields

		if h.enableStackTrace && appErr.StackTrace() != "" {
			stackTrace = appErr.StackTrace()
		}

		h.logError(requestID, traceID, appErr)
	} else {
		code = http.StatusInternalServerError
		message = "An internal error occurred"
		errorCode = apperror.CodeInternal

		h.logger.Error().
			Err(err).
			Str("requestId", requestID).
			Str("traceId", traceID).
			Str("path", r.URL.Path).
			Str("method", r.Method).
			Msg("Unhandled error occurred")
	}

	_ = StandardError(w, r, code, message, errorCode, fields, stackTrace)
}

func (h *HTTPExceptionHandler) logError(requestID, traceID string, appErr *apperror.AppError) {
	logEvent := h.logger.Error().
		Str("requestId", requestID).
		Str("traceId", traceID).
		Str("errorCode", appErr.Code).
		Int("httpStatus", appErr.HTTPStatus).
		Str("message", appErr.Message)

	if len(appErr.Fields) > 0 {
		logEvent = logEvent.Interface("fields", appErr.Fields)
	}

	if h.enableStackTrace && appErr.StackTrace() != "" {
		logEvent = logEvent.Str("stackTrace", appErr.StackTrace())
	}

	logEvent.Msg("Application error occurred")
}
