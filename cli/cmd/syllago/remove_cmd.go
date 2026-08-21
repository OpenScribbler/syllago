package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

// removeResult is the JSON-serializable output for syllago remove.
type removeResult struct {
	Name            string   `json:"name"`
	Type            string   `json:"type"`
	UninstalledFrom []string `json:"uninstalled_from,omitempty"`
	RemovedPath     string   `json:"removed_path"`
}

var removeCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove content from your library and uninstall from providers",
	Long: `Removes a content item from your library (~/.syllago/content/) and
uninstalls it from any providers where it is currently installed.`,
	Example: `  # Remove a skill from library and all providers
  syllago remove my-skill

  # Disambiguate by type when name exists in multiple types
  syllago remove my-rule --type rules

  # Skip confirmation prompt
  syllago remove my-skill --force

  # Preview what would happen
  syllago remove my-skill --dry-run`,
	Args: cobra.ExactArgs(1),
	RunE: runRemove,
}

func init() {
	removeCmd.Flags().String("type", "", "Disambiguate when name exists in multiple types")
	removeCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	removeCmd.Flags().BoolP("dry-run", "n", false, "Show what would happen without making changes")
	removeCmd.Flags().Bool("no-input", false, "Disable interactive prompts, use defaults")
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
	typeFilter, _ := cmd.Flags().GetString("type")
	force, _ := cmd.Flags().GetBool("force")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	noInput, _ := cmd.Flags().GetBool("no-input")

	globalDir := catalog.GlobalContentDir()
	if globalDir == "" {
		return output.NewStructuredError(output.ErrSystemHomedir, "cannot determine home directory", "Ensure $HOME is set in your environment")
	}

	// Use an empty temp dir as the project root so the project scan finds
	// nothing, leaving only global items in the catalog.
	emptyProjectRoot, err := os.MkdirTemp("", "syllago-remove-*")
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrSystemIO, "creating temp dir failed", "Check filesystem permissions and disk space", err.Error())
	}
	defer func() { _ = os.RemoveAll(emptyProjectRoot) }()

	cat, err := catalog.ScanWithGlobalAndRegistries(emptyProjectRoot, emptyProjectRoot, nil)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrCatalogScanFailed, "scanning library failed", "Check that ~/.syllago/content/ exists and is readable", err.Error())
	}

	var matches []catalog.ContentItem
	for _, item := range cat.Items {
		if item.Source != "global" || item.Name != name {
			continue
		}
		if typeFilter != "" && string(item.Type) != typeFilter {
			continue
		}
		matches = append(matches, item)
	}

	if len(matches) == 0 {
		return output.NewStructuredError(output.ErrItemNotFound, fmt.Sprintf("no item named %q found in your library", name), "Run 'syllago list' to show all library items")
	}

	if len(matches) > 1 {
		var types []string
		for _, m := range matches {
			types = append(types, string(m.Type))
		}
		return output.NewStructuredError(output.ErrItemAmbiguous, fmt.Sprintf("%q exists in multiple types: %s", name, strings.Join(types, ", ")), fmt.Sprintf("Use --type to disambiguate: syllago remove %s --type <type>", name))
	}

	item := matches[0]

	providerNames := providerNamesBySlug(provider.AllProviders)
	installedByProvider := make(map[string]bool)
	var installedIn []string
	for _, prov := range provider.AllProviders {
		if installer.CheckStatus(item, prov, globalDir) == installer.StatusInstalled {
			installedIn = append(installedIn, prov.Name)
			installedByProvider[prov.Slug] = true
		}
	}

	home, _ := os.UserHomeDir()
	providerLinks := installer.ScanProviderLinks(provider.AllProviders, home, []string{item.Path})
	extraProviderLinks := extraRemoveProviderLinks(item, providerLinks, installedByProvider)
	for _, link := range extraProviderLinks {
		installedIn = append(installedIn, providerLinkLabel(link, providerNames))
	}

	if dryRun {
		if len(installedIn) > 0 {
			fmt.Fprintf(output.Writer, "[dry-run] Would uninstall from: %s\n", strings.Join(installedIn, ", "))
		}
		for _, link := range extraProviderLinks {
			fmt.Fprintf(output.Writer, "[dry-run] Would remove provider link: %s\n", link.Path)
		}
		fmt.Fprintf(output.Writer, "[dry-run] Would remove from library: %s (%s)\n", name, item.Type.Label())
		return nil
	}

	if !force && !noInput && isInteractive() {
		provList := "none"
		if len(installedIn) > 0 {
			provList = strings.Join(installedIn, ", ")
		}
		fmt.Fprintf(output.Writer, "This will remove %q from your library.\n", name)
		fmt.Fprintf(output.Writer, "  Type: %s\n", item.Type.Label())
		fmt.Fprintf(output.Writer, "  Installed in: %s\n\n", provList)
		fmt.Fprintf(output.Writer, "Continue? [y/N] ")

		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			return output.NewStructuredError(output.ErrInputTerminal, "no input received", "Run with --force to skip confirmation")
		}
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if answer != "y" && answer != "yes" {
			fmt.Fprintln(output.Writer, "Cancelled.")
			return nil
		}
	}

	var uninstalledFrom []string
	for _, prov := range provider.AllProviders {
		if installer.CheckStatus(item, prov, globalDir) != installer.StatusInstalled {
			continue
		}
		if _, err := installer.Uninstall(item, prov, globalDir); err != nil {
			fmt.Fprintf(output.ErrWriter, "  warning: failed to uninstall from %s: %s\n", prov.Name, err)
		} else {
			uninstalledFrom = append(uninstalledFrom, prov.Name)
		}
	}

	home, _ = os.UserHomeDir()
	remainingLinks := installer.ScanProviderLinks(provider.AllProviders, home, []string{item.Path})
	var linkRemoveErrs []string
	for _, link := range remainingLinks {
		if err := os.Remove(link.Path); err != nil {
			msg := fmt.Sprintf("failed to remove provider link %s: %s", link.Path, err)
			fmt.Fprintf(output.ErrWriter, "  warning: %s\n", msg)
			linkRemoveErrs = append(linkRemoveErrs, msg)
			continue
		}
		uninstalledFrom = append(uninstalledFrom, providerLinkLabel(link, providerNames))
	}
	if len(linkRemoveErrs) > 0 {
		return output.NewStructuredErrorDetail(output.ErrSystemIO, fmt.Sprintf("%q is still installed and was not removed", name), "Remove the provider link(s) manually, then retry 'syllago remove'", strings.Join(linkRemoveErrs, "; "))
	}

	removedPath := item.Path
	if err := catalog.RemoveLibraryItem(item.Path); err != nil {
		return output.NewStructuredErrorDetail(output.ErrSystemIO, "removing from library failed", "Check filesystem permissions", err.Error())
	}

	if output.JSON {
		output.Print(removeResult{
			Name:            name,
			Type:            string(item.Type),
			UninstalledFrom: uninstalledFrom,
			RemovedPath:     removedPath,
		})
		return nil
	}

	if !output.Quiet {
		if len(uninstalledFrom) > 0 {
			fmt.Fprintf(output.Writer, "Uninstalled from: %s\n", strings.Join(uninstalledFrom, ", "))
		}
		fmt.Fprintf(output.Writer, "Removed %q from library (%s).\n", name, item.Type.Label())
		fmt.Fprintf(output.Writer, "\n  Next: syllago add <type>/<name> --from <provider>    (re-add to library)\n")
	}

	telemetry.Enrich("content_type", typeFilter)
	telemetry.Enrich("dry_run", dryRun)
	return nil
}

func providerNamesBySlug(providers []provider.Provider) map[string]string {
	names := make(map[string]string, len(providers))
	for _, prov := range providers {
		if prov.Slug == "" {
			continue
		}
		names[prov.Slug] = prov.Name
	}
	return names
}

func providerLinkLabel(link installer.ScannedLink, names map[string]string) string {
	name := names[link.Provider]
	if name == "" {
		name = link.Provider
	}
	return fmt.Sprintf("%s (%s)", name, filepath.Base(link.Path))
}

func extraRemoveProviderLinks(item catalog.ContentItem, links []installer.ScannedLink, installedByProvider map[string]bool) []installer.ScannedLink {
	expectedLeaf := expectedRemoveInstallLeaf(item)
	extra := make([]installer.ScannedLink, 0, len(links))
	for _, link := range links {
		if installedByProvider[link.Provider] && link.ContentType == item.Type && filepath.Base(link.Path) == expectedLeaf {
			continue
		}
		extra = append(extra, link)
	}
	return extra
}

func expectedRemoveInstallLeaf(item catalog.ContentItem) string {
	if item.Type == catalog.Agents {
		return item.Name + ".md"
	}
	if item.Type.IsUniversal() {
		return item.Name
	}
	return filepath.Base(item.Path)
}
