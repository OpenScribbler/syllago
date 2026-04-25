package registryops

// Shared registry-add orchestrator (bead syllago-nb5ed Phase B).
//
// Both the CLI's `syllago registry add` (cmd/syllago/registry_cmd.go) and the
// TUI's add wizard (internal/tui/actions.go:doRegistryAddCmd) used to fork
// this logic, which produced two notable bugs: the TUI bypassed name +
// allowedRegistries validation (S3 / syllago-mpold) and the CLI didn't auto-
// sync after add (S4 / syllago-43qoo). Centralising here closes both for
// free — both surfaces now run the same gates and the same persistence.
//
// Surface-specific responsibilities (e.g., the security banner, the alias-
// expansion message, the sandbox allowlist prompt, toasts) stay with the
// caller; this file only owns "what gates run, what gets cloned/scanned,
// what gets saved."

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/analyzer"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

// NowFn is the clock seam for the orchestrator. Tests pin it; production
// uses time.Now. Exported so cross-surface tests can swap it consistently.
var NowFn = func() time.Time { return time.Now().UTC() }

func nowFunc() time.Time { return NowFn() }

// Sentinel errors for AddRegistry. Callers wrap their surface-specific
// detail (registry name, URL, structured-error code) around these via
// errors.Is dispatch.
var (
	// ErrAddInvalidName: registry name is invalid per catalog.IsValidRegistryName.
	ErrAddInvalidName = errors.New("invalid registry name")

	// ErrAddDuplicate: a registry with the same name already exists in config.
	ErrAddDuplicate = errors.New("registry already exists")

	// ErrAddNotAllowed: URL is not in the allowedRegistries policy list.
	ErrAddNotAllowed = errors.New("registry not in allowedRegistries")

	// ErrAddNotSyllago: the cloned repo is not a syllago registry. The clone
	// has been removed before the orchestrator returns this.
	ErrAddNotSyllago = errors.New("not a syllago registry")

	// ErrAddCloneFailed: git clone failed. The original error is wrapped.
	ErrAddCloneFailed = errors.New("clone failed")

	// ErrAddSaveFailed: writing config.SaveGlobal failed after a successful
	// clone. The clone has been rolled back before the orchestrator returns.
	ErrAddSaveFailed = errors.New("save config failed")
)

// CloneFn is the test seam for cloning. Production calls registry.Clone.
// The TUI and CLI used to each have their own cloneFn; now there is one.
var CloneFn = registry.Clone

// AddOpts controls a single registry add. URL is expected to be
// alias-expanded by the caller — the orchestrator does not look at aliases
// because expansion is presentation (it prints "Expanding alias..." in the
// CLI but is silent in the TUI).
type AddOpts struct {
	// URL is the resolved git URL (post alias expansion).
	URL string

	// Name is the registry's identity. When empty the orchestrator derives
	// it from URL. Mirrors the CLI's --name flag.
	Name string

	// Ref is the branch/tag/commit to checkout. Empty means default branch.
	Ref string

	// IsLocal=true skips the clone step entirely. Used by `syllago registry
	// create --new` flows where the registry is already on local disk.
	IsLocal bool

	// SigningProfile is non-nil when the caller resolved a signing profile
	// from CLI flags or a bundled allowlist match. The orchestrator pins
	// it and flips Type=moat. TUI callers pass nil (no signing flags) and
	// rely on registry.yaml self-declaration for MOAT detection.
	SigningProfile *config.SigningProfile

	// SigningManifestURI accompanies SigningProfile when sourced from the
	// allowlist. Empty for flag-sourced profiles.
	SigningManifestURI string
}

// AddOutcome reports what the orchestrator did so callers can render
// surface-appropriate messages without re-deriving state.
type AddOutcome struct {
	// Registry is the persisted entry as appended to cfg.Registries. Read
	// IsMOAT(), Type, and SigningProfile here for follow-up logic
	// (e.g., the TUI auto-syncs MOAT registries after add).
	Registry config.Registry

	// Cloned is true when a fresh clone happened (false when IsLocal=true).
	Cloned bool

	// Visibility is the resolved visibility ("public" / "private"); the
	// CLI prints a one-liner from this.
	Visibility string

	// NoContentFound is true when the cloned repo had no recognized syllago
	// or provider-native content. The orchestrator still saves the registry
	// (the user may know the repo is empty — e.g., a new scaffold) but the
	// caller can render a "Warning: no recognized content" line.
	NoContentFound bool

	// SelfDeclaredMOAT is true when the orchestrator upgraded the registry
	// to MOAT via registry.yaml's manifest_uri field (no incoming
	// SigningProfile from the caller). Callers print "Run sync --yes to pin"
	// since first sync hits TOFU.
	SelfDeclaredMOAT bool

	// CloneDir is the on-disk path of the clone (or the local path when
	// IsLocal=true). Empty for IsLocal=true paths that don't resolve a dir.
	// Surfaces use this for follow-up scans.
	CloneDir string
}

