package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/lifecycle"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

// uninstallResult is the JSON-serializable output for syllago uninstall.
type uninstallResult struct {
	Name            string   `json:"name"`
	UninstalledFrom []string `json:"uninstalled_from"`
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall <name>",
	Short: "Deactivate content from a provider",
	Long: `Removes installed content from a provider's location.

For symlinked content: removes the symlink.
For copied content: removes the copied file or directory.
For hooks/MCP: reverses the JSON merge from the provider's settings file.

The content remains in your library (~/.syllago/content/) and can be
reinstalled at any time with "syllago install".`,
	Example: `  # Uninstall from a specific provider
  syllago uninstall my-skill --from claude-code

  # Uninstall from all providers
  syllago uninstall my-agent

  # Skip confirmation prompt
  syllago uninstall my-rule --from cursor --force

  # Preview what would happen
  syllago uninstall my-skill --dry-run`,
	Args: cobra.ExactArgs(1),
	RunE: runUninstall,
}

func init() {
	uninstallCmd.Flags().String("from", "", "Provider to uninstall from (omit to uninstall from all)")
	uninstallCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	uninstallCmd.Flags().BoolP("dry-run", "n", false, "Show what would happen without making changes")
	uninstallCmd.Flags().Bool("no-input", false, "Disable interactive prompts, use defaults")
	uninstallCmd.Flags().String("type", "", "Disambiguate when name exists in multiple types")
	rootCmd.AddCommand(uninstallCmd)
}

