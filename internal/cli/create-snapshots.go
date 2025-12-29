package cli

import (
	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/workflow"
	"github.com/spf13/cobra"
)

var createSnapshotCommand = &cobra.Command{
	Use:     "create-snapshots",
	GroupID: "snapsentry",
	Short:   "Execute the snapshot creation workflow",
	Long:    `Scans for volumes with enabled policies, evaluates their schedules against the current time, and creates snapshots if required.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return workflow.RunProjectSnapshotWorkflow(
			cloudProfile,
			timeout,
			logLevel,
		)
	},
}

func init() {
	rootCommand.AddCommand(createSnapshotCommand)
}
