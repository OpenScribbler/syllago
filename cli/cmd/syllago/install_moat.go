package main

// MOAT registry-sourced install dispatcher (ADR 0007 Phase 2, bead
// syllago-elvv3 — first slice of parent bead syllago-svdwc).
//
// Scope of this slice:
//
//   - Parse the `<registry>/<item>` positional syntax in `syllago install`
//     and route to this file's runInstallFromRegistry when present.
//   - Run the MOAT sync for the named registry and map SyncResult flags to
//     the G-18 non-interactive exit codes (10 TOFU / 11 profile change /
//     13 stale). The exit-code path bypasses cobra's RunE via moatSyncExit
//     (os.Exit) — cobra only knows about 0/1.
//   - Resolve the ContentEntry in the freshly-synced manifest via
//     moat.FindContentEntry (bead syllago-kvf66).
//   - Under `--dry-run`, print a diagnostic summary of what was resolved
//     (trust tier, content hash prefix, source URI) so operators can
//     validate the syntax without mutating state.
//   - Under non-dry-run, return a structured "gate wiring lands in
//     syllago-raivj" error with exit 1. This is deliberate: the gate
//     primitives (installer.PreInstallCheck), interactive prompts, and the
//     actual file side-effect land in the next sub-bead so this commit
//     stays reviewable.
//
// What this slice intentionally does NOT do:
//
//   - Call installer.PreInstallCheck. No gate branching, no Y/n prompts.
//   - Touch the filesystem. No file copy, no symlink, no audit log.
//   - Append LockEntry rows. lockfile.Save is never called.
//
// Side effects on success: registry.ManifestETag and registry.LastFetchedAt
// are updated (and cfg saved) exactly as the existing syncMOATRegistry
// dispatcher does. This mirroring is intentional — an install that begins
// with a 200 + verified sync has legitimately advanced the trust clock,
// even if the install itself is not yet performed. Pipelines that run
// `syllago install foo/bar --dry-run` get the same fetched_at advance
// they would have gotten from `syllago registry sync foo`, which is the
// correct semantics for the 72h staleness window.

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/moatinstall"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
)

// moatInstallNow is the clock seam for the install-from-registry path.
// Tests swap this out to pin the staleness classification and fetched_at
// persistence. Mirrors moatNow in moat_cmd.go — kept separate because the
// two surfaces are developed and tested independently, and a single shared
// override would couple their test lifetimes.
var moatInstallNow = time.Now

// moatInstallPromptFn is the Y/n prompt seam for publisher-warn and
// private-prompt gates. Default reads a single line from os.Stdin; tests
// swap this to inject answers without touching TTY state. Returns true on
// affirmative input ("y"/"yes"/""), false otherwise.
var moatInstallPromptFn = defaultMoatInstallPrompt

// moatInstallInteractiveFn mirrors cmd/syllago/helpers.go:isInteractive
// but as its own seam so install-gate tests can force headless mode
// independently of other command tests. Default delegates to the shared
// helper.
var moatInstallInteractiveFn = func() bool { return isInteractive() }

// moatInstallMinTier is the policy floor the install gate enforces. ADR
// 0007 leaves the knob up to the operator; until a CLI/config surface
// lands this defaults to TrustTierUnsigned (accept any tier). Tests
// override to exercise TierBelowPolicy.
var moatInstallMinTier = moat.TrustTierUnsigned

// defaultMoatInstallPrompt reads a Y/n answer from stdin. Empty input is
// treated as Yes — matches the default in resolveConflictInteractively and
// the install UX expectations operators already have. Scanner errors are
// treated as No (safer to block than to silently proceed on EOF).
func defaultMoatInstallPrompt(w io.Writer, question string) bool {
	fmt.Fprint(w, question)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return false
	}
	ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return ans == "" || ans == "y" || ans == "yes"
}

