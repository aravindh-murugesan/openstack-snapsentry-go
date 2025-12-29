package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/cloud"
	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/cloud/openstack"
	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/policy"
)

// initClient is a helper to spin up the OpenStack client for short-lived CLI operations.
func initClient(cloudName string, logLevel string) (*openstack.Client, error) {
	ostk := openstack.Client{
		ProfileName: cloudName,
		RetryConfig: cloud.RetryConfig{
			MaxRetries:       1,
			BaseDelay:        1 * time.Second,
			MaxDelay:         2 * time.Second,
			OperationTimeout: 10 * time.Second,
		},
	}
	if err := ostk.NewClient(); err != nil {
		return nil, fmt.Errorf("failed to connect to cloud: %w", err)
	}
	return &ostk, nil
}

// SubscribeVolumeDaily configures the Daily policy on a volume.
func SubscribeVolumeDaily(cloudName, logLevel, volID string, enabled bool, retention int, start, tz string) error {
	logger := setupLogger(logLevel, cloudName).With("workflow", "subscribe-daily", "volume_id", volID)

	p := policy.SnapshotPolicyDaily{
		Enabled:       enabled,
		RetentionDays: retention,
		RetentionType: "time",
		StartTime:     start,
		TimeZone:      tz,
	}

	if err := p.Normalize(); err != nil {
		logger.Error("Invalid policy configuration", "error", err)
		return err
	}

	return applySubscription(cloudName, logLevel, volID, p.ToOpenstackMetadata(), logger)
}

// SubscribeVolumeWeekly configures the Weekly policy on a volume.
func SubscribeVolumeWeekly(cloudName, logLevel, volID string, enabled bool, retention int, start, tz, weekday string) error {
	logger := setupLogger(logLevel, cloudName).With("workflow", "subscribe-weekly", "volume_id", volID)

	p := policy.SnapshotPolicyWeekly{
		Enabled:       enabled,
		RetentionDays: retention,
		RetentionType: "count",
		StartTime:     start,
		TimeZone:      tz,
		DayOfWeek:     weekday,
	}

	if err := p.Normalize(); err != nil {
		logger.Error("Invalid policy configuration", "error", err)
		return err
	}

	return applySubscription(cloudName, logLevel, volID, p.ToOpenstackMetadata(), logger)
}

// SubscribeVolumeMonthly configures the Monthly policy on a volume.
func SubscribeVolumeMonthly(cloudName, logLevel, volID string, enabled bool, retention int, start, tz string, day int) error {
	logger := setupLogger(logLevel, cloudName).With("workflow", "subscribe-monthly", "volume_id", volID)

	p := policy.SnapshotPolicyMonthly{
		Enabled:       enabled,
		RetentionDays: retention,
		RetentionType: "count",
		StartTime:     start,
		TimeZone:      tz,
		DayOfMonth:    day,
	}

	if err := p.Normalize(); err != nil {
		logger.Error("Invalid policy configuration", "error", err)
		return err
	}

	return applySubscription(cloudName, logLevel, volID, p.ToOpenstackMetadata(), logger)
}

// applySubscription handles the actual API call to update the volume metadata.
func applySubscription(cloudName, logLevel, volID string, metadata map[string]string, logger interface {
	Info(string, ...interface{})
	Error(string, ...interface{})
}) error {
	client, err := initClient(cloudName, logLevel)
	if err != nil {
		return err
	}

	logger.Info("Applying subscription policy to volume")

	// CreateVolumeSubscription handles fetching existing metadata and merging the new tags.
	_, reqID, err := client.CreateVolumeSubscription(context.Background(), volID, metadata)
	if err != nil {
		logger.Error("Failed to update volume metadata", "error", err)
		return err
	}

	logger.Info("Subscription applied successfully", "request_id", reqID)
	return nil
}
