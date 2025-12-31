package policy

import (
	"testing"
	"time"
)

func TestSnapshotPolicyExpress_Normalize(t *testing.T) {
	tests := []struct {
		name         string
		input        SnapshotPolicyExpress
		wantErr      bool
		wantInterval int
		wantRetDays  int
	}{
		{
			name: "Happy Path (6 Hours)",
			input: SnapshotPolicyExpress{
				Enabled:       true,
				IntervalHours: 6,
				RetentionDays: 3,
				TimeZone:      "UTC",
			},
			wantErr:      false,
			wantInterval: 6,
			wantRetDays:  3,
		},
		{
			name: "Happy Path (8 Hours)",
			input: SnapshotPolicyExpress{
				Enabled:       true,
				IntervalHours: 8,
				RetentionDays: 1,
				TimeZone:      "UTC",
			},
			wantErr:      false,
			wantInterval: 8,
			wantRetDays:  1,
		},
		{
			name: "Default Values (Input 0 -> Default 6)",
			input: SnapshotPolicyExpress{
				Enabled:       true,
				IntervalHours: 0,   // Should default to 6
				RetentionDays: -10, // Should default to 1
				TimeZone:      "",  // Should default to UTC
			},
			wantErr:      false,
			wantInterval: 6,
			wantRetDays:  1,
		},
		{
			name: "Invalid Interval (5 Hours)",
			input: SnapshotPolicyExpress{
				Enabled:       true,
				IntervalHours: 5, // Not 6, 8, or 12
			},
			wantErr: true,
		},
		{
			name: "Invalid Interval (24 Hours - Should use Daily)",
			input: SnapshotPolicyExpress{
				Enabled:       true,
				IntervalHours: 24,
			},
			wantErr: true,
		},
		{
			name: "Invalid Timezone",
			input: SnapshotPolicyExpress{
				TimeZone: "Mars/Phobos",
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
				if policy.IntervalHours != tt.wantInterval {
					t.Errorf("IntervalHours = %d, want %d", policy.IntervalHours, tt.wantInterval)
				}
				if policy.RetentionDays != tt.wantRetDays {
					t.Errorf("RetentionDays = %d, want %d", policy.RetentionDays, tt.wantRetDays)
				}
			}
		})
	}
}

func TestSnapshotPolicyExpress_Evaluate(t *testing.T) {
	// Setup a fixed timezone for testing (Paris = UTC+1 in Winter)
	loc, _ := time.LoadLocation("Europe/Paris")

	tests := []struct {
		name           string
		interval       int       // Configured Interval
		now            time.Time // The "Current" time
		lastSnap       LastSnapshotInfo
		wantSnapshot   bool
		wantReasonPart string
	}{
		// --- SCENARIO 1: 6 Hour Interval (Slots: 00-06, 06-12, 12-18, 18-24) ---
		{
			name:     "6h - Fresh Volume (No history)",
			interval: 6,
			// Now is 14:00. Bucket is 12:00 - 18:00.
			now:            time.Date(2025, 12, 21, 14, 0, 0, 0, loc),
			lastSnap:       LastSnapshotInfo{},
			wantSnapshot:   true,
			wantReasonPart: "no existing snapshot",
		},
		{
			name:     "6h - Idempotency (Already done in slot)",
			interval: 6,
			// Now is 14:00. Bucket is 12:00 - 18:00.
			now: time.Date(2025, 12, 21, 14, 0, 0, 0, loc),
			lastSnap: LastSnapshotInfo{
				// Snapshot taken at 12:05 (Same bucket)
				CreatedAt: time.Date(2025, 12, 21, 12, 5, 0, 0, loc),
				ID:        "snap-same-bucket",
			},
			wantSnapshot:   false,
			wantReasonPart: "exists in active window",
		},
		{
			name:     "6h - New Slot Arrived",
			interval: 6,
			// Now is 14:00. Bucket is 12:00 - 18:00.
			now: time.Date(2025, 12, 21, 14, 0, 0, 0, loc),
			lastSnap: LastSnapshotInfo{
				// Snapshot taken at 11:55 (Previous bucket 06-12)
				CreatedAt: time.Date(2025, 12, 21, 11, 55, 0, 0, loc),
				ID:        "snap-prev-bucket",
			},
			wantSnapshot:   true,
			wantReasonPart: "no existing snapshot",
		},
		{
			name:     "6h - Exact Boundary Start (12:00)",
			interval: 6,
			// Now is 12:00. Bucket is 12:00 - 18:00.
			now:            time.Date(2025, 12, 21, 12, 0, 0, 0, loc),
			lastSnap:       LastSnapshotInfo{},
			wantSnapshot:   true,
			wantReasonPart: "Window is active",
		},

		// --- SCENARIO 2: 8 Hour Interval (Slots: 00-08, 08-16, 16-24) ---
		{
			name:     "8h - Shifted Buckets",
			interval: 8,
			// Now is 09:00. Bucket is 08:00 - 16:00.
			now: time.Date(2025, 12, 21, 9, 0, 0, 0, loc),
			lastSnap: LastSnapshotInfo{
				// Snapshot taken at 07:55 (Previous bucket 00-08)
				CreatedAt: time.Date(2025, 12, 21, 7, 55, 0, 0, loc),
			},
			wantSnapshot:   true,
			wantReasonPart: "no existing snapshot",
		},
		{
			name:     "8h - Idempotency",
			interval: 8,
			// Now is 09:00. Bucket is 08:00 - 16:00.
			now: time.Date(2025, 12, 21, 9, 0, 0, 0, loc),
			lastSnap: LastSnapshotInfo{
				// Snapshot taken at 08:05 (Current bucket)
				CreatedAt: time.Date(2025, 12, 21, 8, 5, 0, 0, loc),
			},
			wantSnapshot:   false,
			wantReasonPart: "exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Construct policy on the fly based on test case
			policy := SnapshotPolicyExpress{
				Enabled:       true,
				IntervalHours: tt.interval,
				RetentionDays: 1,
				TimeZone:      "Europe/Paris",
			}
			_ = policy.Normalize()

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
