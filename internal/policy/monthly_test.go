package policy

import (
	"testing"
	"time"
)

func TestSnapshotPolicyMonthly_Normalize(t *testing.T) {
	tests := []struct {
		name          string
		input         SnapshotPolicyMonthly
		wantErr       bool
		wantRetention int
		wantDay       int
		wantTime      string
	}{
		{
			name: "Happy Path (15th)",
			input: SnapshotPolicyMonthly{
				Enabled:       true,
				RetentionDays: 90,
				TimeZone:      "UTC",
				StartTime:     "14:00",
				DayOfMonth:    15,
			},
			wantErr:       false,
			wantRetention: 90,
			wantDay:       15,
			wantTime:      "14:00",
		},
		{
			name: "Clamp Day High (32 -> 31)",
			input: SnapshotPolicyMonthly{
				Enabled:    true,
				DayOfMonth: 32, // Invalid
			},
			wantErr: false,
			wantDay: 31, // Should clamp
		},
		{
			name: "Clamp Day Low (0 -> 1)",
			input: SnapshotPolicyMonthly{
				Enabled:    true,
				DayOfMonth: 0, // Invalid
			},
			wantErr: false,
			wantDay: 1, // Should clamp
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := tt.input
			err := policy.Normalize()

			if (err != nil) != tt.wantErr {
				t.Errorf("Normalize() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if policy.DayOfMonth != tt.wantDay {
					t.Errorf("DayOfMonth = %d, want %d", policy.DayOfMonth, tt.wantDay)
				}
			}
		})
	}
}

func TestSnapshotPolicyMonthly_Evaluate(t *testing.T) {
	// Setup: Fixed Timezone
	loc, _ := time.LoadLocation("Europe/Paris")

	// Helper: Year, Month, Day, Hour
	mkDate := func(y int, m time.Month, d int, h int) time.Time {
		return time.Date(y, m, d, h, 0, 0, 0, loc)
	}

	// Policy: Run on the 31st at 14:00
	// This is the hardest case to test because of Feb/April/June.
	policy := SnapshotPolicyMonthly{
		Enabled:       true,
		RetentionDays: 90,
		TimeZone:      "Europe/Paris",
		StartTime:     "14:00",
		DayOfMonth:    31, // Target the END of the month
	}
	_ = policy.Normalize()

	tests := []struct {
		name           string
		now            time.Time
		lastSnap       LastSnapshotInfo
		wantSnapshot   bool
		wantReasonPart string
	}{
		// --- SCENARIO 1: STANDARD MONTH (January) ---
		// Jan has 31 days. Target is Jan 31.
		{
			name: "Standard Match: Jan 31st 15:00",
			now:  mkDate(2025, time.January, 31, 15),
			lastSnap: LastSnapshotInfo{
				CreatedAt: mkDate(2024, time.December, 31, 14), // Last month
			},
			wantSnapshot:   true,
			wantReasonPart: "no existing snapshot",
		},

		// --- SCENARIO 2: THE "FEBRUARY 28" CLAMP ---
		// Policy is 31st. But it is Feb 2025 (Non-Leap).
		// System should realize "31st doesn't exist", clamp to 28th.
		// If Now is Feb 28th 15:00, we should run.
		{
			name: "Feb Clamping: Feb 28th (Target was 31st)",
			now:  mkDate(2025, time.February, 28, 15),
			lastSnap: LastSnapshotInfo{
				CreatedAt: mkDate(2025, time.January, 31, 14),
			},
			wantSnapshot:   true, // Should run on the 28th!
			wantReasonPart: "no existing snapshot",
		},

		// --- SCENARIO 3: LEAP YEAR (Feb 29) ---
		// Year 2024 is Leap. Policy 31st.
		// Should clamp to 29th, NOT 28th.
		{
			name: "Leap Year: Feb 29th 2024",
			now:  mkDate(2024, time.February, 29, 15),
			lastSnap: LastSnapshotInfo{
				CreatedAt: mkDate(2024, time.January, 31, 14),
			},
			wantSnapshot:   true, // Should run on 29th
			wantReasonPart: "no existing snapshot",
		},

		// --- SCENARIO 4: RECOVERY / CATCH UP ---
		// Policy 31st.
		// It is April 2nd. (April has 30 days).
		// We missed the March 31st window? No wait.
		// Window: [March 31, April 30).
		// Now (April 2) is inside that window.
		{
			name: "Catch Up: April 2nd (Missed March 31st)",
			now:  mkDate(2025, time.April, 2, 10),
			lastSnap: LastSnapshotInfo{
				CreatedAt: mkDate(2025, time.February, 28, 14), // Last snap was Feb
			},
			wantSnapshot:   true,
			wantReasonPart: "no existing snapshot",
		},

		// --- SCENARIO 5: IDEMPOTENCY ---
		// We already ran today.
		{
			name: "Idempotency: Jan 31st (Already done)",
			now:  mkDate(2025, time.January, 31, 16),
			lastSnap: LastSnapshotInfo{
				CreatedAt: mkDate(2025, time.January, 31, 14),
				ID:        "snap-jan",
			},
			wantSnapshot:   false,
			wantReasonPart: "already exists",
		},

		// --- SCENARIO 6: TOO EARLY ---
		// Policy 31st. Today is Jan 10th.
		// Window shifts back to Dec 31.
		// Dec 31 snap exists. Do nothing.
		{
			name: "Too Early: Jan 10th",
			now:  mkDate(2025, time.January, 10, 10),
			lastSnap: LastSnapshotInfo{
				CreatedAt: mkDate(2024, time.December, 31, 14),
			},
			wantSnapshot:   false,            // Waiting for Jan 31
			wantReasonPart: "already exists", // Found Dec snap in shifted window
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
					result.Window.StartTime.Format("2006-01-02"),
					result.Window.EndTime.Format("2006-01-02"),
				)
			}
		})
	}
}
