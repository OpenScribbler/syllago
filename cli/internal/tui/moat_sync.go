package tui

// MOAT-aware registry sync for the TUI. Mirrors `syllago registry sync` so
// pressing Sync / clicking the gallery's sync action runs the same
// fetch+verify+cache+persist flow the CLI does — without that, MOAT
// registries only ever show Trust: Unknown because the cache that
// EnrichFromMOATManifests reads is never populated.
//
// Layering: this file imports moat + config directly. The TUI is normally a
// presentation layer (.claude/rules/tui-elm.md rule #8), but registry sync
// orchestration is the same shape on both surfaces and duplicating the
// dispatcher in cmd/syllago would split the persistence contract across two
// places. Calling moat + config from inside a tea.Cmd keeps the I/O off the
// event loop, which is what the rule actually guards against.

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
)

// moatSyncDoneMsg reports the outcome of a MOAT-aware sync. Exactly one of
// the gating booleans (TOFU / ProfileChanged / Stale) may be true; the rest
// of the fields are populated only on the happy path.
type moatSyncDoneMsg struct {
	name string

	// err is set on transport, parse, or unexpected verification failures.
	// Gating outcomes (TOFU, ProfileChanged, Stale) are NOT errors — they
	// are deliberate flows the user resolves interactively.
	err error

	// requiresTOFU is true when the registry has no pinned signing profile
	// and the sync just observed a wire profile that needs human approval.
	// Non-nil incomingProfile + manifestURL accompany this so the modal
	// can show what the user is being asked to trust.
	requiresTOFU    bool
	incomingProfile config.SigningProfile
	manifestURL     string

	// profileChanged is true when an existing pinned profile no longer
	// matches the wire profile. The user must remove + re-add the registry
	// from a fresh consent step (G-18 row 2). Surfaced as an error toast.
	profileChanged bool

	// stale is true when the manifest has aged past its 72h window and is
	// not safe to extend trust from. Surfaced as a warning toast.
	stale bool
}

// moatSyncFn is the indirection point so tests can stub the end-to-end
// orchestrator without standing up an httptest + sigstore bundle pair.
// Production calls runMOATSync.
var moatSyncFnTUI func(ctx context.Context, name, projectRoot string, acceptTOFU bool) tea.Msg = runMOATSync

// doMOATSyncCmd builds a tea.Cmd that runs the full MOAT sync pipeline for
// `name`. acceptTOFU=true means the user already approved a TOFU prompt and
// the orchestrator should pin the wire profile on success; false means
// fail-fast on TOFU and surface the modal trigger.
func (a App) doMOATSyncCmd(name string, acceptTOFU bool) tea.Cmd {
	projectRoot := a.projectRoot
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		return moatSyncFnTUI(ctx, name, projectRoot, acceptTOFU)
	}
}

// runMOATSync performs the sync end-to-end and shapes the result into a
// moatSyncDoneMsg. The function is exported (lowercase) only to the package
// so tests can stub it via moatSyncFnTUI.
//
// projectRoot is the root the lockfile is co-located with (matches the CLI
// dispatcher's `root` arg from `findContentRepoRoot`). cacheDir is always
// the global syllago dir — manifest cache is shared across projects.
//
// Persistence happens here, not in handleSync, because the sync depends on
// values (pinned profile, ETag, fetched_at) that must be written together.
// Splitting the persistence across the goroutine and the message handler
// would re-introduce the kind of partial-success bug the registry-remove
// path was just hardened against.
func runMOATSync(ctx context.Context, name, projectRoot string, acceptTOFU bool) tea.Msg {
	cfg, err := config.LoadGlobal()
	if err != nil {
		return moatSyncDoneMsg{name: name, err: fmt.Errorf("load global config: %w", err)}
	}
	var reg *config.Registry
	for i := range cfg.Registries {
		if cfg.Registries[i].Name == name {
			reg = &cfg.Registries[i]
			break
		}
	}
	if reg == nil {
		return moatSyncDoneMsg{name: name, err: fmt.Errorf("registry %q not found in global config", name)}
	}
	if !reg.IsMOAT() {
		return moatSyncDoneMsg{name: name, err: fmt.Errorf("registry %q is not MOAT-typed", name)}
	}

	now := time.Now()
	rootInfo := moat.BundledTrustedRoot(now)
	switch rootInfo.Status {
	case moat.TrustedRootStatusExpired, moat.TrustedRootStatusMissing, moat.TrustedRootStatusCorrupt:
		return moatSyncDoneMsg{name: name, err: fmt.Errorf("bundled trusted root unusable (%s); run `syllago update`", rootInfo.Status.String())}
	}

	cacheDir, err := config.GlobalDirPath()
	if err != nil {
		return moatSyncDoneMsg{name: name, err: fmt.Errorf("resolve cache dir: %w", err)}
	}

	lockfilePath := moat.LockfilePath(projectRoot)
	lf, err := moat.LoadLockfile(lockfilePath)
	if err != nil {
		return moatSyncDoneMsg{name: name, err: fmt.Errorf("load lockfile: %w", err)}
	}

	res, err := moat.Sync(ctx, reg, lf, rootInfo.Bytes, nil, now)
	if err != nil {
		return moatSyncDoneMsg{name: name, err: err}
	}

	// Map gate flags. Precedence matches the CLI dispatcher: ProfileChanged
	// is non-recoverable from this surface; TOFU is interactive; Stale is a
	// warning that does not block persistence (the catalog will still
	// downgrade trust to Unsigned per attachRegistryTrust).
	if res.ProfileChanged {
		return moatSyncDoneMsg{name: name, profileChanged: true, manifestURL: res.ManifestURL, incomingProfile: res.IncomingProfile}
	}
	if res.IsTOFU && !acceptTOFU {
		return moatSyncDoneMsg{name: name, requiresTOFU: true, manifestURL: res.ManifestURL, incomingProfile: res.IncomingProfile}
	}
	if res.Staleness == moat.StalenessExpired {
		// Persist the fresh manifest (the catalog enricher will mark its
		// items Unsigned because the staleness clock is past expiry); the
		// stale flag is informational so the user knows why trust badges
		// downgraded.
		// Fall through to persistence.
	}

	// Happy / TOFU-accepted path: persist trust state.
	if res.IsTOFU {
		profile := res.IncomingProfile
		reg.SigningProfile = &profile
	}
	reg.ManifestETag = res.ETag
	fetchedAt := res.FetchedAt
	reg.LastFetchedAt = &fetchedAt

	if err := config.SaveGlobal(cfg); err != nil {
		return moatSyncDoneMsg{name: name, err: fmt.Errorf("save config: %w", err)}
	}
	if err := lf.Save(lockfilePath); err != nil {
		return moatSyncDoneMsg{name: name, err: fmt.Errorf("save lockfile: %w", err)}
	}

	if !res.NotModified && len(res.ManifestBytes) > 0 && len(res.BundleBytes) > 0 {
		if err := moat.WriteManifestCache(cacheDir, reg.Name, res.ManifestBytes, res.BundleBytes); err != nil {
			return moatSyncDoneMsg{name: name, err: fmt.Errorf("write manifest cache: %w", err)}
		}
	}

	return moatSyncDoneMsg{name: name, stale: res.Staleness == moat.StalenessExpired}
}
