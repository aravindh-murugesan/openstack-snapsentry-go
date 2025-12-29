package workflow

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

// setupLogger configures the application-wide logger.
// It uses "tint" for colorized, structured logging that is easy to read in terminals.
func SetupLogger(level string, cloudName string) *slog.Logger {
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

// GenerateSnapshotName creates a consistent naming convention for snapshots.
// Format: managed-<policyType>-<volumeID>-<windowStart>
func generateSnapshotName(policyType string, windowStart time.Time, volumeID string) string {
	// Using a concise time format for the name to avoid illegal characters
	timestamp := windowStart.Format(time.RFC3339)
	return fmt.Sprintf("managed-%s-%s-%s", policyType, volumeID, timestamp)
}
