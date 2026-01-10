package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cloudProfile, logLevel string
	timeout                int
	webhookURL             string
	webhookUsername        string
	webhookPassword        string
)

var rootCommand = &cobra.Command{
	Use:     "snapsentry-go",
	Aliases: []string{"snapsentry"},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// 1. Allow 'version' (and 'help') to run without the flag
		if cmd.Name() == "version" || cmd.Name() == "help" {
			return nil
		}

		// 2. Manually enforce the flag for all other commands
		// Assuming 'cloudProfile' is the variable bound to your flag
		if cloudProfile == "" {
			return fmt.Errorf("required flag(s) \"cloud\" not set")
		}

		return nil
	},
	Short: "SnapSentry: OpenStack Snapshot Lifecycle Manager",
	Long: `SnapSentry is a policy-based snapshot scheduler for OpenStack volumes.
It allows you to define Daily, Weekly, and Monthly snapshot policies via volume metadata
and automatically manages the lifecycle (creation and expiry) of those snapshots.

Author: Aravindh Murugesan`,
}

func Execute() error {
	return rootCommand.Execute()
}

func init() {
	rootCommand.AddGroup(&cobra.Group{ID: "snapsentry", Title: "Snapsentry"})

	// Global Peristent Flags with env vars support
	rootCommand.PersistentFlags().StringVar(&cloudProfile, "cloud", "", "Name of the cloud profile as in clouds.yaml (required)")
	rootCommand.PersistentFlags().IntVar(&timeout, "timeout", 0, "Global execution timeout in seconds (0 = run indefinitely)")
	rootCommand.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Logging level (debug, info, warn, error)")
	rootCommand.PersistentFlags().StringVar(&webhookURL, "webhook-url", "", "Webhook URL for alerting")
	rootCommand.PersistentFlags().StringVar(&webhookUsername, "webhook-username", "", "Webhook username for alerting")
	rootCommand.PersistentFlags().StringVar(&webhookPassword, "webhook-password", "", "Webhook password for alerting")
	// Bind to env vars
	_ = viper.BindPFlag("cloud", rootCommand.PersistentFlags().Lookup("cloud"))
	_ = viper.BindPFlag("timeout", rootCommand.PersistentFlags().Lookup("timeout"))
	_ = viper.BindPFlag("log-level", rootCommand.PersistentFlags().Lookup("log_level"))

	viper.SetEnvPrefix("SNAPSENTRY")
	viper.AutomaticEnv()

}
