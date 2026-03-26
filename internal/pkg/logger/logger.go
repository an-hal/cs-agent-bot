package logger

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// InitLogger creates and returns a configured zerolog.Logger
func InitLogger(env string, logLevel string) zerolog.Logger {
	level := parseLogLevel(logLevel)
	zerolog.SetGlobalLevel(level)

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	if env == "local" {
		output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
		return zerolog.New(output).With().Timestamp().Logger()
	}

	return zerolog.New(os.Stdout).With().Timestamp().Logger()
}

// parseLogLevel converts string log level to zerolog.Level
func parseLogLevel(logLevel string) zerolog.Level {
	switch strings.ToLower(logLevel) {
	case "trace":
		return zerolog.TraceLevel
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	case "panic":
		return zerolog.PanicLevel
	case "disabled":
		return zerolog.Disabled
	default:
		return zerolog.InfoLevel // Default fallback
	}
}
