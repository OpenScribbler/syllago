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
	"github.com/spf13/cobra"
)

var loadoutApplyCmd = &cobra.Command{
	Use:   "apply [name]",
	Short: "Apply a loadout",
	Long: `Apply a loadout to configure the current project for Claude Code.

Default mode (no flags): preview what would happen without making changes.
--try: apply temporarily; reverts automatically when the session ends.
--keep: apply permanently; run "syllago loadout remove" to undo.`,
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
		return fmt.Errorf("could not find syllago repo: %w", err)
	}
	if projectRoot == "" {
		projectRoot = root
	}

	cat, err := catalog.Scan(root, projectRoot)
	if err != nil {
		return fmt.Errorf("scanning catalog: %w", err)
	}

	// Collect loadout items from catalog
	var loadoutItems []catalog.ContentItem
	for _, item := range cat.Items {
		if item.Type == catalog.Loadouts {
			loadoutItems = append(loadoutItems, item)
		}
	}

	if len(loadoutItems) == 0 {
		fmt.Fprintln(output.ErrWriter, "No loadouts found in catalog.")
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
			output.PrintError(1, fmt.Sprintf("loadout %q not found", name), "Run 'syllago loadout list' to see available loadouts.")
			return output.SilentError(fmt.Errorf("loadout not found: %s", name))
		}
	} else {
		// Interactive selection
		if !isInteractive() {
			return fmt.Errorf("no loadout name provided and stdin is not a terminal; pass a loadout name as argument")
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
			return fmt.Errorf("no selection made")
		}
		choice, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
		if err != nil || choice < 1 || choice > len(loadoutItems) {
			return fmt.Errorf("invalid selection: %s", scanner.Text())
		}
		selected = loadoutItems[choice-1]
	}

	// Parse loadout.yaml
	manifest, err := loadout.Parse(filepath.Join(selected.Path, "loadout.yaml"))
	if err != nil {
		return fmt.Errorf("parsing loadout: %w", err)
	}

	// Determine mode
	tryMode, _ := cmd.Flags().GetBool("try")
	keepMode, _ := cmd.Flags().GetBool("keep")
	previewMode, _ := cmd.Flags().GetBool("preview")

	mode := "preview"
	if tryMode && keepMode {
		return fmt.Errorf("--try and --keep are mutually exclusive")
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
			return fmt.Errorf("a loadout is already active. Run 'syllago loadout remove' first")
		} else if !errors.Is(snapErr, snapshot.ErrNoSnapshot) {
			return fmt.Errorf("checking existing snapshot: %w", snapErr)
		}
	}

	// Build resolver from merged config + CLI flag.
	baseDir, _ := cmd.Flags().GetString("base-dir")
	globalCfg, cfgErr := config.LoadGlobal()
	if cfgErr != nil {
		return fmt.Errorf("loading global config: %w", cfgErr)
	}
	projectCfg, cfgErr := config.Load(projectRoot)
	if cfgErr != nil {
		return fmt.Errorf("loading project config: %w", cfgErr)
	}
	mergedCfg := config.Merge(globalCfg, projectCfg)
	resolver := config.NewResolver(mergedCfg, baseDir)
	if err := resolver.ExpandPaths(); err != nil {
		return fmt.Errorf("expanding paths: %w", err)
	}

	// Resolve target provider: --to flag > manifest field > default to claude-code
	toSlug, _ := cmd.Flags().GetString("to")
	methodStr, _ := cmd.Flags().GetString("method")
	method := installer.MethodSymlink
	if methodStr == "copy" {
		method = installer.MethodCopy
	}

	var prov provider.Provider
	if toSlug != "" {
		p := findProviderBySlug(toSlug)
		if p == nil {
			slugs := providerSlugs()
			output.PrintError(1, "unknown provider: "+toSlug,
				"Available: "+strings.Join(slugs, ", "))
			return output.SilentError(fmt.Errorf("unknown provider: %s", toSlug))
		}
		prov = *p
	} else if manifest.Provider != "" {
		p := findProviderBySlug(manifest.Provider)
		if p == nil {
			return fmt.Errorf("loadout manifest specifies unknown provider: %s", manifest.Provider)
		}
		prov = *p
	} else {
		// Default to ClaudeCode for backwards compatibility
		prov = provider.ClaudeCode
	}

	opts := loadout.ApplyOptions{
		Mode:        mode,
		Method:      method,
		ProjectRoot: projectRoot,
		RepoRoot:    root,
		Resolver:    resolver,
	}

	result, err := loadout.Apply(manifest, cat, prov, opts)
	if err != nil {
		return fmt.Errorf("applying loadout: %w", err)
	}

	// Print results
	if output.JSON {
		output.Print(result)
		return nil
	}

	if mode == "preview" {
		fmt.Fprintf(output.Writer, "Preview for loadout %q:\n\n", manifest.Name)
	} else {
		fmt.Fprintf(output.Writer, "Applied loadout %q (%s mode):\n\n", manifest.Name, mode)
	}

	for _, action := range result.Actions {
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

	for _, w := range result.Warnings {
		fmt.Fprintf(output.ErrWriter, "\nWarning: %s\n", w)
	}

	if mode == "try" {
		fmt.Fprintln(output.Writer, "\nThis loadout is temporary. It will auto-revert when the session ends.")
		fmt.Fprintln(output.Writer, "If auto-revert fails, run: syllago loadout remove")
	}

	return nil
}
