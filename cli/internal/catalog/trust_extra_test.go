package catalog

import (
	"strings"
	"testing"
)

// TestTrustTier_ShortLabel covers the metadata-panel chip phrasing. Like
// the drill-down String(), Unknown collapses to empty so the panel suppresses
// the row entirely.
func TestTrustTier_ShortLabel(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		tier TrustTier
		want string
	}{
		{TrustTierUnknown, ""},
		{TrustTierUnsigned, "Unsigned"},
		{TrustTierSigned, "Signed"},
		{TrustTierDualAttested, "Dual attested"},
	} {
		if got := tc.tier.ShortLabel(); got != tc.want {
			t.Errorf("ShortLabel(%v) = %q, want %q", tc.tier, got, tc.want)
		}
	}
}

// TestTrustDetailExplanation covers the long-form Trust Inspector prose.
// Revoked branches take precedence over the tier branch, including the
// previously-attested wording when the tier was Signed/Dual-Attested.
func TestTrustDetailExplanation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		tier        TrustTier
		revoked     bool
		mustContain string // partial match keeps the test resilient to copy edits
	}{
		{"unknown not revoked", TrustTierUnknown, false, ""},
		{"unsigned not revoked", TrustTierUnsigned, false, "Unsigned means"},
		{"signed not revoked", TrustTierSigned, false, "Signed means"},
		{"dual-attested not revoked", TrustTierDualAttested, false, "Dual-attested means"},

		// Revoked branches: previously-attested vs. plain.
		{"signed revoked", TrustTierSigned, true, "previously attested"},
		{"dual-attested revoked", TrustTierDualAttested, true, "previously attested"},
		{"unsigned revoked", TrustTierUnsigned, true, "revoked by the registry"},
		{"unknown revoked", TrustTierUnknown, true, "revoked by the registry"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := TrustDetailExplanation(tc.tier, tc.revoked)
			if tc.mustContain == "" {
				if got != "" {
					t.Errorf("got %q, want empty", got)
				}
				return
			}
			if !strings.Contains(got, tc.mustContain) {
				t.Errorf("got %q, want substring %q", got, tc.mustContain)
			}
		})
	}
}
