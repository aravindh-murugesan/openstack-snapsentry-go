package policy

import (
	"testing"
	"time"
)

func TestSnapshotPolicyWeekly_Normalize(t *testing.T) {
	tests := []struct {
		name          string
		input         SnapshotPolicyWeekly
		wantErr       bool
		wantRetention int
		wantWeekday   time.Weekday
		wantStartTime string
	}{
		{
			name: "Happy Path (Monday)",
			input: SnapshotPolicyWeekly{
				Enabled:       true,
				RetentionDays: 14,
				TimeZone:      "UTC",
				StartTime:     "14:00",
				DayOfWeek:     "Monday",
			},
			wantErr:       false,
			wantRetention: 14,
			wantWeekday:   time.Monday,
			wantStartTime: "14:00",
		},
		{
			name: "Case Insensitivity (fri -> Friday)",
			input: SnapshotPolicyWeekly{
				Enabled:       true,
				RetentionDays: 7,
				TimeZone:      "UTC",
				StartTime:     "09:30",
				DayOfWeek:     "fri", // Lowercase short
			},
			wantErr:       false,
			wantRetention: 7,
			wantWeekday:   time.Friday,
			wantStartTime: "09:30",
		},
		{
			name: "Default Values (Retention & Time)",
			input: SnapshotPolicyWeekly{
				Enabled:   true,
				DayOfWeek: "Sun",
				// Missing Retention (defaults to 7 based on your code)
				// Missing StartTime (defaults to 00:00)
			},
			wantErr:       false,
			wantRetention: 7, // Based on your code: helperNormalizeRetentionDays(..., 7)
			wantWeekday:   time.Sunday,
			wantStartTime: "00:00",
		},
		{
			name: "Invalid Day",
			input: SnapshotPolicyWeekly{
				DayOfWeek: "Funday",
			},
			wantErr: true,
		},
		{
			name: "Invalid Time",
			input: SnapshotPolicyWeekly{
				DayOfWeek: "Mon",
				StartTime: "25:00",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := tt.input
			err := policy.Normalize()

			if (err != nil) != tt.wantErr {
				t.Errorf("Normalize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if policy.RetentionDays != tt.wantRetention {
					t.Errorf("RetentionDays = %d, want %d", policy.RetentionDays, tt.wantRetention)
				}
				if policy.startDayWeekday != tt.wantWeekday {
					t.Errorf("startDayWeekday = %v, want %v", policy.startDayWeekday, tt.wantWeekday)
				}
				if policy.StartTime != tt.wantStartTime {
					t.Errorf("StartTime = %s, want %s", policy.StartTime, tt.wantStartTime)
				}
				// Verify the StartDay string was normalized to lowercase (optional, based on your implementation)
				// if policy.StartDay != strings.ToLower(tt.wantWeekday.String()) { ... }
			}
		})
	}
}

func TestSnapshotPolicyWeekly_Evaluate(t *testing.T) {
	// Setup: Fixed Timezone (Paris = UTC+1 in Winter)
	loc, _ := time.LoadLocation("Europe/Paris")

	// Helper to create dates easily (Year 2025, Dec is Month 12)
	// Dec 22, 2025 is a MONDAY.
	mkDate := func(day int, hour int, min int) time.Time {
		return time.Date(2025, 12, day, hour, min, 0, 0, loc)
	}

	// Policy: Run every MONDAY at 14:00
	policy := SnapshotPolicyWeekly{
		Enabled:       true,
		RetentionDays: 4,
		TimeZone:      "Europe/Paris",
		StartTime:     "14:00",
		DayOfWeek:     "Monday",
	}
	_ = policy.Normalize() // Parsing "Monday" -> time.Monday

	tests := []struct {
		name           string
		now            time.Time        // Current time
		lastSnap       LastSnapshotInfo // State of the world
		wantSnapshot   bool             // Expectation
		wantReasonPart string           // Debug text
	}{
		// --- SCENARIO 1: THE HAPPY PATH ---
		{
			name: "Exact Match: Monday 14:05 (Policy Mon 14:00)",
			now:  mkDate(22, 14, 05), // Dec 22 (Mon)
			lastSnap: LastSnapshotInfo{
				CreatedAt: mkDate(15, 14, 00), // Last week's snap (Dec 15)
			},
			wantSnapshot:   true,
			wantReasonPart: "no existing snapshot",
		},

		// --- SCENARIO 2: THE "TUESDAY RECOVERY" ---
		// System was down Monday. It's now Tuesday.
		// Logic should see we are inside [Mon, Next Mon) and snapshot is missing.
		{
			name: "Recovery: Tuesday 10:00 (Missed Mon 14:00)",
			now:  mkDate(23, 10, 00), // Dec 23 (Tue)
			lastSnap: LastSnapshotInfo{
				CreatedAt: mkDate(15, 14, 00), // Last week's snap
			},
			wantSnapshot:   true, // Should catch up!
			wantReasonPart: "no existing snapshot",
		},

		// --- SCENARIO 3: IDEMPOTENCY ---
		// It's Tuesday, but we DID capture the snapshot on Monday.
		// Should do nothing.
		{
			name: "Idempotency: Tuesday 10:00 (Already did Mon 14:05)",
			now:  mkDate(23, 10, 00), // Dec 23 (Tue)
			lastSnap: LastSnapshotInfo{
				CreatedAt: mkDate(22, 14, 05), // This week's snap (Dec 22)
				ID:        "snap-done",
			},
			wantSnapshot:   false,
			wantReasonPart: "already exists",
		},

		// --- SCENARIO 4: THE "SUNDAY LOOKBACK" ---
		// It is Sunday (Dec 28). Policy is Monday.
		// We haven't reached NEXT Monday yet. We are technically in "This Week's" window
		// which started Last Monday (Dec 22).
		// If we haven't snapped since Dec 22, we should do it now.
		{
			name: "Lookback: Sunday 10:00 (Checking window started Last Mon)",
			now:  mkDate(28, 10, 00), // Dec 28 (Sun)
			lastSnap: LastSnapshotInfo{
				CreatedAt: mkDate(15, 14, 00), // Two weeks ago!
			},
			wantSnapshot:   true,
			wantReasonPart: "no existing snapshot",
		},

		// --- SCENARIO 5: FUTURE PROTECTION ---
		// It is Monday Morning (10:00). Policy is Monday 14:00.
		// We are BEFORE the start time.
		// Helper logic: Shifts window back to PREVIOUS Monday.
		// If Previous Monday is done, we wait.
		{
			name: "Too Early: Monday 10:00 (Starts 14:00)",
			now:  mkDate(22, 10, 00), // Dec 22 (Mon) 10:00
			lastSnap: LastSnapshotInfo{
				CreatedAt: mkDate(15, 14, 05), // Last week is done
			},
			// Technically, because of the shift, it checks "Did we do last week?".
			// Since yes -> False.
			// Since we haven't entered TODAY's window (14:00) -> False.
			// Result -> False. Correct.
			wantSnapshot:   false,
			wantReasonPart: "already exists", // It finds LAST week's snap in the shifted window
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := policy.Evaluate(tt.now, tt.lastSnap)

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result.ShouldSnapshot != tt.wantSnapshot {
				t.Errorf("ShouldSnapshot = %v, want %v.\nReason: %s\nWindow: %s -> %s",
					result.ShouldSnapshot, tt.wantSnapshot, result.Reason,
					result.Window.StartTime.Format("Mon 15:04"),
					result.Window.EndTime.Format("Mon 15:04"),
				)
			}
		})
	}
}