// parseRegistryItemSyntax splits a positional "registry/item" argument.
// Returns (registryName, itemName, true) when the arg contains exactly one
// "/", both halves are non-empty, and neither half is blank. Any other
// shape returns ok=false so the caller can fall through to the existing
// library-install path.
//
// A blank registry ("/foo") or blank item ("foo/") is a user mistake, not
// an implicit library install — ok=false surfaces it as "no item named
// '/foo' found" which is a better diagnostic than a silent fallback.
// Multi-slash args ("a/b/c") also return false because a registry name
// cannot contain "/" (config.Registry.Name rules forbid it).
func parseRegistryItemSyntax(arg string) (string, string, bool) {
	idx := strings.Index(arg, "/")
	if idx < 0 {
		return "", "", false
	}
	if strings.Count(arg, "/") != 1 {
		return "", "", false
	}
	reg := arg[:idx]
	item := arg[idx+1:]
	if reg == "" || item == "" {
		return "", "", false
	}
	return reg, item, true
}

// runInstallFromRegistry is the MOAT-backed install path. Returns a normal
// error for sync/parse/lookup failures (cobra maps to exit 1); on a G-18
// non-interactive failure it calls moatSyncExit(code) and returns nil —
// cobra cannot express exit codes beyond 1.
//
// The function re-uses the same seams as syncMOATRegistry so test harnesses
// written for that path (moatSyncFn, moatSyncExit) compose without
// additional stubbing.
// targetProv is the provider that the resolved MOAT item should be
// installed to once fetchAndRecord has staged its source-cache directory.
// May be nil for dry-run paths or when the caller has not yet selected
// a provider (in non-dry-run that's a programmer error and we surface it
// as a structured input-missing failure rather than panicking).
//
// method/baseDir thread the same install semantics as the library install
// path (--method, --base-dir) into the MOAT pipeline so the resulting
// symlink/copy ends up where operators expect.
func runInstallFromRegistry(
	ctx context.Context,
	out, errW io.Writer,
	cfg *config.Config,
	cfgRoot string,
	registryName, itemName string,
	targetProv *provider.Provider,
	method installer.InstallMethod,
	baseDir string,
	dryRun bool,
	now time.Time,
) error {
	reg := findRegistryByName(cfg, registryName)
	if reg == nil {
		return output.NewStructuredError(
			output.ErrRegistryNotFound,
			fmt.Sprintf("registry %q not found in config", registryName),
			"Run 'syllago registry list' to see configured registries, or 'syllago registry add' to add one.",
		)
	}
	if !reg.IsMOAT() {
		return output.NewStructuredError(
			output.ErrMoatInvalid,
			fmt.Sprintf("registry %q is not MOAT-backed; <registry>/<item> install syntax requires a MOAT registry", registryName),
			"Use the plain 'syllago install <item>' form for git-registry content, or configure this registry as MOAT with a manifest_uri.",
		)
	}

	rootInfo := moat.BundledTrustedRoot(now)
	if rootInfo.Status == moat.TrustedRootStatusExpired ||
		rootInfo.Status == moat.TrustedRootStatusMissing ||
		rootInfo.Status == moat.TrustedRootStatusCorrupt {
		return output.NewStructuredErrorDetail(
			output.ErrMoatTrustedRootStale,
			fmt.Sprintf("bundled trusted root unusable while installing from registry %q", registryName),
			"Run `syllago update` to refresh the bundled Sigstore trusted root.",
			rootInfo.Status.String(),
		)
	}

	lockfilePath := moat.LockfilePath(cfgRoot)
	lf, err := moat.LoadLockfile(lockfilePath)
	if err != nil {
		return fmt.Errorf("load lockfile: %w", err)
	}

	// Sync *before* the gate. A stale manifest cannot correctly classify
	// revocations or tiers; re-fetching here means the gate always reads
	// from the freshest view the network can provide. TOFU and profile-
	// change paths short-circuit with G-18 exits — install never proceeds
	// past a trust-state surprise.
	//
	// --yes is intentionally NOT exposed on install. Accepting TOFU at
	// install time would let a pipeline silently pin a new registry by
	// scheduling one job; the spec's row-1 MUST-exit is meant to prevent
	// exactly that shortcut. Operators run `syllago registry add --yes`
	// interactively first; only then can install succeed.
	res, err := moatSyncFn(ctx, reg, lf, rootInfo.Bytes, nil, now)
	if err != nil {
		var ve *moat.VerifyError
		if errors.As(err, &ve) {
			return classifyVerifyError(reg.Name, err)
		}
		return output.NewStructuredErrorDetail(
			output.ErrMoatInvalid,
			fmt.Sprintf("sync failed while installing from registry %q", reg.Name),
			"Run `syllago registry sync` to surface the error in isolation, then retry install.",
			err.Error(),
		)
	}

	if res.ProfileChanged {
		fmt.Fprintf(errW, "syllago: %s\n", moat.FailureSigningProfileChange.Message())
		moatSyncExit(moat.ExitMoatSigningProfileChange)
		return nil
	}
	if res.IsTOFU {
		fmt.Fprintf(errW, "syllago: %s\n", moat.FailureTOFUAcceptance.Message())
		moatSyncExit(moat.ExitMoatTOFUAcceptance)
		return nil
	}
	if res.Staleness == moat.StalenessExpired {
		fmt.Fprintf(errW, "syllago: %s\n", moat.FailureManifestStale.Message())
		moatSyncExit(moat.ExitMoatManifestStale)
		return nil
	}

	// Happy path — persist the sync state before any further action.
	// Failing to persist here would force the next install to re-fetch +
	// re-verify, losing the freshness progress we just paid for.
	reg.ManifestETag = res.ETag
	fetchedAt := res.FetchedAt
	reg.LastFetchedAt = &fetchedAt
	if err := config.Save(cfgRoot, cfg); err != nil {
		return output.NewStructuredErrorDetail(
			output.ErrRegistrySaveFailed,
			fmt.Sprintf("could not persist sync state for registry %q", reg.Name),
			"Check filesystem permissions on .syllago/config.json.",
			err.Error(),
		)
	}
	if err := lf.Save(lockfilePath); err != nil {
		return output.NewStructuredErrorDetail(
			output.ErrMoatInvalid,
			fmt.Sprintf("could not persist moat lockfile after syncing %q", reg.Name),
			"Check filesystem permissions on .syllago/moat-lockfile.json.",
			err.Error(),
		)
	}

	// NotModified: the previously-cached manifest is still authoritative,
	// but res.Manifest is nil (no bytes re-parsed). We currently have no
	// on-disk cache of the verified manifest body, so we cannot resolve
	// the ContentEntry without re-fetching. Direct the operator to run a
	// full sync — a 200 path will repopulate res.Manifest next time.
	if res.NotModified {
		return output.NewStructuredError(
			output.ErrMoatInvalid,
			fmt.Sprintf("registry %q is at the pinned revision but the manifest body is not cached locally", reg.Name),
			"Clear the cached ETag by removing the `manifest_etag` field for registry \""+reg.Name+"\" from .syllago/config.json, then run `syllago registry sync "+reg.Name+"` to force a full re-fetch.",
		)
	}

	entry, ok := moat.FindContentEntry(res.Manifest, itemName)
	if !ok {
		return output.NewStructuredError(
			output.ErrInstallItemNotFound,
			fmt.Sprintf("registry %q does not list an item named %q in its manifest", reg.Name, itemName),
			"Run `syllago registry items "+reg.Name+"` to see available content.",
		)
	}

	// Gate evaluation. PreInstallCheck is CLI-agnostic — we map its five
	// decisions to CLI behaviour here (structured errors, Y/n prompts,
	// G-18 non-interactive exits). Session and RevocationSet are
	// per-invocation: a fresh CLI process is a fresh session. If the
	// operator answers Y to a PublisherWarn, the suppression lasts only
	// for this single `syllago install` call — which is the right
	// granularity because each install is a discrete authorization act.
	revSet := moat.NewRevocationSet()
	revSet.AddFromManifest(res.Manifest, reg.ManifestURI)
	session := moat.NewSession()

	gate := installer.PreInstallCheck(entry, reg.ManifestURI, lf, revSet, session, moatInstallMinTier)

	// Telemetry on both proceed and refuse paths — ops want to see what
	// tier users are actually landing on and how often gates fire. Enrich
	// here rather than at each branch so every exit carries the pair.
	telemetry.Enrich("moat_tier", entry.TrustTier().String())
	telemetry.Enrich("moat_gated", gate.Decision.String())

	proceed, gateErr := resolveGateDecision(errW, reg.ManifestURI, entry, session, gate)
	if gateErr != nil {
		return gateErr
	}
	if !proceed {
		// A non-interactive refusal or exit(10|12) has already fired via
		// moatSyncExit; callers below must not execute. Returning nil
		// matches the TOFU/profile-change/stale paths above — cobra maps
		// nil to exit 0, but os.Exit has already overridden that.
		return nil
	}

	if dryRun {
		printRegistryItemSummaryWithGate(out, reg.Name, entry, res.RevocationsAdded, res.Staleness, gate.Decision)
		return nil
	}

	// Gates cleared. For SIGNED/DUAL-ATTESTED tiers, fetchAndRecord first
	// fetches the per-item Rekor entry and verifies it against the pinned
	// signing profile (per-item if present, else the manifest-level
	// RegistrySigningProfile) before paying for the source-artifact
	// download. UNSIGNED tiers skip the verification path and land with a
	// null AttestationBundle. The trustedRoot bytes were already validated
	// at sync time (rootInfo.Status check above).
	cacheDir, fetchErr := moatinstall.FetchAndRecord(
		ctx, entry, reg.Name, reg.ManifestURI, lockfilePath, lf,
		&res.Manifest.RegistrySigningProfile, rootInfo.Bytes,
	)
	if fetchErr != nil {
		return fetchErr
	}

	// Provider-side install: stage the cached source tree into the
	// provider's content directory (~/.claude/skills/foo, ~/.codex/agents,
	// etc.). Until this lands, fetchAndRecord pinned the lockfile + cached
	// the bytes but the operator's chosen provider directory was empty —
	// install was effectively a no-op from the user's perspective.
	if targetProv == nil {
		return output.NewStructuredError(
			output.ErrInputMissing,
			"install destination not specified",
			"Pass --to <provider> to choose where the registry item is installed.",
		)
	}
	installPath, installErr := installer.InstallCachedMOATToProvider(
		cacheDir, entry, *targetProv, cfgRoot, method, baseDir,
	)
	if installErr != nil {
		return output.NewStructuredErrorDetail(
			output.ErrInstallNotWritable,
			fmt.Sprintf("could not install %s/%s to %s", reg.Name, entry.Name, targetProv.Name),
			"The source artifact was fetched and verified but the provider-side install failed. Check filesystem permissions, that the target directory is writable, and that the provider supports this content type.",
			installErr.Error(),
		)
	}
	fmt.Fprintf(out, "installed %s/%s (%s) to %s\n", reg.Name, entry.Name, entry.TrustTier().String(), installPath)
	return nil
}

