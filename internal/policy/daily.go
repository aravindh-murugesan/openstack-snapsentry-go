package policy

import (
	"fmt"
	"strconv"
	"time"
)

// SnapshotPolicyDaily implements the SnapshotPolicy interface for daily snapshot schedules.
// It allows users to define a specific time of day (e.g., "14:00") and a timezone
// to trigger a snapshot once every 24 hours.
//
// Behavior:
//   - Window: The valid window for a snapshot is exactly 24 hours, starting from the configured StartTime.
//   - Idempotency: It checks if a snapshot already exists within the current 24-hour cycle to prevent duplicates.
//   - Expiry: It calculates an expiration date based on the StartTime + RetentionDays.
//
// Fields:
//   - Enabled: Master switch to turn this policy on/off.
//   - RetentionDays: How long (in days) the snapshot should be kept. Defaults to 2 if invalid.
//   - TimeZone: The IANA timezone database name (e.g., "America/New_York"). Defaults to "UTC".
//   - StartTime: The target trigger time in "HH:MM" format.
//
// Internal Fields (populated during Normalize):
//   - Loc: The parsed time.Location object for timezone calculations.
//   - startHour/startMinute: Integers parsed from StartTime for date arithmetic.
type SnapshotPolicyDaily struct {
	Enabled       bool   `json:"x-snapsentry-daily-enabled"`
	RetentionDays int    `json:"x-snapsentry-daily-retention-days"`
	RetentionType string `json:"x-snapsentry-daily-retention-type"`
	TimeZone      string `json:"x-snapsentry-daily-timezone"`
	StartTime     string `json:"x-snapsentry-daily-start-time"`

	Loc         *time.Location
	startHour   int
	startMinute int
}

// IsEnabled checks if the daily policy is active.
// Returns false if the policy is explicitly disabled in the configuration/metadata.
func (s *SnapshotPolicyDaily) IsEnabled() bool {
	return s.Enabled
}

// GetPolicyType returns the unique identifier "daily".
// This is used for logging and metadata tagging.
func (s *SnapshotPolicyDaily) GetPolicyType() string {
	return "daily"
}

// GetPolicyRetention returns the configured retention period in days.
func (s *SnapshotPolicyDaily) GetPolicyRetention() int {
	return s.RetentionDays
}

// Normalize validates and prepares the policy for evaluation.
// It performs the following operations:
//  1. Parses the TimeZone string into a time.Location (defaults to UTC).
//  2. Validates RetentionDays (defaults to 2 if <= 0).
//  3. Parses StartTime string ("HH:MM") into internal hour/minute integers.
//
// Returns an error if the TimeZone or StartTime formats are invalid.
func (s *SnapshotPolicyDaily) Normalize() error {
	// Normalize Timezone
	timezone, loc, err := helperNormalizeTimezone(s.TimeZone)
	if err != nil {
		return err
	}
	s.Loc = loc
	s.TimeZone = timezone

	// Normalize Retention Days
	s.RetentionDays = helperNormalizeRetentionDays(s.RetentionDays, 2)

	// Normalize Start Time
	starttime, err := helperNormalizeStartTime(s.StartTime)
	if err != nil {
		return err
	}
	s.startHour = starttime.Hour()
	s.startMinute = starttime.Minute()
	s.StartTime = fmt.Sprintf("%02d:%02d", s.startHour, s.startMinute)

	return nil
}

// ToOpenstackMetadata serializes the policy configuration into OpenStack Volume metadata tags.
// This allows the policy state to be persisted directly on the storage volume.
func (s *SnapshotPolicyDaily) ToOpenstackMetadata() map[string]string {
	return map[string]string{
		ManagedTag:                          "true",
		"x-snapsentry-daily-enabled":        strconv.FormatBool(s.Enabled),
		"x-snapsentry-daily-retention-days": strconv.Itoa(s.RetentionDays),
		"x-snapsentry-daily-retention-type": s.RetentionType,
		"x-snapsentry-daily-timezone":       s.TimeZone,
		"x-snapsentry-daily-start-time":     s.StartTime,
	}
}

// ParseFromMetadata hydrates the policy struct from a map of OpenStack metadata.
// It uses the generic ParseSnapSentryMetadataFromSDK helper to handle type coercion
// (string to bool/int) and struct tag mapping.
func (s *SnapshotPolicyDaily) ParseFromMetadata(metadata map[string]string) error {
	parsed, err := ParseSnapSentryMetadataFromSDK[SnapshotPolicyDaily](metadata)
	if err != nil {
		return err
	}
	*s = *parsed
	return nil
}

// Evaluate determines if a snapshot should be taken right now based on the daily schedule.
//
// Logic:
//  1. Converts 'now' to the policy's configured TimeZone.
//  2. Calculates the "Potential Start Time" for today (e.g., Today @ 14:00).
//  3. Uses helperEvaluateWindow to check if 'now' is within the 24h window starting from that time.
//     - If 'now' < 'Today @ 14:00', the window is shifted to start 'Yesterday @ 14:00'.
//  4. Checks 'lastSnapshot' to ensure no snapshot was already taken in this specific window.
func (s *SnapshotPolicyDaily) Evaluate(now time.Time, lastSnapshot LastSnapshotInfo) (PolicyEvalResult, error) {

	// Initialize a result struct with sane defaults
	result := PolicyEvalResult{
		ShouldSnapshot: false,
		Metadata:       SnapshotMetadata{},
		Window:         SnapshotPolicyWindow{},
	}

	if !s.Enabled {
		result.Reason = "Daily Snapshot Policy is disabled"
		return result, nil
	}

	// Calucate the Schedule window
	referenceTime := now.In(s.Loc)

	// Potential Start Window
	todayStart := time.Date(
		referenceTime.Year(), referenceTime.Month(), referenceTime.Day(),
		s.startHour, s.startMinute, 0, 0, s.Loc,
	)

	// We must ensure lastSnapshot is also localized before passing, or handle it in helper.
	// Let's localize here for safety.
	localizedSnap := lastSnapshot
	if !lastSnapshot.CreatedAt.IsZero() {
		localizedSnap.CreatedAt = lastSnapshot.CreatedAt.In(s.Loc)
	}

	result = helperEvaluateWindow(referenceTime, todayStart, 24*time.Hour, localizedSnap)

	if !result.ShouldSnapshot {
		return result, nil
	}

	result.Metadata = SnapshotMetadata{
		Managed:       true,
		ExpiryDate:    result.Window.StartTime.AddDate(0, 0, s.RetentionDays),
		PolicyType:    "daily",
		RetentionDays: s.RetentionDays,
	}

	return result, nil
}
