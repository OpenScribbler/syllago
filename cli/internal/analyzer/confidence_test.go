package analyzer

import "testing"

func TestTierForMeta(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		confidence float64
		method     string
		want       ConfidenceTier
	}{
		{"user-directed regardless of confidence", 0.0, "user-directed", TierUser},
		{"user-directed high confidence", 0.95, "user-directed", TierUser},
		{"low below 0.60", 0.55, "", TierLow},
		{"medium exactly 0.60", 0.60, "", TierMedium},
		{"medium at 0.65", 0.65, "", TierMedium},
		{"high at 0.70", 0.70, "", TierHigh},
		{"high at 0.85", 0.85, "", TierHigh},
		{"zero confidence no method", 0.0, "", TierLow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TierForMeta(tt.confidence, tt.method)
			if got != tt.want {
				t.Errorf("TierForMeta(%v, %q) = %q, want %q", tt.confidence, tt.method, got, tt.want)
			}
		})
	}
}
