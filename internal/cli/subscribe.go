package cli

import (
	"fmt"

	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/workflow"
	"github.com/spf13/cobra"
)

// Flags for subscribe sub-commands
var (
	volumeID      string
	enablePolicy  bool
	retentionDays int
	startTime     string
	timeZone      string
	weekDay       string // Weekly only
	dayOfMonth    int    // Monthly only
	intervalHours int    // Express only
)

var subscribeCommand = &cobra.Command{
	Use:     "subscribe",
	Short:   "Configure snapshot policies for a volume",
	Long:    `Updates the metadata of a specific OpenStack volume to attach Daily, Weekly, or Monthly snapshot schedules. It validates the provided configuration (e.g., time formats, retention periods) and applies the changes immediately.`,
	GroupID: "snapsentry",
}

var subscribeDailyCommand = &cobra.Command{
	Use:   "daily",
	Short: "Applies a daily snapshot schedule",
	Long:  `Configures the target volume with a daily snapshot policy. This command updates the volume's metadata to enable daily backups, setting the specific retention period (in days) and the precise time of day (HH:MM) for the snapshot trigger.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(headerStyle.Render("Snapsentry - Daily Subscription"))
		return workflow.SubscribeVolumeDaily(
			cloudProfile, logLevel, volumeID, enablePolicy, retentionDays, startTime, timeZone,
		)
	},
}

var subscribeWeeklyCmd = &cobra.Command{
	Use:   "weekly",
	Short: "Applies a weekly snapshot schedule",
	Long:  `Configures the target volume with a weekly snapshot policy. This command updates the volume's metadata to enable weekly backups, allowing you to specify the exact day of the week (e.g., "Sunday"), the retention period, and the execution time.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(headerStyle.Render("Snapsentry - Weekly Subscription"))
		return workflow.SubscribeVolumeWeekly(
			cloudProfile, logLevel, volumeID, enablePolicy, retentionDays, startTime, timeZone, weekDay,
		)
	},
}

var subscribeMonthlyCmd = &cobra.Command{
	Use:   "monthly",
	Short: "Applies a monthly snapshot schedule",
	Long:  `Configures the target volume with a monthly snapshot policy. This command updates the volume's metadata to enable monthly backups, allowing you to specify the calendar day (1-31) for execution, along with the retention period and start time.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(headerStyle.Render("Snapsentry - Monthly Subscription"))
		return workflow.SubscribeVolumeMonthly(
			cloudProfile, logLevel, volumeID, enablePolicy, retentionDays, startTime, timeZone, dayOfMonth,
		)
	},
}

var subscribeExpressCmd = &cobra.Command{
	Use:   "express",
	Short: "Applies an express snapshot policy",
	Long:  `Configures the target volume with an express (high-frequency) snapshot policy. This divides the day into fixed time buckets (e.g., every 6 hours) starting from midnight in the specified timezone. Valid intervals are 6, 8, or 12 hours.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(headerStyle.Render("Snapsentry - Express Subscription"))

		return workflow.SubscribeVolumeExpress(
			cloudProfile,
			logLevel,
			volumeID,
			enablePolicy,
			retentionDays,
			timeZone,
			intervalHours,
		)
	},
}

func init() {

	// Shared Flags
	// These flags apply to 'subscribe daily', 'subscribe weekly', and 'subscribe monthly'
	subscribeCommand.PersistentFlags().StringVar(&volumeID, "volume-id", "", "UUID of the OpenStack volume (required)")
	subscribeCommand.PersistentFlags().BoolVar(&enablePolicy, "enabled", true, "Enable or disable this specific policy")
	subscribeCommand.PersistentFlags().IntVar(&retentionDays, "retention", 0, "Retention period in days (required)")
	subscribeCommand.PersistentFlags().StringVar(&timeZone, "timezone", "", "Timezone (e.g. 'UTC', 'America/New_York')")

	_ = subscribeCommand.MarkPersistentFlagRequired("volume-id")
	_ = subscribeCommand.MarkPersistentFlagRequired("retention")

	// Flags specific to 'subscribe express'
	subscribeExpressCmd.PersistentFlags().IntVar(&intervalHours, "interval-hours", 6, "Time interval between snapshots.")

	// Flags specific to 'subscribe daily'
	subscribeDailyCommand.PersistentFlags().StringVar(&startTime, "start-time", "", "Snapshot trigger time in HH:MM format (required)")
	_ = subscribeDailyCommand.MarkPersistentFlagRequired("start-time")

	// Flags specific to 'subscribe weekly'
	subscribeWeeklyCmd.PersistentFlags().StringVar(&startTime, "start-time", "", "Snapshot trigger time in HH:MM format (required)")
	subscribeWeeklyCmd.Flags().StringVar(&weekDay, "week-day", "Sunday", "Day of the week (Monday, Tuesday, etc.) (required)")
	_ = subscribeWeeklyCmd.MarkFlagRequired("week-day")
	_ = subscribeWeeklyCmd.MarkPersistentFlagRequired("start-time")

	// Flags specific to 'subscribe monthly'
	subscribeMonthlyCmd.PersistentFlags().StringVar(&startTime, "start-time", "", "Snapshot trigger time in HH:MM format (required)")
	subscribeMonthlyCmd.Flags().IntVar(&dayOfMonth, "month-day", 1, "Day of the month (1-31) (required)")
	_ = subscribeMonthlyCmd.MarkFlagRequired("month-day")
	_ = subscribeMonthlyCmd.MarkPersistentFlagRequired("start-time")

	rootCommand.AddCommand(subscribeCommand)
	subscribeCommand.AddCommand(subscribeDailyCommand)
	subscribeCommand.AddCommand(subscribeWeeklyCmd)
	subscribeCommand.AddCommand(subscribeMonthlyCmd)
	subscribeCommand.AddCommand(subscribeExpressCmd)
}
