package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/gitutil"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
	"github.com/OpenScribbler/syllago/cli/internal/registryops"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

var registryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Manage git-based content registries",
	Long: `Add, remove, list, and sync git repositories as content registries.

Registries are read-only git repos containing shared content (skills, rules,
hooks, MCP configs, etc.). Use "registry sync" to pull updates, and
"registry items" to browse what's available.

To use registry content, browse it in the TUI ("syllago") or install it
directly with "syllago install --to <provider>".`,
	Example: `  syllago registry add https://github.com/team/rules.git
  syllago registry sync
  syllago registry items --type skills`,
}

var registryAddCmd = &cobra.Command{
	Use:   "add <git-url>",
	Short: "Add a git registry",
	Long: `Registers a git repository as a content registry.

By default, only the registry URL and name are saved to config — no network
request is made. Run "registry sync" to fetch content when ready.

Use --sync to clone immediately (original behaviour).`,
	Example: `  # Register a registry (no clone — run 'registry sync' to fetch content)
  syllago registry add https://github.com/team/rules.git

  # Register and clone immediately
  syllago registry add https://github.com/team/rules.git --sync

  # Register with a custom name
  syllago registry add https://github.com/team/rules.git --name team-rules

  # Pin to a specific branch
  syllago registry add https://github.com/team/rules.git --ref main`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		gitURL := args[0]

		// Expand short aliases before any other processing. The orchestrator
		// expects an already-resolved URL — alias expansion is a CLI
		// presentation step ("Expanding alias..." line) that the TUI
		// doesn't replicate.
		if fullURL, wasExpanded := registry.ExpandAlias(gitURL); wasExpanded {
			fmt.Fprintf(output.Writer, "Expanding alias %q → %s\n", gitURL, fullURL)
			gitURL = fullURL
		}

		nameFlag, _ := cmd.Flags().GetString("name")
		refFlag, _ := cmd.Flags().GetString("ref")

		moatFlag, _ := cmd.Flags().GetBool("moat")
		rawSigningFlags := signingFlagSet{
			Identity:          mustStringFlag(cmd, "signing-identity"),
			Issuer:            mustStringFlag(cmd, "signing-issuer"),
			RepositoryID:      mustStringFlag(cmd, "signing-repository-id"),
			RepositoryOwnerID: mustStringFlag(cmd, "signing-repository-owner-id"),
		}
		rawSigningFlags = trimAllFlagValues(rawSigningFlags)
		rawSigningFlags.UserRequestedMOAT = moatFlag || anySigningFlagSet(rawSigningFlags)

		// Resolve signing profile BEFORE delegating to the orchestrator so
		// flag validation errors surface before any clone attempt. Cheap —
		// local allowlist lookup + flag validation only.
		signing, err := resolveSigningProfile(gitURL, rawSigningFlags)
		if err != nil {
			return err
		}

		// Pre-flight peek at cfg only to decide which security banner to
		// print (prominent for first registry, brief otherwise). The
		// orchestrator does the authoritative load + duplicate check below.
		cfgPreview, _ := config.LoadGlobal()
		isFirstRegistry := cfgPreview != nil && len(cfgPreview.Registries) == 0

		if isFirstRegistry {
			fmt.Fprintf(output.Writer, `
┌──────────────────────────────────────────────────────┐
│                   SECURITY NOTICE                    │
│                                                      │
│  Registries contain AI tool content (skills, rules,  │
│  hooks, commands) that will be available for install.  │
│  This content can influence how AI tools behave.     │
│                                                      │
│  Syllago does not operate, verify, or audit any      │
│  registry. You are responsible for reviewing what    │
│  you install. Only add registries you trust.         │
│                                                      │
│  The syllago maintainers are not affiliated with and │
│  accept no liability for any third-party registry.   │
└──────────────────────────────────────────────────────┘

`)
		} else {
			fmt.Fprintf(output.ErrWriter, "Warning: Registry content is unverified. Only add registries you trust.\n")
		}

		// Announce signing-identity resolution before the clone so the
		// operator sees the pinning decision in context.
		if msg := describeProfileSource(signing, gitURL); msg != "" {
			fmt.Fprintf(output.Writer, "%s\n", msg)
		}

		syncFlag, _ := cmd.Flags().GetBool("sync")

		opts := registryops.AddOpts{
			URL:       gitURL,
			Name:      nameFlag,
			Ref:       refFlag,
			SkipClone: !syncFlag,
		}
		if signing != nil && signing.Profile != nil {
			opts.SigningProfile = signing.Profile
			opts.SigningManifestURI = signing.ManifestURI
		}

		// Resolve the effective name now (matches orchestrator's derivation)
		// so the output line matches what gets persisted.
		effectiveName := nameFlag
		if effectiveName == "" {
			effectiveName = registry.NameFromURL(gitURL)
		}
		if syncFlag {
			fmt.Fprintf(output.Writer, "Cloning %s as %q...\n", gitURL, effectiveName)
		} else {
			fmt.Fprintf(output.Writer, "Registering %s as %q...\n", gitURL, effectiveName)
		}

		outcome, err := registryops.AddRegistry(cmd.Context(), opts)
		if err != nil {
			return classifyAddError(err, effectiveName, gitURL, outcome)
		}

		if outcome.NoContentFound {
			fmt.Fprintf(output.ErrWriter, "Warning: registry %q doesn't appear to contain any recognized content. Added anyway.\n", outcome.Registry.Name)
		}
		for _, similar := range outcome.SimilarRegistries {
			fmt.Fprintf(output.ErrWriter, "Warning: registry %q looks like a duplicate of existing registry %q (same name or URL after normalization). Remove one with 'syllago registry remove'.\n", outcome.Registry.Name, similar)
		}

		if outcome.Cloned {
			if registry.IsPrivate(outcome.Visibility) {
				fmt.Fprintf(output.Writer, "Visibility: private (content from this registry will be tainted)\n")
			} else {
				fmt.Fprintf(output.Writer, "Visibility: public\n")
			}
		}

		if outcome.SelfDeclaredMOAT {
			fmt.Fprintf(output.Writer, "MOAT compliance detected via registry.yaml.\n")
		}

		fmt.Fprintf(output.Writer, "Added registry: %s\n", outcome.Registry.Name)

		// When --sync was passed, chain a sync for MOAT registries so the
		// manifest cache is populated before the next rescan. Without this,
		// EnrichFromMOATManifests sees an empty cache, downgrades trust to
		// Unknown, and the listing shows zero content-type counts. Pinned
		// profiles (allowlist or flag) sync silently. Self-declared profiles
		// need --yes to clear TOFU; without it we print the manual hint.
		if syncFlag && outcome.Registry.IsMOAT() {
			yes, _ := cmd.Flags().GetBool("yes")
			pinned := outcome.Registry.SigningProfile != nil
			if !pinned && !yes {
				fmt.Fprintf(output.Writer, "Run `syllago registry sync --yes %s` to verify and pin the signing identity.\n", outcome.Registry.Name)
			} else {
				// Lockfile root is the bare project root (spec §Lockfile:
				// <project root>/.syllago/moat-lockfile.json) — NOT
				// findContentRepoRoot(), which appends the project config's
				// content_root and would relocate the lockfile where doctor
				// (moat.LoadAndScan(projectRoot, ...)) never looks.
				lockfileRoot, rootErr := findProjectRoot()
				if rootErr != nil {
					return rootErr
				}
				freshCfg, cfgErr := config.LoadGlobal()
				if cfgErr != nil {
					return cfgErr
				}
				reg := findRegistryByName(freshCfg, outcome.Registry.Name)
				if reg == nil {
					return fmt.Errorf("registry %q vanished from config between add and auto-sync", outcome.Registry.Name)
				}
				cacheDir, _ := config.GlobalDirPath()
				fmt.Fprintf(output.Writer, "Verifying signing identity for %s...\n", outcome.Registry.Name)
				code, syncErr := syncMOATRegistry(cmd.Context(), output.Writer, output.ErrWriter, freshCfg, reg, lockfileRoot, cacheDir, time.Now(), yes)
				if syncErr != nil {
					return syncErr
				}
				if code != 0 {
					moatSyncExit(code)
					return nil
				}
			}
		}

		// SAND-003: Offer to add registry domain to sandbox allowlist.
		parsed, parseErr := url.Parse(gitURL)
		if parseErr == nil && parsed.Hostname() != "" {
			host := parsed.Hostname()
			fmt.Fprintf(output.Writer, "\nSecurity: Syllago does not verify registry content. Registry servers can supply\n")
			fmt.Fprintf(output.Writer, "hooks and MCP servers that run on your machine.\n")
			fmt.Fprintf(output.Writer, "Sandbox: Add %s to the sandbox network allowlist? [y/N] ", host)
			var answer string
			fmt.Fscan(os.Stdin, &answer)
			if strings.ToLower(strings.TrimSpace(answer)) == "y" {
				cfg, loadErr := config.LoadGlobal()
				if loadErr != nil {
					fmt.Fprintf(output.Writer, "Warning: failed to load config for sandbox update: %s\n", loadErr)
					return nil
				}
				alreadyPresent := false
				for _, d := range cfg.Sandbox.AllowedDomains {
					if d == host {
						alreadyPresent = true
						break
					}
				}
				if !alreadyPresent {
					cfg.Sandbox.AllowedDomains = append(cfg.Sandbox.AllowedDomains, host)
				}
				if saveErr := config.SaveGlobal(cfg); saveErr != nil {
					fmt.Fprintf(output.Writer, "Warning: failed to save sandbox allowlist: %s\n", saveErr)
				} else {
					fmt.Fprintf(output.Writer, "Added %s to sandbox allowlist.\n", host)
				}
			}
		}

		// One-time next-steps hint so the user knows to browse/install content.
		freshCfgForHint, _ := config.LoadGlobal()
		if shouldShowRegistryAddHint(freshCfgForHint) {
			if !syncFlag {
				fmt.Fprintf(output.Writer, "\nRun `syllago registry sync %s` to fetch content.\n", outcome.Registry.Name)
			}
			fmt.Fprintf(output.Writer, "Tip: registry content is browsable with `syllago list --source registry`.\n")
			fmt.Fprintf(output.Writer, "     Install items with `syllago add <name> --from %s`.\n", outcome.Registry.Name)
		}

		return nil
	},
}

