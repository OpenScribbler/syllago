package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/audit"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/lifecycle"
	"github.com/OpenScribbler/syllago/cli/internal/moatinstall"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

// resolveConflict is called when DetectConflicts finds install-path collisions.
// Override in tests to avoid blocking on stdin.
var resolveConflict = resolveConflictInteractively

// resolveConflictInteractively prints a conflict warning and prompts the user
// to choose how to resolve it. Returns the chosen ConflictResolution.
func resolveConflictInteractively(conflicts []installer.Conflict, w, errW io.Writer) (installer.ConflictResolution, error) {
	fmt.Fprintln(errW, "\nWarning: install path conflict detected")
	fmt.Fprintln(errW, "The following providers share a read path with another provider's install directory:")
	for _, c := range conflicts {
		readers := make([]string, len(c.AlsoReadBy))
		for i, r := range c.AlsoReadBy {
			readers[i] = r.Name
		}
		fmt.Fprintf(errW, "  %s installs to %s (also read by: %s)\n",
			c.InstallingTo.Name, c.SharedPath, strings.Join(readers, ", "))
	}
	fmt.Fprintln(w, "\nHow would you like to resolve this?")
	fmt.Fprintln(w, "  [1] Shared path only  — install once to shared path; other providers read it automatically")
	fmt.Fprintln(w, "  [2] Own dirs only     — each provider gets its own directory; skip the shared path owner")
	fmt.Fprintln(w, "  [3] All paths         — install everywhere (may cause duplicate content warnings)")
	fmt.Fprint(w, "\nChoice [1/2/3] (default: 3): ")

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		switch strings.TrimSpace(scanner.Text()) {
		case "1":
			return installer.ResolutionSharedOnly, nil
		case "2":
			return installer.ResolutionOwnDirsOnly, nil
		}
	}
	return installer.ResolutionAll, nil
}

// installResult is the JSON-serializable output for syllago install.
type installResult struct {
	Installed []installedItem `json:"installed"`
	Skipped   []skippedItem   `json:"skipped,omitempty"`
}

type installedItem struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Method   string   `json:"method"`
	Path     string   `json:"path"`
	Warnings []string `json:"warnings,omitempty"` // portability warnings
	// Trust is the MOAT trust tier drill-down description, e.g. "Verified
	// (registry-attested)". Empty when the item was not sourced from a MOAT
	// manifest — absent key rather than empty string so JSON consumers can
	// distinguish "no trust data" from "empty trust data."
	Trust string `json:"trust,omitempty"`
}

type skippedItem struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

var installCmd = &cobra.Command{
	Use:   "install [name]",
	Short: "Activate library content in a provider",
	Long: `Install content from your library into a provider's location.

By default uses a symlink so edits in your library are reflected immediately.
Use --method copy to place a standalone copy instead.

Hooks and MCP configs are merged into the provider's settings file rather than linked.`,
	Example: `  # Install a skill to Claude Code
  syllago install my-skill --to claude-code

  # Install with a standalone copy instead of symlink
  syllago install my-skill --to cursor --method copy

  # Install all skills to a provider
  syllago install --to claude-code --type skills

  # Install to every detected provider at once
  syllago install --type skills --to-all

  # Install everything from your library
  syllago install --all --to claude-code

  # Preview what would happen
  syllago install my-skill --to claude-code --dry-run`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInstall,
}

