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
// of the spec table is covered — Verified/Revoked/NoBadge cases plus the
// "Revoked wins over everything" precedence rule.
func TestUserFacingBadge(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		tier    TrustTier
		revoked bool
		want    TrustBadge
	}{
		{"unknown no revocation", TrustTierUnknown, false, TrustBadgeNone},
		{"unsigned no revocation", TrustTierUnsigned, false, TrustBadgeNone},
		{"signed no revocation", TrustTierSigned, false, TrustBadgeVerified},
		{"dual-attested no revocation", TrustTierDualAttested, false, TrustBadgeVerified},

		// Revoked overrides every tier — this is the invariant that makes
		// the three-state display safe.
		{"unknown revoked", TrustTierUnknown, true, TrustBadgeRevoked},
		{"unsigned revoked", TrustTierUnsigned, true, TrustBadgeRevoked},
		{"signed revoked", TrustTierSigned, true, TrustBadgeRevoked},
		{"dual-attested revoked", TrustTierDualAttested, true, TrustBadgeRevoked},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := UserFacingBadge(tc.tier, tc.revoked); got != tc.want {
				t.Errorf("UserFacingBadge(%v, %v) = %v; want %v",
					tc.tier, tc.revoked, got, tc.want)
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
		{TrustBadgeRevoked, "Revoked", "R"},
	} {
		if got := tc.badge.Label(); got != tc.wantLabel {
			t.Errorf("Badge(%d).Label() = %q; want %q", tc.badge, got, tc.wantLabel)
		}
		if got := tc.badge.Glyph(); got != tc.wantGlyph {
			t.Errorf("Badge(%d).Glyph() = %q; want %q", tc.badge, got, tc.wantGlyph)
		}
	}
}

// TestTrustDescription covers the drill-down phrasing. Revoked content
// preserves the reason verbatim because registries publish that string
// verbatim per MOAT spec; unknown reasons are allowed. Empty output for
// Unknown lets callers skip the field entirely.
func TestTrustDescription(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		tier    TrustTier
		revoked bool
		reason  string
		want    string
	}{
		{"dual-attested", TrustTierDualAttested, false, "", "Verified (dual-attested by publisher and registry)"},
		{"signed", TrustTierSigned, false, "", "Verified (registry-attested)"},
		{"unsigned", TrustTierUnsigned, false, "", "Unsigned (registry declares no attestation)"},
		{"unknown", TrustTierUnknown, false, "", ""},
		{"revoked with reason", TrustTierSigned, true, "compromised", "Revoked — compromised"},
		{"revoked without reason", TrustTierSigned, true, "", "Revoked"},

		// Even a Dual-Attested item becomes "Revoked" on drill-down; the
		// user must see the reason, not the old tier.
		{"dual-attested then revoked", TrustTierDualAttested, true, "malicious", "Revoked — malicious"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := TrustDescription(tc.tier, tc.revoked, tc.reason); got != tc.want {
				t.Errorf("TrustDescription(%v, %v, %q) = %q; want %q",
					tc.tier, tc.revoked, tc.reason, got, tc.want)
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
	if ci.Revoked {
		t.Error("zero-value Revoked must be false")
	}
	if UserFacingBadge(ci.TrustTier, ci.Revoked) != TrustBadgeNone {
		t.Error("zero-value ContentItem must produce TrustBadgeNone")
	}
	if TrustDescription(ci.TrustTier, ci.Revoked, ci.RevocationReason) != "" {
		t.Error("zero-value ContentItem must produce empty trust description")
	}

	// The drill-down fields added in MOAT Phase 2c must also stay at their
	// Go zero values for a non-MOAT item. The AD-7 collapse rule plus the
	// enrich-boundary contract together require that consumers never see
	// stale drill-down data on items the producer never enriched.
	if ci.PrivateRepo {
		t.Error("zero-value PrivateRepo must be false")
	}
	if ci.RevocationSource != "" {
		t.Errorf("zero-value RevocationSource = %q; want empty", ci.RevocationSource)
	}
	if ci.RevocationDetailsURL != "" {
		t.Errorf("zero-value RevocationDetailsURL = %q; want empty", ci.RevocationDetailsURL)
	}
	if ci.Revoker != "" {
		t.Errorf("zero-value Revoker = %q; want empty", ci.Revoker)
	}
}