// shouldShowRegistryAddHint reports whether the registry-add next-steps hint
// should be printed. It returns false when the user has previously dismissed
// the hint (hints.registry_add_dismissed=true in global config preferences).
func shouldShowRegistryAddHint(cfg *config.Config) bool {
	if cfg == nil {
		return true
	}
	return cfg.Preferences["hints.registry_add_dismissed"] != "true"
}

// classifyAddError maps the orchestrator's sentinel errors to the CLI's
// structured-error codes. Each branch produces the same CLI surface the
// pre-extraction RunE produced — exit codes, JSON shapes, error codes — so
// downstream callers (CI, scripts) don't notice the refactor.
func classifyAddError(err error, name, url string, outcome registryops.AddOutcome) error {
	switch {
	case errors.Is(err, registryops.ErrAddInvalidName):
		return output.NewStructuredError(output.ErrRegistryInvalid, fmt.Sprintf("registry name %q is invalid", name), "Use letters, numbers, - and _ with optional owner/repo format")
	case errors.Is(err, registryops.ErrAddDuplicate):
		return output.NewStructuredError(output.ErrRegistryDuplicate, fmt.Sprintf("registry %q already exists", name), "Use a different --name or remove it first")
	case errors.Is(err, registryops.ErrAddNotAllowed):
		return output.NewStructuredError(output.ErrRegistryNotAllowed, fmt.Sprintf("registry URL %q is not in the allowedRegistries list", url), "Contact your team lead to add it to .syllago/config.json")
	case errors.Is(err, registryops.ErrAddNotSyllago):
		fmt.Fprintf(output.ErrWriter, "\nThis repo doesn't appear to be a syllago registry.\n")
		fmt.Fprintf(output.ErrWriter, "This content cannot be added as a registry (registries require syllago format).\n")
		fmt.Fprintf(output.ErrWriter, "To add this content to your library, use: syllago add <path> (coming soon)\n")
		return output.NewStructuredError(output.ErrRegistryInvalid, "not a syllago registry -- clone removed", "This content cannot be added as a registry (registries require syllago format)")
	case errors.Is(err, registryops.ErrAddSaveFailed):
		return output.NewStructuredErrorDetail(output.ErrRegistrySaveFailed, "saving registry config", "Check write permissions on .syllago/config.json", err.Error())
	case errors.Is(err, registryops.ErrAddCloneFailed):
		return err // already shaped — git clone errors propagate as-is
	default:
		return err
	}
}

var registryRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Short:   "Remove a registry and delete its local clone",
	Example: `  syllago registry remove team-rules`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		projectRoot, _ := findProjectRoot()
		cacheDir, _ := config.GlobalDirPath()
		outcome, err := registryops.RemoveRegistry(registryops.RemoveOpts{
			Name:        name,
			ProjectRoot: projectRoot,
			CacheDir:    cacheDir,
		})
		if err != nil {
			switch {
			case errors.Is(err, registryops.ErrRemoveNotFound):
				return output.NewStructuredError(output.ErrRegistryNotFound, fmt.Sprintf("registry %q not found in config", name), "Run 'syllago registry list' to see configured registries")
			case errors.Is(err, registryops.ErrRemoveSaveFailed):
				return output.NewStructuredErrorDetail(output.ErrConfigSave, "saving config after registry removal", "Check write permissions on .syllago/config.json", err.Error())
			default:
				return err
			}
		}

		if outcome.CloneRemoveErr != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: could not delete clone for %q: %s\n", name, outcome.CloneRemoveErr)
		}
		if outcome.ManifestCacheRemoveErr != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: could not delete MOAT manifest cache for %q: %s\n", name, outcome.ManifestCacheRemoveErr)
		}
		if outcome.LockfilePruneErr != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: could not prune MOAT lockfile pin for %q: %s\n", name, outcome.LockfilePruneErr)
		}

		fmt.Fprintf(output.Writer, "Removed registry: %s\n", name)
		return nil
	},
}

