package policy

import (
	"testing"
	"time"
)

func TestSnapshotPolicyDaily_Normalize(t *testing.T) {
	tests := []struct {
		name          string
		input         SnapshotPolicyDaily
		wantErr       bool
		wantRetention int
		wantHour      int
		wantMinute    int
	}{
		{
			name: "Happy Path",
			input: SnapshotPolicyDaily{
				Enabled:       true,
				RetentionDays: 5,
				TimeZone:      "UTC",
				StartTime:     "14:30",
			},
			wantErr:       false,
			wantRetention: 5,
			wantHour:      14,
			wantMinute:    30,
		},
		{
			name: "Default Values (Negative Retention)",
			input: SnapshotPolicyDaily{
				Enabled:       true,
				RetentionDays: -10, // Should become 2
				TimeZone:      "",  // Should become UTC
				StartTime:     "",  // Should become 00:00
			},
			wantErr:       false,
			wantRetention: 2,
			wantHour:      0,
			wantMinute:    0,
		},
		{
			name: "Invalid Time Format",
			input: SnapshotPolicyDaily{
				StartTime: "25:00", // Invalid hour
			},
			wantErr: true,
		},
		{
			name: "Invalid Timezone",
			input: SnapshotPolicyDaily{
				TimeZone: "Mars/Phobos",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy so we don't modify the test case input
			policy := tt.input
			err := policy.Normalize()

			// Check Error
			if (err != nil) != tt.wantErr {
				t.Errorf("Normalize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// If we expected success, check the normalized fields
			if !tt.wantErr {
				if policy.RetentionDays != tt.wantRetention {
					t.Errorf("RetentionDays = %d, want %d", policy.RetentionDays, tt.wantRetention)
				}
				// Accessing private fields (allowed because we are in package policy)
				if policy.startHour != tt.wantHour {
					t.Errorf("startHour = %d, want %d", policy.startHour, tt.wantHour)
				}
				if policy.startMinute != tt.wantMinute {
					t.Errorf("startMinute = %d, want %d", policy.startMinute, tt.wantMinute)
				}
			}
		})
	}
}

func TestSnapshotPolicyDaily_Evaluate(t *testing.T) {
	// Setup a fixed timezone for testing (Paris = UTC+1 in Winter)
	loc, _ := time.LoadLocation("Europe/Paris")

	// Define a "Today" for our test: Dec 21, 2025 (Winter)
	// 14:00 Paris = 13:00 UTC
	policy := SnapshotPolicyDaily{
		Enabled:       true,
		RetentionDays: 7,
		TimeZone:      "Europe/Paris",
		StartTime:     "14:00",
	}
	_ = policy.Normalize() // Prepare the policy

	tests := []struct {
		name           string
		now            time.Time        // The "Current" time
		lastSnap       LastSnapshotInfo // The state of the world
		wantSnapshot   bool             // Do we expect a snapshot?
		wantReasonPart string           // Part of the reason string to match
	}{
		{
			name: "Too Early (10:00 Paris)",
			now:  time.Date(2025, 12, 21, 10, 0, 0, 0, loc),
			lastSnap: LastSnapshotInfo{
				// Last snapshot was yesterday, correctly.
				CreatedAt: time.Date(2025, 12, 20, 14, 5, 0, 0, loc),
			},
			wantSnapshot:   false,
			wantReasonPart: "before daily start time",
		},
		{
			name:           "Window Open & No Snapshot (15:00 Paris)",
			now:            time.Date(2025, 12, 21, 15, 0, 0, 0, loc),
			lastSnap:       LastSnapshotInfo{}, // Brand new volume
			wantSnapshot:   true,
			wantReasonPart: "no existing snapshot",
		},
		{
			name: "Idempotency: Already Done Today (15:00 Paris)",
			now:  time.Date(2025, 12, 21, 15, 0, 0, 0, loc),
			lastSnap: LastSnapshotInfo{
				// Snapshot taken at 14:05 Paris today
				CreatedAt: time.Date(2025, 12, 21, 14, 5, 0, 0, loc),
				ID:        "snap-123",
			},
			wantSnapshot:   false,
			wantReasonPart: "already exists",
		},
		{
			name: "Recovery Mode: Early Today, But Missed Yesterday",
			// It is 10:00 AM (Early for today's 14:00 slot)
			now: time.Date(2025, 12, 21, 10, 0, 0, 0, loc),
			lastSnap: LastSnapshotInfo{
				// Last snapshot was 2 days ago! (Dec 19)
				CreatedAt: time.Date(2025, 12, 19, 14, 0, 0, 0, loc),
			},
			wantSnapshot:   true, // Should trigger "Catch Up"
			wantReasonPart: "Recovery Mode",
		},
		{
			name: "Strict Window Closed (Next Day 14:01 is technically new window, but let's test strictness)",
			// If we configured strict windows, this would fail.
			// But our current logic is "Start + 24h".
			// So 14:00 + 24h = Next Day 14:00.
			// Let's test "Just Inside" the window.
			now:            time.Date(2025, 12, 21, 14, 0, 0, 0, loc), // Exact start time
			lastSnap:       LastSnapshotInfo{},
			wantSnapshot:   true,
			wantReasonPart: "Window is active",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := policy.Evaluate(tt.now, tt.lastSnap)

			// 1. Check Technical Errors
			if err != nil {
				t.Fatalf("Evaluate() unexpected error: %v", err)
			}

			// 2. Check Decision
			if result.ShouldSnapshot != tt.wantSnapshot {
				t.Errorf("ShouldSnapshot = %v, want %v. Reason: %s", result.ShouldSnapshot, tt.wantSnapshot, result.Reason)
			}
		})
	}
}
