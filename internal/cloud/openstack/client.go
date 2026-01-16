package openstack

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/cloud"
	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/utils/v2/openstack/clientconfig"
)

// Client manages the connection and service clients for OpenStack interactions.
// It wraps standard gophercloud clients with retry logic and profile management.
type Client struct {
	// ProfileName corresponds to the entry in clouds.yaml
	ProfileName string
	// RetryConfig defines the behavior for transient error handling
	RetryConfig cloud.RetryConfig

	// Internal service clients
	ComputeClient      *gophercloud.ServiceClient
	BlockStorageClient *gophercloud.ServiceClient
	IdentityClient     *gophercloud.ServiceClient
}

// executeWithRetry is a helper to run any operation using the client's retry configuration.
func (c *Client) executeWithRetry(ctx context.Context, opName string, operation func(ctx context.Context) error) error {
	return ExecuteAction(ctx, c.RetryConfig, opName, operation)
}

// GetCloudProviderName returns the identifier for this provider.
func (c *Client) GetCloudProviderName() string {
	return "openstack"
}

// NewClient initializes the OpenStack provider and specific service clients (Cinder, Nova).
// It attempts to authenticate using the configured ProfileName with retry logic.
func (c *Client) NewClient() error {
	slog.Debug("Initializing OpenStack client", "profile", c.ProfileName)

	var provider *gophercloud.ProviderClient

	// authenticateOperation encapsulates the authentication logic to allow
	// the retry helper to re-run it in case of transient network issues.
	authenticateOperation := func(ctx context.Context) error {
		opts := &clientconfig.ClientOpts{
			Cloud: c.ProfileName,
		}

		p, err := clientconfig.AuthenticatedClient(ctx, opts)
		if err != nil {
			return err
		}

		provider = p
		return nil
	}

	// 1. Establish Connection & Authentication
	err := c.executeWithRetry(context.Background(), "OpenStack Authentication", authenticateOperation)
	if err != nil {
		return fmt.Errorf("authentication failed for profile '%s': %w", c.ProfileName, err)
	}

	opts := &clientconfig.ClientOpts{
		Cloud: c.ProfileName,
	}

	// Parse the cloud config yaml file
	cloudConfig, err := clientconfig.GetCloudFromYAML(opts)
	if err != nil {
		return fmt.Errorf("failed to parse cloud config: %w", err)
	}

	// Get Endpoint type
	var availability gophercloud.Availability
	switch cloudConfig.EndpointType {
	case "internal":
		availability = gophercloud.AvailabilityInternal
	case "admin":
		availability = gophercloud.AvailabilityAdmin
	default:
		availability = gophercloud.AvailabilityPublic
	}

	// Prepare endpoint options
	endpointOpts := gophercloud.EndpointOpts{
		Availability: availability,
		Region:       cloudConfig.RegionName,
	}

	// 2. Initialize Block Storage (Cinder) Client
	blockStorage, err := openstack.NewBlockStorageV3(provider, endpointOpts)
	if err != nil {
		return fmt.Errorf("failed to initialize Block Storage v3 client: %w", err)
	}

	// 3. Initialize Compute (Nova) Client
	compute, err := openstack.NewComputeV2(provider, endpointOpts)
	if err != nil {
		return fmt.Errorf("failed to initialize Compute v2 client: %w", err)
	}

	// 4. Initialize Identity (Keystone) NewClient
	identity, err := openstack.NewIdentityV3(provider, endpointOpts)
	if err != nil {
		return fmt.Errorf("failed to initialize Identity V3 client: %w", err)
	}

	// 4. Assign Clients
	c.BlockStorageClient = blockStorage
	c.ComputeClient = compute
	c.IdentityClient = identity

	return nil
}
