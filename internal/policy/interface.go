package policy

import (
	"time"
)

// SnapshotPolicy defines the contract that all scheduling strategies (Daily, Weekly, Monthly) must implement.
// It decouples the scheduling logic from the specific storage mechanism (OpenStack, oVirt, etc.).
type SnapshotPolicy interface {
	// Normalize validates the policy configuration and sets sane defaults
	// (e.g., defaulting to UTC if no timezone is provided).
	Normalize() error

	// ParseFromMetadata hydrates the policy struct from a metadata map
	// retrieved from a volume or disk.
	ParseFromMetadata(metadata map[string]string) error

	// ToOpenstackMetadata serializes the policy configuration into a map of strings
	// suitable for storage as OpenStack Volume metadata.
	ToOpenstackMetadata() map[string]string

	// Evaluate determines if a snapshot should be triggered right now.
	// It compares the current time ('now') against the policy schedule and checks
	// the 'lastSnapshot' to ensure idempotency (preventing duplicate snapshots).
	Evaluate(now time.Time, lastSnapshot LastSnapshotInfo) (PolicyEvalResult, error)

	// GetPolicyType returns the unique identifier for this policy (e.g., "daily", "weekly").
	GetPolicyType() string

	// GetPolicyRetention returns the configured retention period in days.
	GetPolicyRetention() int

	// IsEnabled returns if the snapshot policy is enabled or not.
	IsEnabled() bool
}
