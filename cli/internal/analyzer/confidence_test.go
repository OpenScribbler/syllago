package analyzer

import "testing"

func TestTierForItem(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		confidence float64
		provider   string
		label      string
		wantTier   ConfidenceTier
	}{
		{"low", 0.55, "content-signal", "content-signal", TierLow},
		{"medium", 0.65, "content-signal", "content-signal", TierMedium},
		{"high", 0.75, "top-level", "", TierHigh},
		{"user-directed zero-signal", 0.60, "content-signal", "content-signal", TierUser},
		{"high auto-detected", 0.85, "claude-code", "", TierHigh},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := &DetectedItem{
				Confidence:    tc.confidence,
				Provider:      tc.provider,
				InternalLabel: tc.label,
			}
			got := TierForItem(item)
			if got != tc.wantTier {
				t.Errorf("TierForItem(conf=%.2f) = %q, want %q", tc.confidence, got, tc.wantTier)
			}
		})
	}
}