func init() {
	installCmd.Flags().String("to", "", "Provider to install into")
	// Note: --to is no longer MarkFlagRequired — mutual exclusion with --to-all
	// is enforced at runtime in RunE.
	installCmd.Flags().Bool("to-all", false, "Install to all detected providers")
	installCmd.Flags().String("type", "", "Filter to a specific content type")
	installCmd.Flags().String("method", "symlink", "Install method: symlink (default), copy, or append (monolithic-file rules)")
	installCmd.Flags().Bool("all", false, "Install all library content (cannot combine with a positional name)")
	installCmd.Flags().BoolP("dry-run", "n", false, "Show what would happen without making changes")
	installCmd.Flags().String("base-dir", "", "Override base directory for content installation")
	installCmd.Flags().Bool("no-input", false, "Disable interactive prompts, use defaults")
	installCmd.Flags().StringSlice("hook-scanner", nil, "Path to external hook scanner binary (repeatable)")
	installCmd.Flags().Bool("force", false, "Proceed past high-severity scanner findings")
	installCmd.Flags().Bool("frozen", false, "Pin the install record created by this command")
	// D17 re-install decision flags for --method=append. Values validated in
	// runInstall so bad inputs error before any disk work. See D17 flag table.
	installCmd.Flags().String("on-clean", "", "Action when rule is already installed cleanly: replace|skip")
	installCmd.Flags().String("on-modified", "", "Action when install record is stale: drop-record|append-fresh|keep")
	rootCmd.AddCommand(installCmd)
}

// validateOnCleanFlag checks that --on-clean is either empty or one of the
// D17-sanctioned values. Returns a structured error otherwise.
func validateOnCleanFlag(value string) error {
	switch value {
	case "", "replace", "skip":
		return nil
	}
	return output.NewStructuredError(
		output.ErrInputConflict,
		"invalid --on-clean value",
		"Allowed values: replace|skip",
	)
}

// validateOnModifiedFlag checks that --on-modified is either empty or one of
// the D17-sanctioned values.
func validateOnModifiedFlag(value string) error {
	switch value {
	case "", "drop-record", "append-fresh", "keep":
		return nil
	}
	return output.NewStructuredError(
		output.ErrInputConflict,
		"invalid --on-modified value",
		"Allowed values: drop-record|append-fresh|keep",
	)
}

