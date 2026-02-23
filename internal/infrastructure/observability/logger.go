package observability

import (
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog"
)

func InitLogger(level string, output io.Writer) zerolog.Logger {
	if output == nil {
		output = os.Stdout
	}

	logLevel := parseLogLevel(level)

	return zerolog.New(output).
		Level(logLevel).
		With().
		Timestamp().
		Caller().
		Logger()
}

func parseLogLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
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
	default:
		return zerolog.InfoLevel
	}
}

func WithContext(logger zerolog.Logger, ctx map[string]any) zerolog.Logger {
	l := logger.With()
	for k, v := range ctx {
		l = l.Interface(k, v)
	}
	return l.Logger()
}
