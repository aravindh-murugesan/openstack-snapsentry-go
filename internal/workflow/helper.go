package workflow

import (
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
)

// setupLogger configures the application-wide logger.
// It uses "tint" for colorized, structured logging that is easy to read in terminals.
func setupLogger(level string, cloudName string) *slog.Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	handler := tint.NewHandler(os.Stderr, &tint.Options{
		Level: logLevel,
	})

	return slog.New(handler).With("cloud_profile", cloudName)
}
