package notifications

import "github.com/aravindh-murugesan/openstack-snapsentry-go/internal/policy"

type Webhook struct {
	URL      string
	Username string
	Password string
	Verify   bool
}

type SnapshotCreationFailure struct {
	Service    string                      `json:"service"`
	VMName     string                      `json:"virtual_machine_name"`
	VMID       string                      `json:"virtual_machine_id"`
	VolumeID   string                      `json:"volume_id"`
	SnapshotID string                      `json:"snapshot_id"`
	Message    string                      `json:"message"`
	Window     policy.SnapshotPolicyWindow `json:"snapshot_window"`
}
