package main

// MOAT registry-sync presentation adapter (ADR 0007 Phase 2a, bead syllago-gj7ad
// for the original; bead syllago-nb5ed for the refactor that moved the
// orchestrator into internal/registryops).
//
// This file is the TTY-owner for `syllago registry sync <name>` when the
// registry is MOAT-backed. It composes registryops.SyncOne — the shared
// orchestrator — with the interactive/non-interactive context only the CLI
// knows about:
//
//   - Maps SyncOutcome flags to the G-18 exit codes (10 / 11 / 13).
//   - Emits a single human stdout line on the happy path; the stderr
//     actionable message on gated paths follows NonInteractiveFailure.Message()
//     so pipelines can grep by the kebab-case label.
//   - Wraps orchestrator errors in CLI-shaped structured errors.
//
// All persistence (config, lockfile, manifest cache) happens inside
// registryops.SyncOne — this file does not touch the on-disk state, exactly
// because it shares that logic with the TUI.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/registryops"
)

// moatSyncFn is the low-level test seam used by install_moat.go (which runs
// its own sync inline rather than going through the orchestrator). The
// `syllago registry sync` path uses registryops.SyncOneFn instead.
//
// Keeping both seams is deliberate: install needs to control the sync ↔
// install ordering itself, while the registry-sync command can delegate the
// whole orchestration. Tests for either path stub their own seam.
var moatSyncFn = moat.Sync

// syncMOATRegistry runs one MOAT sync against `reg` and returns a
// NonInteractiveFailure-shaped exit code.
//
// Returns:
//
//	(0, nil)     — happy path (verified, fresh, pinned match or TOFU+yes), or
//	              304 short-circuit. Caller proceeds normally.
//	(code, nil)  — MOAT gate tripped; code is one of ExitMoatTOFUAcceptance
//	              (10), ExitMoatSigningProfileChange (11), ExitMoatManifestStale
//	              (13). Caller must os.Exit(code) — cobra only knows about 0/1.
//	(0, err)     — transport, parse, or cryptographic failure. Caller surfaces
//	              the structured error via cobra's normal error path.
//
// All persistence happens inside registryops.SyncOne — see that function's
// doc for the side-effect list.
func syncMOATRegistry(
	ctx context.Context,
	out, errW io.Writer,
	cfg *config.Config,
	reg *config.Registry,
	cfgRoot, cacheDir string,
	now time.Time,
	yes bool,
) (int, error) {
	if reg == nil {
		return 0, errors.New("syncMOATRegistry: registry is nil")
	}
	if !reg.IsMOAT() {
		return 0, fmt.Errorf("syncMOATRegistry: registry %q is not MOAT", reg.Name)
	}

	outcome, err := registryops.SyncOne(ctx, reg.Name, registryops.SyncOpts{
		AcceptTOFU:   yes,
		LockfileRoot: cfgRoot,
		CacheDir:     cacheDir,
		Now:          now,
	})
	if err != nil {
		if status, ok := registryops.IsTrustedRootStale(err); ok {
			return 0, output.NewStructuredErrorDetail(
				output.ErrMoatTrustedRootStale,
				fmt.Sprintf("bundled trusted root unusable while syncing registry %q", reg.Name),
				"Run `syllago update` to refresh the bundled Sigstore trusted root.",
				status.String(),
			)
		}
		var ve *moat.VerifyError
		if errors.As(err, &ve) {
			return 0, classifyVerifyError(reg.Name, err)
		}
		return 0, output.NewStructuredErrorDetail(
			output.ErrMoatInvalid,
			fmt.Sprintf("sync failed for registry %q", reg.Name),
			"Run `syllago registry sync` again after checking network connectivity and the registry's manifest URL.",
			err.Error(),
		)
	}

	// Map gate flags to NonInteractiveFailure codes. ProfileChanged always
	// gates regardless of --yes — re-approval requires removing and
	// re-adding the registry interactively.
	if outcome.GateProfileChanged {
		fmt.Fprintf(errW, "syllago: %s\n", moat.FailureSigningProfileChange.Message())
		return moat.ExitMoatSigningProfileChange, nil
	}
	if outcome.GateTOFUNeeded {
		fmt.Fprintf(errW, "syllago: %s\n", moat.FailureTOFUAcceptance.Message())
		return moat.ExitMoatTOFUAcceptance, nil
	}
	if outcome.MoatResult.Staleness == moat.StalenessExpired {
		fmt.Fprintf(errW, "syllago: %s\n", moat.FailureManifestStale.Message())
		return moat.ExitMoatManifestStale, nil
	}

	// One-line human status. The NotModified branch is distinct because the
	// body was not re-verified (nothing to re-verify), so operators reading
	// logs should know they're still trusting the previously-verified bytes.
	res := outcome.MoatResult
	switch {
	case res.NotModified:
		fmt.Fprintf(out, "Synced: %s (not-modified, fetched_at=%s)\n",
			reg.Name, res.FetchedAt.UTC().Format(time.RFC3339))
	case res.IsTOFU:
		fmt.Fprintf(out, "Synced: %s (tofu-accepted, revocations+%d, private=%d)\n",
			reg.Name, res.RevocationsAdded, res.PrivateContentCount)
	default:
		fmt.Fprintf(out, "Synced: %s (verified, revocations+%d, private=%d)\n",
			reg.Name, res.RevocationsAdded, res.PrivateContentCount)
	}

	return 0, nil
}
