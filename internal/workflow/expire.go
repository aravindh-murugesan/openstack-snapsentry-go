package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/cloud"
	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/cloud/openstack"
	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/policy"
	"github.com/google/uuid"
	"github.com/gophercloud/gophercloud/v2/openstack/blockstorage/v3/snapshots"
)

// RunProjectSnapshotExpiryWorkflow executes the retention enforcement process for a tenant.
//
// Responsibilities:
//  1. Discovery: Retrieves *all* snapshots in the project that bear the SnapSentry management tag.
//     This is a "Sweep" operation, independent of the source volumes (which might have been deleted).
//  2. Evaluation: Checks the `ExpiryDate` metadata on each snapshot against the current reference time.
//  3. cleanup: Permanently deletes snapshots that have exceeded their retention period.
//
// Parameters:
//   - now: The reference time for expiry (usually time.Now(), but injected for deterministic testing. UTC).
func RunProjectSnapshotExpiryWorkflow(cloudName string, timeoutSeconds int, logLevel string, now time.Time) error {
	// 1. Setup Logger & Context
	logger := SetupLogger(logLevel, cloudName).With("workflow", "expiry", "validation_time", now)
	snapsentryRunID := fmt.Sprintf("req-%s", uuid.New().String())
	logger = logger.With("snapsentry_id", snapsentryRunID)

	logger.Info("Initializing snapshot lifecycle workflow - expiry")

	ctx := context.Background()
	if timeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
		defer cancel()
	}

	// 2. Initialize OpenStack Client
	ostk := openstack.Client{
		ProfileName: cloudName,
		RetryConfig: cloud.RetryConfig{
			MaxRetries:       3,
			BaseDelay:        2 * time.Second,
			MaxDelay:         10 * time.Second,
			OperationTimeout: 30 * time.Second,
		},
	}

	if err := ostk.NewClient(); err != nil {
		logger.Error("OpenStack client initialization failed", "error", err)
		return fmt.Errorf("client init failed: %w", err)
	}
	logger.Info("OpenStack connection established")

	// 3. List Managed Snapshots
	managedSnapshots, err := ostk.ListManagedSnapshots(ctx)
	if err != nil {
		logger.Error("Failed to fetch managed snapshots", "error", err)
		return err
	}
	logger.Info("Found managed snapshots", "count", len(managedSnapshots))

	if len(managedSnapshots) == 0 {
		return nil
	}

	// 4. Process Snapshots Sequentially
	for _, snap := range managedSnapshots {
		// Stop if global timeout is reached
		if ctx.Err() != nil {
			logger.Warn("Workflow timed out, stopping early")
			return ctx.Err()
		}

		processSnapshotExpiry(ctx, ostk, snap, now, logger)
	}

	logger.Info("Expiry workflow completed")
	return nil
}

// processSnapshotExpiry handles the logic for a single snapshot
func processSnapshotExpiry(ctx context.Context, client openstack.Client, snap snapshots.Snapshot, now time.Time, logger *slog.Logger) {
	snapLog := logger.With("snapshot_id", snap.ID, "volume_id", snap.VolumeID)

	// A. Parse Metadata
	meta, err := policy.ParseSnapSentryMetadataFromSDK[policy.SnapshotMetadata](snap.Metadata)
	if err != nil {
		snapLog.Warn("Skipping snapshot: invalid metadata", "error", err)
		return
	}

	// B. Check Logic
	if now.Before(meta.ExpiryDate) {
		snapLog.Debug("Snapshot is in active retention peroid", "expires_at", meta.ExpiryDate)
		return // Not expired yet
	}

	// C. Execute Deletion
	snapLog.Info("Snapshot has expired", "expires_at", meta.ExpiryDate)

	reqID, err := client.DeleteSnapshot(ctx, snap.ID)
	if err != nil {
		snapLog.Error("Failed to delete snapshot", "error", err, "request_id", reqID, "expires_at", meta.ExpiryDate)
		return
	}

	// D. Success
	snapLog.Info("Snapshot deleted successfully", "request_id", reqID, "expires_at", meta.ExpiryDate)
}
