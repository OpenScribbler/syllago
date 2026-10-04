package main

// MOAT registry-sourced install: `syllago install <registry>/<item>`.
//
// moatinstall.Operation does the work: it syncs the registry through the
// same registryops.SyncOne that `syllago registry sync` uses, checks the
// item against the trust gate, fetches and verifies it, stages it into the
// Library, and installs it. This file parses the syntax, answers the
// operation's decisions at a prompt or with a G-18 non-interactive exit
// (10 TOFU or private source / 11 profile change / 12 publisher
// revocation / 13 stale), and prints the result.

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/lifecycle"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/moatinstall"
	"github.com/OpenScribbler/syllago/cli/internal/output"
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

// moatInstallMinTier is the policy floor the install gate enforces. The
// floor is an operator choice; until a CLI/config surface
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

// parseRegistryItemSyntax splits a positional "registry/item" argument
// on the LAST "/". Per MOAT spec §"Repository Layout", item names are a
// single path segment under the canonical category dir (e.g.
// "skills/split-rules-llm/" → item is "split-rules-llm"), so splitting on
// the last slash always yields the correct (registry, item) tuple — even
// when the registry name itself contains slashes (e.g. GitHub-prefixed
// names like "OpenScribbler/syllago-meta-registry").
//
// Returns (registryName, itemName, true) when arg contains at least one
// "/", and both halves are non-empty. Returns ok=false otherwise so the
// caller can fall through to the existing library-install path.
//
// A blank registry ("/foo") or blank item ("foo/") surfaces as ok=false
// — a user mistake that produces "no item named ..." rather than a silent
// fallback.
func parseRegistryItemSyntax(arg string) (string, string, bool) {
	idx := strings.LastIndex(arg, "/")
	if idx < 0 {
		return "", "", false
	}
	reg := arg[:idx]
	item := arg[idx+1:]
	if reg == "" || item == "" {
		return "", "", false
	}
	return reg, item, true
}

// runInstallFromRegistry installs one MOAT registry item through
// moatinstall.Operation and turns what it reports into the CLI's output,
// prompts, and exits. A G-18 non-interactive failure calls
// moatSyncExit(code) and returns nil, because cobra cannot express exit
// codes beyond 1.
//
// --yes is intentionally not exposed on install. Accepting TOFU at install
// time would let a pipeline silently pin a new registry by scheduling one
// job; the spec's row-1 MUST-exit is meant to prevent exactly that
// shortcut. Operators run `syllago registry add --yes` interactively first.
func runInstallFromRegistry(ctx context.Context, out, errW io.Writer, req moatinstall.Request, now time.Time) error {
	if !req.DryRun && len(req.Targets) == 0 {
		return output.NewStructuredError(
			output.ErrInputMissing,
			"install destination not specified",
			"Pass --to <provider> to choose where the registry item is installed.",
		)
	}
	req.Frozen = isFrozenFromContext(ctx)
	// One session for every attempt, so a warning confirmed once stays
	// confirmed for this `syllago install` call and no longer.
	req.Session = moat.NewSession()
	op := moatinstall.Operation{MinTier: moatInstallMinTier, Now: func() time.Time { return now }}
	for {
		res, err := op.Install(ctx, req)
		// Telemetry on both proceed and refuse paths, so ops see which tiers
		// users land on and how often gates fire.
		if len(res.Items) > 0 && res.Items[0].Entry != nil {
			telemetry.Enrich("moat_tier", res.Items[0].Entry.TrustTier().String())
			telemetry.Enrich("moat_gated", res.Items[0].Gate.Decision.String())
		}
		var decision *lifecycle.DecisionRequired
		if errors.As(err, &decision) {
			retry, derr := decideRegistryInstall(errW, &req, decision)
			if retry {
				continue
			}
			return derr
		}
		if err != nil {
			return registryInstallError(errW, req.Registry, err)
		}
		return reportRegistryInstall(out, errW, req, res)
	}
}

