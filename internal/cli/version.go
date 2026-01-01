package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	SnapsentryVersion, SnapsentryCommit, SnapsentryDate string
)

var versionCommand = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  "Display version, commit hash, build date, and other build information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("SnapSentry version: %s\n", SnapsentryVersion)
		fmt.Printf("Commit: %s\n", SnapsentryCommit)
		fmt.Printf("Built: %s\n", SnapsentryDate)
	},
}

func init() {
	rootCommand.AddCommand(versionCommand)
}
