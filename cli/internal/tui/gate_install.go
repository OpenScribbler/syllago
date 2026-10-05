package tui

// TUI install-gate adapter.
//
// Gates installs of Library copies of MOAT registry items. A registry item's
// own install goes through moatinstall.Operation, which runs the gate itself.
// This adapter maps each MOATGateDecision to TUI primitives (modals + toasts).
// It reads the same installer.PreInstallCheck output as the operation, so the
// TUI and CLI cannot disagree about whether a given (registry, hash) is gated.
//
// Lifecycle:
//   - resolveInstallGate runs synchronously from handleInstallResult /
//     handleInstallAllResult. Reads on-disk state (BuildGateInputs, Lockfile)
//     are already done in rescanCatalog, so this layer is CPU-only.
//   - When the decision requires operator input (PublisherWarn /
//     PrivatePrompt), the caller stashes the install msg on App and opens
//     the shared confirmModal. On Y, handleConfirmResult calls the correct
//     MarkConfirmed variant based on pendingGateKind and runs the gate
//     again, so a second question (a private prompt after a recall) is
//     still asked.
//   - When the decision hard-refuses (HardBlock / TierBelowPolicy), the
//     caller pushes an error toast and returns without stashing.
//
// Items with no MOAT lineage (local content, or content from a registry
// that is not MOAT-backed) bypass the gate entirely.

import (
	"fmt"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
)

// pendingGateKind identifies which Session.MarkConfirmed variant should fire
// on Y. gateKindNone means the current install has no stashed gate awaiting
// confirmation — the App's pendingInstall/pendingInstallAll fields are
// cleared and the confirm modal is not in the gate path.
type pendingGateKind int

const (
	gateKindNone pendingGateKind = iota
	gateKindPublisherWarn
	gateKindPrivatePrompt
)

// gateEvaluation bundles the inputs PreInstallCheck consumed plus the
// resulting decision. Returned from evaluateInstallGate so callers can stash
// registryURL/contentHash on App without re-deriving them from the item.
type gateEvaluation struct {
	decision    installer.GateBlock
	registryURL string
	contentHash string
	entryName   string
}

// evaluateInstallGate resolves the MOAT gate for an install attempt on
// `item`, a registry item or a Library copy of one. Returns (eval, true)
// when the item has a MOAT lineage that yields a gate decision, or
// (_, false) when it should bypass gating (see installer.CheckItem).
func evaluateInstallGate(a *App, item catalog.ContentItem) (gateEvaluation, bool) {
	if a == nil {
		return gateEvaluation{}, false
	}
	check, ok := installer.CheckItem(item, a.moatGate, a.moatLockfile, a.moatSession, a.moatMinTier)
	if !ok {
		return gateEvaluation{}, false
	}
	// A publisher confirmation is keyed by the registry that issued the
	// revocation, which the check looks up, and that may not be the
	// registry the item came from.
	registryURL := check.RegistryURL
	if check.Decision == installer.MOATGatePublisherWarn {
		registryURL = check.Revocation.IssuingRegistryURL
	}
	return gateEvaluation{
		decision:    check.GateBlock,
		registryURL: registryURL,
		contentHash: check.Entry.ContentHash,
		entryName:   check.Entry.Name,
	}, true
}

// tierBelowPolicyMessage renders a user-facing toast string for the
// MOATGateTierBelowPolicy branch. Mirrors the CLI structured-error detail
// shape so log-grepping scripts can match on observed= / min= fragments
// across surfaces.
func tierBelowPolicyMessage(name string, observed, min moat.TrustTier) string {
	return fmt.Sprintf(
		"Refused %q: trust tier %s is below policy minimum %s",
		name, observed.String(), min.String(),
	)
}

// lockfileUnreadableMessage renders the toast that refuses an install of
// registry content while the lockfile, whose archived revocations the
// gate checks, cannot be read. The CLI refuses the same install.
func lockfileUnreadableMessage(name string, err error) string {
	return fmt.Sprintf("Refused %q: the MOAT lockfile could not be read (%v)", name, err)
}

// hardBlockMessage renders a user-facing toast string for the
// MOATGateHardBlock branch. Registry-source revocations are permanent — a
// modal would be deceptive since confirm cannot override
// a registry block.
func hardBlockMessage(name string, rev *moat.RevocationRecord) string {
	reason := ""
	if rev != nil {
		reason = moat.SanitizeForDisplay(rev.Reason)
	}
	if reason == "" {
		return fmt.Sprintf("Refused %q: registry has revoked this item", name)
	}
	return fmt.Sprintf("Refused %q: registry revoked (%s)", name, reason)
}

// gateCancelledToastText labels the warning toast shown when the user
// cancels a publisher-warn or private-prompt modal. Kept gate-aware so the
// operator sees which gate they just dismissed (both reach the same code
// path but carry different expectations).
func gateCancelledToastText(kind pendingGateKind) string {
	switch kind {
	case gateKindPublisherWarn:
		return "Install cancelled (recalled item)"
	case gateKindPrivatePrompt:
		return "Install cancelled (private source)"
	default:
		return "Install cancelled"
	}
}
