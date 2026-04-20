package catalog

// Trust primitives (ADR 0007 AD-7, G-6).
//
// The catalog package owns the *presentation layer* for MOAT trust tiers.
// The normative tier classification (Dual-Attested / Signed / Unsigned)
// lives in the moat package; this file mirrors it without importing moat
// because the dependency graph goes moat → catalog, not the other way.
//
// Why three separate types:
//   - TrustTier is the normative internal state (matches moat.TrustTier
//     plus TrustTierUnknown for content that was never sourced from a
//     MOAT manifest at all — e.g., a git registry).
//   - TrustBadge is the user-facing collapsed state (Verified / Recalled
//     / none) per AD-7's three-state model. Users never see "Dual-Attested"
//     as a distinct surface; the collapse is intentional.
//   - Description text is derived from the internal TrustTier so the
//     drill-down ("Verified (dual-attested by publisher and registry)")
//     can show the full tier even though the headline badge collapses.

// TrustTier is the internal, normative classification of an item's trust
// state. The zero value is TrustTierUnknown — applied to items that were
// never sourced from a MOAT manifest (git registries, local content).
// Once MOATClient lands (ADR 0007 AD-6), it sets this field during
// catalog scan by calling moat.ContentEntry.TrustTier() and mapping to
// these values.
type TrustTier int

const (
	TrustTierUnknown      TrustTier = iota // not MOAT-sourced; no trust claim either way
	TrustTierUnsigned                      // MOAT manifest entry with no rekor_log_index
	TrustTierSigned                        // rekor_log_index present, no per-item signing_profile
	TrustTierDualAttested                  // both rekor_log_index and signing_profile present
)

// String returns the human-readable tier name used in drill-down displays
// and JSON output. TrustTierUnknown renders as the empty string because
// callers omit it from serialized output (absence == "no claim made").
func (t TrustTier) String() string {
	switch t {
	case TrustTierDualAttested:
		return "Dual-Attested"
	case TrustTierSigned:
		return "Signed"
	case TrustTierUnsigned:
		return "Unsigned"
	}
	return ""
}

// TrustBadge is the user-facing three-state collapse from AD-7 Panel C9.
// Dual-Attested and Signed both collapse to Verified; Unsigned and Unknown
// collapse to NoBadge (AD-7: "absence is not a negative signal"). Recalled
// is driven by revocation state, not by TrustTier — set independently by
// the caller (e.g., revocation check in G-8).
type TrustBadge int

const (
	TrustBadgeNone     TrustBadge = iota // nothing to display
	TrustBadgeVerified                   // green checkmark ("Verified")
	TrustBadgeRecalled                   // red X or yellow warning ("Recalled")
)

// Label returns the one-word user-facing label for a badge. Empty string
// for TrustBadgeNone so callers can write `if lbl := b.Label(); lbl != ""`.
func (b TrustBadge) Label() string {
	switch b {
	case TrustBadgeVerified:
		return "Verified"
	case TrustBadgeRecalled:
		return "Recalled"
	}
	return ""
}

// Glyph returns a single-character visual indicator sized to render beside
// a name in a list row. Uses Unicode symbols (not emojis) so output stays
// terminal-safe and the in-repo "no emojis" convention holds.
func (b TrustBadge) Glyph() string {
	switch b {
	case TrustBadgeVerified:
		return "\u2713" // ✓
	case TrustBadgeRecalled:
		return "\u2717" // ✗
	}
	return ""
}

// UserFacingBadge applies the AD-7 collapse rule. Recalled takes precedence
// over any trust tier: a revoked item is Recalled regardless of whether it
// was previously Verified — this is the only safe default. Dual-Attested
// and Signed both produce Verified. Unsigned and Unknown produce no badge
// per AD-7 ("Git registries show no trust badge — absence is not a
// negative signal"; an explicit Unsigned MOAT entry carries the same
// semantics — no verified claim was made).
func UserFacingBadge(tier TrustTier, recalled bool) TrustBadge {
	if recalled {
		return TrustBadgeRecalled
	}
	switch tier {
	case TrustTierDualAttested, TrustTierSigned:
		return TrustBadgeVerified
	}
	return TrustBadgeNone
}

// TrustDescription returns the drill-down text shown on an item detail
// surface (metadata panel, `syllago show`, install-confirmation screen).
// It preserves the full tier distinction that UserFacingBadge collapses —
// a user who wants to know *why* an item is Verified gets the answer here.
// Returns empty string for Unknown; callers suppress the line entirely
// rather than print "Trust: ".
func TrustDescription(tier TrustTier, recalled bool, recallReason string) string {
	if recalled {
		if recallReason != "" {
			return "Recalled — " + recallReason
		}
		return "Recalled"
	}
	switch tier {
	case TrustTierDualAttested:
		return "Verified (dual-attested by publisher and registry)"
	case TrustTierSigned:
		return "Verified (registry-attested)"
	case TrustTierUnsigned:
		return "Unsigned (registry declares no attestation)"
	}
	return ""
}
