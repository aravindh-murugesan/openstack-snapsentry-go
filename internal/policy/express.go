package policy

import (
	"fmt"
	"strconv"
	"time"
)

type SnapshotPolicyExpress struct {
	Enabled       bool   `json:"x-snapsentry-express-enabled"`
	IntervalHours int    `json:"x-snapsentry-express-interval-hours"`
	RetentionDays int    `json:"x-snapsentry-express-retention-days"`
	RetentionType string `json:"x-snapsentry-express-retention-type"`
	TimeZone      string `json:"x-snapsentry-express-timezone"`

	// Internal fields that would be poluplated during normalize
	Loc         *time.Location
	startHour   int
	startMinute int
}

// IsEnabled checks if the interval policy is active.
func (s *SnapshotPolicyExpress) IsEnabled() bool {
	return s.Enabled
}

// GetPolicyType returns the unique identifier "interval".
func (s *SnapshotPolicyExpress) GetPolicyType() string {
	return "express"
}

// GetPolicyRetention returns the configured retention period in days.
func (s *SnapshotPolicyExpress) GetPolicyRetention() int {
	return s.RetentionDays
}

func (s *SnapshotPolicyExpress) Normalize() error {
	// 1. Normalize Timezone
	timezone, loc, err := helperNormalizeTimezone(s.TimeZone)
	if err != nil {
		return err
	}
	s.Loc = loc
	s.TimeZone = timezone

	// 2. Normalize Interval Hours
	if s.IntervalHours <= 0 {
		s.IntervalHours = 6 // Default to 6
	}

	switch s.IntervalHours {
	case 6, 8, 12:
		// Valid
	case 0:
		s.IntervalHours = 6
	default:
		return fmt.Errorf("express interval must be 6, 8, or 12 hours; got %d", s.IntervalHours)
	}

	// 4. Normalize Retention Days (default to 1 day for high-frequency snapshots)
	s.RetentionDays = helperNormalizeRetentionDays(s.RetentionDays, 1)
	s.startHour = 00
	s.startMinute = 00

	return nil
}

// ToOpenstackMetadata serializes the policy configuration into OpenStack Volume metadata tags.
// This allows the policy state to be persisted directly on the storage volume.
func (s *SnapshotPolicyExpress) ToOpenstackMetadata() map[string]string {
	return map[string]string{
		ManagedTag:                            "true",
		"x-snapsentry-express-enabled":        strconv.FormatBool(s.Enabled),
		"x-snapsentry-express-retention-days": strconv.Itoa(s.RetentionDays),
		"x-snapsentry-express-retention-type": s.RetentionType,
		"x-snapsentry-express-timezone":       s.TimeZone,
		"x-snapsentry-express-interval-hours": strconv.Itoa(s.IntervalHours),
	}
}

func (s *SnapshotPolicyExpress) ParseFromMetadata(metadata map[string]string) error {
	parsed, err := ParseSnapSentryMetadataFromSDK[SnapshotPolicyExpress](metadata)
	if err != nil {
		return err
	}
	*s = *parsed
	return nil
}

func (s *SnapshotPolicyExpress) Evaluate(now time.Time, lastSnapshot LastSnapshotInfo) (PolicyEvalResult, error) {

	// Initalize a result struct with sane defaults
	result := PolicyEvalResult{
		ShouldSnapshot: false,
		Metadata:       SnapshotMetadata{},
		Window:         SnapshotPolicyWindow{},
	}

	if !s.Enabled {
		result.Reason = "Express Snapshot policy is disabled"
		return result, nil
	}

	referenceTime := now.In(s.Loc)

	// Calculate the current start time slot
	year, month, day := referenceTime.Date()
	midnight := time.Date(year, month, day, 0, 0, 0, 0, s.Loc)
	currentHour := referenceTime.Hour()

	slotStartHour := (currentHour / s.IntervalHours) * s.IntervalHours
	windowStart := midnight.Add(time.Duration(slotStartHour) * time.Hour)

	// We must ensure lastSnapshot is also localized before passing, or handle it in helper.
	// Let's localize here for safety.
	localizedSnap := lastSnapshot
	if !lastSnapshot.CreatedAt.IsZero() {
		localizedSnap.CreatedAt = lastSnapshot.CreatedAt.In(s.Loc)
	}

	result = helperEvaluateWindow(referenceTime, windowStart, time.Duration(s.IntervalHours)*time.Hour, localizedSnap)

	if !result.ShouldSnapshot {
		return result, nil
	}

	result.Metadata = SnapshotMetadata{
		Managed:       true,
		ExpiryDate:    result.Window.StartTime.AddDate(0, 0, s.RetentionDays),
		PolicyType:    "express",
		RetentionDays: s.RetentionDays,
	}

	return result, nil
}
