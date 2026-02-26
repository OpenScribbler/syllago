package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
	"github.com/OpenScribbler/nesco/cli/internal/config"
	"github.com/OpenScribbler/nesco/cli/internal/output"
	"github.com/OpenScribbler/nesco/cli/internal/registry"
	"github.com/spf13/cobra"
)

var registryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Manage git-based content registries",
	Long:  "Add, remove, list, and sync git repositories as content registries in .nesco/config.json.",
}

var registryAddCmd = &cobra.Command{
	Use:   "add <git-url>",
	Short: "Add a git registry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		gitURL := args[0]

		nameFlag, _ := cmd.Flags().GetString("name")
		refFlag, _ := cmd.Flags().GetString("ref")

		name := nameFlag
		if name == "" {
			name = registry.NameFromURL(gitURL)
		}
		if !catalog.IsValidItemName(name) {
			return fmt.Errorf("registry name %q contains invalid characters (use letters, numbers, - and _)", name)
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

		// Security warning: prominent box on first registry, brief reminder otherwise
		if len(cfg.Registries) == 0 {
			fmt.Fprintf(output.Writer, `
┌──────────────────────────────────────────────────────┐
│                   SECURITY NOTICE                    │
│                                                      │
│  Registries contain AI tool content (skills, rules,  │
│  hooks, prompts) that will be available for install.  │
│  This content can influence how AI tools behave.     │
│                                                      │
│  Nesco does not operate, verify, or audit any        │
│  registry. You are responsible for reviewing what    │
│  you install. Only add registries you trust.         │
│                                                      │
│  The nesco maintainers are not affiliated with and   │
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

		// Scan to verify it has content — warn but don't fail
		dir, _ := registry.CloneDir(name)
		hasDirs := false
		for _, ct := range catalog.AllContentTypes() {
			info, statErr := os.Stat(filepath.Join(dir, string(ct)))
			if statErr == nil && info.IsDir() {
				hasDirs = true
				break
			}
		}
		if !hasDirs {
			fmt.Fprintf(output.ErrWriter, "Warning: registry %q doesn't appear to contain content directories (skills/, rules/, etc.). Added anyway.\n", name)
		}

		// Save to config
		cfg.Registries = append(cfg.Registries, config.Registry{
			Name: name,
			URL:  gitURL,
			Ref:  refFlag,
		})
		if err := config.Save(root, cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Fprintf(output.Writer, "Added registry: %s\n", name)

		// SAND-003: Offer to add registry domain to sandbox allowlist.
		parsed, parseErr := url.Parse(gitURL)
		if parseErr == nil && parsed.Hostname() != "" {
			host := parsed.Hostname()
			fmt.Fprintf(output.Writer, "\nSecurity: Nesco does not verify registry content. Registry servers can supply\n")
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
	Use:   "remove <name>",
	Short: "Remove a registry and delete its local clone",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
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

var registryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered registries",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, err := config.Load(root)
		if err != nil {
			return err
		}

		if output.JSON {
			output.Print(cfg.Registries)
			return nil
		}

		if len(cfg.Registries) == 0 {
			fmt.Println("No registries configured. Run `nesco registry add <url>` to add one.")
			return nil
		}

		fmt.Printf("%-20s  %-8s  %s\n", "NAME", "STATUS", "URL")
		fmt.Printf("%-20s  %-8s  %s\n", strings.Repeat("─", 20), strings.Repeat("─", 8), strings.Repeat("─", 40))
		for _, r := range cfg.Registries {
			status := "missing"
			if registry.IsCloned(r.Name) {
				status = "cloned"
			}
			ref := r.Ref
			if ref == "" {
				ref = "default"
			}
			fmt.Printf("%-20s  %-8s  %s  [%s]\n", r.Name, status, r.URL, ref)
		}
		return nil
	},
}

var registrySyncCmd = &cobra.Command{
	Use:   "sync [name]",
	Short: "Sync (git pull) one or all registries",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
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
				return fmt.Errorf("registry %q is not cloned locally — run `nesco registry add` first", name)
			}
			fmt.Fprintf(output.Writer, "Syncing %s...\n", name)
			if err := registry.Sync(name); err != nil {
				return err
			}
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
	Short: "List items from a registry (or all registries)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
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
				return fmt.Errorf("registry %q not cloned — run `nesco registry sync %s` first", name, name)
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

func init() {
	registryAddCmd.Flags().String("name", "", "Override the registry name (default: derived from URL)")
	registryAddCmd.Flags().String("ref", "", "Branch, tag, or commit to checkout (default: repo default branch)")
	registryItemsCmd.Flags().String("type", "", "Filter by content type (skills, rules, hooks, etc.)")

	registryCmd.AddCommand(registryAddCmd, registryRemoveCmd, registryListCmd, registrySyncCmd, registryItemsCmd)
	rootCmd.AddCommand(registryCmd)
}
