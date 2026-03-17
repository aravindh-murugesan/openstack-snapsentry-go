package cli

import (
	"fmt"
	"os"

	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/notifications"
	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/workflow"
	"github.com/spf13/cobra"
)

var (
	kubeconfig               string
	incluster                bool
	controllerRequestCpu     string
	controllerRequestMem     string
	controllerLimitCpu       string
	controllerLimitMem       string
	controllerNamespace      string
	controllerSnasentryImage string
)

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
			URL:      webhookURL,
			Username: webhookUsername,
			Password: webhookPassword,
		}

		return workflow.RunAdminProjectDisoceryWorkflow(
			cloudProfile,
			timeout,
			webhookProvider,
			logLevel,
		)
	},
}

var orchestratorCommand = &cobra.Command{
	Use: "orchestrator",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if !incluster && kubeconfig == "" {
			return fmt.Errorf("Either kubeconfig path or incluster flag has to be provided. Both of them cannot be empty")
		}

		if incluster && kubeconfig != "" {
			return fmt.Errorf("Either kubeconfig path or incluster flag has to be provider. Both of them cannot be provided at once.")
		}

		if kubeconfig != "" {
			_, err := os.Stat(kubeconfig)
			if err != nil {
				return fmt.Errorf("Failed to access the kubeconfig file: %w", err)
			}
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {

		webhookProvider := notifications.Webhook{
			URL:      webhookURL,
			Username: webhookUsername,
			Password: webhookPassword,
		}

		workflow.RunKubeOperatorWorkflow(
			"snapsentry",
			cloudProfile,
			timeout,
			webhookProvider,
			logLevel,
			kubeconfig,
			incluster,
			controllerRequestCpu,
			controllerRequestMem,
			controllerLimitCpu,
			controllerLimitMem,
			controllerSnasentryImage,
		)
	},
}

func init() {
	// Orcherstrator command flags
	orchestratorCommand.PersistentFlags().BoolVar(
		&incluster, "incluster", false,
		"Set this flag when you deploy the snapsentry orchestrator in the same kubernetes cluster as your snapsentry controller",
	)
	orchestratorCommand.PersistentFlags().StringVar(
		&kubeconfig, "kubeconfig", "",
		"Path to the kubernetes config to connect to a remote cluster",
	)
	orchestratorCommand.PersistentFlags().StringVar(
		&controllerRequestCpu, "controller-requests-cpu", "64m",
		"CPU Requests for Snapsentry Kubernetes Deployment",
	)
	orchestratorCommand.PersistentFlags().StringVar(
		&controllerRequestMem, "controller-requests-memory", "32Mi",
		"Memory Requests for Snapsentry Kubernetes Deployment",
	)
	orchestratorCommand.PersistentFlags().StringVar(
		&controllerLimitCpu, "controller-limit-cpu", "256m",
		"CPU Limits for Snapsentry Kubernetes Deployment",
	)
	orchestratorCommand.PersistentFlags().StringVar(
		&controllerLimitMem, "controller-limit-memory", "128Mi",
		"CPU Memory for Snapsentry Kubernetes Deployment",
	)
	orchestratorCommand.PersistentFlags().StringVar(
		&controllerNamespace, "controller-namespace", "snapsentry",
		"Target namespace for the snapsentry controller",
	)
	orchestratorCommand.PersistentFlags().StringVar(
		&controllerSnasentryImage, "workload-snapsentry-image", "ghcr.io/aravindh-murugesan/openstack-snapsentry-go:sha-5d331af",
		"Container Image for the Snapsentry controller",
	)

	adminCommand.AddCommand(subscribedProjectsCommand)
	adminCommand.AddCommand(orchestratorCommand)
	rootCommand.AddCommand(adminCommand)
}
