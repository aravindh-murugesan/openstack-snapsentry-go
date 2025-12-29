package policy

import (
	"fmt"
	"strconv"
	"time"
)

// SnapshotPolicyMonthly implements the SnapshotPolicy interface for monthly snapshot schedules.
// It triggers a snapshot on a specific numeric day of the month (e.g., the 1st, 15th, or 31st).
//
// Behavior:
//   - Date Clamping: Handles months with fewer days than the target.
//     Example: If configured for the 31st, it triggers on Feb 28th (or 29th) and April 30th.
//   - Variable Windows: Unlike Daily (24h) or Weekly (168h), the window duration varies
//     (28, 29, 30, or 31 days) depending on the specific month.
//   - Idempotency: Ensures only one snapshot is taken per calendar month cycle.
type SnapshotPolicyMonthly struct {
	Enabled       bool   `json:"x-snapsentry-monthly-enabled"`
	RetentionDays int    `json:"x-snapsentry-monthly-retention-days"`
	RetentionType string `json:"x-snapsentry-monthly-retention-type"`
	TimeZone      string `json:"x-snapsentry-monthly-timezone"`
	StartTime     string `json:"x-snapsentry-monthly-start-time"`
	DayOfMonth    int    `json:"x-snapsentry-monthly-start-day-of-month"`

	// Internal fields for calculation
	Loc         *time.Location
	startHour   int
	startMinute int
}

func (s *SnapshotPolicyMonthly) IsEnabled() bool {
	return s.Enabled
}

// GetPolicyType returns the string identifier for this policy ("monthly").
func (s *SnapshotPolicyMonthly) GetPolicyType() string {
	return "monthly"
}

// GetPolicyRetention returns the configured retention period in days.
func (s *SnapshotPolicyMonthly) GetPolicyRetention() int {
	return s.RetentionDays
}

// ParseFromMetadata hydrates the policy struct from an OpenStack metadata map.
func (s *SnapshotPolicyMonthly) ParseFromMetadata(metadata map[string]string) error {
	parsed, err := ParseSnapSentryMetadataFromSDK[SnapshotPolicyMonthly](metadata)
	if err != nil {
		return err
	}
	*s = *parsed
	return nil
}

// ToOpenstackMetadata serializes configuration to OpenStack metadata.
// Keys: x-snapsentry-monthly-*
func (s *SnapshotPolicyMonthly) ToOpenstackMetadata() map[string]string {
	return map[string]string{
		ManagedTag:                            "true",
		"x-snapsentry-monthly-enabled":        strconv.FormatBool(s.Enabled),
		"x-snapsentry-monthly-retention-days": strconv.Itoa(s.RetentionDays),
		"x-snapsentry-monthly-retention-type": s.RetentionType,
		"x-snapsentry-monthly-timezone":       s.TimeZone,
		"x-snapsentry-monthly-start-time":     s.StartTime,
		"x-snapsentry-monthly-day-of-month":   strconv.Itoa(s.DayOfMonth),
	}
}

// Normalize validates inputs and sets defaults.
//  1. TimeZone -> time.Location (Def: UTC)
//  2. Retention -> int (Def: 30)
//  3. StartTime -> HH:MM
//  4. DayOfMonth -> Clamped to 1-31 range.
func (s *SnapshotPolicyMonthly) Normalize() error {
	// 1. Normalize Timezone
	timezone, loc, err := helperNormalizeTimezone(s.TimeZone)
	if err != nil {
		return err
	}
	s.Loc = loc
	s.TimeZone = timezone

	// 2. Normalize Retention (Default to 30 days)
	s.RetentionDays = helperNormalizeRetentionDays(s.RetentionDays, 30)

	// 3. Normalize Start Time
	starttime, err := helperNormalizeStartTime(s.StartTime)
	if err != nil {
		return err
	}
	s.startHour = starttime.Hour()
	s.startMinute = starttime.Minute()
	s.StartTime = fmt.Sprintf("%02d:%02d", s.startHour, s.startMinute)

	// 4. Normalize Day of Month (Clamp to 1-31 range)
	if s.DayOfMonth < 1 {
		s.DayOfMonth = 1
	}
	if s.DayOfMonth > 31 {
		s.DayOfMonth = 31
	}

	return nil
}

