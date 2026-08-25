package cli

import (
	"fmt"

	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/workflow"
	"github.com/spf13/cobra"
)

// Flags for subscribe sub-commands
type SubscribeOptions struct {
	VolumeID      string
	EnablePolicy  bool
	RetentionDays int
	StartTime     string
	TimeZone      string
	WeekDay       string // Weekly only
	DayOfMonth    int    // Monthly only
	IntervalHours int    // Express only
}

var subscribeOpts = &SubscribeOptions{}

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
	RunE:  runSubscribeDaily,
}

func runSubscribeDaily(cmd *cobra.Command, args []string) error {
	fmt.Println(headerStyle.Render("Snapsentry - Daily Subscription"))
	return workflow.SubscribeVolumeDaily(
		rootOpts.CloudProfile, rootOpts.LogLevel, subscribeOpts.VolumeID, subscribeOpts.EnablePolicy, subscribeOpts.RetentionDays, subscribeOpts.StartTime, subscribeOpts.TimeZone,
	)
}

var subscribeWeeklyCommand = &cobra.Command{
	Use:   "weekly",
	Short: "Applies a weekly snapshot schedule",
	Long:  `Configures the target volume with a weekly snapshot policy. This command updates the volume's metadata to enable weekly backups, allowing you to specify the exact day of the week (e.g., "Sunday"), the retention period, and the execution time.`,
	RunE:  runSubscribeWeekly,
}

func runSubscribeWeekly(cmd *cobra.Command, args []string) error {
	fmt.Println(headerStyle.Render("Snapsentry - Weekly Subscription"))
	return workflow.SubscribeVolumeWeekly(
		rootOpts.CloudProfile, rootOpts.LogLevel, subscribeOpts.VolumeID, subscribeOpts.EnablePolicy, subscribeOpts.RetentionDays, subscribeOpts.StartTime, subscribeOpts.TimeZone, subscribeOpts.WeekDay,
	)
}

var subscribeMonthlyCommand = &cobra.Command{
	Use:   "monthly",
	Short: "Applies a monthly snapshot schedule",
	Long:  `Configures the target volume with a monthly snapshot policy. This command updates the volume's metadata to enable monthly backups, allowing you to specify the calendar day (1-31) for execution, along with the retention period and start time.`,
	RunE:  runSubscribeMonthly,
}

func runSubscribeMonthly(cmd *cobra.Command, args []string) error {
	fmt.Println(headerStyle.Render("Snapsentry - Monthly Subscription"))
	return workflow.SubscribeVolumeMonthly(
		rootOpts.CloudProfile, rootOpts.LogLevel, subscribeOpts.VolumeID, subscribeOpts.EnablePolicy, subscribeOpts.RetentionDays, subscribeOpts.StartTime, subscribeOpts.TimeZone, subscribeOpts.DayOfMonth,
	)
}

var subscribeExpressCommand = &cobra.Command{
	Use:   "express",
	Short: "Applies an express snapshot policy",
	Long:  `Configures the target volume with an express (high-frequency) snapshot policy. This divides the day into fixed time buckets (e.g., every 6 hours) starting from midnight in the specified timezone. Valid intervals are 6, 8, or 12 hours.`,
	RunE:  runSubscribeExpress,
}

func runSubscribeExpress(cmd *cobra.Command, args []string) error {
	fmt.Println(headerStyle.Render("Snapsentry - Express Subscription"))

	return workflow.SubscribeVolumeExpress(
		rootOpts.CloudProfile,
		rootOpts.LogLevel,
		subscribeOpts.VolumeID,
		subscribeOpts.EnablePolicy,
		subscribeOpts.RetentionDays,
		subscribeOpts.TimeZone,
		subscribeOpts.IntervalHours,
	)
}

func addStartTimeFlag(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&subscribeOpts.StartTime, "start-time", "", "Snapshot trigger time in HH:MM format (required)")
	_ = cmd.MarkPersistentFlagRequired("start-time")
}

func init() {
	// Shared Flags
	// These flags apply to 'subscribe daily', 'subscribe weekly', and 'subscribe monthly'
	subscribeCommand.PersistentFlags().StringVar(&subscribeOpts.VolumeID, "volume-id", "", "UUID of the OpenStack volume (required)")
	subscribeCommand.PersistentFlags().BoolVar(&subscribeOpts.EnablePolicy, "enabled", true, "Enable or disable this specific policy")
	subscribeCommand.PersistentFlags().IntVar(&subscribeOpts.RetentionDays, "retention", 0, "Retention period in days (required)")
	subscribeCommand.PersistentFlags().StringVar(&subscribeOpts.TimeZone, "timezone", "", "Timezone (e.g. 'UTC', 'America/New_York')")

	_ = subscribeCommand.MarkPersistentFlagRequired("volume-id")
	_ = subscribeCommand.MarkPersistentFlagRequired("retention")

	// Flags specific to 'subscribe express'
	subscribeExpressCommand.PersistentFlags().IntVar(&subscribeOpts.IntervalHours, "interval-hours", 6, "Time interval between snapshots.")

	// Flags specific to 'subscribe daily'
	addStartTimeFlag(subscribeDailyCommand)

	// Flags specific to 'subscribe weekly'
	addStartTimeFlag(subscribeWeeklyCommand)
	subscribeWeeklyCommand.Flags().StringVar(&subscribeOpts.WeekDay, "week-day", "Sunday", "Day of the week (Monday, Tuesday, etc.) (required)")
	_ = subscribeWeeklyCommand.MarkFlagRequired("week-day")

	// Flags specific to 'subscribe monthly'
	addStartTimeFlag(subscribeMonthlyCommand)
	subscribeMonthlyCommand.Flags().IntVar(&subscribeOpts.DayOfMonth, "month-day", 1, "Day of the month (1-31) (required)")
	_ = subscribeMonthlyCommand.MarkFlagRequired("month-day")

	rootCommand.AddCommand(subscribeCommand)
	subscribeCommand.AddCommand(subscribeDailyCommand)
	subscribeCommand.AddCommand(subscribeWeeklyCommand)
	subscribeCommand.AddCommand(subscribeMonthlyCommand)
	subscribeCommand.AddCommand(subscribeExpressCommand)
}
