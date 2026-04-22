package tui

// TUI install-gate adapter (ADR 0007 Phase 2c, bead syllago-u0jna).
//
// Mirrors cmd/syllago/install_moat.go:resolveGateDecision but maps each
// MOATGateDecision to TUI primitives (modals + toasts) instead of stderr +
// exit codes. The two adapters read from the same installer.PreInstallCheck
// output so the TUI and CLI cannot disagree about whether a given
// (registry, hash) is gated.
//
// Lifecycle:
//   - resolveInstallGate runs synchronously from handleInstallResult /
//     handleInstallAllResult. Reads on-disk state (BuildGateInputs, Lockfile)
//     are already done in rescanCatalog, so this layer is CPU-only.
//   - When the decision requires operator input (PublisherWarn /
//     PrivatePrompt), the caller stashes the install msg on App and opens
//     the shared confirmModal. handleConfirmResult re-dispatches on Y and
//     calls the correct MarkConfirmed variant based on pendingGateKind.
//   - When the decision hard-refuses (HardBlock / TierBelowPolicy), the
//     caller pushes an error toast and returns without stashing.
//
// Non-MOAT items (local library content with no Registry, or items whose
// registry is not MOAT-backed) bypass the gate entirely — the same safe
// default the legacy install path has always had.

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
// `item`. Returns (eval, true) when the item has a MOAT lineage that yields
// a gate decision, or (_, false) when the item is not MOAT-backed and should
// bypass gating entirely (legacy install path).
//
// A missing manifest entry (item.Registry is MOAT but the name was not
// listed in the manifest at scan time) is treated as bypass: the library
// copy was presumably added outside the MOAT flow, and gating a phantom
// entry would be more confusing than helpful. This matches
// BuildGateInputs's silent-skip policy for unparseable manifests.
func evaluateInstallGate(a *App, item catalog.ContentItem) (gateEvaluation, bool) {
	if a == nil || a.moatGate == nil {
		return gateEvaluation{}, false
	}
	if item.Registry == "" {
		return gateEvaluation{}, false
	}
	if !a.moatGate.HasRegistry(item.Registry) {
		return gateEvaluation{}, false
	}
	manifest := a.moatGate.Manifests[item.Registry]
	entry, ok := moat.FindContentEntry(manifest, item.Name)
	if !ok {
		return gateEvaluation{}, false
	}
	registryURL := a.moatGate.ManifestURIs[item.Registry]
	gate := installer.PreInstallCheck(
		entry,
		registryURL,
		a.moatLockfile,
		a.moatGate.RevSet,
		a.moatSession,
		a.moatMinTier,
	)
	return gateEvaluation{
		decision:    gate,
		registryURL: registryURL,
		contentHash: entry.ContentHash,
		entryName:   entry.Name,
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

// hardBlockMessage renders a user-facing toast string for the
// MOATGateHardBlock branch. Registry-source revocations are permanent per
// ADR 0007 G-15 — a modal would be deceptive since confirm cannot override
// a registry block.
func hardBlockMessage(name string, rev *moat.RevocationRecord) string {
	reason := ""
	if rev != nil {
		reason = rev.Reason
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
