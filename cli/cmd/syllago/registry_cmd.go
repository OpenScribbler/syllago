package main

import (
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
	Example: `  # Add a registry by URL
  syllago registry add https://github.com/team/rules.git

  # Add with a custom name
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

		opts := registryops.AddOpts{
			URL:  gitURL,
			Name: nameFlag,
			Ref:  refFlag,
		}
		if signing != nil && signing.Profile != nil {
			opts.SigningProfile = signing.Profile
			opts.SigningManifestURI = signing.ManifestURI
		}

		// Resolve the effective name now (matches orchestrator's derivation)
		// so the "Cloning X as Y" line matches what gets persisted.
		effectiveName := nameFlag
		if effectiveName == "" {
			effectiveName = registry.NameFromURL(gitURL)
		}
		fmt.Fprintf(output.Writer, "Cloning %s as %q...\n", gitURL, effectiveName)

		outcome, err := registryops.AddRegistry(cmd.Context(), opts)
		if err != nil {
			return classifyAddError(err, effectiveName, gitURL, outcome)
		}

		if outcome.NoContentFound {
			fmt.Fprintf(output.ErrWriter, "Warning: registry %q doesn't appear to contain any recognized content. Added anyway.\n", outcome.Registry.Name)
		}

		if registry.IsPrivate(outcome.Visibility) {
			fmt.Fprintf(output.Writer, "Visibility: private (content from this registry will be tainted)\n")
		} else {
			fmt.Fprintf(output.Writer, "Visibility: public\n")
		}

		if outcome.SelfDeclaredMOAT {
			fmt.Fprintf(output.Writer, "MOAT compliance detected via registry.yaml.\n")
		}

		fmt.Fprintf(output.Writer, "Added registry: %s\n", outcome.Registry.Name)

		// Chain a sync for MOAT registries so the manifest cache is populated
		// before the next rescan — without it, EnrichFromMOATManifests sees
		// an empty cache, downgrades trust to Unknown, and the listing shows
		// zero content-type counts. Pinned profiles (allowlist or flag) sync
		// silently. Self-declared profiles need --yes to clear TOFU; without
		// it we print the manual hint and exit cleanly.
		if outcome.Registry.IsMOAT() {
			yes, _ := cmd.Flags().GetBool("yes")
			pinned := outcome.Registry.SigningProfile != nil
			if !pinned && !yes {
				fmt.Fprintf(output.Writer, "Run `syllago registry sync --yes %s` to verify and pin the signing identity.\n", outcome.Registry.Name)
			} else {
				root, rootErr := findContentRepoRoot()
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
				code, syncErr := syncMOATRegistry(cmd.Context(), output.Writer, output.ErrWriter, freshCfg, reg, root, cacheDir, time.Now(), yes)
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

		return nil
	},
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
		root, err := findContentRepoRoot()
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

		// Single registry sync
		if len(args) == 1 {
			name := args[0]
			reg := findRegistryByName(cfg, name)
			if reg == nil {
				return output.NewStructuredError(output.ErrRegistryNotFound, fmt.Sprintf("registry %q not found in config", name), "Run 'syllago registry list' to see configured registries")
			}
			if reg.IsGit() {
				if _, err := tryUpgradeToMOAT(reg, cfg, output.Writer); err != nil {
					return err
				}
			}
			if reg.IsMOAT() {
				fmt.Fprintf(output.Writer, "Syncing %s (moat)...\n", name)
				cacheDir, _ := config.GlobalDirPath()
				code, err := syncMOATRegistry(cmd.Context(), output.Writer, output.ErrWriter, cfg, reg, root, cacheDir, time.Now(), yes)
				if err != nil {
					return err
				}
				if code != 0 {
					moatSyncExit(code)
					return nil
				}
				return nil
			}
			if !registry.IsCloned(name) {
				return output.NewStructuredError(output.ErrRegistryNotCloned, fmt.Sprintf("registry %q is not cloned locally", name), "Run 'syllago registry add' first")
			}
			fmt.Fprintf(output.Writer, "Syncing %s...\n", name)
			client, err := registry.Open(name)
			if err != nil {
				return err
			}
			if err := client.Sync(cmd.Context()); err != nil {
				return err
			}
			// Re-probe visibility on sync
			reprobeRegistryVisibility(cfg, name)
			fmt.Fprintf(output.Writer, "Synced: %s\n", name)
			return nil
		}

		// Sync all. MOAT registries run through the dispatcher one at a time
		// so each can update its own lockfile row; git registries go through
		// the existing SyncAll fan-out. A MOAT gate on any single registry
		// trips the exit code for the whole command — if operators need to
		// isolate which one, they sync by name.

		// Pre-scan: auto-upgrade git registries to MOAT before dispatching.
		// Runs before the MOAT/git split so newly-upgraded registries flow
		// into the MOAT loop on the same sync invocation.
		for i := range cfg.Registries {
			r := &cfg.Registries[i]
			if r.IsGit() {
				if _, err := tryUpgradeToMOAT(r, cfg, output.Writer); err != nil {
					return err
				}
			}
		}

		var moatGateExit int
		cacheDir, _ := config.GlobalDirPath()
		for i := range cfg.Registries {
			r := &cfg.Registries[i]
			if !r.IsMOAT() {
				continue
			}
			fmt.Fprintf(output.Writer, "Syncing %s (moat)...\n", r.Name)
			code, err := syncMOATRegistry(cmd.Context(), output.Writer, output.ErrWriter, cfg, r, root, cacheDir, time.Now(), yes)
			if err != nil {
				return err
			}
			if code != 0 && moatGateExit == 0 {
				moatGateExit = code
			}
		}

		var gitNames []string
		for _, r := range cfg.Registries {
			if r.IsMOAT() {
				continue
			}
			gitNames = append(gitNames, r.Name)
		}

		hasErrors := false
		if len(gitNames) > 0 {
			results := registry.SyncAll(gitNames)
			for _, res := range results {
				if res.Err != nil {
					fmt.Fprintf(output.ErrWriter, "Error syncing %s: %s\n", res.Name, res.Err)
					hasErrors = true
				} else {
					reprobeRegistryVisibility(cfg, res.Name)
					fmt.Fprintf(output.Writer, "Synced: %s\n", res.Name)
				}
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

	registryCmd.AddCommand(registryAddCmd, registryRemoveCmd, registryListCmd, registrySyncCmd, registryItemsCmd, registryCreateCmd)
	rootCmd.AddCommand(registryCmd)
}
