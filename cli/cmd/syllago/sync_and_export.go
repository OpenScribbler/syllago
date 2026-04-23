package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

var syncInstallCmd = &cobra.Command{
	Use:   "sync-install",
	Short: "Sync registries then install content to a provider",
	Long: `Convenience command that syncs all registries then installs content.

Equivalent to running:
  syllago registry sync && syllago install --to <provider>

This is useful in CI/CD or automation where you want a single command
to ensure registries are up-to-date before installing.`,
	Example: `  # Sync registries and install to Cursor
  syllago sync-install --to cursor

  # Install to all providers
  syllago sync-install --to all --type skills

  # Install only registry content to Kiro
  syllago sync-install --to kiro --source registry`,
	RunE: runSyncInstall,
}

// syncAllRegistries is a test seam for registry.SyncAll so tests can stub
// out network I/O without running a real git fetch.
var syncAllRegistries = registry.SyncAll

func init() {
	syncInstallCmd.Flags().String("to", "", "Provider slug to install to, or \"all\" for every provider (required)")
	syncInstallCmd.MarkFlagRequired("to")
	syncInstallCmd.Flags().String("type", "", "Filter to a specific content type (e.g., skills, rules)")
	syncInstallCmd.Flags().String("name", "", "Filter by item name (substring match)")
	syncInstallCmd.Flags().String("source", "local", "Which items to install: local (default), shared, registry, builtin, all")
	syncInstallCmd.Flags().String("llm-hooks", "skip", "How to handle LLM-evaluated hooks: skip (drop with warning) or generate (create wrapper scripts)")
	syncInstallCmd.Flags().BoolP("dry-run", "n", false, "Show what would be installed without making changes")
	rootCmd.AddCommand(syncInstallCmd)
}

func runSyncInstall(cmd *cobra.Command, args []string) error {
	// Find project root and load config to get registry list.
	root, err := findProjectRoot()
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrCatalogNotFound, "could not find project root", "Run 'syllago init' to initialize a project", err.Error())
	}

	cfg, err := config.Load(root)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrConfigInvalid, "loading config failed", "Check .syllago/config.json for syntax errors", err.Error())
	}

	// Sync registries if any are configured.
	if len(cfg.Registries) > 0 {
		names := make([]string, len(cfg.Registries))
		for i, r := range cfg.Registries {
			names[i] = r.Name
		}

		if !output.JSON {
			fmt.Fprintf(output.Writer, "Syncing %d registries...\n", len(names))
		}

		results := syncAllRegistries(names)
		for _, res := range results {
			if res.Err != nil {
				return output.NewStructuredErrorDetail(output.ErrRegistrySyncFailed, fmt.Sprintf("registry sync failed for %q", res.Name), "Check network connectivity and registry URL", res.Err.Error())
			}
			if !output.JSON {
				fmt.Fprintf(output.Writer, "Synced: %s\n", res.Name)
			}
		}
	}

	toSlug, _ := cmd.Flags().GetString("to")
	typeFilter, _ := cmd.Flags().GetString("type")
	nameFilter, _ := cmd.Flags().GetString("name")
	sourceFilter, _ := cmd.Flags().GetString("source")
	llmHooksMode, _ := cmd.Flags().GetString("llm-hooks")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	telemetry.Enrich("provider", toSlug)
	telemetry.Enrich("content_type", typeFilter)
	telemetry.Enrich("dry_run", dryRun)
	return runInstallOp(root, toSlug, typeFilter, nameFilter, sourceFilter, llmHooksMode, "", dryRun)
}

// syncInstallResult is the JSON-serializable output for sync-install operations.
type syncInstallResult struct {
	Installed []syncInstalledItem `json:"installed"`
	Skipped   []syncSkippedItem   `json:"skipped,omitempty"`
}

