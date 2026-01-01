package openstack

import (
	"context"
	"fmt"
	"maps"

	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/policy"
	"github.com/gophercloud/gophercloud/v2/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/v2/pagination"
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

// ListSubscribedVolumes discovers all volumes that are currently managed by SnapSentry.
// It filters the OpenStack volume list by checking for the presence of the
// generic management tag (defined in policy.ManagedTag).
//
// Features:
//   - Pagination: Automatically traverses all pages of results from the OpenStack API
//     to ensure a complete dataset is returned, even for large environments.
//
// Returns:
//   - SubscribedVolumes: A slice containing every volume with the managed tag.
//   - Error: Detailed error if the operation fails after max retries.
func (c *Client) ListSubscribedVolumes(ctx context.Context) (SubscribedVolumes []volumes.Volume, Error error) {
	var allVolumes []volumes.Volume

	// Define the operation to be wrapped in the retry loop
	listOperation := func(innerCtx context.Context) error {
		// Reset slice on every retry attempt to avoid duplicate data if a retry happens halfway
		allVolumes = []volumes.Volume{}

		opts := volumes.ListOpts{
			Metadata: map[string]string{
				policy.ManagedTag: "true",
			},
		}

		pager := volumes.List(c.BlockStorageClient, opts)

		// Iterate through pages
		// innerCtx is the context controlled by executeWithRetry (has the timeout)
		err := pager.EachPage(innerCtx, func(ctx context.Context, page pagination.Page) (bool, error) {
			vols, err := volumes.ExtractVolumes(page)
			if err != nil {
				return false, err // Stop iteration and return error
			}

			allVolumes = append(allVolumes, vols...)
			return true, nil // Continue to next page
		})

		return err
	}

	// Execute with resilience
	if err := c.executeWithRetry(ctx, "ListSubscribedVolumes", listOperation); err != nil {
		return nil, fmt.Errorf("failed to list subscribed volumes: %w", err)
	}

	return allVolumes, nil
}

func (c *Client) GroupVolumeByVMAttachment(volumeList []volumes.Volume) VMGroupedVolumeList {

	result := VMGroupedVolumeList{
		Attached:      make(map[string][]volumes.Volume),
		MultiAttached: make([]volumes.Volume, 0),
		Unattached:    make([]volumes.Volume, 0),
	}

	for _, v := range volumeList {
		if len(v.Attachments) == 0 {
			result.Unattached = append(result.Unattached, v)
		} else if len(v.Attachments) > 1 {
			result.MultiAttached = append(result.MultiAttached, v)
		} else {
			serverID := v.Attachments[0].ServerID
			result.Attached[serverID] = append(result.Attached[serverID], v)
		}
	}

	return result

}
