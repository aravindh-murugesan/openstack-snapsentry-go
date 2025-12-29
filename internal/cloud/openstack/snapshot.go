package openstack

import (
	"context"
	"fmt"

	"github.com/gophercloud/gophercloud/v2/openstack/blockstorage/v3/snapshots"
)

// CreateManagedSnapshot triggers the creation of a new snapshot and waits for it to become available.
//
// Behavior:
//   - Force Creation: Uses the `Force: true` flag, allowing snapshots to be taken even if the
//     volume is currently attached ("in-use") by an instance.
//   - Synchronous Wait: This method blocks until the snapshot reaches the "available" state
//     in OpenStack (or until the context times out). This ensures the snapshot is fully
//     persisted before the function returns.
//   - Metadata: Applies the provided policy tags (e.g., Expiry Date, Policy Type) at creation time.
//
// Returns:
//   - CreatedSnapshot: The struct containing details of the finished snapshot.
//   - RequestID: The OpenStack tracing ID.
//   - Error: Returns an error if the API call fails or if the snapshot ends up in an "error" state.
func (c *Client) CreateManagedSnapshot(
	ctx context.Context,
	volumeID string,
	name string,
	metadata map[string]string,
) (
	CreatedSnapshot snapshots.Snapshot, RequestID string, Error error,
) {

	var requestID string
	var createdSnapshot snapshots.Snapshot

	createOperation := func(innerCtx context.Context) error {
		opts := snapshots.CreateOpts{
			VolumeID:    volumeID,
			Force:       true, // Allows snapshotting 'in-use' volumes
			Name:        name,
			Description: "Created and managed by Snapsentry",
			Metadata:    metadata,
		}

		// 1. Trigger Creation
		result := snapshots.Create(innerCtx, c.BlockStorageClient, opts)
		requestID = result.Header.Get("X-Openstack-Request-Id")

		snap, err := result.Extract()
		if err != nil {
			return err
		}

		createdSnapshot = *snap

		// 2. Wait for Completion
		// We block here until the snapshot is ready or the context times out.
		if err := snapshots.WaitForStatus(innerCtx, c.BlockStorageClient, snap.ID, "available"); err != nil {
			return fmt.Errorf("failed waiting for snapshot %s to become available: %w", snap.ID, err)
		}

		return nil
	}

	if err := c.executeWithRetry(ctx, "CreateVolumeSnapshot", createOperation); err != nil {
		return snapshots.Snapshot{}, requestID, err
	}

	return createdSnapshot, requestID, nil
}
