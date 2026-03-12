package cli

import (
	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/notifications"
	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/workflow"
	"github.com/spf13/cobra"
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

func init() {
	adminCommand.AddCommand(subscribedProjectsCommand)
	rootCommand.AddCommand(adminCommand)
}
