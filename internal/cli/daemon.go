package cli

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/notifications"
	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/workflow"
	"github.com/go-co-op/gocron-ui/server"
	"github.com/go-co-op/gocron/v2"
	"github.com/spf13/cobra"
)

var (
	createSchedule string
	expireSchedule string
	bindAddress    string
)

var daemonCommand = &cobra.Command{
	Use:     "daemon",
	Short:   "Run Snapsentry in daemon mode",
	GroupID: "snapsentry",
	Long:    `Starts Snapsentry as a background service that continuously manages snapshot creation and expiry based on configured policies.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		banner := fmt.Sprintf("Snapsentry - Daemon Mode \n\nVersion: %s\nBuild Date: %s", SnapsentryVersion, SnapsentryDate)
		fmt.Println(headerStyle.Render(banner))

		webhookProvider := notifications.Webhook{
			URL:      webhookURL,
			Username: webhookUsername,
			Password: webhookPassword,
		}

		dlog := workflow.SetupLogger(logLevel, cloudProfile).With("component", "daemon")

		s, err := gocron.NewScheduler()
		if err != nil {
			return fmt.Errorf("failed to create scheduler: %w", err)
		}
		s.Start()
		dlog.Info("Scheduler started", "cloud", cloudProfile)

		// 1. Declare the variable first so it can be used INSIDE the task closure
		var snapshotJob gocron.Job

		// 2. Define the Job
		snapshotJob, snapshotJobError := s.NewJob(
			gocron.CronJob(
				createSchedule,
				false,
			),
			gocron.NewTask(func() {
				// A. Run the Workflow
				workflow.RunProjectSnapshotWorkflow(cloudProfile, timeout, webhookProvider, logLevel)

				// B. Calculate and Log the Next Run (Post-Execution)
				if snapshotJob != nil {
					if nextRun, err := snapshotJob.NextRun(); err == nil {
						dlog.Info("Snapshot Workflow completed",
							"next_run", nextRun.Format(time.RFC3339),
							"job_id", snapshotJob.ID())
					}
				}
			}),
			gocron.WithName("Snapshot Creation Workflow"),
			gocron.WithSingletonMode(gocron.LimitModeReschedule),
		)
		if snapshotJobError != nil {
			return snapshotJobError
		}

		// 3. Log the Initial Next Run (Pre-Execution)
		if nextRunSnapshot, err := snapshotJob.NextRun(); err == nil {
			dlog.Info("Job Scheduled",
				"job_name", snapshotJob.Name(),
				"job_id", snapshotJob.ID(),
				"schedule", createSchedule,
				"next_run", nextRunSnapshot.Format(time.RFC3339))
		}

		// --- Expiry Workflow ---
		var expireJob gocron.Job

		expireJob, expireErr := s.NewJob(
			gocron.CronJob(
				expireSchedule,
				false,
			),
			gocron.NewTask(func() {
				// A. Run the Workflow
				workflow.RunProjectSnapshotExpiryWorkflow(cloudProfile, timeout, logLevel, time.Now().UTC(), webhookProvider)

				// B. Calculate and Log the Next Run (Post-Execution)
				if expireJob != nil {
					if nextRun, err := expireJob.NextRun(); err == nil {
						dlog.Info("Snapshot Workflow completed",
							"next_run", nextRun.Format(time.RFC3339),
							"job_id", expireJob.ID())
					}
				}
			}),
			gocron.WithName("Snapshot Expiry Workflow"),
			gocron.WithSingletonMode(gocron.LimitModeReschedule),
		)
		if expireErr != nil {
			return expireErr
		}

		// 3. Log the Initial Next Run (Pre-Execution)
		if nextRunSnapshot, err := expireJob.NextRun(); err == nil {
			dlog.Info("Job Scheduled",
				"job_name", expireJob.Name(),
				"job_id", expireJob.ID(),
				"schedule", expireSchedule,
				"next_run", nextRunSnapshot.Format(time.RFC3339))
		}

		srv := server.NewServer(s, 8080, server.WithTitle("Snapsentry Go - Dashboard")) // with custom title if you want to customize the title of the UI (optional)
		dlog.Info("Snapsentry Scheduler UI started", "address", bindAddress)
		if err := http.ListenAndServe(bindAddress, srv.Router); err != nil {
			dlog.Error("Failed to start UI server", "error", err)
			return s.Shutdown()
		}

		// 4. Block Main Thread until Signal
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		dlog.Warn("Shutting down scheduler due to system signal...")
		return s.Shutdown()
	},
}

func init() {
	rootCommand.AddCommand(daemonCommand)
	daemonCommand.Flags().StringVar(&createSchedule, "create-schedule", "*/10 * * * *", "Cron schedule for snapshot creation")
	daemonCommand.Flags().StringVar(&expireSchedule, "expire-schedule", "0 */6 * * *", "Cron schedule for snapshot expiration")
	daemonCommand.Flags().StringVar(&bindAddress, "bind-address", "0.0.0.0:8080", "Address to bind the UI server")
}
