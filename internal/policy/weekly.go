package policy

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// SnapshotPolicyWeekly implements the SnapshotPolicy interface for weekly snapshot schedules.
// It triggers a snapshot on a specific day of the week (e.g., "Monday") at a specific time.
//
// Behavior:
//   - Window: The valid window for a snapshot is 7 days (168 hours), starting from the configured Day/Time.
//   - Idempotency: Checks if a snapshot exists within the current weekly cycle.
//   - Date Alignment: "Today" is dynamically shifted to align with the target weekday to determine the window start.
//
// Fields:
//   - Enabled: Master switch.
//   - RetentionDays: How long to keep the snapshot. Defaults to 7 days.
//   - TimeZone: IANA timezone (e.g., "Asia/Kolkata"). Defaults to UTC.
//   - StartTime: Trigger time in "HH:MM".
//   - DayOfWeek: Target day (e.g., "Monday", "sun", "1").
//
// Internal Fields:
//   - Loc: Parsed time.Location.
//   - startHour: Parsed hour (0-23).
//   - startMinute: Parsed minute (0-59).
//   - startDayWeekday: Parsed time.Weekday (0=Sunday, 6=Saturday).
type SnapshotPolicyWeekly struct {
	Enabled       bool   `json:"x-snapsentry-weekly-enabled"`
	RetentionDays int    `json:"x-snapsentry-weekly-retention-days"`
	RetentionType string `json:"x-snapsentry-weekly-retention-type"`
	TimeZone      string `json:"x-snapsentry-weekly-timezone"`
	StartTime     string `json:"x-snapsentry-weekly-start-time"`
	DayOfWeek     string `json:"x-snapsentry-weekly-start-day-of-week"`

	// Internal fields for calculation
	Loc             *time.Location
	startHour       int
	startMinute     int
	startDayWeekday time.Weekday
}

// IsEnabled checks if the weekly policy is active.
// Returns false if the policy is explicitly disabled in the configuration/metadata.
func (s *SnapshotPolicyWeekly) IsEnabled() bool {
	return s.Enabled
}

// GetPolicyType returns the unique identifier "weekly".
// This is used for logging and metadata tagging.
func (s *SnapshotPolicyWeekly) GetPolicyType() string {
	return "weekly"
}

// GetPolicyRetention returns the configured retention period in days.
func (s *SnapshotPolicyWeekly) GetPolicyRetention() int {
	return s.RetentionDays
}

// ParseFromMetadata hydrates the policy struct from a map of OpenStack metadata.
// It uses the generic ParseSnapSentryMetadataFromSDK helper to handle type coercion
// (string to bool/int) and struct tag mapping.
func (s *SnapshotPolicyWeekly) ParseFromMetadata(metadata map[string]string) error {
	parsed, err := ParseSnapSentryMetadataFromSDK[SnapshotPolicyWeekly](metadata)
	if err != nil {
		return err
	}
	*s = *parsed
	return nil
}

// ToOpenstackMetadata serializes the policy configuration into OpenStack Volume metadata tags.
// This allows the policy state to be persisted directly on the storage volume.
func (s *SnapshotPolicyWeekly) ToOpenstackMetadata() map[string]string {
	return map[string]string{
		ManagedTag:                              "true",
		"x-snapsentry-weekly-enabled":           strconv.FormatBool(s.Enabled),
		"x-snapsentry-weekly-retention-days":    strconv.Itoa(s.RetentionDays),
		"x-snapsentry-weekly-retention-type":    s.RetentionType,
		"x-snapsentry-weekly-timezone":          s.TimeZone,
		"x-snapsentry-weekly-start-time":        s.StartTime,
		"x-snapsentry-weekly-start-day-of-week": s.DayOfWeek,
	}
}

// Normalize validates inputs and sets defaults.
//  1. TimeZone -> time.Location (Def: UTC)
//  2. Retention -> int (Def: 7)
//  3. StartTime -> HH:MM
//  4. DayOfWeek -> time.Weekday (Def: Sunday)
func (s *SnapshotPolicyWeekly) Normalize() error {
	// 1. Normalize Timezone
	timezone, loc, err := helperNormalizeTimezone(s.TimeZone)
	if err != nil {
		return err
	}
	s.Loc = loc
	s.TimeZone = timezone

	// 2. Normalize Retention Days (Default to 7 days / 1 week)
	s.RetentionDays = helperNormalizeRetentionDays(s.RetentionDays, 7)

	// 3. Normalize Start Time
	starttime, err := helperNormalizeStartTime(s.StartTime)
	if err != nil {
		return err
	}
	s.startHour = starttime.Hour()
	s.startMinute = starttime.Minute()
	s.StartTime = fmt.Sprintf("%02d:%02d", s.startHour, s.startMinute)

	// 4. Normalize Day of Week
	weekday, err := helperNormalizeDay(s.DayOfWeek)
	if err != nil {
		return err
	}
	s.startDayWeekday = weekday
	s.DayOfWeek = strings.ToLower(weekday.String())

	return nil
}

// Evaluate determines if a snapshot is required.
// Logic:
//  1. Localizes 'now'.
//  2. Calculates the 'potential start' by shifting 'now' to the target weekday.
//     (e.g., if Now=Tue and Target=Mon, potential start is Yesterday).
//  3. Passes this calculated start time to helperEvaluateWindow with a 7-day duration.
func (s *SnapshotPolicyWeekly) Evaluate(now time.Time, lastSnapshot LastSnapshotInfo) (PolicyEvalResult, error) {

	// Initialize a result struct with sane defaults
	result := PolicyEvalResult{
		ShouldSnapshot: false,
		Metadata:       SnapshotMetadata{},
		Window:         SnapshotPolicyWindow{},
	}

	if !s.Enabled {
		result.Reason = "Weekly Snapshot Policy is disabled"
		return result, nil
	}

	// 1. Localize current time
	referenceTime := now.In(s.Loc)

	// 2. Calculate "This Week's" Target Date (Alignment Logic)
	// We determine how far the target day is from the current day.
	currentWeekday := referenceTime.Weekday()
	targetWeekday := s.startDayWeekday

	// Shift date to align with the target weekday.
	// Example: Today is Tue (2). Target is Mon (1). Shift = 1 - 2 = -1 (Yesterday).
	// Example: Today is Sun (0). Target is Mon (1). Shift = 1 - 0 = +1 (Tomorrow).
	daysToShift := int(targetWeekday) - int(currentWeekday)
	alignedDate := referenceTime.AddDate(0, 0, daysToShift)

	// Construct the potential start time based on the aligned date
	potentialStart := time.Date(
		alignedDate.Year(), alignedDate.Month(), alignedDate.Day(),
		s.startHour, s.startMinute, 0, 0, s.Loc,
	)

	// 3. Localize last snapshot
	localizedSnap := lastSnapshot
	if !lastSnapshot.CreatedAt.IsZero() {
		localizedSnap.CreatedAt = lastSnapshot.CreatedAt.In(s.Loc)
	}

	// 4. Delegate to Helper
	// We pass the potential start. The helper will automatically handle the case
	// where potentialStart is in the future (shift back 7 days) vs past.
	result = helperEvaluateWindow(referenceTime, potentialStart, 7*24*time.Hour, localizedSnap)

	if !result.ShouldSnapshot {
		return result, nil
	}

	// 5. Success
	result.Metadata = SnapshotMetadata{
		Managed:       true,
		ExpiryDate:    result.Window.StartTime.AddDate(0, 0, s.RetentionDays),
		PolicyType:    "weekly",
		RetentionDays: s.RetentionDays,
	}

	return result, nil
}
