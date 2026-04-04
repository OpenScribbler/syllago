package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/audit"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

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
Use --method copy to place a standalone copy instead.`,
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
	installCmd.Flags().String("method", "symlink", "Install method: symlink (default) or copy")
	installCmd.Flags().Bool("all", false, "Install all library content (cannot combine with a positional name)")
	installCmd.Flags().BoolP("dry-run", "n", false, "Show what would happen without making changes")
	installCmd.Flags().String("base-dir", "", "Override base directory for content installation")
	installCmd.Flags().Bool("no-input", false, "Disable interactive prompts, use defaults")
	rootCmd.AddCommand(installCmd)
}

func runInstall(cmd *cobra.Command, args []string) error {
	toSlug, _ := cmd.Flags().GetString("to")
	toAll, _ := cmd.Flags().GetBool("to-all")
	typeFilter, _ := cmd.Flags().GetString("type")
	methodStr, _ := cmd.Flags().GetString("method")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	baseDir, _ := cmd.Flags().GetString("base-dir")
	installAll, _ := cmd.Flags().GetBool("all")

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

	// --to-all path: detect providers and delegate.
	if toAll {
		return runInstallToAll(cmd, args, typeFilter, methodStr, method, dryRun, baseDir, installAll)
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

	// Warn if the target provider is not detected on disk.
	if !output.JSON && !output.Quiet {
		detected := provider.DetectProvidersWithResolver(resolver)
		for _, dp := range detected {
			if dp.Slug == prov.Slug && !dp.Detected {
				fmt.Fprintf(output.ErrWriter, "Warning: %s not detected at default locations.\n", prov.Name)
				fmt.Fprintf(output.ErrWriter, "If installed at a custom path, configure it:\n")
				fmt.Fprintf(output.ErrWriter, "  syllago config paths --provider %s --path /your/path\n", prov.Slug)
				break
			}
		}
	}

	// Scan global library only.
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

	if len(items) > 1 && !output.Quiet && !output.JSON {
		fmt.Fprintf(output.Writer, "Installing %d items to %s...\n", len(items), prov.Name)
	}

	result, _ := installToProvider(items, *prov, globalDir, method, dryRun, resolver, toSlug, projectRoot)

	if output.JSON {
		output.Print(result)
		return nil
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
	return nil
}

// installToProvider installs the given items to a single provider and returns
// the result. Returns (result, nil) in all normal cases — individual item
// failures are recorded in result.Skipped, not propagated as a return error.
func installToProvider(
	items []catalog.ContentItem,
	prov provider.Provider,
	globalDir string,
	method installer.InstallMethod,
	dryRun bool,
	resolver *config.PathResolver,
	toSlug string,
	projectRoot string,
) (installResult, error) {
	var result installResult

	for _, item := range items {
		if dryRun {
			if !output.Quiet {
				m := "symlink"
				if installer.IsJSONMerge(prov, item.Type) {
					m = "json-merge"
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

		desc, err := installer.InstallWithResolver(item, prov, globalDir, method, resolver)
		if err != nil {
			result.Skipped = append(result.Skipped, skippedItem{Name: item.Name, Reason: err.Error()})
			if !output.JSON {
				fmt.Fprintf(output.ErrWriter, "  skip %s: %s\n", item.Name, err)
			}
			continue
		}

		// Check for portability warnings by running the converter.
		var warnings []string
		if conv := converter.For(item.Type); conv != nil {
			contentFile := converter.ResolveContentFile(item)
			if contentFile != "" {
				if raw, readErr := os.ReadFile(contentFile); readErr == nil {
					srcProv := ""
					if item.Meta != nil {
						srcProv = item.Meta.SourceProvider
					}
					if canonical, cErr := conv.Canonicalize(raw, srcProv); cErr == nil {
						if rendered, rErr := conv.Render(canonical.Content, prov); rErr == nil {
							warnings = rendered.Warnings
						}
					}
				}
			}
		}

		result.Installed = append(result.Installed, installedItem{
			Name:     item.Name,
			Type:     string(item.Type),
			Method:   string(method),
			Path:     desc,
			Warnings: warnings,
		})

		// Audit log (best-effort).
		if auditLogger, aErr := audit.NewLogger(audit.DefaultLogPath(projectRoot)); aErr == nil {
			_ = auditLogger.LogContent(audit.EventContentInstall, item.Name, string(item.Type), toSlug)
			_ = auditLogger.Close()
		}

		if !output.JSON && !output.Quiet {
			if method == installer.MethodSymlink {
				fmt.Fprintf(output.Writer, "  Symlinked %s to %s\n", item.Name, desc)
			} else {
				fmt.Fprintf(output.Writer, "  Copied %s to %s\n", item.Name, desc)
			}
			for _, w := range warnings {
				fmt.Fprintf(output.ErrWriter, "    - %s\n", w)
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
	_ *cobra.Command,
	args []string,
	typeFilter string,
	_ string,
	method installer.InstallMethod,
	dryRun bool,
	baseDir string,
	installAll bool,
) error {
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

		result, _ := installToProvider(items, prov, globalDir, method, dryRun, resolver, prov.Slug, projectRoot)

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
		return nil
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
