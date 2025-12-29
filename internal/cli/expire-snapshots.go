package cli

import (
	"fmt"
	"time"

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
		return workflow.RunProjectSnapshotExpiryWorkflow(
			cloudProfile,
			timeout,
			logLevel,
			time.Now().UTC())
	},
}

func init() {
	rootCommand.AddCommand(expireSnapshotCommand)
}