func runUninstall(cmd *cobra.Command, args []string) error {
	name := args[0]
	fromSlug, _ := cmd.Flags().GetString("from")
	fromSlug = provider.CanonicalSlug(fromSlug)
	force, _ := cmd.Flags().GetBool("force")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	noInput, _ := cmd.Flags().GetBool("no-input")
	typeFilter, _ := cmd.Flags().GetString("type")

	globalDir := catalog.GlobalContentDir()
	if globalDir == "" {
		return output.NewStructuredError(output.ErrSystemHomedir, "cannot determine home directory", "Set the HOME environment variable")
	}
	projectRoot, _ := findProjectRoot()

	// D7 routing: if installed.json has a matching RuleAppend record, the
	// rule was installed via --method=append and the uninstall must route
	// through UninstallRuleAppend (exact-match byte search). The generic
	// scan-and-symlink-remove path below cannot handle monolithic appends.
	if handled, err := tryUninstallMonolithicRule(name, fromSlug, typeFilter, dryRun, globalDir); err != nil || handled {
		return err
	}

	// Use an empty temp dir as contentRoot to avoid scan shadowing.
	// When contentRoot == globalDir, items get tagged "project" instead of "global".
	emptyRoot, err := os.MkdirTemp("", "syllago-uninstall-*")
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrSystemIO, "creating temp dir", "Check filesystem permissions", err.Error())
	}
	defer func() { _ = os.RemoveAll(emptyRoot) }()

	cat, err := catalog.ScanWithGlobalAndRegistries(emptyRoot, emptyRoot, nil)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrCatalogScanFailed, "scanning library", "Check file permissions in ~/.syllago/content/", err.Error())
	}

	// Find the item in the global library
	var item *catalog.ContentItem
	for i := range cat.Items {
		if cat.Items[i].Source != "global" || cat.Items[i].Name != name {
			continue
		}
		if typeFilter != "" && string(cat.Items[i].Type) != typeFilter {
			continue
		}
		item = &cat.Items[i]
		break
	}
	if item == nil {
		return output.NewStructuredError(output.ErrInstallItemNotFound,
			fmt.Sprintf("no item named %q found in your library", name),
			"Hint: syllago list    (show all library items)")
	}

	// Resolve install paths the way install does, so an item placed under a
	// configured base directory or per-type path is found and removed.
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrConfigInvalid, "loading global config", "Check ~/.syllago/config.json syntax", err.Error())
	}
	projectCfg, err := config.Load(projectRoot)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrConfigInvalid, "loading project config", "Check .syllago/config.json syntax", err.Error())
	}
	resolver := config.NewResolver(config.Merge(globalCfg, projectCfg), "")
	if err := resolver.ExpandPaths(); err != nil {
		return output.NewStructuredErrorDetail(output.ErrConfigPath, "expanding paths", "Check path overrides in config", err.Error())
	}

	// Determine which providers to uninstall from
	var targets []provider.Provider
	if fromSlug != "" {
		prov := findProviderBySlug(fromSlug)
		if prov == nil {
			slugs := providerSlugs()
			return output.NewStructuredError(output.ErrProviderNotFound, "unknown provider: "+fromSlug, "Available: "+strings.Join(slugs, ", "))
		}
		// Verify it is actually installed there
		status := installer.CheckStatusWithResolver(*item, *prov, projectRoot, resolver)
		if status != installer.StatusInstalled {
			return output.NewStructuredError(output.ErrInstallNotInstalled,
				fmt.Sprintf("%q is not installed in %s", name, prov.Name),
				"Run 'syllago list --installed' to see installed items")
		}
		targets = []provider.Provider{*prov}
	} else {
		// Uninstall from all providers where it is currently installed
		for _, prov := range provider.AllProviders {
			status := installer.CheckStatusWithResolver(*item, prov, projectRoot, resolver)
			if status == installer.StatusInstalled {
				targets = append(targets, prov)
			}
		}
		if len(targets) == 0 {
			return output.NewStructuredError(output.ErrInstallNotInstalled,
				fmt.Sprintf("%q is not installed in any provider", name),
				"Run 'syllago list --installed' to see installed items")
		}
	}

	// Build a summary of what will be affected
	var targetNames []string
	for _, prov := range targets {
		targetNames = append(targetNames, prov.Name)
	}

	// Confirm unless --force, --dry-run, --no-input, or non-interactive
	if !force && !dryRun && !noInput && isInteractive() {
		fmt.Fprintf(output.Writer, "This will uninstall %q from: %s\n", name, strings.Join(targetNames, ", "))
		fmt.Fprintf(output.Writer, "  The content stays in your library.\n")
		fmt.Fprintf(output.Writer, "\nContinue? [y/N] ")
		var answer string
		fmt.Scanln(&answer)
		if strings.ToLower(strings.TrimSpace(answer)) != "y" {
			fmt.Fprintln(output.Writer, "Cancelled.")
			return nil
		}
	}

	if dryRun {
		for _, prov := range targets {
			fmt.Fprintf(output.Writer, "[dry-run] Would uninstall %q from %s\n", name, prov.Name)
		}
		return nil
	}

	// Perform uninstall
	lcTargets := make([]lifecycle.Target, len(targets))
	for i, prov := range targets {
		lcTargets[i] = lifecycle.Target{Provider: prov, Resolver: resolver}
	}
	outcome, err := lifecycle.New().Uninstall(lifecycle.UninstallRequest{Item: *item, ProjectRoot: projectRoot, Targets: lcTargets})
	if len(outcome.Unattempted) > 0 {
		return err
	}
	printUninstallFailures(outcome, func(p provider.Provider) string { return p.Name })
	var removedFrom []string
	for _, step := range outcome.Completed {
		removedFrom = append(removedFrom, step.Target.Provider.Name)
		if !output.JSON && !output.Quiet {
			if desc := step.Placement.String(); desc != "" {
				fmt.Fprintf(output.Writer, "Removed %s\n", desc)
			} else {
				fmt.Fprintf(output.Writer, "Removed from %s\n", step.Target.Provider.Name)
			}
		}
	}

	if output.JSON {
		output.Print(uninstallResult{Name: name, UninstalledFrom: removedFrom})
		return nil
	}

	if len(removedFrom) > 0 && !output.Quiet {
		fmt.Fprintf(output.Writer, "\n  %q is still in your library.\n", name)
		fmt.Fprintf(output.Writer, "  Remove with: syllago remove %s\n", name)
	}

	telemetry.Enrich("provider", fromSlug)
	telemetry.Enrich("content_type", typeFilter)
	telemetry.Enrich("dry_run", dryRun)
	return nil
}