// resolveGateDecision maps a GateBlock to proceed/error/exit semantics.
// Returns (proceed, err):
//   - proceed==true, err==nil: caller continues to install side-effect.
//   - proceed==false, err==nil: a G-18 non-interactive exit has already
//     fired via moatSyncExit, or the user answered No at an interactive
//     prompt. Caller returns nil.
//   - err != nil: structured error to surface via cobra (exit 1).
func resolveGateDecision(
	errW io.Writer,
	registryURL string,
	entry *moat.ContentEntry,
	session *moat.Session,
	gate installer.GateBlock,
) (bool, error) {
	switch gate.Decision {
	case installer.MOATGateProceed:
		return true, nil

	case installer.MOATGateHardBlock:
		rev := gate.Revocation
		reason := ""
		if rev != nil {
			reason = rev.Reason
		}
		return false, output.NewStructuredErrorDetail(
			output.ErrMoatRevocationBlock,
			fmt.Sprintf("registry-source revocation refuses install of %q", entry.Name),
			"Registry-source revocations are permanent (ADR 0007 G-15). Contact the publisher or choose an alternative item.",
			"reason="+reason,
		)

	case installer.MOATGatePublisherWarn:
		rev := gate.Revocation
		reason := ""
		if rev != nil {
			reason = rev.Reason
		}
		fmt.Fprintf(errW, "\nPublisher-source revocation for %q: %s\n", entry.Name, reason)
		if !moatInstallInteractiveFn() {
			fmt.Fprintf(errW, "syllago: %s\n", moat.FailurePublisherRevocation.Message())
			moatSyncExit(moat.ExitMoatPublisherRevocation)
			return false, nil
		}
		if moatInstallPromptFn(errW, "Proceed anyway? [Y/n]: ") {
			installer.MarkPublisherConfirmed(session, registryURL, entry.ContentHash)
			return true, nil
		}
		fmt.Fprintln(errW, "install refused by operator")
		return false, nil

	case installer.MOATGatePrivatePrompt:
		fmt.Fprintf(errW, "\n%q is declared as coming from a private repository.\n", entry.Name)
		if !moatInstallInteractiveFn() {
			// Per moat_gate.go docstring: private-prompt's acceptance
			// shape matches TOFU semantically, so we reuse the TOFU
			// exit + message rather than inventing a new code.
			fmt.Fprintf(errW, "syllago: %s (private content requires operator acknowledgement)\n", moat.FailureTOFUAcceptance.Message())
			moatSyncExit(moat.ExitMoatTOFUAcceptance)
			return false, nil
		}
		if moatInstallPromptFn(errW, "Install from private source? [Y/n]: ") {
			installer.MarkPrivateConfirmed(session, registryURL, entry.ContentHash)
			return true, nil
		}
		fmt.Fprintln(errW, "install refused by operator")
		return false, nil

	case installer.MOATGateTierBelowPolicy:
		return false, output.NewStructuredErrorDetail(
			output.ErrMoatTierBelowPolicy,
			fmt.Sprintf("trust tier of %q (%s) is below configured minimum (%s)",
				entry.Name, gate.ObservedTier.String(), gate.MinTier.String()),
			"Raise the item's trust tier (ask the publisher to sign / dual-attest), or lower the install-gate minimum.",
			fmt.Sprintf("observed=%s min=%s", gate.ObservedTier.String(), gate.MinTier.String()),
		)
	}
	// Unreachable: MOATGateDecision is a closed enum. Defensive default.
	return false, fmt.Errorf("install_moat: unhandled gate decision %v", gate.Decision)
}

