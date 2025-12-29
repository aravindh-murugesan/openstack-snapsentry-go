package policy

import (
	"strconv"
	"time"
)

// SnapshotMetadata defines the schema for the metadata stored on a created snapshot.
// It is used by the expiry workflow to determine when a snapshot can be safely deleted
// and during snapshot creation workflow.
type SnapshotMetadata struct {
	// Managed indicates if this snapshot is owned by SnapSentry.
	// If false, the expiry workflow will ignore this snapshot.
	Managed bool `json:"x-snapsentry-managed"`

	// ExpiryDate is the computed timestamp after which this snapshot is eligible for deletion.
	// This is calculated at creation time (StartWindow + RetentionDays).
	ExpiryDate time.Time `json:"x-snapsentry-snapshot-expiry-date"`

	// PolicyType is stored for reference/debugging (e.g., "daily", "weekly").
	// It is not strictly used by the expiry logic, which relies solely on ExpiryDate.
	PolicyType string `json:"x-snapsentry-snapshot-policy-type"`

	// RetentionDays is stored for reference/debugging to show how long the policy was configured for.
	RetentionDays int `json:"x-snapsentry-snapshot-retention-days"`
}

// ToOpenstackMetadata serializes the snapshot metadata into a string map
// suitable for the OpenStack/Cinder API.
// It handles the conversion of Time objects to RFC3339 strings.
func (s SnapshotMetadata) ToOpenstackMetadata() map[string]string {
	// Safely format the ExpiryDate
	var expiryDateStr string
	var expiryDateStrTZ string

	if !s.ExpiryDate.IsZero() {
		expiryDateStr = s.ExpiryDate.UTC().Format(time.RFC3339)
		expiryDateStrTZ = s.ExpiryDate.Format(time.RFC3339)
	}

	return map[string]string{
		"x-snapsentry-snapshot-managed":             strconv.FormatBool(s.Managed),
		"x-snapsentry-snapshot-expiry-date":         expiryDateStr,
		"x-snapsentry-snapshot-expiry-date-user-tz": expiryDateStrTZ,
		"x-snapsentry-snapshot-policy-type":         s.PolicyType,
		"x-snapsentry-snapshot-retention-days":      strconv.Itoa(s.RetentionDays),
	}
}
