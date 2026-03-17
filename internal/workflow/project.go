package workflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/cloud"
	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/cloud/openstack"
	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/notifications"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/google/uuid"
)

func RunAdminProjectDisoceryWorkflow(cloudName string, timeoutSeconds int, notifyProvider notifications.Webhook, logLevel string) error {
	// 1. Initialize Structured Logger
	// We use slog with tint for colorized, human-readable logs in development/CLI usage.
	logger := SetupLogger(logLevel, cloudName)

	snapsentryRunID := fmt.Sprintf("req-%s", uuid.New().String())
	logger = logger.With("snapsentry_id", snapsentryRunID)
	logger.Info("Initializing Project discovery with tag `snapsentry-enabled`")

	// 2. Setup Context (Optional Timeout)
	// This ensures the job doesn't hang indefinitely if the API becomes unresponsive.
	ctx := context.Background()

	if timeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
		defer cancel()
		logger.Debug("Global workflow timeout configured", "timeout_seconds", timeoutSeconds)
	}

	// 3. Initialize OpenStack Client
	// Configures retries to handle transient network glitches during API calls.
	ostk := openstack.Client{
		ProfileName: cloudName,
		RetryConfig: cloud.RetryConfig{
			MaxRetries:       3,
			BaseDelay:        2 * time.Second,
			MaxDelay:         10 * time.Second,
			OperationTimeout: 30 * time.Second,
		},
	}

	logger.Debug("Attempting to connect to OpenStack", "profile", cloudName)
	if err := ostk.NewClient(); err != nil {
		logger.Error("OpenStack client initialization failed", "error", err)
		return fmt.Errorf("client initialization failed: %w", err)
	}

	logger.Debug("OpenStack connection established successfully")

	subedProjects, err := ostk.ListSubscribedProjects(ctx)
	if err != nil {
		logger.Error("Project discovery failed", "error", err)
		return fmt.Errorf("listing project failed: %w", err)
	}

	var (
		purple    = lipgloss.Color("99")
		gray      = lipgloss.Color("245")
		lightGray = lipgloss.Color("241")

		headerStyle  = lipgloss.NewStyle().Foreground(purple).Bold(true).Align(lipgloss.Center)
		cellStyle    = lipgloss.NewStyle().Padding(0, 1)
		oddRowStyle  = cellStyle.Foreground(gray)
		evenRowStyle = cellStyle.Foreground(lightGray)
	)

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(purple)).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row == table.HeaderRow:
				return headerStyle
			case row%2 == 0:
				return evenRowStyle
			default:
				return oddRowStyle
			}
		}).
		Headers("PROJECT ID", "PROJECT NAME", "DOMAIN ID", "TAGS")

	logger.Info("Fetched subscribed projects", "count", len(subedProjects))
	for _, i := range subedProjects {
		t.Row(i.ID, i.Name, i.DomainID, strings.Join(i.Tags, ", "))
	}

	fmt.Println(t)
	return nil
}
