package openstack

import (
	"context"
	"fmt"

	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/policy"
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

// DeleteSnapshot removes a snapshot from the backend storage.
//
// Behavior:
//   - Force Delete: This method explicitly triggers a "Force Delete" operation.
//     This ensures the snapshot is removed even if the storage backend indicates
//     it is busy or in a stuck state, preventing "zombie" snapshots from accumulating.
//   - Asynchronous: Unlike creation, deletion is often asynchronous. This method returns
//     success once the delete request is accepted by the API, but does not wait for
//     the resource to disappear completely.
//
// Returns:
//   - RequestID: The OpenStack tracing ID for the delete operation.
//   - Error: Returns an error if the delete request fails (e.g., 404 Not Found or 403 Forbidden).
func (c *Client) DeleteSnapshot(ctx context.Context, snapshotID string) (RequestID string, Error error) {
	var requestID string
	deleteOperation := func(innerCtx context.Context) error {
		result := snapshots.ForceDelete(innerCtx, c.BlockStorageClient, snapshotID)
		requestID = result.Header.Get("X-Openstack-Request-Id")

		if result.Err != nil {
			return result.Err
		}
		return nil
	}

	if err := c.executeWithRetry(ctx, "DeleteVolumeSnapshot", deleteOperation); err != nil {
		return requestID, err
	}

	return requestID, nil
}

// ListManagedVolumeSnapshots fetches the snapshot history for a specific volume, filtered by policy type.
//
// Parameters:
//   - volumeID: The UUID of the volume to inspect.
//   - policyType: The policy identifier to filter by (e.g., "daily", "weekly").
//   - lastSnapshotOnly: Optimization flag. If true, the function stops after finding the
//     first match. This is used during the "Evaluate" phase to quickly find the most
//     recent snapshot for idempotency checks.
//
// Note: This relies on the OpenStack API returning snapshots sorted by creation date (Newest First),
// which is the default behavior for Cinder.
func (c *Client) ListManagedVolumeSnapshots(ctx context.Context, volumeID string, policyType string, lastSnapshotOnly bool) (
	ManagedSnapshots []snapshots.Snapshot, Error error,
) {
	// TODO (aravindh-murugesan): Refactor this method with a helper func to reduce code duplication with ListManagedSnapshots.
	var managedSnapshots []snapshots.Snapshot

	listOperation := func(innerCtx context.Context) error {
		// Reset the slice on retry to avoid duplicates
		managedSnapshots = []snapshots.Snapshot{}

		// Constuct opts to list all the snapshot
		opts := snapshots.ListOpts{
			AllTenants: false,
			Status:     "available",
			VolumeID:   volumeID,
		}

		pages, err := snapshots.List(c.BlockStorageClient, opts).AllPages(innerCtx)
		if err != nil {
			return err
		}
		snaps, err := snapshots.ExtractSnapshots(pages)
		if err != nil {
			return err
		}

		// Filter by Metadata Policy Type
		for _, snap := range snaps {
			metadata := policy.SnapshotMetadata{}
			// We ignore errors here; if metadata is missing/malformed, it's simply not a managed snapshot.
			_ = metadata.ParseFromMetadata(snap.Metadata)

			if metadata.PolicyType == policyType {
				managedSnapshots = append(managedSnapshots, snap)

				// Optimization: Relying on API default sort order.
				if lastSnapshotOnly {
					return nil
				}
			}
		}

		return nil
	}

	if err := c.executeWithRetry(ctx, "ListManagedSnapshots", listOperation); err != nil {
		return []snapshots.Snapshot{}, err
	}

	return managedSnapshots, nil
}

// ListManagedSnapshots retrieves every snapshot in the project that is managed by SnapSentry.
// This is primarily used by the Expiry/Cleanup workflow to find candidates for deletion.
//
// Filtering:
// Since OpenStack API filtering is limited for custom metadata keys, this method performs
// "Client-Side Filtering": it fetches all 'available' snapshots and iterates through them,
// parsing the metadata to find those with the 'x-snapsentry-managed' tag set to true.
func (c *Client) ListManagedSnapshots(ctx context.Context) (
	ManagedSnapshots []snapshots.Snapshot, Error error,
) {
	// TODO (aravindh-murugesan): Refactor this method with a helper func to reduce code duplication with ListManagedVolumeSnapshots.
	var managedSnapshots []snapshots.Snapshot

	listOperation := func(innerCtx context.Context) error {
		// Reset the slice on retry to avoid duplicates
		managedSnapshots = []snapshots.Snapshot{}

		// Constuct opts to list all the snapshot
		opts := snapshots.ListOpts{
			AllTenants: false,
			Status:     "available",
		}

		pages, err := snapshots.List(c.BlockStorageClient, opts).AllPages(innerCtx)
		if err != nil {
			return err
		}
		snaps, err := snapshots.ExtractSnapshots(pages)
		if err != nil {
			return err
		}

		// Filter by Metadata Policy Type
		for _, snap := range snaps {
			metadata := policy.SnapshotMetadata{}
			// We ignore errors here; if metadata is missing/malformed, it's simply not a managed snapshot.
			_ = metadata.ParseFromMetadata(snap.Metadata)

			if metadata.Managed {
				managedSnapshots = append(managedSnapshots, snap)
			}
		}
		return nil
	}

	if err := c.executeWithRetry(ctx, "ListManagedSnapshots", listOperation); err != nil {
		return []snapshots.Snapshot{}, err
	}

	return managedSnapshots, nil
}