type syncInstalledItem struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Destination string   `json:"destination"`
	Converted   bool     `json:"converted,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

type syncSkippedItem struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Reason string `json:"reason"`
}

// runInstallOp contains the core export logic shared by sync-and-export.
//
//nolint:gocyclo // CLI command runner with many flags
func runInstallOp(root, toSlug, typeFilter, nameFilter, sourceFilter, llmHooksMode, baseDir string, dryRun bool) error {
	if toSlug == "all" {
		return runInstallAll(root, typeFilter, nameFilter, sourceFilter, llmHooksMode, baseDir, dryRun)
	}

	prov := findProviderBySlug(toSlug)
	if prov == nil {
		slugs := providerSlugs()
		slugs = append(slugs, "all")
		return output.NewStructuredError(output.ErrProviderNotFound, "unknown provider: "+toSlug, "Available: "+strings.Join(slugs, ", "))
	}

	// Build resolver from merged config + CLI flag.
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrConfigInvalid, "loading global config failed", "Check ~/.syllago/config.json for syntax errors", err.Error())
	}
	projectRoot, _ := findProjectRoot()
	if projectRoot == "" {
		projectRoot = root
	}
	projectCfg, err := config.Load(projectRoot)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrConfigNotFound, "loading project config failed", "Run 'syllago init' to create a project config", err.Error())
	}
	mergedCfg := config.Merge(globalCfg, projectCfg)
	resolver := config.NewResolver(mergedCfg, baseDir)
	if err := resolver.ExpandPaths(); err != nil {
		return output.NewStructuredErrorDetail(output.ErrConfigPath, "expanding paths failed", "Check path overrides in config", err.Error())
	}

	// Configure the hooks converter with the LLM hooks mode.
	if hooksConv, ok := converter.For(catalog.Hooks).(*converter.HooksConverter); ok {
		hooksConv.LLMHooksMode = llmHooksMode
	}

	// Scan the catalog.
	cat, err := catalog.Scan(root, projectRoot)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrCatalogScanFailed, "scanning catalog failed", "Check that the content repository is valid", err.Error())
	}

	// Collect items matching source, type, and name filters.
	var items []catalog.ContentItem
	for _, item := range cat.Items {
		if !filterBySource(item, sourceFilter) {
			continue
		}
		if typeFilter != "" && string(item.Type) != typeFilter {
			continue
		}
		if nameFilter != "" && !strings.Contains(item.Name, nameFilter) {
			continue
		}
		items = append(items, item)
	}

	if len(items) == 0 {
		msg := "no items found"
		if sourceFilter != "all" {
			msg += " in " + sourceFilter
		}
		if typeFilter != "" || nameFilter != "" {
			msg += " matching filters"
		}
		fmt.Fprintln(output.ErrWriter, msg)
		return nil
	}

	if dryRun {
		if !output.Quiet {
			fmt.Fprintf(output.Writer, "[dry-run] would install %d item(s) to %s\n", len(items), toSlug)
			for _, item := range items {
				fmt.Fprintf(output.Writer, "  %s (%s)\n", item.Name, item.Type.Label())
			}
		}
		return nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrSystemHomedir, "cannot determine home directory", "Set the HOME environment variable", err.Error())
	}

	result := syncInstallResult{}

	for _, item := range items {
		// Warn about built-in or example content before processing.
		if msg := exportWarnMessage(item); msg != "" {
			fmt.Fprintf(output.ErrWriter, "  warning: %s is %s\n", item.Name, msg)
		}

		// Privacy warning: alert if exporting private-tainted content.
		if item.Meta != nil && item.Meta.SourceRegistry != "" && registry.IsPrivate(item.Meta.SourceVisibility) {
			fmt.Fprintf(output.ErrWriter, "  warning: %s originated from private registry %q — do not commit exported files to public repositories\n",
				item.Name, item.Meta.SourceRegistry)
		}

		// Check if provider supports this type via SupportsType.
		if prov.SupportsType != nil && !prov.SupportsType(item.Type) {
			skip := syncSkippedItem{
				Name:   item.Name,
				Type:   string(item.Type),
				Reason: fmt.Sprintf("%s does not support %s", prov.Name, item.Type.Label()),
			}
			result.Skipped = append(result.Skipped, skip)
			if !output.JSON {
				fmt.Fprintf(output.ErrWriter, "Skipping %s (%s): %s does not support %s\n",
					item.Name, item.Type.Label(), prov.Name, item.Type.Label())
			}
			continue
		}

		// Check for JSON merge sentinel — these types require config-file
		// merging rather than filesystem copy.
		installDir := resolver.InstallDir(*prov, item.Type, homeDir)
		if installDir == provider.JSONMergeSentinel {
			// Allow converter-based cross-provider export for JSON merge types
			srcProv := effectiveProvider(item)
			if conv := converter.For(item.Type); conv != nil && srcProv != "" && srcProv != toSlug {
				contentFile := converter.ResolveContentFile(item)
				if contentFile != "" {
					content, readErr := os.ReadFile(contentFile)
					if readErr == nil {
						canonical, canonErr := conv.Canonicalize(content, srcProv)
						if canonErr != nil {
							canonical = &converter.Result{Content: content}
						}
						rendered, renderErr := conv.Render(canonical.Content, *prov)
						if renderErr == nil && rendered.Content != nil {
							dest := filepath.Join(item.Path, "exported-"+toSlug+"-"+rendered.Filename)
							if writeErr := os.WriteFile(dest, rendered.Content, 0644); writeErr == nil {
								for name, extraContent := range rendered.ExtraFiles {
									extraPath := filepath.Join(item.Path, name)
									if extraErr := os.WriteFile(extraPath, extraContent, 0755); extraErr != nil {
										rendered.Warnings = append(rendered.Warnings, fmt.Sprintf("failed to write %s: %s", name, extraErr))
									}
								}
								result.Installed = append(result.Installed, syncInstalledItem{
									Name:        item.Name,
									Type:        string(item.Type),
									Destination: dest,
									Converted:   true,
									Warnings:    append(rendered.Warnings, fmt.Sprintf("JSON merge type: saved to %s (merge manually into provider config)", dest)),
								})
								if !output.JSON {
									fmt.Fprintf(output.Writer, "Installed %s to %s (converted, merge manually)\n", item.Name, dest)
									for _, w := range rendered.Warnings {
										fmt.Fprintf(output.ErrWriter, "  warning: %s\n", w)
									}
								}
								continue
							}
						}
					}
				}
			}

			skip := syncSkippedItem{
				Name:   item.Name,
				Type:   string(item.Type),
				Reason: fmt.Sprintf("%s for %s requires JSON merge (not supported by export)", item.Type.Label(), prov.Name),
			}
			result.Skipped = append(result.Skipped, skip)
			if !output.JSON {
				fmt.Fprintf(output.ErrWriter, "Skipping %s (%s): requires JSON merge for %s (use the TUI to install)\n",
					item.Name, item.Type.Label(), prov.Name)
			}
			continue
		}

		// Project-scope types: resolve install dir from DiscoveryPaths using CWD.
		if installDir == provider.ProjectScopeSentinel {
			cwd, cwdErr := os.Getwd()
			if cwdErr != nil {
				return output.NewStructuredErrorDetail(output.ErrSystemIO, "getting working directory failed", "Check filesystem permissions", cwdErr.Error())
			}
			if prov.DiscoveryPaths != nil {
				paths := resolver.DiscoveryPaths(*prov, item.Type, cwd)
				if len(paths) > 0 {
					installDir = paths[0]
				}
			}
			if installDir == provider.ProjectScopeSentinel {
				skip := syncSkippedItem{
					Name:   item.Name,
					Type:   string(item.Type),
					Reason: fmt.Sprintf("%s %s requires a project directory (no discovery path configured)", prov.Name, item.Type.Label()),
				}
				result.Skipped = append(result.Skipped, skip)
				if !output.JSON {
					fmt.Fprintf(output.ErrWriter, "Skipping %s (%s): %s %s requires a project directory\n",
						item.Name, item.Type.Label(), prov.Name, item.Type.Label())
				}
				continue
			}
		}

		if installDir == "" {
			skip := syncSkippedItem{
				Name:   item.Name,
				Type:   string(item.Type),
				Reason: fmt.Sprintf("%s does not support %s", prov.Name, item.Type.Label()),
			}
			result.Skipped = append(result.Skipped, skip)
			if !output.JSON {
				fmt.Fprintf(output.ErrWriter, "Skipping %s (%s): %s does not support %s\n",
					item.Name, item.Type.Label(), prov.Name, item.Type.Label())
			}
			continue
		}

		if err := os.MkdirAll(installDir, 0755); err != nil {
			return output.NewStructuredErrorDetail(output.ErrSystemIO, fmt.Sprintf("creating directory %s failed", installDir), "Check filesystem permissions", err.Error())
		}

		// Try cross-provider rendering via converter
		if conv := converter.For(item.Type); conv != nil {
			exported, handled := exportWithConverter(item, *prov, toSlug, conv, installDir)
			if handled {
				if exported != nil {
					result.Installed = append(result.Installed, *exported)
					if !output.JSON {
						fmt.Fprintf(output.Writer, "Installed %s to %s (converted)\n", item.Name, exported.Destination)
						for _, w := range exported.Warnings {
							fmt.Fprintf(output.ErrWriter, "  warning: %s\n", w)
						}
					}
				} else {
					// Skipped by converter (e.g. non-alwaysApply for single-file provider)
					skip := syncSkippedItem{
						Name:   item.Name,
						Type:   string(item.Type),
						Reason: fmt.Sprintf("not compatible with %s format", prov.Name),
					}
					result.Skipped = append(result.Skipped, skip)
					if !output.JSON {
						fmt.Fprintf(output.ErrWriter, "Skipping %s: not compatible with %s format\n", item.Name, prov.Name)
					}
				}
				continue
			}
		}

		// Fallback: direct copy (no converter or same-provider without .source/)
		dest := filepath.Join(installDir, item.Name)

		if err := installer.CopyContent(item.Path, dest); err != nil {
			return output.NewStructuredErrorDetail(output.ErrExportFailed, fmt.Sprintf("copying %s failed", item.Name), "Check filesystem permissions", err.Error())
		}

		result.Installed = append(result.Installed, syncInstalledItem{
			Name:        item.Name,
			Type:        string(item.Type),
			Destination: dest,
		})

		if !output.JSON {
			fmt.Fprintf(output.Writer, "Installed %s to %s\n", item.Name, dest)
		}
	}

	if output.JSON {
		output.Print(result)
	} else if len(result.Installed) == 0 && len(result.Skipped) > 0 {
		fmt.Fprintln(output.ErrWriter, "No items were installed (all skipped).")
	}

	return nil
}

// runInstallAll installs to every known provider in sequence.
func runInstallAll(root, typeFilter, nameFilter, sourceFilter, llmHooksMode, baseDir string, dryRun bool) error {
	type providerSummary struct {
		Slug string
		Err  error
	}

	var summaries []providerSummary

	for _, prov := range provider.AllProviders {
		if !output.JSON {
			fmt.Fprintf(output.Writer, "\n--- %s (%s) ---\n", prov.Name, prov.Slug)
		}

		err := runInstallOp(root, prov.Slug, typeFilter, nameFilter, sourceFilter, llmHooksMode, baseDir, dryRun)
		summaries = append(summaries, providerSummary{Slug: prov.Slug, Err: err})
	}

	// Print summary.
	if !output.JSON {
		fmt.Fprintf(output.Writer, "\n=== Install All Summary ===\n")
		hasErrors := false
		for _, s := range summaries {
			status := "ok"
			if s.Err != nil {
				status = "error: " + s.Err.Error()
				hasErrors = true
			}
			fmt.Fprintf(output.Writer, "  %-20s %s\n", s.Slug, status)
		}

		// Show filter reminder if filters were active.
		if typeFilter != "" || nameFilter != "" {
			filters := []string{}
			if typeFilter != "" {
				filters = append(filters, "type="+typeFilter)
			}
			if nameFilter != "" {
				filters = append(filters, "name="+nameFilter)
			}
			fmt.Fprintf(output.Writer, "  (filtered by %s)\n", strings.Join(filters, ", "))
		}

		if hasErrors {
			return output.NewStructuredError(output.ErrExportFailed, "one or more provider exports failed", "Review the per-provider errors above")
		}
	}

	return nil
}

// exportWithConverter handles export with cross-provider conversion.
// Returns (syncInstalledItem, true) if the converter handled the item.
// Returns (nil, true) if the converter skipped it (not compatible).
// Returns (nil, false) if the converter doesn't apply (fall through to default copy).
func exportWithConverter(item catalog.ContentItem, prov provider.Provider, toSlug string, conv converter.Converter, installDir string) (*syncInstalledItem, bool) {
	srcProvider := effectiveProvider(item)

	// Same provider + has .source/ → copy original verbatim (lossless)
	if converter.HasSourceFile(item) && srcProvider == toSlug {
		srcPath := converter.SourceFilePath(item)
		if srcPath == "" {
			return nil, false
		}
		dest := filepath.Join(installDir, item.Name, filepath.Base(srcPath))
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return nil, false
		}
		if err := installer.CopyContent(srcPath, dest); err != nil {
			return nil, false
		}
		return &syncInstalledItem{
			Name:        item.Name,
			Type:        string(item.Type),
			Destination: dest,
		}, true
	}

	// Cross-provider → canonicalize then render
	if srcProvider != "" && srcProvider != toSlug {
		contentFile := converter.ResolveContentFile(item)
		if contentFile == "" {
			return nil, false
		}
		content, err := os.ReadFile(contentFile)
		if err != nil {
			return nil, false
		}

		// Canonicalize from source provider format, then render to target
		canonical, err := conv.Canonicalize(content, srcProvider)
		if err != nil {
			return nil, false
		}

		rendered, err := conv.Render(canonical.Content, prov)
		if err != nil {
			return nil, false
		}

		// nil Content means skip
		if rendered.Content == nil {
			return nil, true
		}

		dest := filepath.Join(installDir, item.Name, rendered.Filename)
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return nil, false
		}
		if err := os.WriteFile(dest, rendered.Content, 0644); err != nil {
			return nil, false
		}

		// Write any extra files (e.g. generated LLM hook wrapper scripts)
		for name, content := range rendered.ExtraFiles {
			extraPath := filepath.Join(filepath.Dir(dest), name)
			if err := os.WriteFile(extraPath, content, 0755); err != nil {
				// Non-fatal: warn but continue
				rendered.Warnings = append(rendered.Warnings, fmt.Sprintf("failed to write %s: %s", name, err))
			}
		}

		return &syncInstalledItem{
			Name:        item.Name,
			Type:        string(item.Type),
			Destination: dest,
			Converted:   true,
			Warnings:    rendered.Warnings,
		}, true
	}

	// No conversion needed — fall through to default copy
	return nil, false
}
