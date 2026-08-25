package cli

import (
	"fmt"
	"time"

	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/notifications"
	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/workflow"
	"github.com/spf13/cobra"
)

var expireSnapshotCommand = &cobra.Command{
	Use:     "expire-snapshots",
	GroupID: "snapsentry",
	Short:   "Execute the snapshot expiry workflow",
	Long:    `Scans all managed snapshots in the project, compares their stored expiry dates against the current UTC time, and permanently deletes those that have exceeded their retention period.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(headerStyle.Render("Snapsentry - Expiry Workflow"))
		webhookProvider := notifications.Webhook{
			URL:      rootOpts.WebhookURL,
			Username: rootOpts.WebhookUsername,
			Password: rootOpts.WebhookPassword,
		}
		return workflow.RunProjectSnapshotExpiryWorkflow(
			rootOpts.CloudProfile,
			rootOpts.Timeout,
			rootOpts.LogLevel,
			time.Now().UTC(),
			webhookProvider,
		)
	},
}

func init() {
	rootCommand.AddCommand(expireSnapshotCommand)
}
