package k8sorchestrator

import (
	"fmt"

	"github.com/aravindh-murugesan/openstack-snapsentry-go/internal/notifications"
	"github.com/robfig/cron/v3"
)

type ProjectInfo struct {
	ProjectID   string
	ProjectName string
	DomainID    string
}

type DeploymentConfig struct {
	Namespace           string
	ProjectInfo         ProjectInfo
	RequestsCPU         string
	RequestsMemory      string
	LimitCPU            string
	LimitMemory         string
	Image               string
	LogLevel            string
	NotificationTargets []notifications.NotificationTarget
	CreationCron        string
	ExpiryCron          string
}

func (d *DeploymentConfig) Validate() error {
	if d.Namespace == "" {
		return fmt.Errorf("Namespace must be provided")
	}

	if d.ProjectInfo.ProjectID == "" || d.ProjectInfo.ProjectName == "" || d.ProjectInfo.DomainID == "" {
		return fmt.Errorf("ProjectID, ProjectName, DomainID must be provided")
	}

	if d.Image == "" {
		return fmt.Errorf("Image name must be provided")
	}

	if d.LogLevel != "info" && d.LogLevel != "debug" && d.LogLevel != "warn" && d.LogLevel != "error" {
		d.LogLevel = "info"
	}

	if d.RequestsCPU == "" {
		d.RequestsCPU = "64m"
	}

	if d.RequestsMemory == "" {
		d.RequestsMemory = "32Mi"
	}

	if d.LimitCPU == "" {
		d.LimitCPU = "256m"
	}

	if d.LimitMemory == "" {
		d.LimitMemory = "64Mi"
	}

	if d.CreationCron != "" {
		if _, err := cron.ParseStandard(d.CreationCron); err != nil {
			return fmt.Errorf("invalid CreationCron: %w", err)
		}
	}

	if d.ExpiryCron != "" {
		if _, err := cron.ParseStandard(d.ExpiryCron); err != nil {
			return fmt.Errorf("invalid ExpiryCron: %w", err)
		}
	}

	return nil
}