type registryListItem struct {
	Name      string             `json:"name"`
	Status    string             `json:"status"`
	URL       string             `json:"url"`
	Ref       string             `json:"ref"`
	Manifest  *registry.Manifest `json:"manifest,omitempty"`
	IsMOAT    bool               `json:"is_moat"`
	TrustTier string             `json:"trust_tier,omitempty"` // "moat", "pending", or "" for git registries
}

var registryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered registries",
	Example: `  # List all configured registries
  syllago registry list

  # JSON output
  syllago registry list --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadGlobal()
		if err != nil {
			return err
		}

		if len(cfg.Registries) == 0 {
			fmt.Println("No registries configured. Run `syllago registry add <url>` to add one.")
			return nil
		}

		var items []registryListItem
		for _, r := range cfg.Registries {
			status := "missing"
			if registry.IsCloned(r.Name) {
				status = "cloned"
			}
			ref := r.Ref
			if ref == "" {
				ref = "default"
			}
			manifest, _ := registry.LoadManifest(r.Name) // ignore error; manifest is optional
			tier := ""
			if r.IsMOAT() {
				if r.LastFetchedAt != nil {
					tier = "moat"
				} else {
					tier = "pending"
				}
			}
			items = append(items, registryListItem{
				Name:      r.Name,
				Status:    status,
				URL:       r.URL,
				Ref:       ref,
				Manifest:  manifest,
				IsMOAT:    r.IsMOAT(),
				TrustTier: tier,
			})
		}

		if output.JSON {
			output.Print(items)
			return nil
		}

		fmt.Fprintf(output.Writer, "%-20s  %-8s  %-8s  %-9s  %s\n", "NAME", "STATUS", "VERSION", "TRUST", "URL / DESCRIPTION")
		fmt.Fprintf(output.Writer, "%-20s  %-8s  %-8s  %-9s  %s\n",
			strings.Repeat("─", 20), strings.Repeat("─", 8),
			strings.Repeat("─", 8), strings.Repeat("─", 9), strings.Repeat("─", 40))
		for _, item := range items {
			version := "─"
			if item.Manifest != nil && item.Manifest.Version != "" {
				version = item.Manifest.Version
			}
			trust := "─"
			if item.TrustTier != "" {
				trust = item.TrustTier
			}
			fmt.Fprintf(output.Writer, "%-20s  %-8s  %-8s  %-9s  %s\n",
				truncateStr(item.Name, 20), item.Status, version, trust, item.URL)
			if item.Manifest != nil && item.Manifest.Description != "" {
				fmt.Fprintf(output.Writer, "  %s\n", item.Manifest.Description)
			}
		}
		return nil
	},
}

var registrySyncCmd = &cobra.Command{
	Use:   "sync [name]",
	Short: "Pull latest content from one or all registries",
	Long: `Runs git pull on registry clones to fetch the latest content.

