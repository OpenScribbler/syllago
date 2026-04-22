package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/loadout"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/snapshot"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

var loadoutApplyCmd = &cobra.Command{
	Use:   "apply [name]",
	Short: "Apply a loadout",
	Long: `Apply a loadout to configure the current project for Claude Code.

Default mode (no flags): preview what would happen without making changes.
--try: apply temporarily; reverts automatically when the session ends.
--keep: apply permanently; run "syllago loadout remove" to undo.`,
	Example: `  # Preview what a loadout would do
  syllago loadout apply my-loadout

  # Try temporarily (auto-reverts on session end)
  syllago loadout apply my-loadout --try

  # Apply permanently
  syllago loadout apply my-loadout --keep

  # Apply to a specific provider
  syllago loadout apply my-loadout --keep --to cursor`,
	Args: cobra.MaximumNArgs(1),
	RunE: runLoadoutApply,
}

func init() {
	loadoutApplyCmd.Flags().Bool("try", false, "Apply temporarily; auto-revert on session end")
	loadoutApplyCmd.Flags().Bool("keep", false, "Apply permanently")
	loadoutApplyCmd.Flags().Bool("preview", false, "Dry run: show planned actions without applying")
	loadoutApplyCmd.Flags().String("base-dir", "", "Override base directory for content installation")
	loadoutApplyCmd.Flags().String("to", "", "Target provider (overrides manifest provider; defaults to claude-code if unset)")
	loadoutApplyCmd.Flags().String("method", "symlink", "Install method: symlink (default) or copy")
	loadoutCmd.AddCommand(loadoutApplyCmd)
}

