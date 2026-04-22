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
//   - TrustBadge is the user-facing collapsed state (Verified / Revoked
//     / none) per AD-7's three-state model. Users never see "Dual-Attested"
//     as a distinct surface; the collapse is intentional. Earlier drafts
//     used "Recalled" as the user-facing label for the revoked state;
//     consolidated to "Revoked" to match the MOAT protocol's Revocation
//     terminology and avoid a two-vocabulary split between code and UI.
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

// ShortLabel returns the compact tier label used in space-constrained
// surfaces like the metadata panel's Trust chip. Unlike TrustDescription
// it drops the parenthetical ("dual-attested by publisher and registry")
// — that long form is reserved for the Trust Inspector drill-down so the
// metapanel row stays a stable fixed width.
func (t TrustTier) ShortLabel() string {
	switch t {
	case TrustTierDualAttested:
		return "Dual attested"
	case TrustTierSigned:
		return "Signed"
	case TrustTierUnsigned:
		return "Unsigned"
	}
	return ""
}

// TrustBadge is the user-facing three-state collapse from AD-7 Panel C9.
// Dual-Attested and Signed both collapse to Verified; Unsigned and Unknown
// collapse to NoBadge (AD-7: "absence is not a negative signal"). Revoked
// is driven by revocation state, not by TrustTier — set independently by
// the caller (e.g., revocation check in G-8).
type TrustBadge int

const (
	TrustBadgeNone     TrustBadge = iota // nothing to display
	TrustBadgeVerified                   // green checkmark ("Verified")
	TrustBadgeRevoked                    // red "R" ("Revoked")
)

// Label returns the one-word user-facing label for a badge. Empty string
// for TrustBadgeNone so callers can write `if lbl := b.Label(); lbl != ""`.
func (b TrustBadge) Label() string {
	switch b {
	case TrustBadgeVerified:
		return "Verified"
	case TrustBadgeRevoked:
		return "Revoked"
	}
	return ""
}

// Glyph returns a single-character visual indicator sized to render beside
// a name in a list row. Uses ASCII / Unicode symbols (not emojis) so output
// stays terminal-safe and the in-repo "no emojis" convention holds.
//
// Revoked renders as the ASCII letter "R" (rendered in dangerColor by
// callers) rather than "✗", because Unicode ✗ reads as "unsupported" in
// other parts of the codebase (converter compat matrix, capmon validate
// output). Using "R" keeps the revoked semantics distinct at a glance and
// avoids cross-context glyph overloading. The bare-letter form (no brackets)
// is intentional — brackets are reserved for hotkey labels like [1]/[a]/[n].
func (b TrustBadge) Glyph() string {
	switch b {
	case TrustBadgeVerified:
		return "\u2713" // ✓
	case TrustBadgeRevoked:
		return "R"
	}
	return ""
}

// UserFacingBadge applies the AD-7 collapse rule. Revoked takes precedence
// over any trust tier: a revoked item is Revoked regardless of whether it
// was previously Verified — this is the only safe default. Dual-Attested
// and Signed both produce Verified. Unsigned and Unknown produce no badge
// per AD-7 ("Git registries show no trust badge — absence is not a
// negative signal"; an explicit Unsigned MOAT entry carries the same
// semantics — no verified claim was made).
func UserFacingBadge(tier TrustTier, revoked bool) TrustBadge {
	if revoked {
		return TrustBadgeRevoked
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
func TrustDescription(tier TrustTier, revoked bool, revocationReason string) string {
	if revoked {
		if revocationReason != "" {
			return "Revoked — " + revocationReason
		}
		return "Revoked"
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

// TrustDetailExplanation returns the long-form prose shown in the Trust
// Inspector's "Detail" row. Where TrustDescription is a single-line label
// ("Verified (dual-attested ...)"), this is the multi-sentence explanation
// a user reads to understand what the tier actually means — what was
// checked, what was not, and how the two attestation sources interact.
// Returns empty for Unknown so callers suppress the row entirely.
//
// The Revoked branches take precedence because once content is revoked,
// the prior attestation chain is background context at best; leading with
// the revocation state matches how users reason about the artifact.
func TrustDetailExplanation(tier TrustTier, revoked bool) string {
	if revoked {
		switch tier {
		case TrustTierDualAttested, TrustTierSigned:
			return "This content was previously attested but has since been revoked. " +
				"Revocation overrides any prior trust claim — the attestation chain is " +
				"no longer considered valid for install. See the revocation details below."
		}
		return "This content has been revoked by the registry or publisher. " +
			"Revocation overrides any prior trust claim. See the revocation details below."
	}
	switch tier {
	case TrustTierDualAttested:
		return "Dual-attested means two independent parties cryptographically vouch " +
			"for this content: the publisher (the identity that built it) and the " +
			"registry (the operator that distributes it). Each party's signature " +
			"is recorded in a public transparency log. This is the strongest trust " +
			"claim MOAT can express."
	case TrustTierSigned:
		return "Signed means the registry cryptographically attests to this content " +
			"and records the attestation in a public transparency log. The individual " +
			"publisher did not attach their own signature, so the registry operator " +
			"is the sole attesting party."
	case TrustTierUnsigned:
		return "Unsigned means the registry explicitly declares that this content " +
			"carries no cryptographic attestation. This is a deliberate signal — " +
			"not a missing signature — but users should rely on other signals " +
			"(source, publisher reputation) before installing."
	}
	return ""
}