Sync updates the local clone only — it does not modify your library or
installed provider content. Use "syllago registry items" to see what changed,
and "syllago install" to activate updated content.`,
	Example: `  # Sync all registries
  syllago registry sync

  # Sync a specific registry
  syllago registry sync my-rules`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Lockfile root is the bare project root (spec §Lockfile:
		// <project root>/.syllago/moat-lockfile.json) — NOT
		// findContentRepoRoot(), which appends the project config's
		// content_root and would relocate the lockfile where doctor
		// (moat.LoadAndScan(projectRoot, ...)) never looks, leaving the
		// "manifest cache stale" warning stuck after a successful sync.
		lockfileRoot, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, err := config.LoadGlobal()
		if err != nil {
			return err
		}

		telemetry.Enrich("registry_count", len(cfg.Registries))

		if len(cfg.Registries) == 0 {
			fmt.Println("No registries configured.")
			return nil
		}

		yes, _ := cmd.Flags().GetBool("yes")
		cacheDir, _ := config.GlobalDirPath()

		// Single registry sync
		if len(args) == 1 {
			name := args[0]
			reg := findRegistryByName(cfg, name)
			if reg == nil {
				return output.NewStructuredError(output.ErrRegistryNotFound, fmt.Sprintf("registry %q not found in config", name), "Run 'syllago registry list' to see configured registries")
			}
			code, err := syncGitOrMOATRegistry(cmd.Context(), cfg, reg, lockfileRoot, cacheDir, yes)
			if err != nil {
				return err
			}
			if code != 0 {
				moatSyncExit(code)
			}
			return nil
		}

		// Sync all. Each registry is cloned-if-needed (register-only registries
		// have no clone until first sync), upgraded to MOAT when it self-declares,
		// then synced. MOAT sync failures abort the batch (matching prior
		// behaviour); git/clone failures accumulate and fail at the end. A MOAT
		// verification gate on any registry trips the whole command's exit code —
		// if operators need to isolate which one, they sync by name.
		var moatGateExit int
		hasErrors := false
		for i := range cfg.Registries {
			r := &cfg.Registries[i]
			code, err := syncGitOrMOATRegistry(cmd.Context(), cfg, r, lockfileRoot, cacheDir, yes)
			if err != nil {
				if r.IsMOAT() {
					return err
				}
				fmt.Fprintf(output.ErrWriter, "Error syncing %s: %s\n", r.Name, err)
				hasErrors = true
				continue
			}
			if code != 0 && moatGateExit == 0 {
				moatGateExit = code
			}
		}

		if moatGateExit != 0 {
			moatSyncExit(moatGateExit)
			return nil
		}
		if hasErrors {
			return output.NewStructuredError(output.ErrRegistrySyncFailed, "one or more registry syncs failed", "Check error messages above and retry")
		}
		return nil
	},
}

// ensureRegistryCloned clones a git registry that has no local clone yet — the
// register-only `registry add` default defers the clone to the first sync.
// Returns true when a clone was performed; callers skip the follow-up git pull
// in that case (a fresh clone is already current, and pulling a ref-pinned
// detached HEAD would fail). Must run before tryUpgradeToMOAT so a registry.yaml
// self-declaration becomes readable. Uses the registryops.CloneFn seam so tests
// can stub the clone.
func ensureRegistryCloned(r *config.Registry, out io.Writer) (bool, error) {
	if registry.IsCloned(r.Name) {
		return false, nil
	}
	fmt.Fprintf(out, "Cloning %s...\n", r.Name)
	if err := registryops.CloneFn(r.URL, r.Name, r.Ref); err != nil {
		return false, err
	}
	return true, nil
}

// syncGitOrMOATRegistry performs a full sync of one registry, handling the
// register-only deferred clone. For git registries it clones first (if needed)
// so a registry.yaml MOAT self-declaration is detectable, upgrades to MOAT when
// declared, then either runs the MOAT sync or pulls the git clone. When the
// clone lands for the first time it runs the content-discovery steps that
// `registry add` skips for register-only registries (manifest stub + no-content
// warning). Returns a non-zero MOAT gate exit code when a verification gate
// trips. lockfileRoot is the bare project root whose
// .syllago/moat-lockfile.json records MOAT trust state — never the
// content_root-adjusted content directory.
func syncGitOrMOATRegistry(ctx context.Context, cfg *config.Config, r *config.Registry, lockfileRoot, cacheDir string, yes bool) (int, error) {
	justCloned := false
	if r.IsGit() {
		cloned, err := ensureRegistryCloned(r, output.Writer)
		if err != nil {
			return 0, output.NewStructuredErrorDetail(output.ErrRegistrySyncFailed, fmt.Sprintf("sync failed for %q", r.Name), "Check network connectivity and git credentials", err.Error())
		}
		justCloned = cloned
		if _, err := tryUpgradeToMOAT(r, cfg, output.Writer); err != nil {
			return 0, err
		}
	}

	if r.IsMOAT() {
		fmt.Fprintf(output.Writer, "Syncing %s (moat)...\n", r.Name)
		return syncMOATRegistry(ctx, output.Writer, output.ErrWriter, cfg, r, lockfileRoot, cacheDir, time.Now(), yes)
	}

	// Plain git registry.
	fmt.Fprintf(output.Writer, "Syncing %s...\n", r.Name)
	var outcome registry.GitSyncOutcome
	if justCloned {
		// Deferred first clone just landed: run the clone-dependent content
		// discovery that `add` deferred for register-only registries.
		finalizeDeferredClone(r.Name)
		head, err := registry.Head(r.Name)
		if err != nil {
			return 0, output.NewStructuredErrorDetail(output.ErrRegistrySyncFailed, fmt.Sprintf("sync failed for %q", r.Name), "Check network connectivity and git credentials", err.Error())
		}
		outcome.NewHead = head
	} else {
		var err error
		outcome, err = registry.Sync(r.Name)
		if err != nil {
			return 0, output.NewStructuredErrorDetail(output.ErrRegistrySyncFailed, fmt.Sprintf("sync failed for %q", r.Name), "Check network connectivity and git credentials", err.Error())
		}
	}
	if err := persistGitSyncBookkeeping(cfg, r, outcome.NewHead, time.Now()); err != nil {
		return 0, output.NewStructuredErrorDetail(output.ErrRegistrySyncFailed, fmt.Sprintf("sync failed for %q", r.Name), "Could not save registry sync bookkeeping", err.Error())
	}
	reprobeRegistryVisibility(cfg, r.Name)
	fmt.Fprintf(output.Writer, "Synced: %s\n", r.Name)
	if !justCloned && outcome.OldHead != "" && outcome.OldHead != outcome.NewHead {
		if d := registryops.GitSyncDiff(r.Name, outcome.OldHead, outcome.NewHead); d != nil {
			printRegistryDiff(output.Writer, d)
		}
	}
	drifts := registryops.InstalledGitDrift(r.Name)
	if len(drifts) > 0 {
		printInstalledDrift(output.Writer, drifts)
	}
	telemetry.Enrich("drift_count", len(drifts))
	return 0, nil
}

func persistGitSyncBookkeeping(cfg *config.Config, r *config.Registry, head string, syncedAt time.Time) error {
	if head == "" {
		return fmt.Errorf("git HEAD is empty after sync")
	}
	now := syncedAt.UTC()
	r.LastSyncedSHA = head
	r.LastSyncedAt = &now
	if err := config.SaveGlobal(cfg); err != nil {
		return fmt.Errorf("saving registry sync bookkeeping: %w", err)
	}
	return nil
}

// finalizeDeferredClone runs the clone-dependent content-discovery steps that
// `registry add` skips for register-only registries: it generates a registry.yaml
// stub when the repo ships none, and warns when the repo has no recognizable
// content. Called on the first sync after a deferred clone lands.
func finalizeDeferredClone(name string) {
	dir, err := registry.CloneDir(name)
	if err != nil {
		return
	}
	registryops.GenerateManifestStub(name, dir)
	scan := catalog.ScanNativeContent(dir)
	if !scan.HasSyllagoStructure && len(scan.Providers) == 0 {
		if manifest, _ := registry.LoadManifestFromDir(dir); manifest == nil || len(manifest.Items) == 0 {
			fmt.Fprintf(output.ErrWriter, "Warning: registry %q doesn't appear to contain any recognized content.\n", name)
		}
	}
}

// findRegistryByName returns a pointer into cfg.Registries (so callers can
// mutate persisted trust state) or nil when no entry matches. Linear scan;
// config registries are typically O(10).
func findRegistryByName(cfg *config.Config, name string) *config.Registry {
	for i := range cfg.Registries {
		if cfg.Registries[i].Name == name {
			return &cfg.Registries[i]
		}
	}
	return nil
}

// moatSyncExit is a package-level seam so tests can observe the exit code
// instead of terminating the test process. Production path calls os.Exit
// directly — cobra's RunE only maps to exit 1, so the G-18 codes (10/11/13)
// must bypass it.
var moatSyncExit = os.Exit

var registryItemsCmd = &cobra.Command{
	Use:   "items [name]",
	Short: "Browse content available in registries",
	Long: `Lists content items from one or all registries.

