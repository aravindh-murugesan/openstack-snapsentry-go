package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/cloud"
	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/cloud/openstack"
	k8sorchestrator "github.com/aravindh-murugesan/openstack-snapsentry-go/internal/k8s-orchestrator"
	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/notifications"
	"github.com/google/uuid"
	"k8s.io/client-go/tools/clientcmd"
)

func RunKubeOperatorWorkflow(namespace string, cloudName string, timeoutSeconds int, notifyProvider notifications.Webhook, logLevel string) error {

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

	config, err := clientcmd.BuildConfigFromFlags("", "/Users/aravindhmurugesan/Projects/opensource-contributions/openstack-snapsentry-go/kubeconfig.yaml")
	if err != nil {
		logger.Error("Failed to connect to kubernetes cluster", "err", err)
		return fmt.Errorf("failed to connect to kubernetes cluster: %w", err)
	}

	projects, err := ostk.ListSubscribedProjects(ctx)
	if err != nil {
		logger.Error("Failed to fetch projects from openstack", "err", err)
	}

	logger.Info("Discovered openstack projects with snapsentry subscription", "count", len(projects))
	for _, proj := range projects {
		plogger := logger.With("project_name", proj.Name, "project_id", proj.ID, "project_domain", proj.DomainID)
		plogger.Debug("Processing project for snapsentry manager orchestrator")
		plogger.Debug("Attempting to fetch the kubernetes deployment for the project")
		deployments, err := k8sorchestrator.GetSnapsentryDeployment(ctx, config, "snapsentry", proj.ID, proj.Name, proj.DomainID)
		if err != nil {
			plogger.Error("Failed to get deployment from kubernetes", "err", err)
		}
		if len(deployments) != 0 {
			plogger.Info("Deployment exists for the project. No further actions operations to do.")
			continue
		}

		plogger.Info("Deployment missing for the project. Attempting to deploy snapsentry for the project")

		// Create openstack secrets of the project
		plogger.Debug("Attempting to create openstack user for the project")
		password := fmt.Sprintf("snapsentry-%s", uuid.New())
		user, userReqID, err := ostk.CreateSnapsentryUser(
			ctx,
			proj.Name,
			proj.ID,
			proj.DomainID,
			"admin",
			password,
			true,
			"snapsentry",
		)
		if err != nil {
			plogger.Error("Failed to create openstack user for snapsentry. Skipping further steps..", "err", err, "request_id", userReqID)
			continue
		}

		plogger.Info("Snapsentry user has been created for the project", "user", user.Name)

		// Formualate the secret data
		k8s_secret_data := fmt.Sprintf(`
clouds:
  snapsentry-%s-%s:
    auth:
      auth_url: %s
      username: %s
      password: %s
      project_name: %s
      user_domain_id: %s
      project_domain_id: %s
    region_name: %s
    interface: %s
    identity_api_version: 3
    auth_type: password
    timeout: 10
    verify: false`,
			proj.Name, proj.ID,
			ostk.IdentityClient.IdentityEndpoint,
			user.Name,
			password,
			proj.Name,
			user.DomainID,
			proj.DomainID,
			ostk.Region,
			ostk.Interface,
		)

		secret, err := k8sorchestrator.CreateSnapsentrySecret(ctx, config, "snapsentry", proj.ID, proj.Name, proj.DomainID, k8s_secret_data)
		if err != nil {
			plogger.Error("Failed to create kubernetes secret", "err", err)
			continue
		}
		plogger.Info("Successfully created kubernetes secret for the project", "secret_name", secret.Name)

		deployment, err := k8sorchestrator.CreateSnapsentryDeployment(
			ctx,
			config, "snapsentry", proj.ID, proj.Name, proj.DomainID,
		)

		if err != nil {
			plogger.Error("Failed to create a snapsentry deployment for project", "err", err)
			continue
		}

		plogger.Info("Successfully created deployment for snapsentry", "deployment_name", deployment.Name, "namespace", deployment.Namespace)

	}
	return nil
}
