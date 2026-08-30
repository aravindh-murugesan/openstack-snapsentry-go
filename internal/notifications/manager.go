package notifications

import (
	"context"
	"fmt"
	"log/slog"
	stdhttp "net/http"
	"strings"

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
	ctx context.Context,
	eventType EventType,
	subject string,
	message string,
) {
	switch eventType {
	case EventTypeSuccess:
		nm.Logger.Debug("Notification Type: Success", slog.String("message", message))
		if err := nm.AllNotifier.Send(ctx, subject, message); err != nil {
			nm.Logger.Error("Failed to send success notification", slog.String("error", err.Error()))
		}
	case EventTypeFailure:
		nm.Logger.Error("Notification Type: Failure", slog.String("message", message))
		if err := nm.FailureNotifier.Send(ctx, subject, message); err != nil {
			nm.Logger.Error("Failed to send failure notification", slog.String("error", err.Error()))
		}
		if err := nm.AllNotifier.Send(ctx, subject, message); err != nil {
			nm.Logger.Error("Failed to send failure notification", slog.String("error", err.Error()))
		}
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
		wh := &http.Webhook{
			ContentType: "application/json; charset=utf-8",
			Header:      stdhttp.Header{},
			Method:      stdhttp.MethodPost,
			URL:         target.URL,
		}

		if strings.Contains(target.URL, "discord.com/api/webhooks") {
			wh.BuildPayload = func(subject, message string) any {
				return map[string]string{
					"content": fmt.Sprintf("**%s**\n```json\n%s\n```", subject, message),
				}
			}
		} else if strings.Contains(target.URL, "hooks.slack.com/services") {
			wh.BuildPayload = func(subject, message string) any {
				return map[string]string{
					"text": fmt.Sprintf("*%s*\n```\n%s\n```", subject, message),
				}
			}
		} else {
			wh.BuildPayload = func(subject, message string) any {
				// The default payload format for nikoksr/notify HTTP service
				return map[string]string{
					"subject": subject,
					"message": message,
				}
			}
		}

		if target.NotifySuccess {
			allHooks.AddReceivers(wh)
		} else {
			failureHooks.AddReceivers(wh)
		}
	}

	nm := &NotificationManager{
		FailureNotifier: notify.NewWithServices(failureHooks),
		AllNotifier:     notify.NewWithServices(allHooks),
		Logger:          logger,
	}

	return nm
}
