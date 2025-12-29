package openstack

import (
	"context"
	"maps"

	"github.com/gophercloud/gophercloud/v2/openstack/blockstorage/v3/volumes"
)

// CreateVolumeSubscription applies the provided metadata tags to a specific volume.
// It serves as the mechanism to "subscribe" a volume to a snapshot policy by writing
// the specific policy configuration (e.g., Schedule, Retention) into the volume's metadata.
//
// Concurrency & Safety:
// This method implements a "Read-Modify-Write" strategy to ensure safety:
//  1. GET: Fetches the current volume details to retrieve existing metadata.
//  2. MERGE: Combines existing tags (e.g., External metadata, Existing policies) with the new policy tags.
//     Incoming keys overwrite existing keys, but unrelated keys are preserved.
//  3. UPDATE: Pushes the merged map back to OpenStack.
//
// Returns:
//   - SubscribedVolume: The updated volume object from OpenStack.
//   - RequestID: The X-Openstack-Request-Id header for tracing.
//   - Error: Any error encountered during the process (after retries).
func (c *Client) CreateVolumeSubscription(
	ctx context.Context,
	volumeID string,
	metadata map[string]string,
) (SubscribedVolume volumes.Volume, RequestID string, Error error) {

	var requestID string
	var subscribedVolume volumes.Volume

	subscriptionOperation := func(innerCtx context.Context) error {
		// 1. Get current volume details to retain existing metadata.
		// We cannot simply overwrite because it might wipe other tags (e.g., billing codes).
		vol, err := volumes.Get(innerCtx, c.BlockStorageClient, volumeID).Extract()
		if err != nil {
			return err
		}

		// 2. Prepare Metadata Map
		currentMeta := vol.Metadata
		if currentMeta == nil {
			currentMeta = make(map[string]string)
		}

		// 3. Merge new policy tags into existing tags
		// Note: New keys overwrite old keys.
		maps.Copy(currentMeta, metadata)

		opts := volumes.UpdateOpts{
			Metadata: currentMeta,
		}

		// 4. Execute Update
		result := volumes.Update(innerCtx, c.BlockStorageClient, volumeID, opts)
		requestID = result.Header.Get("X-Openstack-Request-Id")

		updatedVol, err := result.Extract()
		if err != nil {
			return err
		}

		subscribedVolume = *updatedVol
		return nil
	}

	if err := c.executeWithRetry(ctx, "CreateVolumeSubscription", subscriptionOperation); err != nil {
		return volumes.Volume{}, requestID, err
	}

	return subscribedVolume, requestID, nil
}