func runInstall(cmd *cobra.Command, args []string) error {
	toSlug, _ := cmd.Flags().GetString("to")
	toSlug = provider.CanonicalSlug(toSlug)
	toAll, _ := cmd.Flags().GetBool("to-all")
	typeFilter, _ := cmd.Flags().GetString("type")
	methodStr, _ := cmd.Flags().GetString("method")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	baseDir, _ := cmd.Flags().GetString("base-dir")
	installAll, _ := cmd.Flags().GetBool("all")
	scannerPaths, _ := cmd.Flags().GetStringSlice("hook-scanner")
	force, _ := cmd.Flags().GetBool("force")
	frozen, _ := cmd.Flags().GetBool("frozen")
	telemetry.Enrich("frozen", frozen)
	scan := installer.ScanOptions{Scanners: scannerPaths, Force: force}
	refreshInstallEntryPointsForInstall()

	// --to-all and --to are mutually exclusive.
	if toAll && toSlug != "" {
		return output.NewStructuredError(output.ErrInputConflict, "--to-all and --to are mutually exclusive", "Use --to-all to install to every detected provider, or --to <provider> for a specific one")
	}

	// Without --to-all, --to is required.
	if !toAll && toSlug == "" {
		slugs := providerSlugs()
		return output.NewStructuredError(output.ErrInputMissing, "--to is required (or use --to-all)", "Available providers: "+strings.Join(slugs, ", "))
	}

	// --all and a positional name are mutually exclusive.
	if installAll && len(args) > 0 {
		return output.NewStructuredError(output.ErrInputConflict, "cannot specify both a name and --all", "Use either a positional argument or --all, not both")
	}

	// Require explicit intent for bulk installs: name, --all, or --type.
	if len(args) == 0 && !installAll && typeFilter == "" {
		return output.NewStructuredError(output.ErrInputMissing, "specify a name, --all, or --type to install", "Examples:\n  syllago install my-skill --to <provider>\n  syllago install --all --to <provider>\n  syllago install --type rules --to <provider>")
	}

	method := installer.MethodSymlink
	if methodStr == "copy" {
		method = installer.MethodCopy
	}

	// --method=append routes to the monolithic-file install path (D5, D10, D14).
	// Only rules are supported at this scope; providers without a monolithic
	// filename are rejected. Per D14, records live in project-scoped
	// installed.json; the default target file is <projectRoot>/<filename>.
	if methodStr == "append" {
		onClean, _ := cmd.Flags().GetString("on-clean")
		onModified, _ := cmd.Flags().GetString("on-modified")
		if err := validateOnCleanFlag(onClean); err != nil {
			return err
		}
		if err := validateOnModifiedFlag(onModified); err != nil {
			return err
		}
		return runInstallAppend(cmd, args, toSlug, typeFilter)
	}

	// --to-all path: detect providers and delegate.
	if toAll {
		noInput, _ := cmd.Flags().GetBool("no-input")
		return runInstallToAll(cmd, args, typeFilter, methodStr, method, dryRun, baseDir, installAll, noInput, scan)
	}

	prov := findProviderBySlug(toSlug)
	if prov == nil {
		slugs := providerSlugs()
		return output.NewStructuredError(
			output.ErrProviderNotFound,
			"unknown provider: "+toSlug,
			"Available: "+strings.Join(slugs, ", "),
		)
	}

	// Build resolver from merged config + CLI flag.
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrConfigInvalid, "loading global config", "Check ~/.syllago/config.json syntax", err.Error())
	}
	projectRoot, _ := findProjectRoot()
	projectCfg, err := config.Load(projectRoot)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrConfigNotFound, "loading project config", "Run 'syllago init' to create project config", err.Error())
	}
	mergedCfg := config.Merge(globalCfg, projectCfg)
	resolver := config.NewResolver(mergedCfg, baseDir)
	if err := resolver.ExpandPaths(); err != nil {
		return output.NewStructuredErrorDetail(output.ErrConfigPath, "expanding paths", "Check path overrides in config", err.Error())
	}

	globalDir := catalog.GlobalContentDir()
	if globalDir == "" {
		return output.NewStructuredError(output.ErrSystemHomedir, "cannot determine home directory", "Set the HOME environment variable")
	}

	// `syllago install <registry>/<item>` installs from a MOAT registry
	// (see install_moat.go). The plain `syllago install <item>` form installs
	// from the Library and falls through to the globalDir scan below.
	if len(args) == 1 {
		if regName, itemName, ok := parseRegistryItemSyntax(args[0]); ok {
			ctx := cmd.Context()
			if frozen {
				ctx = ContextWithFrozen(ctx, true)
			}
			return runInstallFromRegistry(ctx, output.Writer, output.ErrWriter, moatinstall.Request{
				Registry:    regName,
				Items:       []moatinstall.Item{{Name: itemName}},
				ProjectRoot: projectRoot,
				Targets:     []lifecycle.Target{{Provider: *prov, BaseDir: baseDir}},
				Method:      method,
				Scan:        scan,
				DryRun:      dryRun,
			}, moatInstallNow())
		}
	}

	// Warn if the target provider is not detected on disk.
	if !output.JSON && !output.Quiet {
		detected := provider.DetectProvidersWithResolver(resolver)
		for _, dp := range detected {
			if dp.Slug == prov.Slug && !dp.Detected {
				fmt.Fprintf(output.ErrWriter, "Warning: %s not detected at default locations.\n", prov.Name)
				fmt.Fprintf(output.ErrWriter, "If installed at a custom path, configure it:\n")
				fmt.Fprintf(output.ErrWriter, "  syllago config paths set %s --base-dir /your/path\n", prov.Slug)
				break
			}
		}
	}

	// Scan global library only.
	globalCat, err := catalog.Scan(globalDir, globalDir)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrCatalogScanFailed, "scanning library", "Check file permissions in ~/.syllago/content/", err.Error())
	}

	var items []catalog.ContentItem
	for _, item := range globalCat.Items {
		if len(args) == 1 && item.Name != args[0] {
			continue
		}
		if typeFilter != "" && string(item.Type) != typeFilter {
			continue
		}
		items = append(items, item)
	}

	if len(items) == 0 {
		if len(args) == 1 {
			hint := registryAddHint(args[0], typeFilter, projectRoot)
			return output.NewStructuredError(output.ErrInstallItemNotFound, fmt.Sprintf("no item named %q found in your library", args[0]), hint)
		}
		fmt.Fprintln(output.ErrWriter, "no items found in library matching filters")
		return nil
	}

	items, refused, stop, refusal := gateLibraryItems(output.ErrWriter, items, mergedCfg, projectRoot)
	if stop {
		return nil
	}

	if len(items) > 1 && !output.Quiet && !output.JSON {
		fmt.Fprintf(output.Writer, "Installing %d items to %s...\n", len(items), prov.Name)
	}

	result, _ := installToProvider(items, *prov, method, dryRun, resolver, toSlug, projectRoot, frozen, scan)
	result.Skipped = append(refused, result.Skipped...)

	if output.JSON {
		output.Print(result)
		return refusal
	}

	if len(result.Installed) > 0 && !output.Quiet {
		fmt.Fprintf(output.Writer, "\n  # Install to another provider\n")
		fmt.Fprintf(output.Writer, "  syllago install %s --to <provider>\n", firstArg(args))
		fmt.Fprintf(output.Writer, "\n  # Convert for sharing\n")
		fmt.Fprintf(output.Writer, "  syllago convert %s --to <provider>\n", firstArg(args))
	}

	telemetry.Enrich("provider", toSlug)
	telemetry.Enrich("content_type", typeFilter)
	telemetry.Enrich("content_count", len(result.Installed))
	telemetry.Enrich("dry_run", dryRun)
	// An item the trust gate refused fails the command, so a script never
	// reads the refusal as success.
	return refusal
}

