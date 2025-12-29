package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	version, commit, date string
)

var versionCommand = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  "Display version, commit hash, build date, and other build information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("SnapSentry version: %s\n", version)
		fmt.Printf("Commit: %s\n", commit)
		fmt.Printf("Built: %s\n", date)
	},
}

func init() {
	rootCommand.AddCommand(versionCommand)
}