// tryUninstallMonolithicRule handles D7 uninstall for rules installed via
// --method=append. Returns (handled=true, err) when the name matches at least
// one RuleAppend record in installed.json; the caller then short-circuits.
// Returns (handled=false, nil) when no matching record exists — the normal
// catalog-driven uninstall path takes over.
//
// typeFilter restricts matching to --type=rules when set; when empty, any
// RuleAppend record with matching Name qualifies (names are namespaced under
// the "rules" type by construction in install_cmd_append.go).
func tryUninstallMonolithicRule(name, fromSlug, typeFilter string, dryRun bool, globalDir string) (bool, error) {
	if typeFilter != "" && typeFilter != string(catalog.Rules) {
		return false, nil
	}
	projectRoot, err := findProjectRoot()
	if err != nil {
		return false, nil // let generic path surface the error in context
	}
	inst, err := installer.LoadInstalled(projectRoot)
	if err != nil {
		return false, nil // same — defer to generic path
	}
	// Match all records whose Name == name, optionally filtered by provider.
	var matches []installer.InstalledRuleAppend
	for _, r := range inst.RuleAppends {
		if r.Name != name {
			continue
		}
		if fromSlug != "" && r.Provider != fromSlug {
			continue
		}
		matches = append(matches, r)
	}
	if len(matches) == 0 {
		return false, nil
	}
	if dryRun {
		for _, r := range matches {
			fmt.Fprintf(output.Writer, "[dry-run] Would remove %s from %s\n", r.Name, r.TargetFile)
		}
		telemetry.Enrich("provider", fromSlug)
		telemetry.Enrich("content_type", string(catalog.Rules))
		telemetry.Enrich("dry_run", true)
		return true, nil
	}
	// The matches share one library rule: the name finds it, and D14
	// uniqueness is per (LibraryID, TargetFile), so one rule has many targets.
	rulesRoot := filepath.Join(globalDir, string(catalog.Rules))
	dir, derr := findLibraryRuleDir(rulesRoot, name)
	if derr != nil {
		return true, output.NewStructuredErrorDetail(
			output.ErrInstallItemNotFound,
			fmt.Sprintf("library rule %q not found for uninstall", name),
			"Library rules required for D7 exact-match uninstall live under ~/.syllago/content/rules/<source>/<name>/",
			derr.Error(),
		)
	}
	if _, lerr := rulestore.LoadRule(dir); lerr != nil {
		return true, output.NewStructuredErrorDetail(
			output.ErrCatalogScanFailed,
			"loading library rule for uninstall",
			"Check .syllago.yaml and .history/ in the library rule dir.",
			lerr.Error(),
		)
	}
	// installed.json names providers by slug, and the slug is all the
	// append path needs.
	var targets []lifecycle.Target
	for _, r := range matches {
		targets = append(targets, lifecycle.Target{Provider: provider.Provider{Name: r.Provider, Slug: r.Provider}})
	}
	outcome, err := lifecycle.New().Uninstall(lifecycle.UninstallRequest{
		Item:        appendRuleItem(dir),
		ProjectRoot: projectRoot,
		Targets:     targets,
		Method:      installer.MethodAppend,
	})
	if len(outcome.Unattempted) > 0 {
		return true, err
	}
	printUninstallFailures(outcome, func(p provider.Provider) string { return p.Slug })
	var uninstalledFrom []string
	for _, step := range outcome.Completed {
		uninstalledFrom = append(uninstalledFrom, step.Target.Provider.Slug)
		if !output.JSON && !output.Quiet {
			fmt.Fprintf(output.Writer, "Removed %s from %s\n", name, step.Placement.Path)
		}
	}
	if output.JSON {
		output.Print(uninstallResult{Name: name, UninstalledFrom: uninstalledFrom})
	} else if len(uninstalledFrom) > 0 && !output.Quiet {
		fmt.Fprintf(output.Writer, "\n  %q is still in your library.\n", name)
	}
	telemetry.Enrich("provider", fromSlug)
	telemetry.Enrich("content_type", string(catalog.Rules))
	telemetry.Enrich("dry_run", dryRun)
	return true, nil
}

// printUninstallFailures warns about each target an uninstall could not
// change, naming it with label, and about any record left out of date.
func printUninstallFailures(o lifecycle.Outcome, label func(provider.Provider) string) {
	for _, f := range o.Failed {
		if f.Stage == lifecycle.StagePlace {
			fmt.Fprintf(output.ErrWriter, "  warning: failed to uninstall from %s: %s\n", label(f.Target.Provider), f.Err)
		}
	}
	printLifecycleWarnings(output.ErrWriter, o)
}