// AddRegistry orchestrates a single registry add. Validation gates run in
// fast-fail order before any network I/O so a duplicate name or
// policy-blocked URL never produces an orphaned clone.
//
// On any post-clone failure (scan rejection, save failure) the clone is
// rolled back via registry.Remove(name) before the error is returned.
func AddRegistry(ctx context.Context, opts AddOpts) (AddOutcome, error) {
	out := AddOutcome{}

	name := opts.Name
	if name == "" {
		name = registry.NameFromURL(opts.URL)
	}
	if !catalog.IsValidRegistryName(name) {
		return out, fmt.Errorf("%w: %q", ErrAddInvalidName, name)
	}

	cfg, err := config.LoadGlobal()
	if err != nil {
		return out, fmt.Errorf("load global config: %w", err)
	}

	for _, r := range cfg.Registries {
		if r.Name == name {
			return out, fmt.Errorf("%w: %q", ErrAddDuplicate, name)
		}
	}

	if !cfg.IsRegistryAllowed(opts.URL) {
		return out, fmt.Errorf("%w: %q", ErrAddNotAllowed, opts.URL)
	}

	// Clone (unless IsLocal). Failure here does not leave state on disk
	// because we haven't touched config yet.
	if !opts.IsLocal {
		if err := CloneFn(opts.URL, name, opts.Ref); err != nil {
			return out, fmt.Errorf("%w: %w", ErrAddCloneFailed, err)
		}
		out.Cloned = true
	}

	dir, _ := registry.CloneDir(name)
	out.CloneDir = dir

	// Generate a registry.yaml stub via the analyzer when the repo lacks
	// one. This lets the scanner discover content regardless of how the
	// repo is organized — a syllago-format registry that's missing only
	// the manifest still works after add. Skipped for IsLocal because the
	// repo already lives at its source path; we don't write into it.
	if !opts.IsLocal {
		if manifest, _ := registry.LoadManifestFromDir(dir); manifest == nil {
			cfgA := analyzer.DefaultConfig()
			a := analyzer.New(cfgA)
			result, analyzeErr := a.Analyze(dir)
			if analyzeErr == nil && len(result.AllItems()) > 0 {
				_ = analyzer.WriteGeneratedManifest(name, result.AllItems())
			}
		}
	}

	// Reject non-syllago repos that contain provider-native content. The
	// rejection reason is "we can't ingest this as a registry"; the user
	// has tools to add provider-native content to their library directly.
	if !opts.IsLocal {
		scanResult := catalog.ScanNativeContent(dir)
		if !scanResult.HasSyllagoStructure && len(scanResult.Providers) > 0 {
			manifest, _ := registry.LoadManifestFromDir(dir)
			if manifest == nil || len(manifest.Items) == 0 {
				_ = os.RemoveAll(dir)
				return out, fmt.Errorf("%w: %q", ErrAddNotSyllago, name)
			}
		} else if !scanResult.HasSyllagoStructure && len(scanResult.Providers) == 0 {
			out.NoContentFound = true
		}
	}

	// Probe visibility from the hosting platform API. Manifest-declared
	// visibility wins when stricter (private overrides public-probe).
	probeResult, _ := registry.ProbeVisibility(opts.URL)
	manifestDecl := ""
	if !opts.IsLocal {
		if manifest, _ := registry.LoadManifestFromDir(dir); manifest != nil {
			manifestDecl = manifest.Visibility
		}
	}
	visibility := registry.ResolveVisibility(probeResult, manifestDecl)
	out.Visibility = visibility

	now := nowFunc()
	newRegistry := config.Registry{
		Name:                name,
		URL:                 opts.URL,
		Ref:                 opts.Ref,
		Visibility:          visibility,
		VisibilityCheckedAt: &now,
	}
	if opts.SigningProfile != nil {
		newRegistry.Type = config.RegistryTypeMOAT
		newRegistry.SigningProfile = opts.SigningProfile
		newRegistry.ManifestURI = opts.SigningManifestURI
	} else if entry, ok := moat.LookupSigningIdentity(opts.URL); ok && entry != nil && entry.Profile != nil {
		// Bundled allowlist match: pin the trusted identity at add time so
		// first sync already has a fixed profile — no TOFU prompt. The CLI's
		// resolveSigningProfile usually beats us here, but the TUI hits this
		// branch and gets allowlist auto-detection for free.
		newRegistry.Type = config.RegistryTypeMOAT
		newRegistry.SigningProfile = entry.Profile
		newRegistry.ManifestURI = entry.ManifestURI
	} else if !opts.IsLocal {
		// MOAT self-declaration via registry.yaml: TOFU on first sync.
		if selfDecl, _ := registry.LoadManifestFromDir(dir); selfDecl != nil && selfDecl.ManifestURI != "" {
			newRegistry.Type = config.RegistryTypeMOAT
			newRegistry.ManifestURI = selfDecl.ManifestURI
			out.SelfDeclaredMOAT = true
		}
	}

	cfg.Registries = append(cfg.Registries, newRegistry)
	if err := config.SaveGlobal(cfg); err != nil {
		// Rollback the clone so we don't leave an orphan that points at no
		// config entry. The caller can't do this — it doesn't know the
		// clone path.
		if out.Cloned {
			_ = os.RemoveAll(dir)
		}
		return out, fmt.Errorf("%w: %w", ErrAddSaveFailed, err)
	}

	out.Registry = newRegistry
	return out, nil
}
