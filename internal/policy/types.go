package policy

import "time"

// General Structs used for returns and inputs.

// LastSnapshotInfo servers as a DataTransferObject for (mainly) evaluate method.
type LastSnapshotInfo struct {
	CreatedAt time.Time
	Status    string
	ID        string
	Metadata  map[string]string
}

// SnapshotPolicyWindow contains information about the policy window.
type SnapshotPolicyWindow struct {
	StartTime     time.Time
	EndTime       time.Time
	ValidatedTime time.Time
}

// PolicyEvalResult contains critical information to decide if the snapshot has to be triggered or not.
type PolicyEvalResult struct {
	ShouldSnapshot bool
	Window         SnapshotPolicyWindow
	Metadata       SnapshotMetadata
	Reason         string
}
