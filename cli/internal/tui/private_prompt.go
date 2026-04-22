package tui

// Private-repo install confirmation helpers (ADR 0007 G-10, bead syllago-u0jna).
//
// Private content (content_entry.private_repo=true in the manifest) requires
// explicit operator acknowledgement before install because the visibility
// expectations are something only the operator can confirm — the registry
// cannot know whether installing a private-sourced item is appropriate for
// this machine. The acceptance shape matches TOFU semantically, which is
// why installer.MarkPrivateConfirmed uses a distinct "private:" prefix on
// the Session key (disjoint from publisher-warn confirmations for the same
// registry+hash pair).
//
// These helpers mirror publisher_warn.go but are kept in a separate file so
// the two gates don't share wording or fall out of sync. The confirm-modal
// component is reused (OpenForItem + confirmResultMsg), keeping the visual
// language identical across gate variants — only the title/body/label
// differ.

import (
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// privatePromptTitle returns the confirm-modal title for a private-repo
// install attempt. Favors DisplayName so the user recognizes the same
// string they saw in the library table.
func privatePromptTitle(item catalog.ContentItem) string {
	name := item.DisplayName
	if name == "" {
		name = item.Name
	}
	return "Install private-source item \"" + name + "\"?"
}

// privatePromptBody builds the confirm-modal body text for a private-repo
// install. Keeps each line self-contained so the modal's multi-line body
// renderer can truncate per line without losing context.
//
// The body does NOT attempt to explain WHY this item is private (the
// manifest does not encode that) — only that the publisher has declared it
// as private and the operator must decide whether the current machine is an
// appropriate target. Matches the CLI prompt's minimalism.
func privatePromptBody(item catalog.ContentItem) string {
	lines := []string{
		"This item is declared as coming from a private source.",
		"",
		"Private content may include credentials, proprietary logic, or artifacts not intended for general distribution. Confirm that this machine is an appropriate install target.",
	}
	_ = item // item retained for parity with publisherWarnBody; future fields may include private-source metadata.
	return strings.Join(lines, "\n")
}