// decideRegistryInstall answers the choice Install asked for, at a prompt
// when one is possible. It returns retry when req now carries the answer;
// otherwise the install stops, and a headless refusal has already exited.
func decideRegistryInstall(errW io.Writer, req *moatinstall.Request, decision *lifecycle.DecisionRequired) (bool, error) {
	switch decision.Kind {
	case lifecycle.TrustOnFirstUse:
		fmt.Fprintf(errW, "syllago: %s\n", moat.FailureTOFUAcceptance.Message())
		moatSyncExit(moat.ExitMoatTOFUAcceptance)
		return false, nil

	case lifecycle.PublisherWarn:
		if req.Decisions.PublisherWarn == nil {
			req.Decisions.PublisherWarn = map[string]bool{}
		}
		for _, p := range decision.Context.([]moatinstall.GatePrompt) {
			reason := ""
			if p.Gate.Revocation != nil {
				reason = moat.SanitizeForDisplay(p.Gate.Revocation.Reason)
			}
			fmt.Fprintf(errW, "\nPublisher-source revocation for %q: %s\n", p.Entry.Name, reason)
			if !moatInstallInteractiveFn() {
				fmt.Fprintf(errW, "syllago: %s\n", moat.FailurePublisherRevocation.Message())
				moatSyncExit(moat.ExitMoatPublisherRevocation)
				return false, nil
			}
			if !moatInstallPromptFn(errW, "Proceed anyway? [Y/n]: ") {
				fmt.Fprintln(errW, "install refused by operator")
				return false, nil
			}
			req.Decisions.PublisherWarn[p.Entry.ContentHash] = true
		}
		return true, nil

	case lifecycle.PrivateSource:
		if req.Decisions.PrivateSource == nil {
			req.Decisions.PrivateSource = map[string]bool{}
		}
		for _, p := range decision.Context.([]moatinstall.GatePrompt) {
			fmt.Fprintf(errW, "\n%q is declared as coming from a private repository.\n", p.Entry.Name)
			if !moatInstallInteractiveFn() {
				// Private-prompt acceptance matches TOFU semantically, so it
				// reuses the TOFU exit and message rather than a new code.
				fmt.Fprintf(errW, "syllago: %s (private content requires operator acknowledgement)\n", moat.FailureTOFUAcceptance.Message())
				moatSyncExit(moat.ExitMoatTOFUAcceptance)
				return false, nil
			}
			if !moatInstallPromptFn(errW, "Install from private source? [Y/n]: ") {
				fmt.Fprintln(errW, "install refused by operator")
				return false, nil
			}
			req.Decisions.PrivateSource[p.Entry.ContentHash] = true
		}
		return true, nil

	case lifecycle.OverwritePinned:
		pinned := decision.Context.([]lifecycle.PinnedDestination)
		return false, output.NewStructuredError(
			output.ErrInstallConflict,
			fmt.Sprintf("could not install %s/%s: item is pinned", req.Registry, pinned[0].Name),
			fmt.Sprintf("syllago unpin %s", pinned[0].Name),
		)
	}
	return false, fmt.Errorf("install_moat: unhandled decision %s", decision.Kind)
}

// registryInstallError maps a failure of the whole install to its exit or
// structured error.
func registryInstallError(errW io.Writer, regName string, err error) error {
	switch {
	case errors.Is(err, moatinstall.ErrManifestExpired):
		fmt.Fprintf(errW, "syllago: %s\n", moat.FailureManifestStale.Message())
		moatSyncExit(moat.ExitMoatManifestStale)
		return nil
	case errors.Is(err, moatinstall.ErrProfileChanged):
		fmt.Fprintf(errW, "syllago: %s\n", moat.FailureSigningProfileChange.Message())
		moatSyncExit(moat.ExitMoatSigningProfileChange)
		return nil
	}
	var syncErr *moatinstall.SyncError
	if errors.As(err, &syncErr) {
		var ve *moat.VerifyError
		if errors.As(syncErr.Err, &ve) {
			return classifyVerifyError(regName, syncErr.Err)
		}
		return output.NewStructuredErrorDetail(
			output.ErrMoatInvalid,
			fmt.Sprintf("sync failed while installing from registry %q", regName),
			"Run `syllago registry sync` to surface the error in isolation, then retry install.",
			syncErr.Err.Error(),
		)
	}
	return err
}

// reportRegistryInstall prints what Install did with the one requested item.
func reportRegistryInstall(out, errW io.Writer, req moatinstall.Request, res moatinstall.Result) error {
	ir := res.Items[0]
	if errors.Is(ir.Err, moatinstall.ErrDeclined) {
		fmt.Fprintln(errW, "install refused by operator")
		return nil
	}
	if req.DryRun {
		if ir.Err != nil {
			return ir.Err
		}
		printRegistryItemSummaryWithGate(out, req.Registry, ir.Entry, res.Sync.MoatResult.RevocationsAdded, res.Staleness, ir.Gate.Decision)
		return nil
	}
	// Report what did install before any failure, since one target can
	// fail after another succeeded.
	printLifecycleWarnings(errW, res.Stage)
	printInstallNotices(errW, ir.Install.Notices)
	printLifecycleWarnings(errW, ir.Install)
	for _, t := range ir.Unsupported {
		fmt.Fprintf(errW, "skipped %s: it does not support %s\n", t.Provider.Name, ir.Library.Type.Label())
	}
	for _, step := range ir.Install.Completed {
		fmt.Fprintf(out, "installed %s/%s (%s) to %s\n", req.Registry, ir.Entry.Name, ir.Entry.TrustTier().String(), step.Placement.String())
	}
	return ir.Err
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

type frozenContextKeyType struct{}

var frozenContextKey = frozenContextKeyType{}

func ContextWithFrozen(ctx context.Context, frozen bool) context.Context {
	return context.WithValue(ctx, frozenContextKey, frozen)
}

func isFrozenFromContext(ctx context.Context) bool {
	val, _ := ctx.Value(frozenContextKey).(bool)
	return val
}
