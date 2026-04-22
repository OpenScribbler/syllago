package tui

// Publisher-warn install confirmation helpers (ADR 0007 G-8, bead syllago-u0jna).
//
// Publisher-source revocations are the "warn-and-confirm" tier of the two-
// tier revocation contract: unlike registry-source revocations (which hard-
// block), a publisher revocation surfaces an operator-facing warning but
// allows install to proceed on explicit Y. Once confirmed in a TUI session,
// subsequent installs of the same (registry, hash) skip the prompt — see
// installer.MarkPublisherConfirmed.
//
// The modal now reads from the gate's *moat.RevocationRecord (populated by
// installer.PreInstallCheck from the freshest RevocationSet) rather than
// from catalog.ContentItem fields. Previously the TUI relied on
// EnrichCatalog-populated fields, which could drift from the CLI install
// path when a revocation landed between rescan and install dispatch. The
// gate-driven path eliminates that drift.

import (
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
)

// isPublisherRevoked reports whether an item carries a publisher-source
// revocation. These items require operator acknowledgement before install
// per ADR 0007 G-8 (the two-tier revocation contract: registry-source always
// hard-blocks, publisher-source warns-and-confirms).
//
// Registry-source revocations are not surfaced here — those hard-block in
// the installer regardless of operator choice, so a modal would be
// deceptive (the "confirm" button wouldn't actually install).
func isPublisherRevoked(item catalog.ContentItem) bool {
	return item.Revoked && strings.EqualFold(item.RevocationSource, "publisher")
}

// publisherWarnTitle returns the confirm-modal title for a publisher-
// revoked item. Uses DisplayName when present so the title matches what
// the user saw in the library table.
func publisherWarnTitle(item catalog.ContentItem) string {
	name := item.DisplayName
	if name == "" {
		name = item.Name
	}
	return "Install revoked item \"" + name + "\"?"
}

// publisherWarnBody builds the confirm-modal body for a publisher-revoked
// item. Prefers the live RevocationRecord (from the install gate) when
// available, falling back to ContentItem fields for items that have a
// Recalled flag but no gate record (e.g. a pre-gate enriched catalog that
// somehow reaches this helper — defense in depth, should be unreachable
// once every caller threads the gate).
//
// Every optional field is on its own line so the multi-line body renderer
// can truncate per line without losing context.
func publisherWarnBody(item catalog.ContentItem, rev *moat.RevocationRecord) string {
	lines := []string{"The publisher has revoked this item."}

	reason, issuer, detailsURL := warnFieldsFrom(item, rev)
	if reason != "" {
		lines = append(lines, "", "Reason: "+reason)
	}
	if issuer != "" {
		lines = append(lines, "Revoked by: "+issuer)
	}
	if detailsURL != "" {
		lines = append(lines, "Details: "+detailsURL)
	}

	lines = append(lines, "", "Installing anyway will proceed with the current content. The registry has not blocked this hash.")
	return strings.Join(lines, "\n")
}

// warnFieldsFrom resolves the three operator-visible strings (reason,
// issuer, details URL) from the gate record with a ContentItem fallback.
// Gate record wins for reason and details URL — it is the authoritative
// live-manifest view and may already reflect a revocation added between
// the last rescan and this install attempt. Issuer is not carried on the
// live record (RevocationRecord intentionally drops the revoker identity
// since the Session keys off (registry, hash)), so Revoker from the
// enriched item is the only available source.
func warnFieldsFrom(item catalog.ContentItem, rev *moat.RevocationRecord) (reason, issuer, detailsURL string) {
	if rev != nil {
		reason = rev.Reason
		detailsURL = rev.DetailsURL
	}
	if reason == "" {
		reason = item.RevocationReason
	}
	if detailsURL == "" {
		detailsURL = item.RevocationDetailsURL
	}
	issuer = item.Revoker
	return reason, issuer, detailsURL
}
