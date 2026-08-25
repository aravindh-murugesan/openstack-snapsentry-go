package cli

import (
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var (
	SnapsentryVersion string
	SnapsentryCommit  string
	SnapsentryDate    string
)

var versionCommand = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  "Display version, commit hash, build date, and other build information",
	Annotations: map[string]string{
		"skipAuth": "true",
	},
	Run: func(cmd *cobra.Command, args []string) {
		// Fallback to built-in Go module info if ldflags weren't explicitly provided
		if SnapsentryVersion == "" || SnapsentryVersion == "dev" {
			SnapsentryVersion = "dev"
			if info, ok := debug.ReadBuildInfo(); ok {
				if info.Main.Version != "(devel)" && info.Main.Version != "" {
					SnapsentryVersion = info.Main.Version
				}
				for _, setting := range info.Settings {
					if setting.Key == "vcs.revision" && SnapsentryCommit == "" {
						// Grab the first 7 chars of the commit hash for brevity
						if len(setting.Value) > 7 {
							SnapsentryCommit = setting.Value[:7]
						} else {
							SnapsentryCommit = setting.Value
						}
					}
					if setting.Key == "vcs.time" && SnapsentryDate == "" {
						SnapsentryDate = setting.Value
					}
				}
			}
		}

		// Provide defaults if still empty
		if SnapsentryCommit == "" {
			SnapsentryCommit = "unknown"
		}
		if SnapsentryDate == "" {
			SnapsentryDate = "unknown"
		}

		fmt.Println(headerStyle.Render("SnapSentry Info"))
		fmt.Printf("Version:    %s\n", SnapsentryVersion)
		fmt.Printf("Commit:     %s\n", SnapsentryCommit)
		fmt.Printf("Build Date: %s\n", SnapsentryDate)
		fmt.Printf("Go Version: %s\n", runtime.Version())
		fmt.Printf("Platform:   %s / %s\n", runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCommand.AddCommand(versionCommand)
}