// Evaluate determines if a snapshot is required.
//
// Logic:
//  1. Localizes 'now'.
//  2. Calculates the specific window boundaries (Start and End) for the current month.
//     - Uses helperGetMonthlyDate to handle "Feb 30th" -> "Feb 28th" logic.
//     - If 'now' is before this month's trigger, it looks back to Last Month's window.
//  3. Calculates the dynamic duration (NextMonth - ThisMonth) to handle variable month lengths.
//  4. Passes these precise boundaries to helperEvaluateWindow.
func (s *SnapshotPolicyMonthly) Evaluate(now time.Time, lastSnapshot LastSnapshotInfo) (PolicyEvalResult, error) {

	// Initialize a result struct with sane defaults
	result := PolicyEvalResult{
		ShouldSnapshot: false,
		Metadata:       SnapshotMetadata{},
		Window:         SnapshotPolicyWindow{},
	}

	if !s.Enabled {
		result.Reason = "Monthly Snapshot Policy is disabled"
		return result, nil
	}

	// 1. Localize current time
	referenceTime := now.In(s.Loc)

	// 2. Calculate "This Month's" Target Date
	// We construct the target date for the current year/month.
	// helperGetMonthlyDate handles the edge case where today is Feb 28th but policy says "31st".
	thisMonthTarget := helperGetMonthlyDate(
		referenceTime.Year(), referenceTime.Month(),
		s.DayOfMonth, s.startHour, s.startMinute, s.Loc,
	)

	// 3. Logic: Determine the Active Window Start
	// Monthly windows are variable, so we cannot simply subtract a fixed duration.
	// We must explicitly calculate the start and end of the relevant window.
	var windowStart time.Time
	var nextWindowStart time.Time

	if referenceTime.Before(thisMonthTarget) {
		// Case A: Too Early (We haven't reached this month's trigger yet).
		// The active window is effectively "Last Month's" window.
		// We calculate the target date for Month - 1.
		windowStart = helperGetMonthlyDate(
			referenceTime.Year(), referenceTime.Month()-1,
			s.DayOfMonth, s.startHour, s.startMinute, s.Loc,
		)
		// The end of the active window is the start of this month's target.
		nextWindowStart = thisMonthTarget

	} else {
		// Case B: On Time / Catch Up (We are past the trigger).
		// The active window starts THIS month.
		windowStart = thisMonthTarget

		// The end of the active window is next month's target.
		nextWindowStart = helperGetMonthlyDate(
			referenceTime.Year(), referenceTime.Month()+1,
			s.DayOfMonth, s.startHour, s.startMinute, s.Loc,
		)
	}

	// 4. Calculate Dynamic Duration
	// The duration is the difference between the two calculated boundaries.
	// This accounts for 28/29/30/31 day variations automatically.
	duration := nextWindowStart.Sub(windowStart)

	// 5. Localize the last snapshot
	localizedSnap := lastSnapshot
	if !lastSnapshot.CreatedAt.IsZero() {
		localizedSnap.CreatedAt = lastSnapshot.CreatedAt.In(s.Loc)
	}

	// 6. Delegate to Helper
	// We pass the exact calculated start and duration.
	// The helper's internal "Too Early" check won't trigger because we already handled it
	// (windowStart is guaranteed <= referenceTime).
	result = helperEvaluateWindow(referenceTime, windowStart, duration, localizedSnap)

	if !result.ShouldSnapshot {
		return result, nil
	}

	// 7. Success
	result.Metadata = SnapshotMetadata{
		Managed:       true,
		ExpiryDate:    result.Window.StartTime.AddDate(0, 0, s.RetentionDays),
		PolicyType:    "monthly",
		RetentionDays: s.RetentionDays,
	}

	return result, nil
}
