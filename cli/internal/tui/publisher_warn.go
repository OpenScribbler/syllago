package tui

import (
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// isPublisherRevoked reports whether an item carries a publisher-source
// revocation. These items require operator acknowledgement before install
// per ADR 0007 G-8 (the two-tier revocation contract: registry-source always
// hard-blocks, publisher-source warns-and-confirms).
//
// The decision uses EnrichCatalog-populated fields (item.Recalled +
// item.RecallSource == "publisher") because the TUI does not yet thread a
// moat.Session / RevocationSet through the install wizard. The CLI install
// path calls installer.PreInstallCheck directly and is the authoritative
// gate; the TUI modal is a UX confirmation in front of that gate.
//
// Registry-source revocations are not surfaced here — those hard-block in
// the installer regardless of operator choice, so a modal would be
// deceptive (the "confirm" button wouldn't actually install).
func isPublisherRevoked(item catalog.ContentItem) bool {
	return item.Recalled && strings.EqualFold(item.RecallSource, "publisher")
}

// publisherWarnBody builds the confirm-modal body text for a publisher-revoked
// item. Collapses gracefully when optional fields are absent. Keep every
// piece on its own line so the modal's multi-line body renderer can handle
// truncation per line.
func publisherWarnBody(item catalog.ContentItem) string {
	var lines []string
	lines = append(lines, "The publisher has revoked this item.")

	if item.RecallReason != "" {
		lines = append(lines, "", "Reason: "+item.RecallReason)
	}
	if item.RecallIssuer != "" {
		lines = append(lines, "Issued by: "+item.RecallIssuer)
	}
	if item.RecallDetailsURL != "" {
		lines = append(lines, "Details: "+item.RecallDetailsURL)
	}

	lines = append(lines, "", "Installing anyway will proceed with the current content. The registry has not blocked this hash.")

	return strings.Join(lines, "\n")
}

// publisherWarnTitle returns the confirm-modal title for a publisher-revoked
// item.
func publisherWarnTitle(item catalog.ContentItem) string {
	name := item.DisplayName
	if name == "" {
		name = item.Name
	}
	return "Install recalled item \"" + name + "\"?"
}