// installToProvider installs the given items to a single provider and returns
// the result. Returns (result, nil) in all normal cases — individual item
// failures are recorded in result.Skipped, not propagated as a return error.
func installToProvider(
	items []catalog.ContentItem,
	prov provider.Provider,
	method installer.InstallMethod,
	dryRun bool,
	resolver *config.PathResolver,
	toSlug string,
	projectRoot string,
	frozen bool,
	scan installer.ScanOptions,
) (installResult, error) {
	var result installResult

	for i, item := range items {
		if dryRun {
			if !output.Quiet {
				m := "symlink"
				if installer.IsJSONMerge(prov, item.Type) {
					m = "json-merge"
				} else if item.Type == catalog.Agents {
					m = "copy"
				}
				fmt.Fprintf(output.Writer, "[dry-run] would install %s (%s) to %s via %s\n", item.Name, item.Type.Label(), prov.Name, m)

				if installer.IsJSONMerge(prov, item.Type) {
					contentFile := converter.ResolveContentFile(item)
					if contentFile != "" {
						if preview, err := os.ReadFile(contentFile); err == nil {
							fmt.Fprintf(output.Writer, "  content preview:\n")
							for _, line := range strings.SplitN(string(preview), "\n", 10) {
								fmt.Fprintf(output.Writer, "    %s\n", line)
							}
						}
					}
				}
			}
			continue
		}

		outcome, err := lifecycle.New().Install(lifecycle.InstallRequest{
			Item:        item,
			ProjectRoot: projectRoot,
			Targets:     []lifecycle.Target{{Provider: prov, Resolver: resolver}},
			Method:      method,
			Scan:        scan,
			Frozen:      frozen,
		})
		printInstallNotices(output.ErrWriter, outcome.Notices)
		if len(outcome.Unattempted) > 0 {
			// Without the lock no later item can install either, so skip
			// them now rather than wait out the lock once per item.
			for _, rest := range items[i:] {
				result.Skipped = append(result.Skipped, skippedItem{Name: rest.Name, Reason: err.Error()})
			}
			if !output.JSON {
				fmt.Fprintf(output.ErrWriter, "  skip %d item(s): %s\n", len(items)-i, err)
			}
			break
		}
		if err != nil {
			result.Skipped = append(result.Skipped, skippedItem{Name: item.Name, Reason: err.Error()})
			if !output.JSON {
				fmt.Fprintf(output.ErrWriter, "  skip %s: %s\n", item.Name, err)
			}
			continue
		}
		printLifecycleWarnings(output.ErrWriter, outcome)
		placement := outcome.Completed[0].Placement
		desc := placement.String()
		// Report the method actually used: agents, cross-provider renders,
		// and Windows mounts are copied even when a symlink was requested.
		usedMethod := method
		if method == installer.MethodSymlink && placement.Mechanism == installer.MechanismCopy {
			usedMethod = installer.MethodCopy
		}

		// The installer already printed what the target loses; the JSON
		// result carries the same warnings.
		var warnings []string
		for _, n := range outcome.Notices {
			if n.Kind == installer.NoticeConversionWarning {
				warnings = append(warnings, strings.TrimPrefix(n.Message, item.Name+": "))
			}
		}

		trustText := catalog.TrustDescription(item.TrustTier, item.Revoked, item.RevocationReason)
		if trustText != "" && !output.JSON && !output.Quiet {
			badge := catalog.UserFacingBadge(item.TrustTier, item.Revoked)
			glyph := badge.Glyph()
			if glyph != "" {
				glyph += " "
			}
			fmt.Fprintf(output.Writer, "  %s%s — %s\n", glyph, item.Name, trustText)
		}

		result.Installed = append(result.Installed, installedItem{
			Name:     item.Name,
			Type:     string(item.Type),
			Method:   string(usedMethod),
			Path:     desc,
			Warnings: warnings,
			Trust:    trustText,
		})

		// Audit log (best-effort).
		if auditLogger, aErr := audit.NewLogger(audit.DefaultLogPath(projectRoot)); aErr == nil {
			_ = auditLogger.LogContent(audit.EventContentInstall, item.Name, string(item.Type), toSlug)
			_ = auditLogger.Close()
		}

		if !output.JSON && !output.Quiet {
			if usedMethod == installer.MethodSymlink {
				fmt.Fprintf(output.Writer, "  Symlinked %s to %s\n", item.Name, desc)
			} else {
				fmt.Fprintf(output.Writer, "  Copied %s to %s\n", item.Name, desc)
			}
		}
	}

	return result, nil
}

