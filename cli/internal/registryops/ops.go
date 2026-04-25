// Package registryops — registry orchestration shared by CLI and TUI.
//
// This package is the home for cross-surface registry orchestrators (sync,
// add, remove). The TTY-owning callers (cmd/syllago and internal/tui) are
// pure presentation: they translate orchestrator outcomes into stderr text
// + exit codes (CLI) or tea.Msg variants (TUI). All persistence and
// verification logic lives here so the two surfaces cannot drift again.
//
// Lives in a sibling package rather than internal/registry/ because moat
// already imports registry (load_scan.go), and the orchestrator must import
// moat — putting it in registry would create an import cycle.
//
// See ADR 0007 (MOAT) for the persistence contract; this package is the
// single implementer of it.

package registryops

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
)

// SyncOpts controls a single MOAT-registry sync.
type SyncOpts struct {
	// AcceptTOFU is the operator's pre-approved consent to pin a wire-observed
	// signing profile when the registry has none on record. False means "fail
	// at the TOFU gate"; true means "pin and proceed". The orchestrator never
	// prompts — both surfaces gather consent in their own way (CLI: --yes
	// flag; TUI: modal) and pass the answer in.
	AcceptTOFU bool

	// LockfileRoot is the directory whose .syllago/moat-lockfile.json the
	// orchestrator should load and save. Caller-supplied because this is
	// project-scoped state per ADR 0007 §Lockfile and the orchestrator must
	// not assume cwd or pick its own root.
	LockfileRoot string

	// CacheDir is the global syllago directory under which the manifest cache
	// is written (<CacheDir>/moat/registries/<name>/). When empty the
	// orchestrator resolves config.GlobalDirPath() itself.
	CacheDir string

	// Now is the clock used for staleness classification and lockfile
	// fetched_at. Tests pin this; real callers pass time.Now().
	Now time.Time
}

// SyncOutcome reports what happened after one Sync call. Exactly one of the
// gate flags (GateProfileChanged / GateTOFUNeeded) may be true; both false
// means the happy path executed and persistence occurred.
type SyncOutcome struct {
	// MoatResult is the underlying moat.Sync output. Exposed because both
	// surfaces read fields like NotModified, RevocationsAdded, ETag, Staleness.
	MoatResult moat.SyncResult

	// GateProfileChanged is true when an existing pinned profile no longer
	// matches the wire profile. Persistence did NOT happen. The user must
	// remove + re-add the registry to re-establish consent (G-18 row 2).
	GateProfileChanged bool

	// GateTOFUNeeded is true when the registry has no pinned profile and
	// AcceptTOFU was false. Persistence did NOT happen. Surfaces handle:
	//   - CLI: exit 10 + FailureTOFUAcceptance message
	//   - TUI: TOFU modal, then re-call with AcceptTOFU=true
	GateTOFUNeeded bool

	// Persisted is true when the orchestrator successfully wrote:
	//   - lockfile (always when no gate tripped)
	//   - cfg.Registries[i] (SigningProfile on TOFU-accept; ManifestETag,
	//     LastFetchedAt always)
	//   - manifest cache (when MoatResult had fresh bytes — skipped on 304)
	//
	// On a gated outcome, Persisted is false and the caller must NOT advance
	// trust state.
	Persisted bool
}

// trustedRootStaleError is returned when the bundled Sigstore trusted root is
// expired/missing/corrupt. Callers errors.As() this to surface a structured
// "run syllago update" error with the appropriate code.
type trustedRootStaleError struct {
	Status moat.TrustedRootStatus
}

func (e *trustedRootStaleError) Error() string {
	return fmt.Sprintf("bundled trusted root unusable: %s", e.Status.String())
}

// TrustedRootStatus reports the underlying moat status when this error
// is unwrapped — useful for surface-specific error wrapping.
func (e *trustedRootStaleError) TrustedRootStatus() moat.TrustedRootStatus {
	return e.Status
}

// IsTrustedRootStale reports whether err is the orchestrator's
// trusted-root-stale sentinel (and returns the underlying status).
func IsTrustedRootStale(err error) (moat.TrustedRootStatus, bool) {
	var tre *trustedRootStaleError
	if errors.As(err, &tre) {
		return tre.Status, true
	}
	return 0, false
}

