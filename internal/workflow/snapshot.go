package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/cloud"
	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/cloud/openstack"
	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/policy"
	"github.com/gophercloud/gophercloud/v2/openstack/blockstorage/v3/volumes"
)

// RunProjectSnapshotWorkflow orchestrates the end-to-end backup process for a specific cloud tenant.
//
// Responsibilities:
//   1. Connection: Initializes the OpenStack client with retry logic and authenticates.
//   2. Discovery: Fetches only the volumes tagged for management (reducing API load).
//   3. Iteration: Processes volumes sequentially to avoid rate-limiting issues.
//      TODO: (aravindh-murugesan) Future enhancement could include controlled parallelism.
//   4. Safety: Respects a global timeout context to prevent hung processes.
//
// Parameters:
//   - cloudName: The profile name from `clouds.yaml`.
//   - timeoutSeconds: Hard limit for the job duration.

func RunProjectSnapshotWorkflow(cloudName string, timeoutSeconds int, logLevel string) error {
	// 1. Initialize Structured Logger
	// We use slog with tint for colorized, human-readable logs in development/CLI usage.
	logger := SetupLogger(logLevel, cloudName)

	logger.Info("Initializing snapshot lifecycle workflow")

	// 2. Setup Context (Optional Timeout)
	// This ensures the job doesn't hang indefinitely if the API becomes unresponsive.
	ctx := context.Background()

	if timeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
		defer cancel()
		logger.Debug("Global workflow timeout configured", "timeout_seconds", timeoutSeconds)
	}

	// 3. Initialize OpenStack Client
	// Configures retries to handle transient network glitches during API calls.
	ostk := openstack.Client{
		ProfileName: cloudName,
		RetryConfig: cloud.RetryConfig{
			MaxRetries:       3,
			BaseDelay:        2 * time.Second,
			MaxDelay:         10 * time.Second,
			OperationTimeout: 30 * time.Second,
		},
	}

	logger.Debug("Attempting to connect to OpenStack", "profile", cloudName)
	if err := ostk.NewClient(); err != nil {
		logger.Error("OpenStack client initialization failed", "error", err)
		return fmt.Errorf("client initialization failed: %w", err)
	}
	logger.Debug("OpenStack connection established successfully")

	// 4. Fetch Subscribed Volumes
	// Only volumes with the specific management tag are retrieved to reduce processing overhead.
	logger.Debug("Querying for subscribed volumes", "tag", policy.ManagedTag)
	managedVolumes, err := ostk.ListSubscribedVolumes(ctx)
	if err != nil {
		logger.Error("Volume discovery failed", "error", err)
		return fmt.Errorf("listing volumes failed: %w", err)
	}

	logger.Info("Subscribed volume discovery completed", "volume_count", len(managedVolumes))

	// 5. Process Volumes Sequentially
	// We process volumes one by one rather than in parallel to avoid hitting OpenStack API rate limits.
	successCount := 0
	errorCount := 0

	for i, vol := range managedVolumes {
		// Fail-safe: Check for global cancellation/timeout between volumes.
		if ctx.Err() != nil {
			logger.Warn("Workflow execution halted due to timeout or cancellation")
			return ctx.Err()
		}

		// Create a context-aware logger for this specific volume to trace logs easily.
		volLogger := logger.With(
			"volume_id", vol.ID,
			"volume_name", vol.Name,
			"progress", fmt.Sprintf("%d/%d", i+1, len(managedVolumes)),
		)

		volLogger.Debug("Starting processing for volume")

		if err := processVolume(ctx, &ostk, vol, volLogger); err != nil {
			volLogger.Error("Volume processing encountered an error", "error", err)
			errorCount++
		} else {
			volLogger.Debug("Volume processing completed successfully")
			successCount++
		}
	}

	logger.Info("Snapshot workflow execution summary",
		"volumes_processed", len(managedVolumes),
		"success_count", successCount,
		"error_count", errorCount)

	return nil
}