// printRegistryItemSummaryWithGate renders the resolved ContentEntry and
// gate decision as a short human-readable block. Output is intentionally
// dense (one line per observation) so scripts wrapping `--dry-run` can
// grep by key=value. Gate decision is surfaced so a dry-run trace previews
// exactly which branch a non-dry-run install would take.
func printRegistryItemSummaryWithGate(w io.Writer, registryName string, entry *moat.ContentEntry, revsAdded int, staleness moat.StalenessStatus, decision installer.MOATGateDecision) {
	if output.JSON || output.Quiet {
		return
	}
	fmt.Fprintf(w, "[dry-run] resolved %s/%s from MOAT manifest:\n", registryName, entry.Name)
	fmt.Fprintf(w, "  type=%s trust=%s content_hash=%s\n",
		entry.Type, entry.TrustTier().String(), shortHash(entry.ContentHash))
	if entry.SourceURI != "" {
		fmt.Fprintf(w, "  source_uri=%s\n", entry.SourceURI)
	}
	if entry.PrivateRepo {
		fmt.Fprintf(w, "  private_repo=true (will prompt under interactive install)\n")
	}
	fmt.Fprintf(w, "  revocations_added=%d staleness=%s gate=%s\n", revsAdded, staleness.String(), decision.String())
	fmt.Fprintf(w, "  next: file-fetch + RecordInstall land in the source-fetcher bead\n")
}

// shortHash trims a full "sha256:abc123..." down to the algo + first 12
// hex chars for compact display. Empty or malformed input is returned
// verbatim — the caller has already validated the hash via manifest parse.
func shortHash(h string) string {
	if h == "" {
		return h
	}
	if i := strings.Index(h, ":"); i >= 0 && len(h) >= i+13 {
		return h[:i+13] + "…"
	}
	return h
}