// SyncOneFn is the indirection point for tests. Production calls moat.Sync.
// Mirrors the moatSyncFn pattern that previously lived in registry_sync_moat.go.
var SyncOneFn = moat.Sync

// SyncOne runs the full MOAT sync pipeline for the registry named `name`. It
// loads the global config, picks the registry, validates trusted-root state,
// loads the lockfile, calls moat.Sync, and — on the happy path only —
// persists trust state into all three sinks (config, lockfile, manifest cache).
//
// Surface-neutral by design: the function returns a typed outcome; callers
// translate it into their own error/message shape.
func SyncOne(ctx context.Context, name string, opts SyncOpts) (SyncOutcome, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	// Trusted-root is environmental state — checking it before LoadGlobal
	// means a stale bundle surfaces with the right "run syllago update" error
	// even on first run before any registries are configured.
	rootInfo := moat.BundledTrustedRoot(now)
	switch rootInfo.Status {
	case moat.TrustedRootStatusExpired,
		moat.TrustedRootStatusMissing,
		moat.TrustedRootStatusCorrupt:
		return SyncOutcome{}, &trustedRootStaleError{Status: rootInfo.Status}
	}

	cfg, err := config.LoadGlobal()
	if err != nil {
		return SyncOutcome{}, fmt.Errorf("load global config: %w", err)
	}
	var reg *config.Registry
	for i := range cfg.Registries {
		if cfg.Registries[i].Name == name {
			reg = &cfg.Registries[i]
			break
		}
	}
	if reg == nil {
		return SyncOutcome{}, fmt.Errorf("registry %q not found in config", name)
	}
	if !reg.IsMOAT() {
		return SyncOutcome{}, fmt.Errorf("registry %q is not MOAT-typed", name)
	}

	cacheDir := opts.CacheDir
	if cacheDir == "" {
		cacheDir, err = config.GlobalDirPath()
		if err != nil {
			return SyncOutcome{}, fmt.Errorf("resolve cache dir: %w", err)
		}
	}

	lockfilePath := moat.LockfilePath(opts.LockfileRoot)
	lf, err := moat.LoadLockfile(lockfilePath)
	if err != nil {
		return SyncOutcome{}, fmt.Errorf("load lockfile: %w", err)
	}

	res, err := SyncOneFn(ctx, reg, lf, rootInfo.Bytes, nil, now)
	if err != nil {
		return SyncOutcome{MoatResult: res}, err
	}

	out := SyncOutcome{MoatResult: res}

	// Gate detection: ProfileChanged is non-recoverable from this surface;
	// TOFU is interactive. Both block persistence — the caller MUST NOT
	// silently advance trust state. Stale falls through to persistence (the
	// fresh manifest is still saved; the catalog enricher will downgrade
	// items to Unsigned because the staleness clock is past expiry).
	if res.ProfileChanged {
		out.GateProfileChanged = true
		return out, nil
	}
	if res.IsTOFU && !opts.AcceptTOFU {
		out.GateTOFUNeeded = true
		return out, nil
	}

	// Happy / TOFU-accepted path: persist trust state. Order matters —
	// config first so a lockfile-save failure leaves a recoverable state
	// (re-running sync detects the existing pinned profile and skips TOFU).
	if res.IsTOFU {
		profile := res.IncomingProfile
		reg.SigningProfile = &profile
	}
	reg.ManifestETag = res.ETag
	fetchedAt := res.FetchedAt
	reg.LastFetchedAt = &fetchedAt

	if err := config.SaveGlobal(cfg); err != nil {
		return out, fmt.Errorf("save config: %w", err)
	}
	if err := lf.Save(lockfilePath); err != nil {
		return out, fmt.Errorf("save lockfile: %w", err)
	}

	// Manifest cache feeds EnrichFromMOATManifests at scan time; without it,
	// a successful sync produces no observable trust state. Skipped on 304
	// (cache is already current) and when bytes are absent (stub-test path).
	if !res.NotModified && len(res.ManifestBytes) > 0 && len(res.BundleBytes) > 0 {
		if err := moat.WriteManifestCache(cacheDir, reg.Name, res.ManifestBytes, res.BundleBytes); err != nil {
			return out, fmt.Errorf("write manifest cache: %w", err)
		}
	}

	out.Persisted = true
	return out, nil
}