func runLoadoutApply(cmd *cobra.Command, args []string) error {
	projectRoot, _ := findProjectRoot()
	checkAndWarnStaleSnapshot(projectRoot)

	root, err := findContentRepoRoot()
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrCatalogNotFound, "could not find syllago repo", "Run 'syllago init' to create a project", err.Error())
	}
	if projectRoot == "" {
		projectRoot = root
	}

	cat, err := catalog.Scan(root, projectRoot)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrCatalogScanFailed, "scanning catalog", "Check file permissions", err.Error())
	}

	// Collect loadout items from catalog
	var loadoutItems []catalog.ContentItem
	for _, item := range cat.Items {
		if item.Type == catalog.Loadouts {
			loadoutItems = append(loadoutItems, item)
		}
	}

	if len(loadoutItems) == 0 {
		fmt.Fprintln(output.ErrWriter, "No loadouts found in library.")
		return nil
	}

	// Determine which loadout to apply
	var selected catalog.ContentItem
	if len(args) == 1 {
		name := args[0]
		found := false
		for _, item := range loadoutItems {
			if item.Name == name {
				selected = item
				found = true
				break
			}
		}
		if !found {
			return output.NewStructuredError(output.ErrLoadoutNotFound, fmt.Sprintf("loadout %q not found", name), "Run 'syllago loadout list' to see available loadouts")
		}
	} else {
		// Interactive selection
		if !isInteractive() {
			return output.NewStructuredError(output.ErrInputTerminal, "no loadout name provided and stdin is not a terminal", "Pass a loadout name as argument")
		}
		fmt.Fprintln(output.Writer, "Available loadouts:")
		for i, item := range loadoutItems {
			desc := item.Description
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}
			fmt.Fprintf(output.Writer, "  %d) %s — %s\n", i+1, item.Name, desc)
		}
		fmt.Fprintf(output.Writer, "\nSelect a loadout [1-%d]: ", len(loadoutItems))

		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			return output.NewStructuredError(output.ErrInputTerminal, "no selection made", "Select a loadout number")
		}
		choice, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
		if err != nil || choice < 1 || choice > len(loadoutItems) {
			return output.NewStructuredError(output.ErrInputInvalid, fmt.Sprintf("invalid selection: %s", scanner.Text()), fmt.Sprintf("Enter a number between 1 and %d", len(loadoutItems)))
		}
		selected = loadoutItems[choice-1]
	}

	// Parse loadout.yaml
	manifest, err := loadout.Parse(filepath.Join(selected.Path, "loadout.yaml"))
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrLoadoutParse, "parsing loadout", "Check loadout.yaml syntax", err.Error())
	}

	// Determine mode
	tryMode, _ := cmd.Flags().GetBool("try")
	keepMode, _ := cmd.Flags().GetBool("keep")
	previewMode, _ := cmd.Flags().GetBool("preview")

	mode := "preview"
	if tryMode && keepMode {
		return output.NewStructuredError(output.ErrInputConflict, "--try and --keep are mutually exclusive", "Use one or the other")
	}
	if keepMode {
		mode = "keep"
	} else if tryMode {
		mode = "try"
	} else if previewMode {
		mode = "preview"
	}

	// For try/keep modes, check for existing active snapshot
	if mode == "try" || mode == "keep" {
		_, _, snapErr := snapshot.Load(projectRoot)
		if snapErr == nil {
			// A snapshot exists — loadout is already active
			return output.NewStructuredError(output.ErrLoadoutConflict, "a loadout is already active", "Run 'syllago loadout remove' first")
		} else if !errors.Is(snapErr, snapshot.ErrNoSnapshot) {
			return output.NewStructuredErrorDetail(output.ErrSystemIO, "checking existing snapshot", "Check .syllago/ directory permissions", snapErr.Error())
		}
	}

	// Build resolver from merged config + CLI flag.
	baseDir, _ := cmd.Flags().GetString("base-dir")
	globalCfg, cfgErr := config.LoadGlobal()
	if cfgErr != nil {
		return output.NewStructuredErrorDetail(output.ErrConfigInvalid, "loading global config", "Check ~/.syllago/config.json syntax", cfgErr.Error())
	}
	projectCfg, cfgErr := config.Load(projectRoot)
	if cfgErr != nil {
		return output.NewStructuredErrorDetail(output.ErrConfigNotFound, "loading project config", "Run 'syllago init' to create project config", cfgErr.Error())
	}
	mergedCfg := config.Merge(globalCfg, projectCfg)
	resolver := config.NewResolver(mergedCfg, baseDir)
	if err := resolver.ExpandPaths(); err != nil {
		return output.NewStructuredErrorDetail(output.ErrConfigPath, "expanding paths", "Check path overrides in config", err.Error())
	}

	toSlug, _ := cmd.Flags().GetString("to")
	methodStr, _ := cmd.Flags().GetString("method")
	method := installer.MethodSymlink
	if methodStr == "copy" {
		method = installer.MethodCopy
	}

	opts := loadout.ApplyOptions{
		Mode:        mode,
		Method:      method,
		ProjectRoot: projectRoot,
		RepoRoot:    root,
		Resolver:    resolver,
	}

	// Multi-provider path: manifest declares providers[] and --to is not set.
	// Apply to each listed provider independently; each gets its own snapshot.
	// Run 'syllago loadout remove' once per provider to undo.
	effectiveProviders := manifest.EffectiveProviders()
	if toSlug == "" && len(effectiveProviders) > 1 {
		totalActions := 0
		for _, slug := range effectiveProviders {
			p := findProviderBySlug(slug)
			if p == nil {
				fmt.Fprintf(output.ErrWriter, "Warning: unknown provider %q in loadout manifest — skipping\n", slug)
				continue
			}
			result, err := loadout.Apply(manifest, cat, *p, opts)
			if err != nil {
				return output.NewStructuredErrorDetail(output.ErrInstallConflict,
					fmt.Sprintf("applying loadout to %s", slug),
					"Check error details and resolve conflicts", err.Error())
			}
			if !output.JSON {
				if mode == "preview" {
					fmt.Fprintf(output.Writer, "[%s] Preview:\n", slug)
				} else {
					fmt.Fprintf(output.Writer, "[%s] Applied (%s mode):\n", slug, mode)
				}
				printLoadoutActions(result.Actions)
				for _, w := range result.Warnings {
					fmt.Fprintf(output.ErrWriter, "  Warning: %s\n", w)
				}
				fmt.Fprintln(output.Writer)
			} else {
				output.Print(result)
			}
			totalActions += len(result.Actions)
		}
		if mode == "try" {
			fmt.Fprintln(output.Writer, "This loadout is temporary. It will auto-revert when the session ends.")
			fmt.Fprintln(output.Writer, "If auto-revert fails, run: syllago loadout remove")
		}
		telemetry.Enrich("provider", strings.Join(effectiveProviders, ","))
		telemetry.Enrich("mode", mode)
		telemetry.Enrich("action_count", totalActions)
		return nil
	}

	// Single-provider path: --to flag > manifest.Providers[0] > manifest.Provider > default claude-code
	var prov provider.Provider
	targetSlug := toSlug
	if targetSlug == "" && len(effectiveProviders) == 1 {
		targetSlug = effectiveProviders[0]
	}
	if targetSlug != "" {
		p := findProviderBySlug(targetSlug)
		if p == nil {
			slugs := providerSlugs()
			return output.NewStructuredError(output.ErrProviderNotFound, "unknown provider: "+targetSlug, "Available: "+strings.Join(slugs, ", "))
		}
		prov = *p
	} else {
		// Default to ClaudeCode for backwards compatibility
		prov = provider.ClaudeCode
	}

	result, err := loadout.Apply(manifest, cat, prov, opts)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrInstallConflict, "applying loadout", "Check error details and resolve conflicts", err.Error())
	}

	if output.JSON {
		output.Print(result)
		return nil
	}

	if mode == "preview" {
		fmt.Fprintf(output.Writer, "Preview for loadout %q:\n\n", manifest.Name)
	} else {
		fmt.Fprintf(output.Writer, "Applied loadout %q (%s mode):\n\n", manifest.Name, mode)
	}

	printLoadoutActions(result.Actions)

	for _, w := range result.Warnings {
		fmt.Fprintf(output.ErrWriter, "\nWarning: %s\n", w)
	}

	if mode == "try" {
		fmt.Fprintln(output.Writer, "\nThis loadout is temporary. It will auto-revert when the session ends.")
		fmt.Fprintln(output.Writer, "If auto-revert fails, run: syllago loadout remove")
	}

	telemetry.Enrich("provider", prov.Slug)
	telemetry.Enrich("mode", mode)
	telemetry.Enrich("action_count", len(result.Actions))
	return nil
}

// printLoadoutActions prints a formatted list of planned actions.
func printLoadoutActions(actions []loadout.PlannedAction) {
	for _, action := range actions {
		symbol := "  "
		switch action.Action {
		case "create-symlink":
			symbol = "+ "
		case "merge-hook", "merge-mcp":
			symbol = "* "
		case "skip-exists":
			symbol = "= "
		case "error-conflict":
			symbol = "! "
		}
		fmt.Fprintf(output.Writer, "  %s%-10s %-25s %s\n", symbol, action.Type.Label(), action.Name, action.Detail)
		if action.Problem != "" {
			fmt.Fprintf(output.Writer, "    %s\n", action.Problem)
		}
	}
}
