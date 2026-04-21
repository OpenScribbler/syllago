package catalog

import "testing"

// TestTrustTier_String asserts the tier → drill-down-name map. The normative
// lockfile labels are UPPER-HYPHEN-CASE (moat package); this drill-down
// rendering uses Title-Case because it lands in user-facing UI, not in
// serialized JSON. Unknown deliberately renders as empty so callers can
// suppress the line.
func TestTrustTier_String(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		tier TrustTier
		want string
	}{
		{TrustTierUnknown, ""},
		{TrustTierUnsigned, "Unsigned"},
		{TrustTierSigned, "Signed"},
		{TrustTierDualAttested, "Dual-Attested"},
	} {
		if got := tc.tier.String(); got != tc.want {
			t.Errorf("TrustTier(%d).String() = %q; want %q", tc.tier, got, tc.want)
		}
	}
}

// TestUserFacingBadge applies the AD-7 Panel C9 collapse table. Every row
// of the spec table is covered — Verified/Recalled/NoBadge cases plus the
// "Recalled wins over everything" precedence rule.
func TestUserFacingBadge(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		tier     TrustTier
		recalled bool
		want     TrustBadge
	}{
		{"unknown no recall", TrustTierUnknown, false, TrustBadgeNone},
		{"unsigned no recall", TrustTierUnsigned, false, TrustBadgeNone},
		{"signed no recall", TrustTierSigned, false, TrustBadgeVerified},
		{"dual-attested no recall", TrustTierDualAttested, false, TrustBadgeVerified},

		// Recalled overrides every tier — this is the invariant that makes
		// the three-state display safe.
		{"unknown recalled", TrustTierUnknown, true, TrustBadgeRecalled},
		{"unsigned recalled", TrustTierUnsigned, true, TrustBadgeRecalled},
		{"signed recalled", TrustTierSigned, true, TrustBadgeRecalled},
		{"dual-attested recalled", TrustTierDualAttested, true, TrustBadgeRecalled},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := UserFacingBadge(tc.tier, tc.recalled); got != tc.want {
				t.Errorf("UserFacingBadge(%v, %v) = %v; want %v",
					tc.tier, tc.recalled, got, tc.want)
			}
		})
	}
}

// TestTrustBadge_LabelAndGlyph covers the two presentation helpers. The
// None case must return empty strings so callers can branch on them
// without a special enum check.
func TestTrustBadge_LabelAndGlyph(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		badge     TrustBadge
		wantLabel string
		wantGlyph string
	}{
		{TrustBadgeNone, "", ""},
		{TrustBadgeVerified, "Verified", "\u2713"},
		{TrustBadgeRecalled, "Recalled", "R"},
	} {
		if got := tc.badge.Label(); got != tc.wantLabel {
			t.Errorf("Badge(%d).Label() = %q; want %q", tc.badge, got, tc.wantLabel)
		}
		if got := tc.badge.Glyph(); got != tc.wantGlyph {
			t.Errorf("Badge(%d).Glyph() = %q; want %q", tc.badge, got, tc.wantGlyph)
		}
	}
}

// TestTrustDescription covers the drill-down phrasing. Recalled content
// preserves the reason verbatim because registries publish that string
// verbatim per MOAT spec; unknown reasons are allowed. Empty output for
// Unknown lets callers skip the field entirely.
func TestTrustDescription(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		tier     TrustTier
		recalled bool
		reason   string
		want     string
	}{
		{"dual-attested", TrustTierDualAttested, false, "", "Verified (dual-attested by publisher and registry)"},
		{"signed", TrustTierSigned, false, "", "Verified (registry-attested)"},
		{"unsigned", TrustTierUnsigned, false, "", "Unsigned (registry declares no attestation)"},
		{"unknown", TrustTierUnknown, false, "", ""},
		{"recalled with reason", TrustTierSigned, true, "compromised", "Recalled — compromised"},
		{"recalled without reason", TrustTierSigned, true, "", "Recalled"},

		// Even a Dual-Attested item becomes "Recalled" on drill-down; the
		// user must see the reason, not the old tier.
		{"dual-attested then recalled", TrustTierDualAttested, true, "malicious", "Recalled — malicious"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := TrustDescription(tc.tier, tc.recalled, tc.reason); got != tc.want {
				t.Errorf("TrustDescription(%v, %v, %q) = %q; want %q",
					tc.tier, tc.recalled, tc.reason, got, tc.want)
			}
		})
	}
}

// TestContentItem_TrustFields confirms the zero-value invariants: a
// ContentItem constructed without MOAT data reports no trust badge and no
// drill-down line. This is the guarantee git-registry items rely on.
func TestContentItem_TrustFields(t *testing.T) {
	t.Parallel()
	ci := ContentItem{Name: "skill-x", Type: "skills"}
	// Touch Name/Type so the zero-value guarantee isn't just "construct
	// an empty struct" — a real item with realistic non-trust fields set
	// must still produce no trust surface.
	if ci.Name == "" || ci.Type == "" {
		t.Fatal("test setup did not populate Name/Type")
	}
	if ci.TrustTier != TrustTierUnknown {
		t.Errorf("zero-value TrustTier = %v; want TrustTierUnknown", ci.TrustTier)
	}
	if ci.Recalled {
		t.Error("zero-value Recalled must be false")
	}
	if UserFacingBadge(ci.TrustTier, ci.Recalled) != TrustBadgeNone {
		t.Error("zero-value ContentItem must produce TrustBadgeNone")
	}
	if TrustDescription(ci.TrustTier, ci.Recalled, ci.RecallReason) != "" {
		t.Error("zero-value ContentItem must produce empty trust description")
	}

	// The drill-down fields added in MOAT Phase 2c must also stay at their
	// Go zero values for a non-MOAT item. The AD-7 collapse rule plus the
	// enrich-boundary contract together require that consumers never see
	// stale drill-down data on items the producer never enriched.
	if ci.PrivateRepo {
		t.Error("zero-value PrivateRepo must be false")
	}
	if ci.RecallSource != "" {
		t.Errorf("zero-value RecallSource = %q; want empty", ci.RecallSource)
	}
	if ci.RecallDetailsURL != "" {
		t.Errorf("zero-value RecallDetailsURL = %q; want empty", ci.RecallDetailsURL)
	}
	if ci.RecallIssuer != "" {
		t.Errorf("zero-value RecallIssuer = %q; want empty", ci.RecallIssuer)
	}
}