Use --type to filter by content type. To install registry content, use
"syllago install --to <provider>" or browse in the TUI with "syllago".`,
	Example: `  # List all items from all registries
  syllago registry items

  # List items from a specific registry
  syllago registry items my-rules

  # Filter by content type
  syllago registry items --type skills`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadGlobal()
		if err != nil {
			return err
		}

		if len(cfg.Registries) == 0 {
			fmt.Println("No registries configured.")
			return nil
		}

		typeFilter, _ := cmd.Flags().GetString("type")

		// Build registry sources
		var sources []catalog.RegistrySource
		if len(args) == 1 {
			name := args[0]
			found := false
			for _, r := range cfg.Registries {
				if r.Name == name {
					found = true
					break
				}
			}
			if !found {
				return output.NewStructuredError(output.ErrRegistryNotFound, fmt.Sprintf("registry %q not found in config", name), "Run 'syllago registry list' to see configured registries")
			}
			if !registry.IsCloned(name) {
				return output.NewStructuredError(output.ErrRegistryNotCloned, fmt.Sprintf("registry %q not cloned", name), fmt.Sprintf("Run 'syllago registry sync %s' first", name))
			}
			dir, _ := registry.CloneDir(name)
			sources = append(sources, catalog.RegistrySource{Name: name, Path: dir})
		} else {
			for _, r := range cfg.Registries {
				if registry.IsCloned(r.Name) {
					dir, _ := registry.CloneDir(r.Name)
					sources = append(sources, catalog.RegistrySource{Name: r.Name, Path: dir})
				}
			}
		}

		cat, scanErr := catalog.ScanRegistriesOnly(sources)
		if scanErr != nil {
			return scanErr
		}
		cat.PrintWarnings()

		// Filter by type if requested
		var items []catalog.ContentItem
		if typeFilter != "" {
			ct := catalog.ContentType(typeFilter)
			items = cat.ByType(ct)
		} else {
			items = cat.Items
		}

		telemetry.Enrich("content_type", typeFilter)
		telemetry.Enrich("item_count", len(items))

		if output.JSON {
			output.Print(items)
			return nil
		}

		if len(items) == 0 {
			fmt.Println("No items found.")
			return nil
		}

		fmt.Printf("%-20s  %-10s  %-15s  %s\n", "Name", "Type", "Registry", "Description")
		fmt.Printf("%-20s  %-10s  %-15s  %s\n", strings.Repeat("─", 20), strings.Repeat("─", 10), strings.Repeat("─", 15), strings.Repeat("─", 30))
		for _, item := range items {
			desc := item.Description
			if len(desc) > 40 {
				desc = desc[:37] + "..."
			}
			fmt.Printf("%-20s  %-10s  %-15s  %s\n",
				truncateStr(item.Name, 20),
				truncateStr(string(item.Type), 10),
				truncateStr(item.Registry, 15),
				desc,
			)
		}
		return nil
	},
}

var registryCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new registry",
	Long: `Create a new registry in one of two modes:

  --new <name>          Scaffold an empty registry directory structure
  --from-native         Index provider-native content in the current repo`,
	Example: `  # Scaffold an empty registry
  syllago registry create --new my-rules

  # Scaffold with a description
  syllago registry create --new my-rules --description "Team coding standards"

  # Index existing provider-native content
  syllago registry create --from-native`,
	RunE: func(cmd *cobra.Command, args []string) error {
		newName, _ := cmd.Flags().GetString("new")
		fromNative, _ := cmd.Flags().GetBool("from-native")

		switch {
		case newName != "":
			return runRegistryCreateNew(cmd, newName)
		case fromNative:
			return runRegistryCreateFromNative(cmd)
		default:
			return cmd.Help()
		}
	},
}

func runRegistryCreateNew(cmd *cobra.Command, name string) error {
	desc, _ := cmd.Flags().GetString("description")
	noGit, _ := cmd.Flags().GetBool("no-git")

	cwd, err := os.Getwd()
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrSystemIO, "getting working directory", "", err.Error())
	}

	// Check if already inside a git repo before creating anything.
	alreadyInGit := gitutil.IsInsideGitRepo(cwd)

	if err := registry.Scaffold(cwd, name, desc); err != nil {
		return err
	}

	dir := filepath.Join(cwd, name)
	fmt.Fprintf(output.Writer, "Created registry scaffold at %s\n", dir)
	fmt.Fprintf(output.Writer, "\nStructure:\n")

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() {
			fmt.Fprintf(output.Writer, "  %s/\n", e.Name())
		} else {
			fmt.Fprintf(output.Writer, "  %s\n", e.Name())
		}
	}

	// Git init + commit.
	didGitInit := false
	if !noGit && !alreadyInGit {
		if gitErr := gitutil.InitAndCommit(dir, "Initial registry scaffold"); gitErr != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: git init failed: %s\n", gitErr)
		} else {
			fmt.Fprintf(output.Writer, "\nInitialized git repository and created initial commit.\n")
			didGitInit = true
		}
	} else if alreadyInGit && !noGit {
		fmt.Fprintf(output.Writer, "\nNote: already inside a git repo — skipping git init.\n")
	}

	fmt.Fprintf(output.Writer, "\nNext steps:\n")
	fmt.Fprintf(output.Writer, "  cd %s\n", name)
	if didGitInit {
		fmt.Fprintf(output.Writer, "  git remote add origin <your-git-url>\n")
		fmt.Fprintf(output.Writer, "  git push -u origin main\n")
	} else {
		fmt.Fprintf(output.Writer, "  git init && git add . && git commit -m 'Initial registry scaffold'\n")
		fmt.Fprintf(output.Writer, "  git remote add origin <your-git-url>\n")
		fmt.Fprintf(output.Writer, "  git push -u origin main\n")
	}
	fmt.Fprintf(output.Writer, "\nThen add your registry locally:\n")
	fmt.Fprintf(output.Writer, "  syllago registry add <your-git-url>\n")
	return nil
}

func runRegistryCreateFromNative(cmd *cobra.Command) error {
	desc, _ := cmd.Flags().GetString("description")
	return registryCreateFromNative(desc)
}

// truncateStr cuts a string to max length with "..." suffix.
func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

// reprobeRegistryVisibility re-probes the visibility for a named registry
// and saves the updated config if the visibility changed or the cache is stale.
func reprobeRegistryVisibility(cfg *config.Config, name string) {
	for i := range cfg.Registries {
		if cfg.Registries[i].Name != name {
			continue
		}
		r := &cfg.Registries[i]
		if !registry.NeedsReprobe(r.VisibilityCheckedAt) {
			return
		}
		probeResult, err := registry.ProbeVisibility(r.URL)
		if err != nil {
			return // don't update on error
		}
		manifestDecl := ""
		if manifest, _ := registry.LoadManifest(name); manifest != nil {
			manifestDecl = manifest.Visibility
		}
		newVis := registry.ResolveVisibility(probeResult, manifestDecl)
		now := time.Now().UTC()
		r.Visibility = newVis
		r.VisibilityCheckedAt = &now
		_ = config.SaveGlobal(cfg) // best-effort save
		return
	}
}

// tryUpgradeToMOAT checks if a git-type registry should be upgraded to MOAT.
// Precedence: bundled allowlist (pre-trusted, no TOFU) > registry.yaml self-declaration (TOFU on first sync).
// Mutates r in place and saves cfg when an upgrade occurs. Returns true if upgraded.
//
// Precondition: r.IsGit() must be true before calling.
// CloneDir may return an error if the registry is configured but not cloned yet —
// in that case registry.yaml self-declaration is skipped (can't read from a non-existent clone).
func tryUpgradeToMOAT(r *config.Registry, cfg *config.Config, out io.Writer) (bool, error) {
	// 1. Allowlist check — pre-trusted, no TOFU on first sync.
	if entry, ok := moat.LookupSigningIdentity(r.URL); ok && entry.ManifestURI != "" {
		r.Type = config.RegistryTypeMOAT
		r.ManifestURI = entry.ManifestURI
		if r.SigningProfile == nil {
			r.SigningProfile = entry.Profile
		}
		fmt.Fprintf(out, "Auto-upgraded %s to MOAT (allowlist match).\n", r.Name)
		if err := config.SaveGlobal(cfg); err != nil {
			return false, fmt.Errorf("saving upgraded registry config: %w", err)
		}
		return true, nil
	}
	// 2. registry.yaml self-declaration — TOFU, requires --yes on first sync.
	cloneDir, err := registry.CloneDir(r.Name)
	if err != nil || !registry.IsCloned(r.Name) {
		return false, nil
	}
	if manifest, _ := registry.LoadManifestFromDir(cloneDir); manifest != nil && manifest.ManifestURI != "" {
		r.Type = config.RegistryTypeMOAT
		r.ManifestURI = manifest.ManifestURI
		fmt.Fprintf(out, "Auto-upgraded %s to MOAT (registry.yaml manifest_uri). Run `syllago registry sync --yes %s` to pin the signing identity.\n", r.Name, r.Name)
		if err := config.SaveGlobal(cfg); err != nil {
			return false, fmt.Errorf("saving upgraded registry config: %w", err)
		}
		return true, nil
	}
	return false, nil
}

func init() {
	registryAddCmd.Flags().String("name", "", "Override the registry name (default: derived from URL)")
	registryAddCmd.Flags().String("ref", "", "Branch, tag, or commit to checkout (default: repo default branch)")
	registryAddCmd.Flags().Bool("sync", false, "Clone and sync immediately after registering (default: register only, sync later with 'registry sync')")
	registryAddCmd.Flags().Bool("moat", false, "Add as a MOAT-signed registry (required when URL is not in the bundled allowlist and no --signing-identity is passed)")
	registryAddCmd.Flags().String("signing-identity", "", "Workflow subject SAN (e.g. https://github.com/OWNER/REPO/.github/workflows/moat.yml@refs/heads/main) — implies --moat")
	registryAddCmd.Flags().String("signing-issuer", "", "OIDC issuer URL (default: GitHub Actions issuer)")
	registryAddCmd.Flags().String("signing-repository-id", "", "GitHub numeric repository ID (required for GitHub Actions issuer)")
	registryAddCmd.Flags().String("signing-repository-owner-id", "", "GitHub numeric repository-owner ID (required for GitHub Actions issuer)")
	registryItemsCmd.Flags().String("type", "", "Filter by content type (skills, rules, hooks, etc.)")
	registrySyncCmd.Flags().Bool("yes", false, "Auto-accept TOFU (trust-on-first-use) for MOAT registries with no pinned signing profile")
	registryAddCmd.Flags().Bool("yes", false, "Auto-accept TOFU during the chained post-add sync (required for MOAT registries that self-declare via registry.yaml without an allowlist match)")

	registryCreateCmd.Flags().String("new", "", "Scaffold an empty registry directory with this name")
	registryCreateCmd.Flags().Bool("from-native", false, "Index provider-native content in the current repo")
	registryCreateCmd.Flags().String("description", "", "Short description of the registry (used with --new)")
	registryCreateCmd.Flags().Bool("no-git", false, "Skip git init and initial commit (used with --new)")

	registryCmd.AddCommand(registryAddCmd, registryRemoveCmd, registryListCmd, registrySyncCmd, registryStatusCmd, registryItemsCmd, registryCreateCmd)
	rootCmd.AddCommand(registryCmd)
}
