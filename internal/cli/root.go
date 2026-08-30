package cli

import (
	"fmt"
	"strings"

	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/notifications"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type RootOptions struct {
	CloudProfile        string
	LogLevel            string
	Timeout             int
	WebhookURL          string   // Deprecated, use --all-notifier or --failure-notifier instead.
	AllNotifiers        []string // Notified for both success and failure events
	FailureNotifiers    []string // Notified only for failure events.
	NotificationTargets []notifications.NotificationTarget
}

var rootCommand = &cobra.Command{
	Use:     "snapsentry-go",
	Aliases: []string{"snapsentry"},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Automatically bind all flags to viper
		viper.BindPFlags(cmd.Flags())

		// Sync any values set via Viper (e.g. env vars) back into the Cobra flags,
		// which automatically updates our structs.
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if !f.Changed && viper.IsSet(f.Name) {
				cmd.Flags().Set(f.Name, viper.GetString(f.Name))
			}
		})

		// 1. Allow 'help' and any annotated command to run without the cloud flag
		if cmd.Name() == "help" || cmd.Annotations["skipAuth"] == "true" {
			return nil
		}

		if rootOpts.CloudProfile == "" {
			return fmt.Errorf("required flag(s) \"cloud\" not set")
		}

		// For backward compatibility, if --webhook-url is set, add it to the failure notifiers.
		if rootOpts.WebhookURL != "" {
			rootOpts.FailureNotifiers = append(rootOpts.FailureNotifiers, rootOpts.WebhookURL)
		}

		// Create Notification targets based on user input.
		var targets []notifications.NotificationTarget
		for _, turl := range rootOpts.AllNotifiers {
			targets = append(targets, notifications.NotificationTarget{
				URL:           turl,
				NotifySuccess: true,
			})
		}
		for _, turl := range rootOpts.FailureNotifiers {
			targets = append(targets, notifications.NotificationTarget{
				URL:           turl,
				NotifySuccess: false,
			})
		}
		rootOpts.NotificationTargets = targets

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

var rootOpts *RootOptions = &RootOptions{}

func init() {
	rootCommand.AddGroup(&cobra.Group{ID: "snapsentry", Title: "Snapsentry"})

	// Global Peristent Flags
	rootCommand.PersistentFlags().StringVar(&rootOpts.CloudProfile, "cloud", "", "Name of the cloud profile as in clouds.yaml (required)")
	rootCommand.PersistentFlags().IntVar(&rootOpts.Timeout, "timeout", 0, "Global execution timeout in seconds (0 = run indefinitely)")
	rootCommand.PersistentFlags().StringVar(&rootOpts.LogLevel, "log-level", "info", "Logging level (debug, info, warn, error)")
	rootCommand.PersistentFlags().StringSliceVar(&rootOpts.AllNotifiers, "all-notifier", []string{}, "List of webhook urls to be notified for both success and failure events. \n (e.g., --all-notifier=url1,url2 or use multiple flags)")
	rootCommand.PersistentFlags().StringSliceVar(&rootOpts.FailureNotifiers, "failure-notifier", []string{}, "List of webhook urls to be notified only for failure events. \n (e.g., --failure-notifier=url1,url2 or use multiple flags)")
	rootCommand.PersistentFlags().StringVar(&rootOpts.WebhookURL, "webhook-url", "", "Webhook URL to be notified for failure events. (Deprecated) Use --failure-notifier instead.")

	// Set up Viper Environment Variable handling
	viper.SetEnvPrefix("SNAPSENTRY")
	// Replace dashes with underscores for bash env vars (e.g., --all-notifier becomes SNAPSENTRY_ALL_NOTIFIER)
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
}