// processVolume applies the business logic to a single volume.
//
// Workflow:
//  1. Policy Loading: Instantiates Daily, Weekly, and Monthly policies and hydrates them from the volume's metadata.
//  2. History Check: Queries OpenStack for the most recent snapshot of the specific policy type.
//  3. Evaluation: Uses the policy's `Evaluate()` method to determine if a snapshot is needed now.
//  4. Execution: Triggers the snapshot creation if the window is open and unsatisfied.
//  5. Auditing: Writes detailed logs (Skipped/Created/Failed) to the database.
//  6. Cleanup: Detects and deletes "zombie" snapshots if creation reports failure but leaves an ID behind.
func processVolume(ctx context.Context, client *openstack.Client, vol volumes.Volume, logger *slog.Logger) error {
	// Define the order of policy evaluation.
	policies := []policy.SnapshotPolicy{
		&policy.SnapshotPolicyExpress{},
		&policy.SnapshotPolicyDaily{},
		&policy.SnapshotPolicyWeekly{},
		&policy.SnapshotPolicyMonthly{},
	}

	for _, p := range policies {
		policyType := p.GetPolicyType()
		policyLogger := logger.With("policy_type", policyType)

		// A. Parse & Validate
		// Extracts configuration from volume metadata (e.g., "x-snapsentry-daily-retention").
		_ = p.ParseFromMetadata(vol.Metadata)

		if !p.IsEnabled() {
			policyLogger.Debug("Policy is disabled. Skip further validation for this policy")
			continue
		}

		if err := p.Normalize(); err != nil {
			// If a policy is unconfigured or disabled, Normalize returns an error.
			// This is normal behavior for optional policies, so we just log debug and skip.
			policyLogger.Debug("Policy configuration skipped or invalid", "err", err)
			continue
		}

		policyLogger.Debug("Policy configuration loaded",
			"retention_days", p.GetPolicyRetention(),
			"type", p.GetPolicyType())

		// B. Fetch Last Snapshot
		// We need the most recent snapshot of THIS policy type to determine if a new one is needed.
		policyLogger.Debug("Fetching snapshot history for policy")
		snapshots, err := client.ListManagedVolumeSnapshots(ctx, vol.ID, policyType, true)
		if err != nil {
			policyLogger.Error("Snapshot history retrieval failed", "error", err)
			continue
		}

		lastSnapshotInfo := policy.LastSnapshotInfo{}
		if len(snapshots) > 0 {
			lastSnapshotInfo = policy.LastSnapshotInfo{
				ID:        snapshots[0].ID,
				CreatedAt: snapshots[0].CreatedAt,
				Status:    snapshots[0].Status,
				Metadata:  snapshots[0].Metadata,
			}
			policyLogger.Debug("Found previous snapshot",
				"snapshot_id", lastSnapshotInfo.ID,
				"created_at", lastSnapshotInfo.CreatedAt)
		} else {
			policyLogger.Debug("No previous snapshot found for this policy")
		}

		// C. Evaluate
		// Compares the last snapshot time against the policy's defined window.
		policyLogger.Debug("Evaluating policy rules against current time")
		result, err := p.Evaluate(time.Now(), lastSnapshotInfo)
		if err != nil {
			policyLogger.Error("Policy evaluation failed", "error", err)
			continue
		}

		if !result.ShouldSnapshot {
			policyLogger.Info("Snapshot creation skipped",
				"reason", result.Reason,
				"window_start", result.Window.StartTime,
				"window_end", result.Window.EndTime,
			)
			continue
		}

		// D. Execute
		policyLogger.Info("Snapshot window active; initiating creation",
			"window_start", result.Window.StartTime,
			"window_end", result.Window.EndTime,
			"reason", result.Reason)

		snapName := generateSnapshotName(policyType, result.Window.StartTime, vol.ID)
		snapMeta := result.Metadata.ToOpenstackMetadata()

		policyLogger.Debug("Sending create request to OpenStack", "snapshot_name", snapName)
		createdSnap, reqID, err := client.CreateManagedSnapshot(ctx, vol.ID, snapName, snapMeta)
		if err == nil {
			// Success path
			policyLogger.Info("Snapshot resource successfully created",
				"snapshot_id", createdSnap.ID,
				"request_id", reqID,
			)
			continue
		} else {
			// Failure path
			policyLogger.Error("Snapshot resource creation failed",
				"error", err,
				"request_id", reqID,
			)

			// SAFETY CHECK: Orphaned Resource Cleanup
			if createdSnap.ID != "" {
				policyLogger.Debug("Orphaned resource detected; initiating cleanup",
					"snapshot_id", createdSnap.ID,
					"status", createdSnap.Status,
				)

				// Attempt to delete the partial/failed snapshot to save quota.
				delReqID, cleanupErr := client.DeleteSnapshot(ctx, createdSnap.ID)

				if cleanupErr != nil {
					// CRITICAL: We failed to create it AND failed to delete the zombie resource.
					policyLogger.Error("Orphaned snapshot cleanup failed; manual intervention required",
						"error", cleanupErr,
						"snapshot_id", createdSnap.ID,
						"cleanup_request_id", delReqID,
					)
				} else {
					// INFO: We failed to create it, but at least we cleaned up the mess.
					policyLogger.Info("Orphaned snapshot successfully cleaned up",
						"snapshot_id", createdSnap.ID,
						"cleanup_request_id", delReqID,
					)
				}
			}
		}
	}

	return nil
}
