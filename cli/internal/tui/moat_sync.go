package tui

// MOAT-aware registry sync presentation adapter. Wraps registryops.SyncOne
// (the shared orchestrator) into a tea.Cmd so the gallery's Sync action runs
// the same fetch+verify+cache+persist flow the CLI does. Without that, MOAT
// registries only ever show Trust: Unknown because the cache that
// EnrichFromMOATManifests reads is never populated.
//
// All persistence (config, lockfile, manifest cache) happens inside
// registryops.SyncOne — this file only translates outcomes into tea.Msg
// variants the App.Update loop knows how to route. See bead syllago-nb5ed
// for the orchestrator extraction.

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/registryops"
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

// moatSyncFnTUI is the indirection point so tests can stub the end-to-end
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

// runMOATSync calls the shared orchestrator and shapes the outcome into a
// moatSyncDoneMsg. projectRoot is the lockfile root (matches the CLI
// dispatcher's cfgRoot from findContentRepoRoot); the manifest cache lives
// under config.GlobalDirPath() and the orchestrator resolves it itself.
func runMOATSync(ctx context.Context, name, projectRoot string, acceptTOFU bool) tea.Msg {
	outcome, err := registryops.SyncOne(ctx, name, registryops.SyncOpts{
		AcceptTOFU:   acceptTOFU,
		LockfileRoot: projectRoot,
		Now:          time.Now(),
	})
	if err != nil {
		if status, ok := registryops.IsTrustedRootStale(err); ok {
			return moatSyncDoneMsg{name: name, err: fmt.Errorf("bundled trusted root unusable (%s); run `syllago update`", status.String())}
		}
		return moatSyncDoneMsg{name: name, err: err}
	}

	if outcome.GateProfileChanged {
		return moatSyncDoneMsg{
			name:            name,
			profileChanged:  true,
			manifestURL:     outcome.MoatResult.ManifestURL,
			incomingProfile: outcome.MoatResult.IncomingProfile,
		}
	}
	if outcome.GateTOFUNeeded {
		return moatSyncDoneMsg{
			name:            name,
			requiresTOFU:    true,
			manifestURL:     outcome.MoatResult.ManifestURL,
			incomingProfile: outcome.MoatResult.IncomingProfile,
		}
	}

	return moatSyncDoneMsg{
		name:  name,
		stale: outcome.MoatResult.Staleness == moat.StalenessExpired,
	}
}