// providerInstallResult holds the per-provider outcome for --to-all.
type providerInstallResult struct {
	Provider string `json:"provider"`
	Slug     string `json:"slug"`
	Status   string `json:"status"` // "installed", "skipped", "failed", "no-items"
	Count    int    `json:"count,omitempty"`
	Details  string `json:"details,omitempty"`
}

// runInstallToAll handles the --to-all branch of runInstall.
// It detects providers, installs to each, and prints a summary.
// Returns a non-nil error if any provider had install failures.
func runInstallToAll(
	cmd *cobra.Command,
	args []string,
	typeFilter string,
	_ string,
	method installer.InstallMethod,
	dryRun bool,
	baseDir string,
	installAll bool,
	noInput bool,
	scan installer.ScanOptions,
) error {
	frozen, _ := cmd.Flags().GetBool("frozen")
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrConfigInvalid, "loading global config", "Check ~/.syllago/config.json syntax", err.Error())
	}
	projectRoot, _ := findProjectRoot()
	projectCfg, err := config.Load(projectRoot)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrConfigNotFound, "loading project config", "Run 'syllago init' to create project config", err.Error())
	}
	mergedCfg := config.Merge(globalCfg, projectCfg)
	resolver := config.NewResolver(mergedCfg, baseDir)
	if err := resolver.ExpandPaths(); err != nil {
		return output.NewStructuredErrorDetail(output.ErrConfigPath, "expanding paths", "Check path overrides in config", err.Error())
	}

	detected := provider.DetectProvidersWithResolver(resolver)
	var active []provider.Provider
	for _, p := range detected {
		if p.Detected {
			active = append(active, p)
		}
	}

	if len(active) == 0 {
		fmt.Fprintln(output.ErrWriter, "no providers detected — install an AI coding tool and retry")
		return nil
	}

	// Detect install-path conflicts (skills only — the only cross-provider shared path).
	if homeDir, err := os.UserHomeDir(); err == nil {
		conflicts := installer.DetectConflicts(active, catalog.Skills, homeDir)
		if len(conflicts) > 0 {
			if dryRun || noInput {
				// Non-interactive: warn and proceed with all providers (no filtering).
				fmt.Fprintln(output.ErrWriter, "\nWarning: install path conflict detected — use without --no-input to choose resolution")
				for _, c := range conflicts {
					readers := make([]string, len(c.AlsoReadBy))
					for i, r := range c.AlsoReadBy {
						readers[i] = r.Name
					}
					fmt.Fprintf(output.ErrWriter, "  %s installs to %s (also read by: %s)\n",
						c.InstallingTo.Name, c.SharedPath, strings.Join(readers, ", "))
				}
			} else {
				resolution, resolveErr := resolveConflict(conflicts, output.Writer, output.ErrWriter)
				if resolveErr != nil {
					return resolveErr
				}
				active = installer.ApplyConflictResolution(active, conflicts, resolution)
			}
		}
	}

	// `<registry>/<item>` installs the registry item to every active
	// provider, as the --to path installs it to one.
	if len(args) == 1 {
		if regName, itemName, ok := parseRegistryItemSyntax(args[0]); ok {
			ctx := cmd.Context()
			if frozen {
				ctx = ContextWithFrozen(ctx, true)
			}
			targets := make([]lifecycle.Target, 0, len(active))
			for _, p := range active {
				targets = append(targets, lifecycle.Target{Provider: p, Resolver: resolver})
			}
			return runInstallFromRegistry(ctx, output.Writer, output.ErrWriter, moatinstall.Request{
				Registry:    regName,
				Items:       []moatinstall.Item{{Name: itemName}},
				ProjectRoot: projectRoot,
				Targets:     targets,
				Method:      method,
				Scan:        scan,
				DryRun:      dryRun,
			}, moatInstallNow())
		}
	}

	globalDir := catalog.GlobalContentDir()
	if globalDir == "" {
		return output.NewStructuredError(output.ErrSystemHomedir, "cannot determine home directory", "Set the HOME environment variable")
	}
	globalCat, err := catalog.Scan(globalDir, globalDir)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrCatalogScanFailed, "scanning library", "Check file permissions in ~/.syllago/content/", err.Error())
	}

	var items []catalog.ContentItem
	for _, item := range globalCat.Items {
		if len(args) == 1 && item.Name != args[0] {
			continue
		}
		if typeFilter != "" && string(item.Type) != typeFilter {
			continue
		}
		if installAll || len(args) == 0 && typeFilter == "" {
			items = append(items, item)
			continue
		}
		items = append(items, item)
	}

	if len(items) == 0 {
		if len(args) == 1 {
			hint := typeFilter
			if hint == "" {
				hint = "skills"
			}
			return output.NewStructuredError(output.ErrInstallItemNotFound, fmt.Sprintf("no item named %q found in your library", args[0]), "Hint: syllago list --type "+hint)
		}
		fmt.Fprintln(output.ErrWriter, "no items found in library matching filters")
		return nil
	}

	items, _, stop, refusal := gateLibraryItems(output.ErrWriter, items, mergedCfg, projectRoot)
	if stop || len(items) == 0 {
		return refusal
	}

	if !output.Quiet && !output.JSON {
		fmt.Fprintf(output.Writer, "Installing %d item(s) to %d detected provider(s)...\n\n", len(items), len(active))
	}

	var (
		provResults []providerInstallResult
		anyFailed   bool
	)

	for _, prov := range active {
		if !output.Quiet && !output.JSON {
			fmt.Fprintf(output.Writer, "→ %s\n", prov.Name)
		}

		result, _ := installToProvider(items, prov, method, dryRun, resolver, prov.Slug, projectRoot, frozen, scan)

		pr := providerInstallResult{
			Provider: prov.Name,
			Slug:     prov.Slug,
		}

		switch {
		case len(result.Installed) > 0:
			pr.Status = "installed"
			pr.Count = len(result.Installed)
		case len(result.Skipped) > 0:
			allUnsupported := true
			for _, s := range result.Skipped {
				if !strings.Contains(s.Reason, "does not support") &&
					!strings.Contains(s.Reason, "project-scoped") &&
					!strings.Contains(s.Reason, "JSON merge") {
					allUnsupported = false
					anyFailed = true
				}
			}
			if allUnsupported {
				pr.Status = "skipped"
				pr.Details = "content type not supported"
			} else {
				pr.Status = "failed"
				pr.Count = len(result.Skipped)
				if len(result.Skipped) > 0 {
					pr.Details = result.Skipped[0].Reason
				}
			}
		default:
			pr.Status = "no-items"
		}

		provResults = append(provResults, pr)

		if !output.Quiet && !output.JSON {
			fmt.Fprintf(output.Writer, "  %s: %s", prov.Name, pr.Status)
			if pr.Count > 0 {
				fmt.Fprintf(output.Writer, " (%d items)", pr.Count)
			}
			if pr.Details != "" {
				fmt.Fprintf(output.Writer, " — %s", pr.Details)
			}
			fmt.Fprintln(output.Writer)
		}
	}

	if output.JSON {
		output.Print(provResults)
		return refusal
	}

	if !output.Quiet {
		fmt.Fprintln(output.Writer)
		for _, pr := range provResults {
			if pr.Count > 0 {
				fmt.Fprintf(output.Writer, "  %-20s  %-10s  %d items\n", pr.Provider, pr.Status, pr.Count)
			} else {
				fmt.Fprintf(output.Writer, "  %-20s  %-10s  %s\n", pr.Provider, pr.Status, pr.Details)
			}
		}
	}

	telemetry.Enrich("content_type", typeFilter)
	telemetry.Enrich("content_count", len(items))
	telemetry.Enrich("dry_run", dryRun)

	if refusal != nil {
		return refusal
	}
	if anyFailed {
		return output.NewStructuredError(output.ErrInstallNotWritable, "one or more providers had install failures", "Check the output above for details per provider")
	}
	return nil
}

