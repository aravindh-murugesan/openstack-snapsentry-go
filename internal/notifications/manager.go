package notifications

import (
	"context"
	"log/slog"

	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/http"
)

type EventType string

const (
	EventTypeSuccess EventType = "success"
	EventTypeFailure EventType = "failure"
)

type NotificationManager struct {
	FailureNotifier *notify.Notify
	AllNotifier     *notify.Notify
	Logger          *slog.Logger
}

func (nm *NotificationManager) Dispatch(
	ctx context.Context, eventType EventType, message string,
) {
	switch eventType {
	case EventTypeSuccess:
		nm.Logger.Debug("Notification Type: Success", slog.String("message", message))
		nm.AllNotifier.Send(ctx, "", message)
	case EventTypeFailure:
		nm.Logger.Error("Notification Type: Failure", slog.String("message", message))
		nm.FailureNotifier.Send(ctx, "", message)
		nm.AllNotifier.Send(ctx, "", message)
	default:
		nm.Logger.Error("Unknown event type", slog.String("event_type", string(eventType)), slog.String("message", message))
	}
}

func NewNotificationManager(targets []NotificationTarget, logger *slog.Logger) *NotificationManager {
	failureHooks := http.New()
	allHooks := http.New()

	for _, target := range targets {
		if err := target.ValidateURL(); err != nil {
			logger.Warn("Skipping invalid notification URL", "url", target.URL, "error", err)
			continue
		}
		if target.NotifySuccess {
			allHooks.AddReceiversURLs(target.URL)
		} else {
			failureHooks.AddReceiversURLs(target.URL)
		}
	}

	nm := &NotificationManager{
		FailureNotifier: notify.NewWithServices(failureHooks),
		AllNotifier:     notify.NewWithServices(allHooks),
		Logger:          logger,
	}

	return nm
}
