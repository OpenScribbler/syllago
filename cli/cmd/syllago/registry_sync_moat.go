package main

// MOAT registry-sync dispatcher (ADR 0007 Phase 2a, bead syllago-gj7ad).
//
// This file is the TTY-owner for `syllago registry sync <name>` when the
// registry is MOAT-backed. It composes the moat.Sync orchestrator with the
// interactive/non-interactive context only the CLI knows about:
//
//   - Maps SyncResult flags (IsTOFU, ProfileChanged, Staleness) to the G-18
//     exit codes (10 / 11 / 13).
//   - Persists trust state on success (reg.SigningProfile on TOFU accept,
//     reg.ManifestETag on every successful fetch, lockfile on every success).
//   - Emits a single human line on happy paths; the stderr actionable message
//     on gated paths follows NonInteractiveFailure.Message() so pipelines can
//     grep by the kebab-case label.
//
// The moat package itself takes no write-access to the config file — that
// isolation is deliberate, and every persistence decision lives here.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// moatSyncFn is a package-level seam so tests can stub the end-to-end sync
// without spinning up an httptest server. Production calls moat.Sync.
var moatSyncFn = moat.Sync

// syncMOATRegistry runs one MOAT sync against `reg` and returns a
// NonInteractiveFailure-shaped exit code. Contract:
//
//	(0, nil)     — happy path (verified, fresh, pinned match or TOFU+yes), or
//	              304 short-circuit. Caller proceeds normally.
//	(code, nil)  — MOAT gate tripped; code is one of ExitMoatTOFUAcceptance
//	              (10), ExitMoatSigningProfileChange (11), ExitMoatManifestStale
//	              (13). Caller must os.Exit(code) — cobra only knows about 0/1.
//	(0, err)     — transport, parse, or cryptographic failure. Caller surfaces
//	              the structured error via cobra's normal error path.
//
// Side effects (only on the (0, nil) return path):
//   - lockfile.registries[manifest_uri].fetched_at is advanced.
//   - lockfile.revoked_hashes is merged additively with registry-source
//     revocations from the fresh manifest.
//   - reg.ManifestETag is set to the ETag returned by the server, so the
//     next Sync can send If-None-Match.
//   - On TOFU+yes, reg.SigningProfile is set to the observed IncomingProfile.
//   - cfg is saved (reg is a pointer into cfg.Registries).
//
// On a gated (code != 0) or errored return, lockfile and cfg are NOT saved.
// The caller's retry (interactive approval, manifest refresh) must re-run
// sync from scratch — partial persistence would silently advance trust state
// without operator acknowledgement.
func syncMOATRegistry(
	ctx context.Context,
	out, errW io.Writer,
	cfg *config.Config,
	reg *config.Registry,
	cfgRoot string,
	now time.Time,
	yes bool,
) (int, error) {
	if reg == nil {
		return 0, errors.New("syncMOATRegistry: registry is nil")
	}
	if !reg.IsMOAT() {
		return 0, fmt.Errorf("syncMOATRegistry: registry %q is not MOAT", reg.Name)
	}

	rootInfo := moat.BundledTrustedRoot(now)
	if rootInfo.Status == moat.TrustedRootStatusExpired ||
		rootInfo.Status == moat.TrustedRootStatusMissing ||
		rootInfo.Status == moat.TrustedRootStatusCorrupt {
		return 0, output.NewStructuredErrorDetail(
			output.ErrMoatTrustedRootStale,
			fmt.Sprintf("bundled trusted root unusable while syncing registry %q", reg.Name),
			"Run `syllago update` to refresh the bundled Sigstore trusted root.",
			rootInfo.Status.String(),
		)
	}

	lockfilePath := moat.LockfilePath(cfgRoot)
	lf, err := moat.LoadLockfile(lockfilePath)
	if err != nil {
		return 0, fmt.Errorf("load lockfile: %w", err)
	}

	res, err := moatSyncFn(ctx, reg, lf, rootInfo.Bytes, nil, now)
	if err != nil {
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

	// Map SyncResult flags to NonInteractiveFailure codes. ProfileChanged
	// always gates, regardless of --yes — re-approval requires removing and
	// re-adding the registry interactively.
	if res.ProfileChanged {
		fmt.Fprintf(errW, "syllago: %s\n", moat.FailureSigningProfileChange.Message())
		return moat.ExitMoatSigningProfileChange, nil
	}
	if res.IsTOFU && !yes {
		fmt.Fprintf(errW, "syllago: %s\n", moat.FailureTOFUAcceptance.Message())
		return moat.ExitMoatTOFUAcceptance, nil
	}
	if res.Staleness == moat.StalenessExpired {
		fmt.Fprintf(errW, "syllago: %s\n", moat.FailureManifestStale.Message())
		return moat.ExitMoatManifestStale, nil
	}

	// Happy path (including TOFU+yes). Persist trust state and lockfile.
	if res.IsTOFU {
		profile := res.IncomingProfile
		reg.SigningProfile = &profile
	}
	reg.ManifestETag = res.ETag
	fetchedAt := res.FetchedAt
	reg.LastFetchedAt = &fetchedAt

	if err := config.Save(cfgRoot, cfg); err != nil {
		return 0, output.NewStructuredErrorDetail(
			output.ErrRegistrySaveFailed,
			fmt.Sprintf("could not persist sync state for registry %q", reg.Name),
			"Check filesystem permissions on .syllago/config.json.",
			err.Error(),
		)
	}
	if err := lf.Save(lockfilePath); err != nil {
		return 0, output.NewStructuredErrorDetail(
			output.ErrMoatInvalid,
			fmt.Sprintf("could not persist moat lockfile after syncing %q", reg.Name),
			"Check filesystem permissions on .syllago/moat-lockfile.json.",
			err.Error(),
		)
	}

	// One-line human status. The NotModified branch is distinct because the
	// body was not re-verified (nothing to re-verify), so operators reading
	// logs should know they're still trusting the previously-verified bytes.
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
