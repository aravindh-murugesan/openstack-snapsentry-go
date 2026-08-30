package notifications

import "github.com/aravindh-murugesan/openstack-snapsentry-go/internal/policy"

type SnapshotCreationFailure struct {
	Service    string                      `json:"service"`
	VMName     string                      `json:"virtual_machine_name"`
	VMID       string                      `json:"virtual_machine_id"`
	VolumeID   string                      `json:"volume_id"`
	SnapshotID string                      `json:"snapshot_id"`
	Message    string                      `json:"message"`
	Window     policy.SnapshotPolicyWindow `json:"snapshot_window"`
}

type SnapshotCreationSuccess struct {
	Service    string                      `json:"service"`
	VMName     string                      `json:"virtual_machine_name,omitempty"`
	VMID       string                      `json:"virtual_machine_id,omitempty"`
	VolumeID   string                      `json:"volume_id"`
	SnapshotID string                      `json:"snapshot_id"`
	Window     policy.SnapshotPolicyWindow `json:"snapshot_window"`
}

type SnapshotExpiryFailure struct {
	Service          string                  `json:"service"`
	SnapshotID       string                  `json:"snapshot_id"`
	VolumeID         string                  `json:"volume_id"`
	SnapshotMetadata policy.SnapshotMetadata `json:"snapshot_metadata"`
	Message          string                  `json:"message"`
}

type SnapshotExpirySuccess struct {
	Service          string                  `json:"service"`
	SnapshotID       string                  `json:"snapshot_id"`
	VolumeID         string                  `json:"volume_id"`
	SnapshotMetadata policy.SnapshotMetadata `json:"snapshot_metadata"`
}
