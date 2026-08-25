package cli

import (
	"fmt"
	"os"

	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/notifications"
	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/workflow"
	"github.com/spf13/cobra"
)

type AdminOptions struct {
	Kubeconfig                string
	Incluster                 bool
	ControllerRequestCpu      string
	ControllerRequestMem      string
	ControllerLimitCpu        string
	ControllerLimitMem        string
	ControllerNamespace       string
	ControllerSnapsentryImage string
}

var adminOpts = &AdminOptions{}

var adminCommand = &cobra.Command{
	Use:     "admin",
	Short:   "Admin Commands",
	GroupID: "snapsentry",
}

var subscribedProjectsCommand = &cobra.Command{
	Use:   "list-subscribed-projects",
	Short: "List all the projects with Snapsentry subscription tags. This is only for adminstators for review",
	RunE: func(cmd *cobra.Command, args []string) error {
		webhookProvider := notifications.Webhook{
			URL:      rootOpts.WebhookURL,
			Username: rootOpts.WebhookUsername,
			Password: rootOpts.WebhookPassword,
		}

		return workflow.RunAdminProjectDisoceryWorkflow(
			rootOpts.CloudProfile,
			rootOpts.Timeout,
			webhookProvider,
			rootOpts.LogLevel,
		)
	},
}

var orchestratorCommand = &cobra.Command{
	Use: "orchestrator",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if !adminOpts.Incluster && adminOpts.Kubeconfig == "" {
			return fmt.Errorf("Either kubeconfig path or incluster flag has to be provided. Both of them cannot be empty")
		}

		if adminOpts.Incluster && adminOpts.Kubeconfig != "" {
			return fmt.Errorf("Either kubeconfig path or incluster flag has to be provider. Both of them cannot be provided at once.")
		}

		if adminOpts.Kubeconfig != "" {
			_, err := os.Stat(adminOpts.Kubeconfig)
			if err != nil {
				return fmt.Errorf("Failed to access the kubeconfig file: %w", err)
			}
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {

		webhookProvider := notifications.Webhook{
			URL:      rootOpts.WebhookURL,
			Username: rootOpts.WebhookUsername,
			Password: rootOpts.WebhookPassword,
		}

		workflow.RunKubeOperatorWorkflow(
			"snapsentry",
			rootOpts.CloudProfile,
			rootOpts.Timeout,
			webhookProvider,
			rootOpts.LogLevel,
			adminOpts.Kubeconfig,
			adminOpts.Incluster,
			adminOpts.ControllerRequestCpu,
			adminOpts.ControllerRequestMem,
			adminOpts.ControllerLimitCpu,
			adminOpts.ControllerLimitMem,
			adminOpts.ControllerSnapsentryImage,
		)
	},
}

func init() {
	// Orcherstrator command flags
	orchestratorCommand.PersistentFlags().BoolVar(
		&adminOpts.Incluster, "incluster", false,
		"Set this flag when you deploy the snapsentry orchestrator in the same kubernetes cluster as your snapsentry controller",
	)
	orchestratorCommand.PersistentFlags().StringVar(
		&adminOpts.Kubeconfig, "kubeconfig", "",
		"Path to the kubernetes config to connect to a remote cluster",
	)
	orchestratorCommand.PersistentFlags().StringVar(
		&adminOpts.ControllerRequestCpu, "controller-requests-cpu", "64m",
		"CPU Requests for Snapsentry Kubernetes Deployment",
	)
	orchestratorCommand.PersistentFlags().StringVar(
		&adminOpts.ControllerRequestMem, "controller-requests-memory", "32Mi",
		"Memory Requests for Snapsentry Kubernetes Deployment",
	)
	orchestratorCommand.PersistentFlags().StringVar(
		&adminOpts.ControllerLimitCpu, "controller-limit-cpu", "256m",
		"CPU Limits for Snapsentry Kubernetes Deployment",
	)
	orchestratorCommand.PersistentFlags().StringVar(
		&adminOpts.ControllerLimitMem, "controller-limit-memory", "128Mi",
		"CPU Memory for Snapsentry Kubernetes Deployment",
	)
	orchestratorCommand.PersistentFlags().StringVar(
		&adminOpts.ControllerNamespace, "controller-namespace", "snapsentry",
		"Target namespace for the snapsentry controller",
	)
	orchestratorCommand.PersistentFlags().StringVar(
		&adminOpts.ControllerSnapsentryImage, "workload-snapsentry-image", "ghcr.io/aravindh-murugesan/openstack-snapsentry-go:sha-5d331af",
		"Container Image for the Snapsentry controller",
	)

	adminCommand.AddCommand(subscribedProjectsCommand)
	adminCommand.AddCommand(orchestratorCommand)
	rootCommand.AddCommand(adminCommand)
}
