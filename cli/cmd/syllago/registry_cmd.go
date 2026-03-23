package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/gitutil"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
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
		root, err := findContentRepoRoot()
		if err != nil {
			return err
		}
		gitURL := args[0]

		// Expand short aliases before any other processing
		if fullURL, wasExpanded := registry.ExpandAlias(gitURL); wasExpanded {
			fmt.Fprintf(output.Writer, "Expanding alias %q → %s\n", gitURL, fullURL)
			gitURL = fullURL
		}

		nameFlag, _ := cmd.Flags().GetString("name")
		refFlag, _ := cmd.Flags().GetString("ref")

		name := nameFlag
		if name == "" {
			name = registry.NameFromURL(gitURL)
		}
		if !catalog.IsValidRegistryName(name) {
			return fmt.Errorf("registry name %q is invalid (use letters, numbers, - and _ with optional owner/repo format)", name)
		}

		cfg, err := config.Load(root)
		if err != nil {
			return err
		}

		// Check for duplicate name
		for _, r := range cfg.Registries {
			if r.Name == name {
				return fmt.Errorf("registry %q already exists (use a different --name or remove it first)", name)
			}
		}

		// Enforce allowedRegistries policy
		if !cfg.IsRegistryAllowed(gitURL) {
			return fmt.Errorf("registry URL %q is not in the allowedRegistries list.\n"+
				"Your project config restricts which registries can be added.\n"+
				"Contact your team lead to add it to .syllago/config.json", gitURL)
		}

		// Security warning: prominent box on first registry, brief reminder otherwise
		if len(cfg.Registries) == 0 {
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

		// Clone the registry
		fmt.Fprintf(output.Writer, "Cloning %s as %q...\n", gitURL, name)
		if err := registry.Clone(gitURL, name, refFlag); err != nil {
			return err
		}

		// Smart detection: check if this is a proper syllago registry.
		dir, _ := registry.CloneDir(name)
		scanResult := catalog.ScanNativeContent(dir)

		if !scanResult.HasSyllagoStructure && len(scanResult.Providers) > 0 {
			// Before rejecting, check if the repo has indexed items via registry.yaml.
			// A repo with a manifest.Items list is a valid indexed native registry.
			manifest, _ := registry.LoadManifestFromDir(dir)
			if manifest == nil || len(manifest.Items) == 0 {
				fmt.Fprintf(output.ErrWriter, "\nThis repo doesn't appear to be a syllago registry.\n")
				fmt.Fprintf(output.ErrWriter, "Found provider-native content:\n\n")
				for _, pc := range scanResult.Providers {
					fmt.Fprintf(output.ErrWriter, "  %s:\n", pc.ProviderName)
					for typeLabel, items := range pc.Items {
						fmt.Fprintf(output.ErrWriter, "    %s: %d item(s)\n", typeLabel, len(items))
					}
				}
				fmt.Fprintf(output.ErrWriter, "\nThis content cannot be added as a registry (registries require syllago format).\n")
				fmt.Fprintf(output.ErrWriter, "To add this content to your library, use: syllago add <path> (coming soon)\n")
				os.RemoveAll(dir)
				return fmt.Errorf("not a syllago registry -- clone removed")
			}
		} else if !scanResult.HasSyllagoStructure && len(scanResult.Providers) == 0 {
			fmt.Fprintf(output.ErrWriter, "Warning: registry %q doesn't appear to contain any recognized content. Added anyway.\n", name)
		}

		// Probe visibility from hosting platform API
		probeResult, _ := registry.ProbeVisibility(gitURL)

		// Check manifest declaration and resolve (stricter wins)
		manifestDecl := ""
		if manifest, _ := registry.LoadManifestFromDir(dir); manifest != nil {
			manifestDecl = manifest.Visibility
		}
		visibility := registry.ResolveVisibility(probeResult, manifestDecl)
		now := time.Now().UTC()

		if registry.IsPrivate(visibility) {
			fmt.Fprintf(output.Writer, "Visibility: private (content from this registry will be tainted)\n")
		} else {
			fmt.Fprintf(output.Writer, "Visibility: public\n")
		}

		// Save to config
		cfg.Registries = append(cfg.Registries, config.Registry{
			Name:                name,
			URL:                 gitURL,
			Ref:                 refFlag,
			Visibility:          visibility,
			VisibilityCheckedAt: &now,
		})
		if err := config.Save(root, cfg); err != nil {
			// Config save failed — clean up the clone so it doesn't become orphaned.
			dir, _ := registry.CloneDir(name)
			os.RemoveAll(dir)
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Fprintf(output.Writer, "Added registry: %s\n", name)

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
				if saveErr := config.Save(root, cfg); saveErr != nil {
					fmt.Fprintf(output.Writer, "Warning: failed to save sandbox allowlist: %s\n", saveErr)
				} else {
					fmt.Fprintf(output.Writer, "Added %s to sandbox allowlist.\n", host)
				}
			}
		}

		return nil
	},
}

var registryRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Short:   "Remove a registry and delete its local clone",
	Example: `  syllago registry remove team-rules`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findContentRepoRoot()
		if err != nil {
			return err
		}
		name := args[0]

		cfg, err := config.Load(root)
		if err != nil {
			return err
		}

		found := false
		var filtered []config.Registry
		for _, r := range cfg.Registries {
			if r.Name == name {
				found = true
				continue
			}
			filtered = append(filtered, r)
		}
		if !found {
			return fmt.Errorf("registry %q not found in config", name)
		}

		cfg.Registries = filtered
		if err := config.Save(root, cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		if err := registry.Remove(name); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: could not delete clone for %q: %s\n", name, err)
		}

		fmt.Fprintf(output.Writer, "Removed registry: %s\n", name)
		return nil
	},
}

type registryListItem struct {
	Name     string             `json:"name"`
	Status   string             `json:"status"`
	URL      string             `json:"url"`
	Ref      string             `json:"ref"`
	Manifest *registry.Manifest `json:"manifest,omitempty"`
}

var registryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered registries",
	Example: `  # List all configured registries
  syllago registry list

  # JSON output
  syllago registry list --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findContentRepoRoot()
		if err != nil {
			return err
		}
		cfg, err := config.Load(root)
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
			items = append(items, registryListItem{
				Name:     r.Name,
				Status:   status,
				URL:      r.URL,
				Ref:      ref,
				Manifest: manifest,
			})
		}

		if output.JSON {
			output.Print(items)
			return nil
		}

		fmt.Fprintf(output.Writer, "%-20s  %-8s  %-8s  %s\n", "NAME", "STATUS", "VERSION", "URL / DESCRIPTION")
		fmt.Fprintf(output.Writer, "%-20s  %-8s  %-8s  %s\n",
			strings.Repeat("─", 20), strings.Repeat("─", 8),
			strings.Repeat("─", 8), strings.Repeat("─", 40))
		for _, item := range items {
			version := "─"
			if item.Manifest != nil && item.Manifest.Version != "" {
				version = item.Manifest.Version
			}
			fmt.Fprintf(output.Writer, "%-20s  %-8s  %-8s  %s\n",
				truncateStr(item.Name, 20), item.Status, version, item.URL)
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
		cfg, err := config.Load(root)
		if err != nil {
			return err
		}

		if len(cfg.Registries) == 0 {
			fmt.Println("No registries configured.")
			return nil
		}

		// Single registry sync
		if len(args) == 1 {
			name := args[0]
			if !registry.IsCloned(name) {
				return fmt.Errorf("registry %q is not cloned locally — run `syllago registry add` first", name)
			}
			fmt.Fprintf(output.Writer, "Syncing %s...\n", name)
			if err := registry.Sync(name); err != nil {
				return err
			}
			// Re-probe visibility on sync
			reprobeRegistryVisibility(cfg, name, root)
			fmt.Fprintf(output.Writer, "Synced: %s\n", name)
			return nil
		}

		// Sync all
		names := make([]string, len(cfg.Registries))
		for i, r := range cfg.Registries {
			names[i] = r.Name
		}

		results := registry.SyncAll(names)
		hasErrors := false
		for _, res := range results {
			if res.Err != nil {
				fmt.Fprintf(output.ErrWriter, "Error syncing %s: %s\n", res.Name, res.Err)
				hasErrors = true
			} else {
				reprobeRegistryVisibility(cfg, res.Name, root)
				fmt.Fprintf(output.Writer, "Synced: %s\n", res.Name)
			}
		}
		if hasErrors {
			return fmt.Errorf("one or more registry syncs failed")
		}
		return nil
	},
}

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
		root, err := findContentRepoRoot()
		if err != nil {
			return err
		}
		cfg, err := config.Load(root)
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
				return fmt.Errorf("registry %q not found in config", name)
			}
			if !registry.IsCloned(name) {
				return fmt.Errorf("registry %q not cloned — run `syllago registry sync %s` first", name, name)
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
		return fmt.Errorf("getting working directory: %w", err)
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
func reprobeRegistryVisibility(cfg *config.Config, name, root string) {
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
		_ = config.Save(root, cfg) // best-effort save
		return
	}
}

func init() {
	registryAddCmd.Flags().String("name", "", "Override the registry name (default: derived from URL)")
	registryAddCmd.Flags().String("ref", "", "Branch, tag, or commit to checkout (default: repo default branch)")
	registryItemsCmd.Flags().String("type", "", "Filter by content type (skills, rules, hooks, etc.)")

	registryCreateCmd.Flags().String("new", "", "Scaffold an empty registry directory with this name")
	registryCreateCmd.Flags().Bool("from-native", false, "Index provider-native content in the current repo")
	registryCreateCmd.Flags().String("description", "", "Short description of the registry (used with --new)")
	registryCreateCmd.Flags().Bool("no-git", false, "Skip git init and initial commit (used with --new)")

	registryCmd.AddCommand(registryAddCmd, registryRemoveCmd, registryListCmd, registrySyncCmd, registryItemsCmd, registryCreateCmd)
	rootCmd.AddCommand(registryCmd)
}
