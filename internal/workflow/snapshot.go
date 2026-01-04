package workflow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/cloud"
	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/cloud/openstack"
	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/notifications"
	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/policy"
	"github.com/google/uuid"
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

	snapsentryRunID := fmt.Sprintf("req-%s", uuid.New().String())
	logger = logger.With("snapsentry_id", snapsentryRunID)
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
	var successCount int32
	var errorCount int32

	groupedVolumes := ostk.GroupVolumeByVMAttachment(managedVolumes)

	logger.Debug("Starting to process single-attached volumes", "vm_count", len(groupedVolumes.Attached))
	for vm, vols := range groupedVolumes.Attached {
		logger.Debug("Starting to process volumes attached to a VM", "vm_id", vm, "volume_count", len(vols))
		processVolumeGroup(ctx, &ostk, vols, &successCount, &errorCount, logger)
	}

	logger.Debug("Starting to process multi-attached volumes", "count", len(groupedVolumes.MultiAttached))
	for _, vol := range groupedVolumes.MultiAttached {
		processVolumeGroup(ctx, &ostk, []volumes.Volume{vol}, &successCount, &errorCount, logger)
	}

	logger.Debug("Starting to process unattached volumes", "count", len(groupedVolumes.Unattached))
	for _, vol := range groupedVolumes.Unattached {
		processVolumeGroup(ctx, &ostk, []volumes.Volume{vol}, &successCount, &errorCount, logger)
	}

	logger.Info("Snapshot workflow execution summary for evaluation. This only refers to snapsentry processing and excludes openstack api errors",
		"volumes_processed", len(managedVolumes),
		"success_count", successCount,
		"error_count", errorCount)

	return nil
}

// processVolumeGroup executes snapshot logic for a list of volumes concurrently.
// This is a wrapper for processVolume for concurrency.
// Design Rationale:
//   - Purpose: Minimizes the time skew between snapshots for multi-disk VMs (simulated atomicity).
//   - Concurrency: Spins up one goroutine per volume. Uses sync.WaitGroup to block until all complete.
//   - Safety: Checks ctx.Err() before starting new goroutines to respect global timeouts immediately.
//   - Metrics: Uses atomic operations to safely update shared counters from multiple threads.
//
// Parameters:
//   - ctx: Global context (handles timeout/cancellation).
//   - client: Authenticated OpenStack client.
//   - vols: Slice of volumes to process (usually belonging to the same VM).
//   - success/errorCounter: Pointers to thread-safe counters.
//   - logger: Base logger (fields like 'vm_id' should already be attached).
func processVolumeGroup(
	ctx context.Context,
	client *openstack.Client,
	vols []volumes.Volume,
	successCounter *int32,
	errorCounter *int32,
	logger *slog.Logger,
) {

	var vgWaitGroup sync.WaitGroup

	for _, v := range vols {

		// Fail-safe: Check context BEFORE spawning a new goroutine.
		// If the global timeout is hit, stop starting new work immediately.
		if ctx.Err() != nil {
			logger.Error("Workflow execution halted due to timeout or cancellation")
			break // (TO SELF) ensures active goroutines are awaited
		}

		vgWaitGroup.Add(1)

		// Each volume gets its own go-routine.
		go func(ctx context.Context, client *openstack.Client, vol volumes.Volume, logger *slog.Logger) {
			defer vgWaitGroup.Done()

			// logger specific to this volume for clear traceability.
			volLogger := logger.With(
				"volume_id", vol.ID,
				"volume_name", vol.Name,
			)

			volLogger.Debug("Starting processing for volume")

			// Execute the core logic (policy checks, snapshot creation, etc.)
			if err := processVolume(ctx, client, vol, volLogger); err != nil {
				volLogger.Error("Volume processing encountered an error", "error", err)
				// Atomic increment is required because multiple goroutines write to this address simultaneously.
				atomic.AddInt32(errorCounter, 1)

			} else {
				volLogger.Debug("Volume processing completed successfully")
				// Atomic increment is required because multiple goroutines write to this address simultaneously.
				atomic.AddInt32(successCounter, 1)
			}
		}(ctx, client, v, logger)
	}

	vgWaitGroup.Wait()
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

	var execErrors error
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
			execErrors = errors.Join(execErrors, fmt.Errorf("%s policy configuration is invalid or skipped. %w", policyType, err))
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
			execErrors = errors.Join(execErrors, fmt.Errorf("%s policy snapshot history retrieval failed. %w", policyType, err))
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
			execErrors = errors.Join(execErrors, fmt.Errorf("%s policy evaluation failed. %w", policyType, err))
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
			execErrors = errors.Join(execErrors, fmt.Errorf("%s policy snapshot resource creation failed. %w", policyType, err))
			policyLogger.Error("Snapshot resource creation failed",
				"error", err,
				"request_id", reqID,
			)

			ntfyprovider := notifications.Webhook{
				URL: "https://cool-lion-02.webhook.cool",
			}
			if err := ntfyprovider.Notify(notifications.SnapshotCreationFailure{
				Service:    "snapsentry",
				VolumeID:   vol.ID,
				SnapshotID: createdSnap.ID,
				Message:    fmt.Sprintf("Volume processing encountered an error - %s", err),
				Window:     result.Window,
			}); err != nil {
				slog.Error("Notification failed", "err", err)
			}

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
					execErrors = errors.Join(execErrors, fmt.Errorf("%s policy orphaned snapshot cleanup failed; manual intervention required. %w", policyType, cleanupErr))
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

	return execErrors
}
