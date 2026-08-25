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
	"github.com/charmbracelet/lipgloss"
	"github.com/go-co-op/gocron-ui/server"
	"github.com/go-co-op/gocron/v2"
	"github.com/spf13/cobra"
)

type DaemonOptions struct {
	CreateSchedule string
	ExpireSchedule string
	BindAddress    string
	BindPort       int
	WebUI          bool
}

var daemonOpts = &DaemonOptions{}

var daemonCommand = &cobra.Command{
	Use:     "daemon",
	Short:   "Run Snapsentry in daemon mode",
	GroupID: "snapsentry",
	Long:    `Starts Snapsentry as a background service that continuously manages snapshot creation and expiry based on configured policies.`,
	RunE: func(cmd *cobra.Command, args []string) error {

		title := headerStyle.Render("Snapsentry - Daemon Mode")
		body := bannerBodyStyle.Render(fmt.Sprintf(
			"Version: %s\n"+
				"Build Date: %s\n"+
				"Commit ID: %s",
			SnapsentryVersion,
			SnapsentryDate,
			SnapsentryCommit,
		))

		// Join them nicely centered using Lipgloss
		banner := lipgloss.JoinVertical(lipgloss.Center, title, body)
		fmt.Println(banner)

		webhookProvider := notifications.Webhook{
			URL:      rootOpts.WebhookURL,
			Username: rootOpts.WebhookUsername,
			Password: rootOpts.WebhookPassword,
		}

		dlog := workflow.SetupLogger(rootOpts.LogLevel, rootOpts.CloudProfile).With("component", "daemon")

		s, err := gocron.NewScheduler()
		if err != nil {
			return fmt.Errorf("failed to create scheduler: %w", err)
		}
		s.Start()
		dlog.Info("Scheduler started", "cloud", rootOpts.CloudProfile)

		// 1. Declare the variable first so it can be used INSIDE the task closure
		var snapshotJob gocron.Job

		// 2. Define the Job
		snapshotJob, snapshotJobError := s.NewJob(
			gocron.CronJob(
				daemonOpts.CreateSchedule,
				false,
			),
			gocron.NewTask(func() {
				// A. Run the Workflow
				workflow.RunProjectSnapshotWorkflow(rootOpts.CloudProfile, rootOpts.Timeout, webhookProvider, rootOpts.LogLevel)

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
				"schedule", daemonOpts.CreateSchedule,
				"next_run", nextRunSnapshot.Format(time.RFC3339))
		}

		// --- Expiry Workflow ---
		var expireJob gocron.Job

		expireJob, expireErr := s.NewJob(
			gocron.CronJob(
				daemonOpts.ExpireSchedule,
				false,
			),
			gocron.NewTask(func() {
				// A. Run the Workflow
				workflow.RunProjectSnapshotExpiryWorkflow(rootOpts.CloudProfile, rootOpts.Timeout, rootOpts.LogLevel, time.Now().UTC(), webhookProvider)

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
				"schedule", daemonOpts.ExpireSchedule,
				"next_run", nextRunSnapshot.Format(time.RFC3339))
		}

		if daemonOpts.WebUI {
			srv := server.NewServer(s, daemonOpts.BindPort, server.WithTitle("Snapsentry Go - Dashboard"))

			go func() {
				addr := fmt.Sprintf("%s:%d", daemonOpts.BindAddress, daemonOpts.BindPort)
				dlog.Info("Snapsentry Scheduler UI started", "address", addr)
				if err := http.ListenAndServe(addr, srv.Router); err != nil {
					dlog.Error("UI server stopped", "error", err)
				}
			}()
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
	daemonCommand.Flags().StringVar(&daemonOpts.CreateSchedule, "create-schedule", "*/10 * * * *", "Cron schedule for snapshot creation")
	daemonCommand.Flags().StringVar(&daemonOpts.ExpireSchedule, "expire-schedule", "0 */6 * * *", "Cron schedule for snapshot expiration")
	daemonCommand.Flags().BoolVar(&daemonOpts.WebUI, "web-ui", false, "Enable the gocron web dashboard")
	daemonCommand.Flags().StringVar(&daemonOpts.BindAddress, "bind-address", "0.0.0.0", "Address to bind the UI server")
	daemonCommand.Flags().IntVar(&daemonOpts.BindPort, "bind-port", 8080, "Port to bind the UI server")
}