func firstArg(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return "<name>"
}

// registryAddHint returns the best hint for a missing-item install error.
// When the named item exists in a registry clone (but not the local library),
// it names the exact `syllago add` command to add it. Otherwise it falls back
// to the generic `syllago list --type` suggestion.
func registryAddHint(itemName, typeFilter, projectRoot string) string {
	fallback := "Hint: syllago list --type "
	if typeFilter != "" {
		fallback += typeFilter
	} else {
		fallback += "skills"
	}

	cfg, err := config.Load(projectRoot)
	if err != nil || len(cfg.Registries) == 0 {
		return fallback
	}

	var sources []catalog.RegistrySource
	for _, r := range cfg.Registries {
		if registry.IsCloned(r.Name) {
			dir, err := registry.CloneDir(r.Name)
			if err == nil {
				sources = append(sources, catalog.RegistrySource{Name: r.Name, Path: dir})
			}
		}
	}
	if len(sources) == 0 {
		return fallback
	}

	regCat, err := catalog.ScanRegistriesOnly(sources)
	if err != nil {
		return fallback
	}

	for _, item := range regCat.Items {
		if item.Name == itemName {
			return fmt.Sprintf("Hint: syllago add %s --from %s", itemName, item.Registry)
		}
	}
	return fallback
}
