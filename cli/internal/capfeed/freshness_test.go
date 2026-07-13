package capfeed

import (
	"testing"
	"time"
)

func TestCheckFreshness_Boundaries(t *testing.T) {
	generated := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		now      time.Time
		maxHours int
		wantErr  bool
	}{
		{
			name:     "one minute under the limit",
			now:      generated.Add(48*time.Hour - time.Minute),
			maxHours: 48,
			wantErr:  false,
		},
		{
			name:     "exactly at the limit",
			now:      generated.Add(48 * time.Hour),
			maxHours: 48,
			wantErr:  false,
		},
		{
			name:     "one minute over the limit",
			now:      generated.Add(48*time.Hour + time.Minute),
			maxHours: 48,
			wantErr:  true,
		},
		{
			name:     "generated_at in the future (publisher clock ahead)",
			now:      generated.Add(-2 * time.Hour),
			maxHours: 48,
			wantErr:  false,
		},
		{
			name:     "feed-published tighter limit is honored",
			now:      generated.Add(13 * time.Hour),
			maxHours: 12,
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckFreshness(generated, tt.now, tt.maxHours)
			if tt.wantErr && err == nil {
				t.Fatal("CheckFreshness returned nil; want staleness error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("CheckFreshness: %v; want nil", err)
			}
		})
	}
}
